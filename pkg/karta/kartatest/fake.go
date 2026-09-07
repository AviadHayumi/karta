// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package kartatest provides a hand-rolled recording fake of karta.Workload
// for consumer tests. No cluster, no fixtures, no mock generation.
package kartatest

import (
	"context"
	"sort"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/karta"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

// Fake is a RECORDING fake of karta.Workload for consumer unit tests: calls
// are recorded, capability behavior mirrors production statically (via
// karta.WritablePodFields when seeded with NewFromKarta). It does not run any
// jq and does not mutate objects; for integration-grade tests use karta.New
// over an in-memory object, which needs no cluster.
type Fake struct {
	// ComponentInfos is returned by Components().
	ComponentInfos []karta.ComponentInfo

	// Interceptors, when set, run first and their error is returned as-is.
	InterceptUpdatePodTemplate func(component string, patch karta.Patch) error
	InterceptSuspend           func() error
	InterceptResume            func() error

	TreeResult   *tree.WorkloadTree
	TreeErr      error
	ObjectResult resource.KubernetesObject

	// UnsupportedFields, keyed by component name, makes UpdatePodTemplate fail with
	// *karta.UnsupportedFieldsError for the listed fields, exactly like
	// production.
	UnsupportedFields map[string][]karta.PodField

	// Suspendable=false makes Suspend and Resume return
	// *karta.UnsupportedOperationError.
	Suspendable bool

	// Recorded calls. Typed patches are recorded in their compiled
	// merge-patch form - the same thing production routes.
	PodUpdates []PodUpdate
	Suspends   int
	Resumes    int
}

// PodUpdate is one recorded UpdatePodTemplate call.
type PodUpdate struct {
	Component string
	Patch     karta.Patch
	Instances []string
}

var _ karta.Workload = (*Fake)(nil)

// NewFromKarta seeds UnsupportedFields and Suspendable from a real Karta
// definition through karta.WritablePodFields, so the fake's capability
// behavior cannot drift from production for that definition.
func NewFromKarta(definition *v1alpha1.Karta) *Fake {
	fake := &Fake{UnsupportedFields: map[string][]karta.PodField{}}
	fake.ComponentInfos = kartaComponents(definition)
	all := []karta.PodField{
		karta.PodFieldSchedulerName, karta.PodFieldPriorityClassName,
		karta.PodFieldLabels, karta.PodFieldAnnotations,
		karta.PodFieldNodeAffinity, karta.PodFieldPodAffinity,
		karta.PodFieldResourceClaims, karta.PodFieldImage,
		karta.PodFieldResources, karta.PodFieldContainers,
	}
	seed := func(def v1alpha1.ComponentDefinition) {
		writable := map[karta.PodField]bool{}
		for _, field := range karta.WritablePodFields(def) {
			writable[field] = true
		}
		var unsupported []karta.PodField
		for _, field := range all {
			if !writable[field] {
				unsupported = append(unsupported, field)
			}
		}
		fake.UnsupportedFields[def.Name] = unsupported
		if def.SuspendDefinition != nil {
			fake.Suspendable = true
		}
	}
	seed(definition.Spec.StructureDefinition.RootComponent)
	for _, child := range definition.Spec.StructureDefinition.ChildComponents {
		seed(child)
	}
	return fake
}

func (f *Fake) Tree(_ context.Context) (*tree.WorkloadTree, error) {
	if f.TreeErr != nil {
		return nil, f.TreeErr
	}
	if f.TreeResult != nil {
		return f.TreeResult, nil
	}
	return &tree.WorkloadTree{}, nil
}

func (f *Fake) Components() []karta.ComponentInfo {
	return f.ComponentInfos
}

func (f *Fake) UpdatePodTemplate(_ context.Context, component string, update karta.PodTemplateUpdate, opts ...karta.UpdateOption) error {
	if update == nil {
		return karta.ErrEmptyPatch
	}
	patch := update.AsPodMergePatch()
	if f.InterceptUpdatePodTemplate != nil {
		if err := f.InterceptUpdatePodTemplate(component, patch); err != nil {
			return err
		}
	}
	if typed, ok := update.(karta.PodPatch); ok {
		if fields := f.unsupported(component, typed); len(fields) > 0 {
			return &karta.UnsupportedFieldsError{Component: component, Fields: fields}
		}
	}
	f.PodUpdates = append(f.PodUpdates, PodUpdate{
		Component: component,
		Patch:     patch,
		Instances: karta.ResolveUpdateOptions(opts...).Instances,
	})
	return nil
}

func (f *Fake) Suspend(_ context.Context) error {
	if f.InterceptSuspend != nil {
		if err := f.InterceptSuspend(); err != nil {
			return err
		}
	}
	if !f.Suspendable {
		return &karta.UnsupportedOperationError{Op: "suspend"}
	}
	f.Suspends++
	return nil
}

func (f *Fake) Resume(_ context.Context) error {
	if f.InterceptResume != nil {
		if err := f.InterceptResume(); err != nil {
			return err
		}
	}
	if !f.Suspendable {
		return &karta.UnsupportedOperationError{Op: "resume"}
	}
	f.Resumes++
	return nil
}

func (f *Fake) Object() (resource.KubernetesObject, error) {
	return f.ObjectResult, nil
}

func (f *Fake) unsupported(component string, patch karta.PodPatch) []karta.PodField {
	blocked := map[karta.PodField]bool{}
	for _, field := range f.UnsupportedFields[component] {
		blocked[field] = true
	}
	var fields []karta.PodField
	for _, field := range patch.SetFields() {
		if blocked[field] {
			fields = append(fields, field)
		}
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i] < fields[j] })
	return fields
}

func kartaComponents(definition *v1alpha1.Karta) []karta.ComponentInfo {
	structure := definition.Spec.StructureDefinition
	infos := make([]karta.ComponentInfo, 0, 1+len(structure.ChildComponents))
	describe := func(component v1alpha1.ComponentDefinition, root bool) karta.ComponentInfo {
		return karta.ComponentInfo{
			Name:        component.Name,
			Root:        root,
			PodFields:   karta.WritablePodFields(component),
			Suspendable: component.SuspendDefinition != nil,
		}
	}
	infos = append(infos, describe(structure.RootComponent, true))
	for _, child := range structure.ChildComponents {
		infos = append(infos, describe(child, false))
	}
	return infos
}
