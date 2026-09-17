// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package blackbox

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/resource"
)

func TestCatalogDynamoV1beta1ImageOnlyWrite(t *testing.T) {
	for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		for _, strategy := range []resource.MergeStrategy{resource.Merge, resource.Replace} {
			t.Run(string(format)+"/"+string(strategy), func(t *testing.T) {
				g := NewWithT(t)
				ctx := context.Background()
				object := dynamoImageWriteFixture()
				before := runtime.DeepCopyJSON(object)
				factory, accessor := newCatalogPathFactory(t, kartas.DynamoV1beta1(), object,
					resource.MutationOptions{PatchType: format, Strategy: strategy})
				component, err := factory.GetComponent("component")
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(component.UpdateFragmentedPodSpec(ctx, map[string]resource.FragmentedPodSpec{
					"worker": {Image: "ghcr.io/example/worker:v2"},
				})).To(Succeed())

				expected := runtime.DeepCopyJSON(before)
				podSpec := expected["spec"].(map[string]any)["components"].([]any)[0].(map[string]any)["podTemplate"].(map[string]any)["spec"].(map[string]any)
				podSpec["containers"].([]any)[1].(map[string]any)["image"] = "ghcr.io/example/worker:v2"
				updated, err := accessor.GetObject()
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(updated).To(Equal(expected), "only the main container image should change, preserving sidecars and unknown fields")
				g.Expect(object).To(Equal(before), "the caller's workload should remain unchanged")

				fragments, err := component.GetFragmentedPodSpec(ctx)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(fragments).To(HaveLen(1))
				g.Expect(fragments).To(HaveKey("worker"))
				g.Expect(fragments["worker"].Image).To(Equal("ghcr.io/example/worker:v2"))
				g.Expect(fragments["worker"].Container).NotTo(BeNil())
				g.Expect(fragments["worker"].Container.Name).To(Equal("main"))
				g.Expect(fragments["worker"].Container.Image).To(Equal(fragments["worker"].Image))
			})
		}
	}
}

// A full extraction contains Image and Container.Image. The caller must choose
// one writable representation; neither stale value silently wins over the other.
func TestCatalogDynamoV1beta1FullFragmentOverlapRollback(t *testing.T) {
	for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		for _, strategy := range []resource.MergeStrategy{resource.Merge, resource.Replace} {
			for _, changeContainer := range []bool{false, true} {
				changedField := "Image"
				if changeContainer {
					changedField = "Container.Image"
				}
				t.Run(string(format)+"/"+string(strategy)+"/new_"+changedField, func(t *testing.T) {
					g := NewWithT(t)
					ctx := context.Background()
					object := dynamoImageWriteFixture()
					before := runtime.DeepCopyJSON(object)
					factory, accessor := newCatalogPathFactory(t, kartas.DynamoV1beta1(), object,
						resource.MutationOptions{PatchType: format, Strategy: strategy})
					component, err := factory.GetComponent("component")
					g.Expect(err).NotTo(HaveOccurred())
					fragments, err := component.GetFragmentedPodSpec(ctx)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(fragments).To(HaveLen(1))
					g.Expect(fragments).To(HaveKey("worker"))
					fragment := fragments["worker"]
					g.Expect(fragment.SchedulerName).To(Equal("original-scheduler"))
					g.Expect(fragment.Image).To(Equal("ghcr.io/example/worker:v1"))
					g.Expect(fragment.Container).NotTo(BeNil())
					g.Expect(fragment.Container.Image).To(Equal(fragment.Image))
					if changeContainer {
						fragment.Container.Image = "ghcr.io/example/worker:v2"
					} else {
						fragment.Image = "ghcr.io/example/worker:v2"
					}
					fragments["worker"] = fragment

					// Make the stale scheduler write observable without changing anything
					// except one image representation in the extracted full fragment.
					g.Expect(component.UpdateFragmentedPodSpec(ctx, map[string]resource.FragmentedPodSpec{
						"worker": {SchedulerName: "newer-scheduler"},
					})).To(Succeed())
					expected := runtime.DeepCopyJSON(before)
					podSpec := expected["spec"].(map[string]any)["components"].([]any)[0].(map[string]any)["podTemplate"].(map[string]any)["spec"].(map[string]any)
					podSpec["schedulerName"] = "newer-scheduler"
					staged, err := accessor.GetObject()
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(staged).To(Equal(expected))
					currentFragments, err := component.GetFragmentedPodSpec(ctx)
					g.Expect(err).NotTo(HaveOccurred())

					err = component.UpdateFragmentedPodSpec(ctx, fragments)
					g.Expect(err).To(MatchError(ContainSubstring("overlapping")))
					updated, err := accessor.GetObject()
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(updated).To(Equal(expected), "the rejected full fragment must roll back scheduler, image, and every other earlier write")
					g.Expect(object).To(Equal(before), "the caller's workload should remain unchanged")
					afterFragments, err := component.GetFragmentedPodSpec(ctx)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(afterFragments).To(Equal(currentFragments), "both Image and Container.Image should still report the original image")
				})
			}
		}
	}
}

func dynamoImageWriteFixture() map[string]any {
	return map[string]any{
		"apiVersion": "nvidia.com/v1beta1", "kind": "DynamoGraphDeployment",
		"metadata": map[string]any{"name": "example-graph", "labels": map[string]any{"team": "ml"}},
		"spec": map[string]any{
			"extension": map[string]any{"setting": "keep"},
			"components": []any{map[string]any{
				"name": "worker", "replicas": float64(2),
				"extension": map[string]any{"componentSetting": "keep"},
				"podTemplate": map[string]any{
					"metadata": map[string]any{
						"labels":      map[string]any{"role": "worker"},
						"annotations": map[string]any{"example.com/setting": "keep"},
					},
					"spec": map[string]any{
						"schedulerName": "original-scheduler", "priorityClassName": "batch",
						"extension": map[string]any{"podSetting": "keep"},
						"containers": []any{
							map[string]any{"name": "metrics", "image": "ghcr.io/example/metrics:v1"},
							map[string]any{
								"name": "main", "image": "ghcr.io/example/worker:v1",
								"env":       []any{map[string]any{"name": "MODE", "value": "serve"}},
								"resources": map[string]any{"limits": map[string]any{"cpu": "2"}},
								"extension": map[string]any{"containerSetting": "keep"},
							},
							map[string]any{"name": "logger", "image": "ghcr.io/example/logger:v1"},
						},
					},
				},
			}},
		},
		"status": map[string]any{"state": "successful"},
	}
}
