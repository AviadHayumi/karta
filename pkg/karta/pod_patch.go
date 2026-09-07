// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package karta

import (
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// PodPatch is a typed partial pod update: nil or empty fields are not
// touched. An all-unset patch is rejected with ErrEmptyPatch, never treated
// as a silent no-op.
type PodPatch struct {
	SchedulerName     *string
	PriorityClassName *string

	// Labels and Annotations are merged by key into the existing maps.
	// A key cannot be deleted through a patch.
	Labels      map[string]string
	Annotations map[string]string

	// NodeAffinity, PodAffinity and ResourceClaims replace the existing
	// value wholesale.
	NodeAffinity   *corev1.NodeAffinity
	PodAffinity    *corev1.PodAffinity
	ResourceClaims []corev1.PodResourceClaim

	// Image and Resources target the pod's single container and fail when
	// the pod has more than one container. Multi-container pods use
	// Containers.
	Image     *string
	Resources *corev1.ResourceRequirements

	// Containers are merged by container name, touching only the fields set
	// on each entry.
	Containers []ContainerPatch
}

// ContainerPatch is a partial update of one named container.
type ContainerPatch struct {
	Name      string
	Image     *string
	Resources *corev1.ResourceRequirements
}

// UpdateOptions is the resolved form of a set of UpdateOption values.
type UpdateOptions struct {
	// Instances limits the update to these instance ids; nil means all.
	Instances []string
}

// UpdateOption configures one UpdatePodTemplate call.
type UpdateOption func(*UpdateOptions)

// ResolveUpdateOptions folds opts into their resolved form. Exposed so fakes
// and decorators interpret options exactly like the real implementation.
func ResolveUpdateOptions(opts ...UpdateOption) UpdateOptions {
	options := UpdateOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// WithInstances limits the update to the given instance ids of a
// multi-instance component. The default is all instances. Unknown ids fail
// before any write.
func WithInstances(ids ...string) UpdateOption {
	return func(o *UpdateOptions) { o.Instances = append(o.Instances, ids...) }
}

// PodField names one routable pod field of a PodPatch.
type PodField string

const (
	PodFieldSchedulerName     PodField = "schedulerName"
	PodFieldPriorityClassName PodField = "priorityClassName"
	PodFieldLabels            PodField = "labels"
	PodFieldAnnotations       PodField = "annotations"
	PodFieldNodeAffinity      PodField = "nodeAffinity"
	PodFieldPodAffinity       PodField = "podAffinity"
	PodFieldResourceClaims    PodField = "resourceClaims"
	PodFieldImage             PodField = "image"
	PodFieldResources         PodField = "resources"
	PodFieldContainers        PodField = "containers"
)

// SetFields lists the fields the patch sets, in stable order. Exposed so
// fakes and tooling classify a patch exactly like the real implementation.
func (p PodPatch) SetFields() []PodField {
	var fields []PodField
	if p.SchedulerName != nil {
		fields = append(fields, PodFieldSchedulerName)
	}
	if p.PriorityClassName != nil {
		fields = append(fields, PodFieldPriorityClassName)
	}
	if len(p.Labels) > 0 {
		fields = append(fields, PodFieldLabels)
	}
	if len(p.Annotations) > 0 {
		fields = append(fields, PodFieldAnnotations)
	}
	if p.NodeAffinity != nil {
		fields = append(fields, PodFieldNodeAffinity)
	}
	if p.PodAffinity != nil {
		fields = append(fields, PodFieldPodAffinity)
	}
	if len(p.ResourceClaims) > 0 {
		fields = append(fields, PodFieldResourceClaims)
	}
	if p.Image != nil {
		fields = append(fields, PodFieldImage)
	}
	if p.Resources != nil {
		fields = append(fields, PodFieldResources)
	}
	if len(p.Containers) > 0 {
		fields = append(fields, PodFieldContainers)
	}
	return fields
}

type podShape int

const (
	shapeNone podShape = iota
	shapeTemplate
	shapePodSpec
	shapeSplit
	shapeFragmented
)

func specShape(def v1alpha1.ComponentDefinition) podShape {
	spec := def.SpecDefinition
	switch {
	case spec == nil:
		return shapeNone
	case spec.PodTemplateSpecPath != nil:
		return shapeTemplate
	case spec.PodSpecPath != nil && spec.MetadataPath != nil:
		return shapeSplit
	case spec.PodSpecPath != nil:
		return shapePodSpec
	case spec.FragmentedPodSpecDefinition != nil:
		return shapeFragmented
	default:
		return shapeNone
	}
}

// WritablePodFields reports which PodPatch fields the component definition can
// route. A field is writable only when a route exists AND every jq path on
// that route is a statically assignable pure path - computed projections,
// formulas and pipe expressions are read-only. containerPath is never a write
// route. Pure function of the definition; the real implementation, the
// kartatest fake and consumers all share it as the one source of capability
// truth.
func WritablePodFields(def v1alpha1.ComponentDefinition) []PodField {
	spec := def.SpecDefinition
	switch specShape(def) {
	case shapeTemplate:
		if !isWritablePath(*spec.PodTemplateSpecPath) {
			return nil
		}
		return []PodField{
			PodFieldSchedulerName, PodFieldPriorityClassName,
			PodFieldLabels, PodFieldAnnotations,
			PodFieldNodeAffinity, PodFieldPodAffinity,
			PodFieldResourceClaims, PodFieldImage, PodFieldResources,
			PodFieldContainers,
		}
	case shapePodSpec, shapeSplit:
		if !isWritablePath(*spec.PodSpecPath) {
			return nil
		}
		fields := []PodField{
			PodFieldSchedulerName, PodFieldPriorityClassName,
			PodFieldNodeAffinity, PodFieldPodAffinity,
			PodFieldResourceClaims, PodFieldImage, PodFieldResources,
			PodFieldContainers,
		}
		if specShape(def) == shapeSplit && isWritablePath(*spec.MetadataPath) {
			fields = append(fields, PodFieldLabels, PodFieldAnnotations)
		}
		return fields
	case shapeFragmented:
		fragmented := spec.FragmentedPodSpecDefinition
		writable := func(path *string) bool {
			return path != nil && isWritablePath(*path)
		}
		var fields []PodField
		if writable(fragmented.SchedulerNamePath) {
			fields = append(fields, PodFieldSchedulerName)
		}
		if writable(fragmented.PriorityClassNamePath) {
			fields = append(fields, PodFieldPriorityClassName)
		}
		if writable(fragmented.LabelsPath) {
			fields = append(fields, PodFieldLabels)
		}
		if writable(fragmented.AnnotationsPath) {
			fields = append(fields, PodFieldAnnotations)
		}
		if writable(fragmented.NodeAffinityPath) {
			fields = append(fields, PodFieldNodeAffinity)
		}
		if writable(fragmented.PodAffinityPath) {
			fields = append(fields, PodFieldPodAffinity)
		}
		if writable(fragmented.ResourceClaimsPath) {
			fields = append(fields, PodFieldResourceClaims)
		}
		if writable(fragmented.ImagePath) {
			fields = append(fields, PodFieldImage)
		}
		if writable(fragmented.ResourcesPath) {
			fields = append(fields, PodFieldResources)
		}
		if writable(fragmented.ContainersPath) {
			fields = append(fields, PodFieldContainers)
		}
		return fields
	default:
		return nil
	}
}

// unsupportedFields returns the set fields the definition cannot route,
// sorted, all offenders at once.
func unsupportedFields(patch PodPatch, def v1alpha1.ComponentDefinition) []PodField {
	writable := make(map[PodField]bool)
	for _, field := range WritablePodFields(def) {
		writable[field] = true
	}
	var unsupported []PodField
	for _, field := range patch.SetFields() {
		if !writable[field] {
			unsupported = append(unsupported, field)
		}
	}
	sort.Slice(unsupported, func(i, j int) bool { return unsupported[i] < unsupported[j] })
	return unsupported
}

// mergeContainersByName applies each ContainerPatch to the container with the
// matching name, touching only set fields. An unmatched name is an error.
func mergeContainersByName(containers []corev1.Container, patches []ContainerPatch) error {
	byName := make(map[string]int, len(containers))
	for i, container := range containers {
		byName[container.Name] = i
	}
	for _, patch := range patches {
		index, ok := byName[patch.Name]
		if !ok {
			return fmt.Errorf("karta: container %q not found in pod", patch.Name)
		}
		if patch.Image != nil {
			containers[index].Image = *patch.Image
		}
		if patch.Resources != nil {
			containers[index].Resources = *patch.Resources.DeepCopy()
		}
	}
	return nil
}
