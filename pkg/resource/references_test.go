// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package resource

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/expression"
	"github.com/run-ai/karta/pkg/references"
)

// countingReader serves one runtime object and counts fetches, so laziness is observable.
type countingReader struct {
	runtime *unstructured.Unstructured
	gets    int
	denied  error
}

func (r *countingReader) Get(context.Context, schema.GroupVersionKind, string, string) (*unstructured.Unstructured, error) {
	r.gets++
	if r.runtime == nil {
		return nil, references.ErrNotFound
	}

	return r.runtime, nil
}

func (r *countingReader) List(context.Context, schema.GroupVersionKind, references.ListQuery) ([]unstructured.Unstructured, error) {
	return nil, nil
}

func (r *countingReader) CheckRead(context.Context, schema.GroupVersionKind, string, string) error {
	return r.denied
}

var _ = Describe("Factory reference options", func() {
	ctx := context.Background()

	referencingKarta := func() *v1alpha1.Karta {
		return &v1alpha1.Karta{Spec: v1alpha1.KartaSpec{
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "trainjob",
					SpecDefinition: &v1alpha1.SpecDefinition{
						FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
							Image: &v1alpha1.ValueAccessor{
								Expression: `object.?spec.?trainer.?image.orValue(references.trainingRuntime.spec.image)`,
							},
						},
					},
				},
				References: []v1alpha1.ResourceReference{{
					Name: "trainingRuntime",
					GVK:  v1alpha1.GroupVersionKind{Group: "trainer.kubeflow.org", Version: "v1alpha1", Kind: "ClusterTrainingRuntime"},
					Lookup: &v1alpha1.LookupReference{
						NameExpression: "object.spec.runtimeRef.name",
					},
				}},
			},
		}}
	}

	workload := func() *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "trainer.kubeflow.org/v1alpha1",
			"kind":       "TrainJob",
			"metadata":   map[string]any{"name": "fine-tune", "namespace": "team-a"},
			"spec":       map[string]any{"runtimeRef": map[string]any{"name": "torch-distributed"}},
		}}
	}

	runtime := &unstructured.Unstructured{Object: map[string]any{
		"kind": "ClusterTrainingRuntime",
		"spec": map[string]any{"image": "pytorch/pytorch:2.3.0"},
	}}

	extractImage := func(factory *ComponentFactory) ([]FragmentedPodSpec, error) {
		component, err := factory.GetRootComponent()
		Expect(err).NotTo(HaveOccurred())

		return factory.accessor.(*Accessor).ExtractFragmentedPodSpec(ctx, component.Definition())
	}

	It("resolves through a reader lazily and reads the referenced base", func() {
		reader := &countingReader{runtime: runtime}
		factory := NewComponentFactoryFromObject(referencingKarta(), workload(), WithReferenceReader(reader))
		Expect(reader.gets).To(Equal(0), "construction must not fetch")

		fragments, err := extractImage(factory)
		Expect(err).NotTo(HaveOccurred())
		Expect(fragments).To(HaveLen(1))
		Expect(fragments[0].Image).To(Equal("pytorch/pytorch:2.3.0"))
		Expect(reader.gets).To(Equal(1))

		_, err = extractImage(factory)
		Expect(err).NotTo(HaveOccurred())
		Expect(reader.gets).To(Equal(1), "resolution is memoized")
	})

	It("uses pre-resolved values without touching any reader", func() {
		resolved := references.ResolvedReferences{"trainingRuntime": {Object: runtime}}
		factory := NewComponentFactoryFromObject(referencingKarta(), workload(), WithReferences(resolved))

		fragments, err := extractImage(factory)
		Expect(err).NotTo(HaveOccurred())
		Expect(fragments[0].Image).To(Equal("pytorch/pytorch:2.3.0"))
	})

	It("fails on first use with ErrReferencesNotSupported when nothing was provided", func() {
		factory := NewComponentFactoryFromObject(referencingKarta(), workload())

		_, err := extractImage(factory)
		Expect(errors.Is(err, expression.ErrReferencesNotSupported)).To(BeTrue())
	})

	It("surfaces a permission denial with the reference name and verb", func() {
		reader := &countingReader{runtime: runtime, denied: fmt.Errorf("missing RBAC: get clustertrainingruntimes")}
		factory := NewComponentFactoryFromObject(referencingKarta(), workload(), WithReferenceReader(reader))

		_, err := extractImage(factory)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`reference "trainingRuntime"`))
		Expect(err.Error()).To(ContainSubstring("may not get"))
		Expect(reader.gets).To(Equal(0))
	})

	It("does not need references for expressions that never mention them", func() {
		karta := referencingKarta()
		karta.Spec.StructureDefinition.RootComponent.SpecDefinition = &v1alpha1.SpecDefinition{
			PodTemplateSpec: &v1alpha1.ValueAccessor{
				Expression: `object[?"spec"][?"template"].orValue(null)`,
			},
		}
		factory := NewComponentFactoryFromObject(karta, workload())

		component, err := factory.GetRootComponent()
		Expect(err).NotTo(HaveOccurred())
		_, err = factory.accessor.(*Accessor).ExtractPodTemplateSpec(ctx, component.Definition())
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("Declared list references with pre-resolved values", func() {
	ctx := context.Background()

	kartaWithList := func() *v1alpha1.Karta {
		karta := &v1alpha1.Karta{Spec: v1alpha1.KartaSpec{
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "trainjob",
					ScaleDefinition: &v1alpha1.ScaleDefinition{
						Replicas: &v1alpha1.ValueAccessor{Expression: `references.pods.size()`},
					},
				},
				References: []v1alpha1.ResourceReference{{
					Name: "pods",
					GVK:  v1alpha1.GroupVersionKind{Version: "v1", Kind: "Pod"},
					List: &v1alpha1.ListReference{MatchLabels: map[string]v1alpha1.LabelValue{
						"app": {Value: ptr.To("x")},
					}},
				}},
			},
		}}

		return karta
	}

	replicasOf := func(factory *ComponentFactory) (int, error) {
		component, err := factory.GetRootComponent()
		Expect(err).NotTo(HaveOccurred())
		scales, err := factory.accessor.(*Accessor).ExtractScale(ctx, component.Definition())
		if err != nil {
			return 0, err
		}
		Expect(scales).To(HaveLen(1))

		return int(*scales[0].Replicas), nil
	}

	pod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Pod", "metadata": map[string]any{"name": "p0"},
	}}

	It("reads a declared but unresolved list as empty, for nil and for partial maps", func() {
		for _, resolved := range []references.ResolvedReferences{nil, {}} {
			replicas, err := replicasOf(NewComponentFactoryFromObject(kartaWithList(),
				&unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "Pod", "metadata": map[string]any{"name": "w"}}},
				WithReferences(resolved)))
			Expect(err).NotTo(HaveOccurred())
			Expect(replicas).To(Equal(0))
		}
	})

	It("keeps a resolved list intact", func() {
		resolved := references.ResolvedReferences{"pods": references.NewListValue([]unstructured.Unstructured{*pod})}
		replicas, err := replicasOf(NewComponentFactoryFromObject(kartaWithList(),
			&unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "Pod", "metadata": map[string]any{"name": "w"}}},
			WithReferences(resolved)))
		Expect(err).NotTo(HaveOccurred())
		Expect(replicas).To(Equal(1))
	})
})
