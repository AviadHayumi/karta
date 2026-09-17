// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/tree"
)

func instanceValidationRay() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "ray.io/v1", "kind": "RayCluster",
		"metadata": map[string]any{"name": "example-ray"},
		"spec": map[string]any{"workerGroupSpecs": []any{
			map[string]any{"groupName": "gpu", "template": map[string]any{"spec": map[string]any{
				"schedulerName": "gpu-scheduler",
				"containers":    []any{map[string]any{"name": "worker", "image": "worker:v1"}},
			}}},
			map[string]any{"groupName": "cpu", "template": map[string]any{"spec": map[string]any{
				"schedulerName": "cpu-scheduler",
				"containers":    []any{map[string]any{"name": "worker", "image": "worker:v1"}},
			}}},
		}},
	}}
}

func TestOpenRejectsMalformedRayInstances(t *testing.T) {
	for _, test := range []struct {
		name      string
		change    func(*v1alpha1.Karta, []any)
		wantError string
	}{
		{name: "missing ID", change: func(_ *v1alpha1.Karta, groups []any) {
			delete(groups[0].(map[string]any), "groupName")
		}, wantError: "must be strings"},
		{name: "null ID", change: func(_ *v1alpha1.Karta, groups []any) {
			groups[0].(map[string]any)["groupName"] = nil
		}, wantError: "must be strings"},
		{name: "numeric ID", change: func(_ *v1alpha1.Karta, groups []any) {
			groups[0].(map[string]any)["groupName"] = int64(7)
		}, wantError: "must be strings"},
		{name: "boolean ID", change: func(_ *v1alpha1.Karta, groups []any) {
			groups[0].(map[string]any)["groupName"] = true
		}, wantError: "must be strings"},
		{name: "empty ID", change: func(_ *v1alpha1.Karta, groups []any) {
			groups[0].(map[string]any)["groupName"] = ""
		}, wantError: "empty string"},
		{name: "duplicate ID", change: func(_ *v1alpha1.Karta, groups []any) {
			groups[0].(map[string]any)["groupName"] = "cpu"
		}, wantError: "duplicate instance id"},
		{name: "IDs not a list", change: func(definition *v1alpha1.Karta, _ []any) {
			definition.Spec.StructureDefinition.ChildComponents[1].InstanceIds.Expression = `"gpu"`
		}, wantError: "must return a list"},
		{name: "too few templates", change: func(definition *v1alpha1.Karta, _ []any) {
			definition.Spec.StructureDefinition.ChildComponents[1].SpecDefinition.PodTemplateSpec.Expression = `[object.spec.workerGroupSpecs[0].template]`
		}, wantError: "instance ids count (2) does not match results count (1)"},
		{name: "invalid template type", change: func(_ *v1alpha1.Karta, groups []any) {
			groups[0].(map[string]any)["template"] = "not an object"
		}, wantError: "cannot unmarshal string"},
	} {
		t.Run(test.name, func(t *testing.T) {
			definition, workload := kartas.Raycluster(), instanceValidationRay()
			groups := workload.Object["spec"].(map[string]any)["workerGroupSpecs"].([]any)
			test.change(definition, groups)
			before := workload.DeepCopy()
			if _, err := tree.Open(context.Background(), definition, workload); err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Open error = %v, want %q", err, test.wantError)
			}
			if !reflect.DeepEqual(workload, before) {
				t.Fatal("rejected workload was modified")
			}
		})
	}
}

func TestMalformedRayInstanceMutationRollsBack(t *testing.T) {
	for _, test := range []struct {
		name string
		id   any
	}{
		{name: "empty ID", id: ""},
		{name: "null ID", id: nil},
		{name: "numeric ID", id: 7},
		{name: "duplicate ID", id: "cpu"},
	} {
		for _, useDraft := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/draft=%t", test.name, useDraft), func(t *testing.T) {
				ctx := context.Background()
				definition := kartas.Raycluster()
				definition.Spec.StructureDefinition.RootComponent.Fields = map[string]v1alpha1.ValueAccessor{
					"workerGroups": {PathWrite: ptr.To("/spec/workerGroupSpecs")},
				}
				editor, err := tree.Open(ctx, definition, instanceValidationRay())
				if err != nil {
					t.Fatal(err)
				}
				beforeObject, beforeTree := safetyResource(t, editor), editor.Snapshot()
				replacement := instanceValidationRay().Object["spec"].(map[string]any)["workerGroupSpecs"].([]any)
				replacement[0].(map[string]any)["groupName"] = test.id
				var mutationErr error
				if useDraft {
					draft, err := tree.BeginEdit(ctx, editor)
					if err != nil {
						t.Fatal(err)
					}
					defer draft.Abort()
					groups, err := draft.Target(ctx, tree.Target{Component: "raycluster", Field: tree.Field("workerGroups")})
					if err != nil {
						t.Fatal(err)
					}
					if err := groups.Replace(replacement); err != nil {
						t.Fatal(err)
					}
					mutationErr = draft.Commit(ctx)
				} else {
					mutationErr = editor.Mutate(ctx, tree.Write{
						Component: "raycluster", Field: tree.Field("workerGroups"), Value: replacement,
					})
				}
				if mutationErr == nil {
					t.Fatal("mutation published invalid worker group IDs")
				}
				if !reflect.DeepEqual(beforeObject, safetyResource(t, editor)) || !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
					t.Fatal("failed mutation changed the workload or extracted tree")
				}
			})
		}
	}
}

func TestRayWriteIndexUsesWorkloadOrder(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reversed=%t", reverse), func(t *testing.T) {
			ctx := context.Background()
			workload := instanceValidationRay()
			groups := workload.Object["spec"].(map[string]any)["workerGroupSpecs"].([]any)
			index := 0
			if reverse {
				groups[0], groups[1] = groups[1], groups[0]
				index = 1
			}
			expected := workload.DeepCopy()
			expectedGroups := expected.Object["spec"].(map[string]any)["workerGroupSpecs"].([]any)
			expectedGroups[index].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["schedulerName"] = "batch-scheduler"
			editor, err := tree.Open(ctx, kartas.Raycluster(), workload)
			if err != nil {
				t.Fatal(err)
			}
			target, err := editor.ResolveWriteTarget(ctx, tree.Target{
				Component: "worker", Instance: "gpu", Field: tree.PodTemplateSpec,
			})
			if err != nil {
				t.Fatal(err)
			}
			if want := fmt.Sprintf("/spec/workerGroupSpecs/%d/template", index); target.Path != want {
				t.Fatalf("gpu target = %q, want %q", target.Path, want)
			}
			if err := editor.Mutate(ctx, tree.Write{
				Component: "worker", Instance: "gpu", Field: tree.PodTemplateSpec,
				Value: map[string]any{"spec": map[string]any{"schedulerName": "batch-scheduler"}},
			}); err != nil {
				t.Fatal(err)
			}
			if got := safetyResource(t, editor); !reflect.DeepEqual(got, expected.Object) {
				t.Fatalf("wrong group or unrelated field changed: got %#v, want %#v", got, expected.Object)
			}
		})
	}
}
