// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package references_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/references"
)

// fakeReader serves objects from memory and records the queries it saw.
type fakeReader struct {
	objects map[string]*unstructured.Unstructured
	lists   map[string][]unstructured.Unstructured

	gets  []string
	listQ []references.ListQuery

	denyVerbs map[string]error
}

func key(gvk schema.GroupVersionKind, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, namespace, name)
}

func (f *fakeReader) Get(_ context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
	f.gets = append(f.gets, key(gvk, namespace, name))
	object, ok := f.objects[key(gvk, namespace, name)]
	if !ok {
		return nil, references.ErrNotFound
	}

	return object, nil
}

func (f *fakeReader) List(_ context.Context, gvk schema.GroupVersionKind, query references.ListQuery) ([]unstructured.Unstructured, error) {
	f.listQ = append(f.listQ, query)

	return f.lists[gvk.Kind], nil
}

// deniedReader wraps fakeReader with a PermissionChecker that denies configured verbs.
type deniedReader struct {
	*fakeReader
}

func (d *deniedReader) CheckRead(_ context.Context, _ schema.GroupVersionKind, _ string, verb string) error {
	if err, ok := d.denyVerbs[verb]; ok {
		return err
	}

	return nil
}

var _ = Describe("Resolve", func() {
	ctx := context.Background()

	runtimeGVK := schema.GroupVersionKind{Group: "trainer.kubeflow.org", Version: "v1alpha1", Kind: "ClusterTrainingRuntime"}

	workload := map[string]any{
		"metadata": map[string]any{"name": "fine-tune", "namespace": "team-a"},
		"spec":     map[string]any{"runtimeRef": map[string]any{"name": "torch-distributed"}},
	}

	kartaWith := func(refs ...v1alpha1.ResourceReference) *v1alpha1.Karta {
		return &v1alpha1.Karta{Spec: v1alpha1.KartaSpec{
			StructureDefinition: v1alpha1.StructureDefinition{References: refs},
		}}
	}

	It("resolves a lookup by the evaluated name in the workload's namespace", func() {
		runtime := &unstructured.Unstructured{Object: map[string]any{
			"kind": "ClusterTrainingRuntime",
			"spec": map[string]any{"template": "the-base"},
		}}
		reader := &fakeReader{objects: map[string]*unstructured.Unstructured{
			key(runtimeGVK, "team-a", "torch-distributed"): runtime,
		}}
		karta := kartaWith(v1alpha1.ResourceReference{
			Name:   "trainingRuntime",
			GVK:    v1alpha1.GroupVersionKind{Group: "trainer.kubeflow.org", Version: "v1alpha1", Kind: "ClusterTrainingRuntime"},
			Lookup: &v1alpha1.LookupReference{NameExpression: "object.spec.runtimeRef.name"},
		})

		resolved, err := references.Resolve(ctx, reader, karta, workload)
		Expect(err).NotTo(HaveOccurred())
		Expect(resolved["trainingRuntime"].Object).To(Equal(runtime))
		Expect(reader.gets).To(ConsistOf(key(runtimeGVK, "team-a", "torch-distributed")))

		bindings, err := resolved.Bindings()
		Expect(err).NotTo(HaveOccurred())
		Expect(bindings["trainingRuntime"]).To(Equal(runtime.Object))
	})

	It("leaves a lookup that finds nothing unbound instead of failing", func() {
		reader := &fakeReader{}
		karta := kartaWith(v1alpha1.ResourceReference{
			Name:   "trainingRuntime",
			GVK:    v1alpha1.GroupVersionKind{Group: "trainer.kubeflow.org", Version: "v1alpha1", Kind: "ClusterTrainingRuntime"},
			Lookup: &v1alpha1.LookupReference{NameExpression: "object.spec.runtimeRef.name"},
		})

		resolved, err := references.Resolve(ctx, reader, karta, workload)
		Expect(err).NotTo(HaveOccurred())
		Expect(resolved).To(HaveKey("trainingRuntime"))
		noBind, err := resolved.Bindings()
		Expect(err).NotTo(HaveOccurred())
		Expect(noBind).NotTo(HaveKey("trainingRuntime"))
	})

	It("resolves a list with literal and expression-sourced selector values", func() {
		pod := unstructured.Unstructured{Object: map[string]any{"kind": "Pod", "metadata": map[string]any{"name": "p0"}}}
		reader := &fakeReader{lists: map[string][]unstructured.Unstructured{"Pod": {pod}}}
		karta := kartaWith(v1alpha1.ResourceReference{
			Name: "pods",
			GVK:  v1alpha1.GroupVersionKind{Version: "v1", Kind: "Pod"},
			List: &v1alpha1.ListReference{
				MatchLabels: map[string]v1alpha1.LabelValue{
					"job-name":   {Expression: ptr.To("object.metadata.name")},
					"managed-by": {Value: ptr.To("karta")},
				},
				MatchExpressions: []v1alpha1.LabelSelectorRequirement{
					{Key: "component", Operator: v1alpha1.LabelSelectorOpExists},
				},
			},
		})

		resolved, err := references.Resolve(ctx, reader, karta, workload)
		Expect(err).NotTo(HaveOccurred())
		Expect(resolved["pods"].List).To(HaveLen(1))
		Expect(reader.listQ).To(HaveLen(1))
		Expect(reader.listQ[0].Namespace).To(Equal("team-a"))
		selector := reader.listQ[0].Selector.String()
		Expect(selector).To(ContainSubstring("job-name=fine-tune"))
		Expect(selector).To(ContainSubstring("managed-by=karta"))
		Expect(selector).To(ContainSubstring("component"))

		bindings, err := resolved.Bindings()
		Expect(err).NotTo(HaveOccurred())
		Expect(bindings["pods"]).To(Equal([]any{pod.Object}))
	})

	It("names the reference and the missing verb when the reader denies permission", func() {
		reader := &deniedReader{fakeReader: &fakeReader{
			denyVerbs: map[string]error{"get": fmt.Errorf("missing RBAC: get clustertrainingruntimes")},
		}}
		karta := kartaWith(v1alpha1.ResourceReference{
			Name:   "trainingRuntime",
			GVK:    v1alpha1.GroupVersionKind{Group: "trainer.kubeflow.org", Version: "v1alpha1", Kind: "ClusterTrainingRuntime"},
			Lookup: &v1alpha1.LookupReference{NameExpression: "object.spec.runtimeRef.name"},
		})

		_, err := references.Resolve(ctx, reader, karta, workload)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`reference "trainingRuntime"`))
		Expect(err.Error()).To(ContainSubstring("may not get"))
		Expect(err.Error()).To(ContainSubstring("missing RBAC"))
		Expect(reader.gets).To(BeEmpty(), "a denied fetch must not reach the reader")
	})

	It("operates when the reader allows the verbs", func() {
		reader := &deniedReader{fakeReader: &fakeReader{denyVerbs: map[string]error{}}}
		karta := kartaWith(v1alpha1.ResourceReference{
			Name: "pods",
			GVK:  v1alpha1.GroupVersionKind{Version: "v1", Kind: "Pod"},
			List: &v1alpha1.ListReference{MatchLabels: map[string]v1alpha1.LabelValue{
				"app": {Value: ptr.To("x")},
			}},
		})

		resolved, err := references.Resolve(ctx, reader, karta, workload)
		Expect(err).NotTo(HaveOccurred())
		Expect(resolved).To(HaveKey("pods"))
	})

	It("fails with the reference name when the name expression yields no string", func() {
		reader := &fakeReader{}
		karta := kartaWith(v1alpha1.ResourceReference{
			Name:   "trainingRuntime",
			GVK:    v1alpha1.GroupVersionKind{Group: "trainer.kubeflow.org", Version: "v1alpha1", Kind: "ClusterTrainingRuntime"},
			Lookup: &v1alpha1.LookupReference{NameExpression: `object[?"spec"][?"missing"].orValue(null)`},
		})

		_, err := references.Resolve(ctx, reader, karta, workload)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`reference "trainingRuntime"`))
		Expect(err.Error()).To(ContainSubstring("expected a string"))
	})
})
