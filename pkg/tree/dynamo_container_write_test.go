// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

func TestDynamoV1beta1ContainerWrite(t *testing.T) {
	for _, patchType := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		for _, strategy := range []resource.MergeStrategy{resource.Merge, resource.Replace} {
			for _, scenario := range []struct {
				name           string
				componentIndex int
				containerIndex int
				edit           func(map[string]any, []any)
				rejected       bool
				disjointBatch  bool
			}{
				{name: "main first"},
				{name: "main last", containerIndex: 1},
				{name: "component reordered", componentIndex: 1},
				{name: "both reordered", componentIndex: 1, containerIndex: 1},
				{name: "cross instance batch", disjointBatch: true},
				{
					name: "nameless sidecar before main", containerIndex: 1,
					edit: func(_ map[string]any, containers []any) { delete(containers[1].(map[string]any), "name") },
				},
				{
					name: "main missing", rejected: true,
					edit: func(_ map[string]any, containers []any) { containers[0].(map[string]any)["name"] = "other" },
				},
				{
					name: "main duplicated", rejected: true,
					edit: func(_ map[string]any, containers []any) { containers[1].(map[string]any)["name"] = "main" },
				},
				{
					name: "containers absent", rejected: true,
					edit: func(spec map[string]any, _ []any) { delete(spec, "containers") },
				},
				{
					name: "containers null", rejected: true,
					edit: func(spec map[string]any, _ []any) { spec["containers"] = nil },
				},
				{
					name: "containers empty", rejected: true,
					edit: func(spec map[string]any, _ []any) { spec["containers"] = []any{} },
				},
			} {
				t.Run(string(patchType)+"/"+string(strategy)+"/"+scenario.name, func(t *testing.T) {
					ctx := context.Background()
					definition := kartas.DynamoV1beta1()
					workload := syntheticCatalogInput(t, dynamoImageWriteInput)
					components := workload.Object["spec"].(map[string]any)["components"].([]any)
					spec := components[0].(map[string]any)["podTemplate"].(map[string]any)["spec"].(map[string]any)
					containers := spec["containers"].([]any)
					if scenario.edit != nil {
						scenario.edit(spec, containers)
					}
					if scenario.containerIndex == 1 {
						containers[0], containers[1] = containers[1], containers[0]
					}
					if scenario.componentIndex == 1 {
						components[0], components[1] = components[1], components[0]
					}
					before := workload.DeepCopy()
					editor, err := tree.Open(ctx, definition, workload)
					if err != nil {
						t.Fatal(err)
					}
					beforeTree := editor.Snapshot()
					target, resolveErr := editor.ResolveWriteTarget(ctx, tree.Target{Component: "component", Instance: "worker", Field: tree.Container})
					assertSyntheticCatalogObject(t, editor, before)
					if !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
						t.Fatal("resolving a container destination changed the extracted tree")
					}
					input := map[string]any{
						"name": "main", "image": "ghcr.io/example/worker:v2", "workingDir": "/app",
						"resources": map[string]any{"limits": map[string]any{"cpu": "2"}},
					}
					beforeInput := runtime.DeepCopyJSONValue(input)
					options := resource.MutationOptions{PatchType: patchType, Strategy: strategy}
					writes := []tree.Write{{Component: "component", Instance: "worker", Field: tree.Container, Value: input, Options: options}}
					if scenario.rejected || scenario.disjointBatch {
						writes = append([]tree.Write{{Component: "component", Instance: "frontend", Field: tree.Image, Value: "ghcr.io/example/frontend:v2", Options: options}}, writes...)
					}
					mutationErr := editor.Mutate(ctx, writes...)
					if !reflect.DeepEqual(before, workload) || !reflect.DeepEqual(beforeInput, input) {
						t.Fatal("container write changed caller-owned input")
					}
					if scenario.rejected {
						if resolveErr == nil || mutationErr == nil {
							t.Fatalf("invalid main selection accepted: resolve = %v, mutation = %v", resolveErr, mutationErr)
						}
						assertSyntheticCatalogObject(t, editor, before)
						if !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
							t.Fatal("rejected container write changed the extracted tree")
						}
						return
					}
					wantPath := fmt.Sprintf("/spec/components/%d/podTemplate/spec/containers/%d", scenario.componentIndex, scenario.containerIndex)
					if resolveErr != nil || target.Path != wantPath || !target.Exists {
						t.Fatalf("container target = %+v, error = %v; want %q", target, resolveErr, wantPath)
					}
					if mutationErr != nil {
						t.Fatal(mutationErr)
					}
					expected := before.DeepCopy()
					expectedComponents := expected.Object["spec"].(map[string]any)["components"].([]any)
					expectedContainers := expectedComponents[scenario.componentIndex].(map[string]any)["podTemplate"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)
					if strategy == resource.Replace {
						expectedContainers[scenario.containerIndex] = runtime.DeepCopyJSONValue(input)
					} else {
						main := expectedContainers[scenario.containerIndex].(map[string]any)
						main["image"], main["workingDir"] = input["image"], input["workingDir"]
						main["resources"].(map[string]any)["limits"] = map[string]any{"cpu": "2"}
					}
					if scenario.disjointBatch {
						expectedComponents[1-scenario.componentIndex].(map[string]any)["podTemplate"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)["image"] = "ghcr.io/example/frontend:v2"
					}
					assertSyntheticCatalogObject(t, editor, expected)
					instances := syntheticCatalogComponents(t, definition, editor.Snapshot())["component"].Instances
					fragment := instances[1].ExtractedInstance.FragmentedPodSpec
					if fragment == nil || fragment.Image != input["image"] || fragment.Container == nil || fragment.Container.Image != fragment.Image || fragment.Container.Name != "main" || fragment.Container.WorkingDir != "/app" || fragment.Container.Resources.Limits.Cpu().String() != "2" {
						t.Fatalf("container read-back = %#v", fragment)
					}
					if strategy == resource.Merge && (len(fragment.Container.Env) != 1 || fragment.Container.Resources.Requests.Cpu().String() != "500m") {
						t.Fatalf("merged container lost existing known properties: %#v", fragment.Container)
					}
					if strategy == resource.Replace && (len(fragment.Container.Env) != 0 || len(fragment.Container.Resources.Requests) != 0) {
						t.Fatalf("replaced container retained omitted properties: %#v", fragment.Container)
					}
					input["image"] = "changed-after-call"
					target.Value.(map[string]any)["image"] = "changed-returned-target"
					assertSyntheticCatalogObject(t, editor, expected)
				})
			}
		}
	}
}

func TestDynamoV1beta1ContainerAndImageOverlapIsAtomic(t *testing.T) {
	for _, patchType := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		for _, strategy := range []resource.MergeStrategy{resource.Merge, resource.Replace} {
			for _, containerFirst := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/container-first-%t", patchType, strategy, containerFirst), func(t *testing.T) {
					ctx := context.Background()
					workload := syntheticCatalogInput(t, dynamoImageWriteInput)
					editor, err := tree.Open(ctx, kartas.DynamoV1beta1(), workload)
					if err != nil {
						t.Fatal(err)
					}
					beforeTree := editor.Snapshot()
					options := resource.MutationOptions{PatchType: patchType, Strategy: strategy}
					writes := []tree.Write{
						{Component: "component", Instance: "worker", Field: tree.Image, Value: "ghcr.io/example/worker:v2", Options: options},
						{Component: "component", Instance: "worker", Field: tree.Container, Value: map[string]any{"name": "main", "image": "ghcr.io/example/worker:v1"}, Options: options},
					}
					if containerFirst {
						writes[0], writes[1] = writes[1], writes[0]
					}
					err = editor.Mutate(ctx, writes...)
					if err == nil || !strings.Contains(err.Error(), "overlapping write targets") {
						t.Fatalf("overlapping container/image writes accepted: %v", err)
					}
					assertSyntheticCatalogObject(t, editor, workload)
					if !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
						t.Fatal("rejected overlapping writes changed the extracted tree")
					}
				})
			}
		}
	}
}
