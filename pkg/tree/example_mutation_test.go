// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree_test

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

func ExampleBuild_deploymentMutation() {
	ctx := context.Background()
	workload := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": "example-api"},
		"spec": map[string]any{
			"selector": map[string]any{"matchLabels": map[string]any{"app": "example-api"}},
			"template": map[string]any{
				"metadata": map[string]any{"labels": map[string]any{"app": "example-api"}},
				"spec": map[string]any{"containers": []any{
					map[string]any{"name": "api", "image": "ghcr.io/example/api:v1"},
				}},
			},
		},
	}}
	factory := resource.NewComponentFactoryFromObject(kartas.Deployment(), workload,
		resource.WithMutationOptions(resource.MutationOptions{
			PatchType: resource.PatchTypeJSONPatch,
			Strategy:  resource.Merge,
		}))
	snapshot, err := tree.Build(ctx, factory)
	if err != nil {
		panic(err)
	}
	fmt.Println("tree child:", snapshot.Children[0].Name)

	// The Deployment root owns the template and is also available as snapshot.Root.
	deployment, err := factory.GetRootComponent()
	if err != nil {
		panic(err)
	}
	fmt.Println("write component:", deployment.Name())
	templates, err := deployment.GetPodTemplateSpec(ctx)
	if err != nil {
		panic(err)
	}
	template := templates[""] // A single-instance component uses the empty ID.
	fmt.Println("before image:", template.Spec.Containers[0].Image)
	newImage := "ghcr.io/example/api:v2" // Supplied by the SDK caller.
	template.Spec.Containers[0].Image = newImage
	templates[""] = template
	if err := deployment.UpdatePodTemplateSpec(ctx, templates); err != nil {
		panic(err)
	}

	updated, err := factory.GetResource() // Local result; no Kubernetes request.
	if err != nil {
		panic(err)
	}
	exported := updated.(*unstructured.Unstructured)
	containers, found, err := unstructured.NestedSlice(exported.Object, "spec", "template", "spec", "containers")
	if err != nil || !found {
		panic("updated containers are missing")
	}
	fmt.Println("after image:", containers[0].(map[string]any)["image"])
	refreshed, err := tree.Build(ctx, factory)
	if err != nil {
		panic(err)
	}
	fmt.Println("rebuilt tree child:", refreshed.Children[0].Name)
	// Output:
	// tree child: replicaset
	// write component: deployment
	// before image: ghcr.io/example/api:v1
	// after image: ghcr.io/example/api:v2
	// rebuilt tree child: replicaset
}

func ExampleBuild_rayWorkerMutation() {
	ctx := context.Background()
	groups := []any{}
	for _, name := range []string{"gpu", "cpu"} {
		groups = append(groups, map[string]any{
			"groupName": name,
			"template": map[string]any{"spec": map[string]any{
				"schedulerName": "default-scheduler",
				"containers": []any{
					map[string]any{"name": "worker", "image": "ghcr.io/example/ray:v1"},
				},
			}},
		})
	}
	workload := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "ray.io/v1", "kind": "RayCluster",
		"metadata": map[string]any{"name": "example-ray"},
		"spec": map[string]any{
			"headGroupSpec": map[string]any{"template": map[string]any{"spec": map[string]any{
				"containers": []any{
					map[string]any{"name": "head", "image": "ghcr.io/example/ray:v1"},
				},
			}}},
			"workerGroupSpecs": groups,
		},
	}}
	// The default SDK policy is MergePatch + Merge + CreateMaps.
	factory := resource.NewComponentFactoryFromObject(kartas.Raycluster(), workload)
	workers, err := factory.GetComponent("worker")
	if err != nil {
		panic(err)
	}
	ids, err := workers.GetInstanceIds(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Println("source order:", ids)
	snapshot, err := tree.Build(ctx, factory)
	if err != nil {
		panic(err)
	}
	for _, component := range snapshot.Children {
		if component.Name == "worker" {
			fmt.Printf("tree order: %s, %s\n", *component.Instances[0].InstanceKey, *component.Instances[1].InstanceKey)
		}
	}

	templates, err := workers.GetPodTemplateSpec(ctx)
	if err != nil {
		panic(err)
	}
	selectedInstanceID := "gpu" // Select a stable ID, not a tree position.
	selected, found := templates[selectedInstanceID]
	if !found {
		panic("selected worker group is missing")
	}
	fmt.Println("before gpu:", selected.Spec.SchedulerName)
	selected.Spec.SchedulerName = "batch-scheduler"
	templates[selectedInstanceID] = selected
	// The current setter requires every instance, including unchanged cpu.
	if err := workers.UpdatePodTemplateSpec(ctx, templates); err != nil {
		panic(err)
	}
	if _, err := factory.GetResource(); err != nil {
		panic(err)
	}
	refreshed, err := tree.Build(ctx, factory)
	if err != nil {
		panic(err)
	}
	for _, component := range refreshed.Children {
		if component.Name == "worker" {
			for _, instance := range component.Instances {
				fmt.Printf("after %s: %s\n", *instance.InstanceKey, instance.ExtractedInstance.PodTemplateSpec.Spec.SchedulerName)
			}
		}
	}
	// Output:
	// source order: [gpu cpu]
	// tree order: cpu, gpu
	// before gpu: default-scheduler
	// after cpu: default-scheduler
	// after gpu: batch-scheduler
}

func ExampleBuild_kserveMutation() {
	ctx := context.Background()
	workload := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "serving.kserve.io/v1beta1", "kind": "InferenceService",
		"metadata": map[string]any{"name": "example-model"},
		"spec": map[string]any{"predictor": map[string]any{
			"model": map[string]any{
				"image":       "ghcr.io/example/inference:v1",
				"storageUri":  "s3://example-models/model",
				"modelFormat": map[string]any{"name": "sklearn"},
			},
		}},
	}}
	factory := resource.NewComponentFactoryFromObject(kartas.KServe(), workload)
	snapshot, err := tree.Build(ctx, factory)
	if err != nil {
		panic(err)
	}
	fmt.Println("tree component:", snapshot.Children[0].Name)
	predictor, err := factory.GetComponent("predictor")
	if err != nil {
		panic(err)
	}
	newImage := "ghcr.io/example/inference:v2"
	// The catalog's Container target is the whole model, not just model/image.
	// Merge keeps the KServe fields that corev1.Container cannot represent.
	if err := predictor.UpdateFragmentedPodSpec(ctx, map[string]resource.FragmentedPodSpec{
		"": {Container: &corev1.Container{Image: newImage}},
	}); err != nil {
		panic(err)
	}
	updated, err := factory.GetResource()
	if err != nil {
		panic(err)
	}
	exported := updated.(*unstructured.Unstructured)
	model, found, err := unstructured.NestedMap(exported.Object, "spec", "predictor", "model")
	if err != nil || !found {
		panic("updated model is missing")
	}
	fmt.Println("after image:", model["image"])
	fmt.Println("preserved storageUri:", model["storageUri"])
	fmt.Println("preserved modelFormat:", model["modelFormat"].(map[string]any)["name"])
	// A typed Container also serializes zero values; this is not image-only JSON.
	fmt.Printf("typed input adds name: %q\n", model["name"])
	fmt.Println("typed input adds resources:", model["resources"])
	refreshed, err := tree.Build(ctx, factory)
	if err != nil {
		panic(err)
	}
	fmt.Println("rebuilt tree image:", refreshed.Children[0].Instances[0].ExtractedInstance.FragmentedPodSpec.Container.Image)
	// Output:
	// tree component: predictor
	// after image: ghcr.io/example/inference:v2
	// preserved storageUri: s3://example-models/model
	// preserved modelFormat: sklearn
	// typed input adds name: ""
	// typed input adds resources: map[]
	// rebuilt tree image: ghcr.io/example/inference:v2
}
