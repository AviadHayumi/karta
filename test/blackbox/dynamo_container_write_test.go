// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package blackbox

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/resource"
)

func TestCatalogDynamoV1beta1FreshContainerOnlyWrite(t *testing.T) {
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
				input := &corev1.Container{Name: "main", Image: "ghcr.io/example/worker:v2"}

				// Image is empty: only the Container representation is being written.
				g.Expect(component.UpdateFragmentedPodSpec(ctx, map[string]resource.FragmentedPodSpec{
					"worker": {Container: input},
				})).To(Succeed())

				expected := runtime.DeepCopyJSON(before)
				containers := dynamoContainerList(expected, 0)
				if strategy == resource.Merge {
					containers[1].(map[string]any)["image"] = input.Image
				} else {
					containers[1] = serializedDynamoContainer(t, input)
				}
				updated, err := accessor.GetObject()
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(updated).To(Equal(expected), "only the selected main container may change; sidecars and all other fields must stay")
				g.Expect(object).To(Equal(before), "the caller's workload must remain unchanged")
				selected := dynamoContainerList(updated, 0)[1].(map[string]any)
				if strategy == resource.Merge {
					g.Expect(selected).To(HaveKeyWithValue("extension", map[string]any{"containerSetting": "keep"}))
					g.Expect(selected).To(HaveKey("env"))
					g.Expect(selected["resources"]).To(Equal(map[string]any{"limits": map[string]any{"cpu": "2"}}))
				} else {
					g.Expect(selected).NotTo(HaveKey("extension"))
					g.Expect(selected).NotTo(HaveKey("env"))
					g.Expect(selected["resources"]).To(Equal(map[string]any{}))
				}
				fragments, err := component.GetFragmentedPodSpec(ctx)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(fragments["worker"].Image).To(Equal(input.Image))
				g.Expect(fragments["worker"].Container).NotTo(BeNil())
				g.Expect(fragments["worker"].Container.Image).To(Equal(input.Image))
			})
		}
	}
}

func TestCatalogDynamoV1beta1ContainerBulkWriteUsesEachMainIndex(t *testing.T) {
	for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		for _, strategy := range []resource.MergeStrategy{resource.Merge, resource.Replace} {
			t.Run(string(format)+"/"+string(strategy), func(t *testing.T) {
				g := NewWithT(t)
				ctx := context.Background()
				object := dynamoMultiContainerFixture()
				before := runtime.DeepCopyJSON(object)
				factory, accessor := newCatalogPathFactory(t, kartas.DynamoV1beta1(), object,
					resource.MutationOptions{PatchType: format, Strategy: strategy})
				component, err := factory.GetComponent("component")
				g.Expect(err).NotTo(HaveOccurred())
				worker := &corev1.Container{Name: "main", Image: "ghcr.io/example/worker:v2"}
				frontend := &corev1.Container{Name: "main", Image: "ghcr.io/example/frontend:v3"}
				g.Expect(component.UpdateFragmentedPodSpec(ctx, map[string]resource.FragmentedPodSpec{
					"frontend": {Container: frontend},
					"worker":   {Container: worker},
				})).To(Succeed())

				expected := runtime.DeepCopyJSON(before)
				for i, input := range []*corev1.Container{worker, frontend} {
					index := 1 - i // The two workloads deliberately place main differently.
					containers := dynamoContainerList(expected, i)
					if strategy == resource.Merge {
						containers[index].(map[string]any)["image"] = input.Image
					} else {
						containers[index] = serializedDynamoContainer(t, input)
					}
				}
				updated, err := accessor.GetObject()
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(updated).To(Equal(expected), "each stable component ID must address its own main container index")
				g.Expect(object).To(Equal(before))
				fragments, err := component.GetFragmentedPodSpec(ctx)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(fragments).To(HaveLen(2))
				g.Expect(fragments["worker"].Image).To(Equal(worker.Image))
				g.Expect(fragments["frontend"].Image).To(Equal(frontend.Image))
				g.Expect(fragments["worker"].Container.Image).To(Equal(worker.Image))
				g.Expect(fragments["frontend"].Container.Image).To(Equal(frontend.Image))
			})
		}
	}
}

func TestCatalogDynamoV1beta1FullFragmentChooseOneRepresentation(t *testing.T) {
	for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		for _, strategy := range []resource.MergeStrategy{resource.Merge, resource.Replace} {
			for _, selectedField := range []string{"Image", "Container"} {
				t.Run(string(format)+"/"+string(strategy)+"/"+selectedField, func(t *testing.T) {
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
					fragment := fragments["worker"]
					g.Expect(fragment.Container).NotTo(BeNil())
					newImage := "ghcr.io/example/worker:v2"
					if selectedField == "Image" {
						fragment.Image = newImage
						fragment.Container = nil
					} else {
						fragment.Container.Image = newImage
						fragment.Image = ""
					}
					fragments["worker"] = fragment
					g.Expect(component.UpdateFragmentedPodSpec(ctx, fragments)).To(Succeed())

					expected := runtime.DeepCopyJSON(before)
					containers := dynamoContainerList(expected, 0)
					if selectedField == "Container" && strategy == resource.Replace {
						containers[1] = serializedDynamoContainer(t, fragment.Container)
					} else {
						containers[1].(map[string]any)["image"] = newImage
					}
					updated, err := accessor.GetObject()
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(updated).To(Equal(expected))
					g.Expect(object).To(Equal(before))
					readBack, err := component.GetFragmentedPodSpec(ctx)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(readBack["worker"].Image).To(Equal(newImage))
					g.Expect(readBack["worker"].Container.Image).To(Equal(newImage))
				})
			}
		}
	}
}

func TestCatalogDynamoV1beta1ContainerBadTargetRollsBackBulkWrite(t *testing.T) {
	for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		for _, strategy := range []resource.MergeStrategy{resource.Merge, resource.Replace} {
			for _, invalid := range []string{"missing main", "duplicate main", "missing containers"} {
				t.Run(string(format)+"/"+string(strategy)+"/"+invalid, func(t *testing.T) {
					g := NewWithT(t)
					ctx := context.Background()
					object := dynamoMultiContainerFixture()
					second := object["spec"].(map[string]any)["components"].([]any)[1].(map[string]any)["podTemplate"].(map[string]any)["spec"].(map[string]any)
					containers := second["containers"].([]any)
					switch invalid {
					case "missing main":
						containers[0].(map[string]any)["name"] = "other"
					case "duplicate main":
						second["containers"] = append(containers, runtime.DeepCopyJSON(containers[0].(map[string]any)))
					case "missing containers":
						delete(second, "containers")
					}
					before := runtime.DeepCopyJSON(object)
					factory, accessor := newCatalogPathFactory(t, kartas.DynamoV1beta1(), object,
						resource.MutationOptions{PatchType: format, Strategy: strategy})
					component, err := factory.GetComponent("component")
					g.Expect(err).NotTo(HaveOccurred())
					beforeFragments, err := component.GetFragmentedPodSpec(ctx)
					g.Expect(err).NotTo(HaveOccurred())
					err = component.UpdateFragmentedPodSpec(ctx, map[string]resource.FragmentedPodSpec{
						"worker": {SchedulerName: "must-not-stick", Container: &corev1.Container{
							Name: "main", Image: "ghcr.io/example/worker:v2",
						}},
						"frontend": {SchedulerName: "must-not-stick", Container: &corev1.Container{
							Name: "main", Image: "ghcr.io/example/frontend:v3",
						}},
					})
					g.Expect(err).To(HaveOccurred(), "an absent or ambiguous main container must not produce a guessed write target")
					updated, err := accessor.GetObject()
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(updated).To(Equal(before), "a bad later instance must leave valid containers and unrelated scheduler fields untouched")
					g.Expect(object).To(Equal(before))
					afterFragments, err := component.GetFragmentedPodSpec(ctx)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(afterFragments).To(Equal(beforeFragments))
				})
			}
		}
	}
}

func dynamoMultiContainerFixture() map[string]any {
	object := dynamoImageWriteFixture()
	components := object["spec"].(map[string]any)["components"].([]any)
	frontend := runtime.DeepCopyJSON(components[0].(map[string]any))
	frontend["name"] = "frontend"
	containers := frontend["podTemplate"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)
	containers[0], containers[1] = containers[1], containers[0]
	containers[0].(map[string]any)["image"] = "ghcr.io/example/frontend:v1"
	object["spec"].(map[string]any)["components"] = append(components, frontend)
	return object
}

func dynamoContainerList(object map[string]any, componentIndex int) []any {
	return object["spec"].(map[string]any)["components"].([]any)[componentIndex].(map[string]any)["podTemplate"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)
}

func serializedDynamoContainer(t *testing.T, container *corev1.Container) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(container)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	return object
}
