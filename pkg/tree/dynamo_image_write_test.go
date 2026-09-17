// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

const dynamoImageWriteInput = `{
  "apiVersion": "nvidia.com/v1beta1", "kind": "DynamoGraphDeployment",
  "metadata": {"name": "example-graph", "annotations": {"example.com/note": "keep"}},
  "spec": {"extension": {"setting": "keep"}, "components": [
    {"name": "worker", "replicas": 2, "extension": "keep-component",
     "podTemplate": {"extension": "keep-template", "metadata": {"labels": {"team": "ml"}},
       "spec": {"schedulerName": "default-scheduler", "extension": "keep-spec", "containers": [
         {"name": "main", "image": "ghcr.io/example/worker:v1", "extension": {"setting": "keep-main"},
          "env": [{"name": "MODE", "value": "serve"}], "resources": {"requests": {"cpu": "500m"}}},
         {"name": "sidecar", "image": "ghcr.io/example/helper:v1", "extension": "keep-sidecar",
          "env": [{"name": "MODE", "value": "observe"}]}
       ]}
     }},
    {"name": "frontend", "replicas": 1, "extension": "keep-frontend",
     "podTemplate": {"spec": {"containers": [{"name": "main", "image": "ghcr.io/example/frontend:v1"}]}}}
  ]},
  "status": {"state": "successful"}
}`

func TestDynamoV1beta1ImageWrite(t *testing.T) {
	for _, patchType := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		for _, strategy := range []resource.MergeStrategy{resource.Merge, resource.Replace} {
			for _, scenario := range []struct {
				name           string
				componentIndex int
				containerIndex int
				edit           func(map[string]any, []any)
				imageMissing   bool
				rejected       bool
			}{
				{name: "main first"},
				{name: "main last", containerIndex: 1},
				{name: "component reordered", componentIndex: 1},
				{name: "both reordered", componentIndex: 1, containerIndex: 1},
				{
					name: "nameless container before main", containerIndex: 1,
					edit: func(_ map[string]any, containers []any) {
						delete(containers[1].(map[string]any), "name")
					},
				},
				{
					name: "image absent", imageMissing: true,
					edit: func(_ map[string]any, containers []any) {
						delete(containers[0].(map[string]any), "image")
					},
				},
				{
					name: "main missing", rejected: true,
					edit: func(_ map[string]any, containers []any) {
						containers[0].(map[string]any)["name"] = "other"
					},
				},
				{
					name: "main duplicated", rejected: true,
					edit: func(_ map[string]any, containers []any) {
						containers[1].(map[string]any)["name"] = "main"
					},
				},
				{
					name: "containers absent", rejected: true,
					edit: func(spec map[string]any, _ []any) {
						delete(spec, "containers")
					},
				},
				{
					name: "containers empty", rejected: true,
					edit: func(spec map[string]any, _ []any) {
						spec["containers"] = []any{}
					},
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
					target, resolveErr := editor.ResolveWriteTarget(ctx, tree.Target{Component: "component", Instance: "worker", Field: tree.Image})
					assertSyntheticCatalogObject(t, editor, before)
					if !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
						t.Fatal("resolving an image destination changed the extracted tree")
					}
					mutationErr := editor.Mutate(ctx, tree.Write{
						Component: "component", Instance: "worker", Field: tree.Image, Value: "ghcr.io/example/worker:v2",
						Options: resource.MutationOptions{PatchType: patchType, Strategy: strategy},
					})
					if !reflect.DeepEqual(before, workload) {
						t.Fatal("image write changed the caller's workload")
					}
					if scenario.rejected {
						if resolveErr == nil || mutationErr == nil {
							t.Fatalf("invalid main container selection was accepted: resolve = %v, mutation = %v", resolveErr, mutationErr)
						}
						assertSyntheticCatalogObject(t, editor, before)
						if !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
							t.Fatal("rejected image write changed the extracted tree")
						}
						return
					}
					wantPath := fmt.Sprintf("/spec/components/%d/podTemplate/spec/containers/%d/image", scenario.componentIndex, scenario.containerIndex)
					if resolveErr != nil || target.Path != wantPath || target.Exists == scenario.imageMissing {
						t.Fatalf("image target = %+v, error = %v; want %q, exists = %t", target, resolveErr, wantPath, !scenario.imageMissing)
					}
					if !scenario.imageMissing && target.Value != "ghcr.io/example/worker:v1" {
						t.Fatalf("initial image target value = %#v", target.Value)
					}
					if mutationErr != nil {
						t.Fatal(mutationErr)
					}
					expected := before.DeepCopy()
					expectedComponents := expected.Object["spec"].(map[string]any)["components"].([]any)
					expectedSpec := expectedComponents[scenario.componentIndex].(map[string]any)["podTemplate"].(map[string]any)["spec"].(map[string]any)
					expectedSpec["containers"].([]any)[scenario.containerIndex].(map[string]any)["image"] = "ghcr.io/example/worker:v2"
					assertSyntheticCatalogObject(t, editor, expected)
					instances := syntheticCatalogComponents(t, definition, editor.Snapshot())["component"].Instances
					if len(instances) != 2 || instances[1].InstanceKey == nil || *instances[1].InstanceKey != "worker" {
						t.Fatalf("worker instance missing from refreshed tree: %#v", instances)
					}
					fragment := instances[1].ExtractedInstance.FragmentedPodSpec
					if fragment == nil || fragment.Image != "ghcr.io/example/worker:v2" || fragment.Container == nil || fragment.Container.Image != fragment.Image {
						t.Fatalf("image and container read-back disagree: %#v", fragment)
					}
				})
			}
		}
	}
}

func TestDynamoV1beta1ImageWriteBatchRejectionIsAtomic(t *testing.T) {
	for _, patchType := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		for _, strategy := range []resource.MergeStrategy{resource.Merge, resource.Replace} {
			t.Run(string(patchType)+"/"+string(strategy), func(t *testing.T) {
				ctx := context.Background()
				workload := syntheticCatalogInput(t, dynamoImageWriteInput)
				editor, err := tree.Open(ctx, kartas.DynamoV1beta1(), workload)
				if err != nil {
					t.Fatal(err)
				}
				beforeTree := editor.Snapshot()
				options := resource.MutationOptions{PatchType: patchType, Strategy: strategy}
				err = editor.Mutate(ctx,
					tree.Write{Component: "component", Instance: "worker", Field: tree.Image, Value: "ghcr.io/example/worker:v2", Options: options},
					tree.Write{Component: "component", Instance: "worker", Field: tree.Container, Value: map[string]any{"image": "ghcr.io/example/worker:v1"}, Options: options},
				)
				if err == nil || !strings.Contains(err.Error(), "overlapping write targets") {
					t.Fatalf("batch containing conflicting image and whole-container writes should be rejected, got %v", err)
				}
				assertSyntheticCatalogObject(t, editor, workload)
				if !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
					t.Fatal("rejected batch changed the extracted tree")
				}
			})
		}
	}
}
