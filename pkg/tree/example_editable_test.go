// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree_test

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

func ExampleOpen_deploymentPath() {
	ctx := context.Background()
	workload := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": "example-api"},
		"spec": map[string]any{"template": map[string]any{
			"metadata": map[string]any{"labels": map[string]any{"app": "example-api"}},
			"spec": map[string]any{
				"schedulerName": "default-scheduler",
				"containers":    []any{map[string]any{"name": "api", "image": "ghcr.io/example/api:v1"}},
			},
		}},
	}}
	editor, err := tree.Open(ctx, kartas.Deployment(), workload)
	if err != nil {
		panic(err)
	}
	fmt.Println("root component:", editor.Snapshot().Root.Name)
	target, err := editor.ResolveWriteTarget(ctx, tree.Target{
		Component: "deployment", Field: tree.PodTemplateSpec,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("catalog path:", target.Path)
	fmt.Println("target exists:", target.Exists)

	// A partial map changes one property without round-tripping a typed template.
	// The SDK's default MergePatch + Merge preserves fields absent from this map.
	if err := editor.Mutate(ctx, tree.Write{
		Component: "deployment", Field: tree.PodTemplateSpec,
		Value: map[string]any{"spec": map[string]any{"schedulerName": "batch-scheduler"}},
	}); err != nil {
		panic(err)
	}
	template := editor.Snapshot().Root.Instances[0].ExtractedInstance.PodTemplateSpec
	fmt.Println("read-back scheduler:", template.Spec.SchedulerName)
	fmt.Println("read-back image:", template.Spec.Containers[0].Image)
	fmt.Println("read-back app:", template.Labels["app"])
	updated, err := editor.GetResource()
	if err != nil {
		panic(err)
	}
	fmt.Println("local result kind:", updated.GetObjectKind().GroupVersionKind().Kind)
	// No API request has been made. A controller can now persist updated.
	// Output:
	// root component: deployment
	// catalog path: /spec/template
	// target exists: true
	// read-back scheduler: batch-scheduler
	// read-back image: ghcr.io/example/api:v1
	// read-back app: example-api
	// local result kind: Deployment
}

func ExampleOpen_rayWorkerByID() {
	ctx := context.Background()
	groups := []any{}
	for _, name := range []string{"gpu", "cpu"} {
		groups = append(groups, map[string]any{
			"groupName": name,
			"template": map[string]any{"spec": map[string]any{
				"schedulerName": "default-scheduler",
				"containers":    []any{map[string]any{"name": "worker", "image": "ghcr.io/example/ray:v1"}},
			}},
		})
	}
	workload := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "ray.io/v1", "kind": "RayCluster",
		"metadata": map[string]any{"name": "example-ray"},
		"spec": map[string]any{
			"headGroupSpec": map[string]any{"template": map[string]any{"spec": map[string]any{
				"containers": []any{map[string]any{"name": "head", "image": "ghcr.io/example/ray:v1"}},
			}}},
			"workerGroupSpecs": groups,
		},
	}}
	editor, err := tree.Open(ctx, kartas.Raycluster(), workload)
	if err != nil {
		panic(err)
	}
	// The tree sorts IDs for display. The workload's source array remains gpu, cpu.
	for _, component := range editor.Snapshot().Children {
		if component.Name == "worker" {
			fmt.Printf("display order: %s, %s\n", *component.Instances[0].InstanceKey, *component.Instances[1].InstanceKey)
		}
	}
	target, err := editor.ResolveWriteTarget(ctx, tree.Target{
		Component: "worker", Instance: "gpu", Field: tree.PodTemplateSpec,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("gpu source path:", target.Path)
	// Only the selected instance is supplied. cpu need not be read or rewritten.
	if err := editor.Mutate(ctx, tree.Write{
		Component: "worker", Instance: "gpu", Field: tree.PodTemplateSpec,
		Value: map[string]any{"spec": map[string]any{"schedulerName": "batch-scheduler"}},
		Options: resource.MutationOptions{
			PatchType: resource.PatchTypeJSONPatch, Strategy: resource.Merge,
		},
	}); err != nil {
		panic(err)
	}
	for _, component := range editor.Snapshot().Children {
		if component.Name == "worker" {
			for _, instance := range component.Instances {
				fmt.Printf("after %s: %s\n", *instance.InstanceKey, instance.ExtractedInstance.PodTemplateSpec.Spec.SchedulerName)
			}
		}
	}
	// Output:
	// display order: cpu, gpu
	// gpu source path: /spec/workerGroupSpecs/0/template
	// after cpu: default-scheduler
	// after gpu: batch-scheduler
}

func ExampleOpen_suspendResume() {
	ctx := context.Background()
	workload := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "batch/v1", "kind": "Job",
		"metadata": map[string]any{"name": "example-job"},
		"spec": map[string]any{
			"suspend": false,
			"template": map[string]any{"spec": map[string]any{
				"restartPolicy": "Never",
				"containers":    []any{map[string]any{"name": "job", "image": "ghcr.io/example/job:v1"}},
			}},
		},
	}}
	editor, err := tree.Open(ctx, kartas.BatchJob(), workload)
	if err != nil {
		panic(err)
	}
	fmt.Println("suspend supported:", editor.IsSuspendable())
	// The interface exposes the capability and both operations.
	if !editor.IsSuspendable() {
		return
	}
	if err := editor.Suspend(ctx); err != nil {
		panic(err)
	}
	suspended, err := editor.GetResource()
	if err != nil {
		panic(err)
	}
	value, found, err := unstructured.NestedBool(suspended.(*unstructured.Unstructured).Object, "spec", "suspend")
	if err != nil || !found {
		panic("suspend field missing")
	}
	fmt.Println("after Suspend:", value)
	if err := editor.Resume(ctx); err != nil {
		panic(err)
	}
	resumed, err := editor.GetResource()
	if err != nil {
		panic(err)
	}
	value, found, err = unstructured.NestedBool(resumed.(*unstructured.Unstructured).Object, "spec", "suspend")
	if err != nil || !found {
		panic("suspend field missing")
	}
	fmt.Println("after Resume:", value)
	// These change the desired spec locally. They do not wait for an operator.
	// Output:
	// suspend supported: true
	// after Suspend: true
	// after Resume: false
}

func ExampleOpen_kserveDynamicPath() {
	ctx := context.Background()
	for _, flavor := range []string{"model", "sklearn"} {
		workload := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "serving.kserve.io/v1beta1", "kind": "InferenceService",
			"metadata": map[string]any{"name": "example-model"},
			"spec": map[string]any{"predictor": map[string]any{
				flavor: map[string]any{
					"image": "ghcr.io/example/inference:v1", "storageUri": "s3://example-models/model",
				},
			}},
		}}
		editor, err := tree.Open(ctx, kartas.KServe(), workload)
		if err != nil {
			panic(err)
		}
		// Karta evaluates variables.containerKeys against the current CR.
		// The caller names the logical field, not the selected predictor flavor.
		field := tree.Target{Component: "predictor", Field: tree.Container}
		before, err := editor.ResolveWriteTarget(ctx, field)
		if err != nil {
			panic(err)
		}
		fmt.Println("resolved path:", before.Path)
		fmt.Println("before:", before.Value.(map[string]any)["image"])
		newImage := "ghcr.io/example/inference:v2" // Supplied by the caller.
		if err := editor.Mutate(ctx, tree.Write{
			Component: "predictor", Field: tree.Container,
			Value: map[string]any{"image": newImage},
			Options: resource.MutationOptions{
				PatchType: resource.PatchTypeMergePatch, Strategy: resource.Merge,
			},
		}); err != nil {
			panic(err)
		}
		// Resolve again after a write. A cached path/value describes the old state.
		after, err := editor.ResolveWriteTarget(ctx, field)
		if err != nil {
			panic(err)
		}
		model := after.Value.(map[string]any)
		fmt.Println("after:", model["image"])
		fmt.Println("kept storageUri:", model["storageUri"])
	}
	// Output:
	// resolved path: /spec/predictor/model
	// before: ghcr.io/example/inference:v1
	// after: ghcr.io/example/inference:v2
	// kept storageUri: s3://example-models/model
	// resolved path: /spec/predictor/sklearn
	// before: ghcr.io/example/inference:v1
	// after: ghcr.io/example/inference:v2
	// kept storageUri: s3://example-models/model
}

func ExampleOpen_kserveUnresolvedPath() {
	ctx := context.Background()
	workload := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "serving.kserve.io/v1beta1", "kind": "InferenceService",
		"metadata": map[string]any{"name": "example-model"},
		"spec": map[string]any{"predictor": map[string]any{
			"model": map[string]any{"image": "ghcr.io/example/inference:v1"},
		}},
	}}
	editor, err := tree.Open(ctx, kartas.KServe(), workload)
	if err != nil {
		panic(err)
	}
	// This catalog finds its container by storageUri. This input has none.
	// Karta reports the missing target; it does not guess that model is writable.
	_, err = editor.ResolveWriteTarget(ctx, tree.Target{
		Component: "predictor", Field: tree.Container,
	})
	fmt.Println("target resolved:", err == nil)
	err = editor.Mutate(ctx, tree.Write{
		Component: "predictor", Field: tree.Container,
		Value: map[string]any{"image": "ghcr.io/example/inference:v2"},
	})
	fmt.Println("write rejected:", err != nil)
	updated, err := editor.GetResource()
	if err != nil {
		panic(err)
	}
	image, found, err := unstructured.NestedString(updated.(*unstructured.Unstructured).Object,
		"spec", "predictor", "model", "image")
	if err != nil || !found {
		panic("original model image is missing")
	}
	fmt.Println("image unchanged:", image)
	// Output:
	// target resolved: false
	// write rejected: true
	// image unchanged: ghcr.io/example/inference:v1
}
