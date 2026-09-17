// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package main

import (
	"context"
	"os"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

func TestTreeQuickstart(t *testing.T) {
	for _, example := range []workloadExample{
		{
			name: "JobSet", kartaPath: "../../catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml",
			workloadYAML: jobsetWorkloadYAML,
		},
		{
			name: "LeaderWorkerSet", kartaPath: "../../catalog/leaderworkerset-x-k8s-io-leaderworkerset-v1.yaml",
			workloadYAML: lwsWorkloadYAML,
		},
	} {
		t.Run(example.name, func(t *testing.T) {
			// run checks the scheduler, label, and continued presence of every
			// mutated instance in the refreshed extraction.
			if err := run(context.Background(), example, opts{scheduler: "test-scheduler"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestQuickstartDraftAndLegacyMergePreserveCompleteRawWorkload(t *testing.T) {
	for _, example := range []struct {
		name      string
		catalog   string
		workload  []byte
		targets   []tree.Target
		templates func(map[string]any) []map[string]any
	}{
		{
			name: "JobSet", catalog: "../../catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml", workload: jobsetWorkloadYAML,
			targets: []tree.Target{
				{Component: "replicatedjob", Instance: "leader", Field: tree.PodTemplateSpec},
				{Component: "replicatedjob", Instance: "workers", Field: tree.PodTemplateSpec},
			},
			templates: func(object map[string]any) []map[string]any {
				var templates []map[string]any
				for _, job := range object["spec"].(map[string]any)["replicatedJobs"].([]any) {
					templates = append(templates, job.(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["template"].(map[string]any))
				}
				return templates
			},
		},
		{
			name: "LeaderWorkerSet", catalog: "../../catalog/leaderworkerset-x-k8s-io-leaderworkerset-v1.yaml", workload: lwsWorkloadYAML,
			targets: []tree.Target{
				{Component: "leader", Field: tree.PodTemplateSpec},
				{Component: "worker", Field: tree.PodTemplateSpec},
			},
			templates: func(object map[string]any) []map[string]any {
				group := object["spec"].(map[string]any)["leaderWorkerTemplate"].(map[string]any)
				return []map[string]any{group["leaderTemplate"].(map[string]any), group["workerTemplate"].(map[string]any)}
			},
		},
	} {
		for _, workflow := range []string{"draft", "legacy merge", "late failure", "null parent"} {
			t.Run(example.name+"/"+workflow, func(t *testing.T) {
				definitionYAML, err := os.ReadFile(example.catalog)
				if err != nil {
					t.Fatal(err)
				}
				var definition v1alpha1.Karta
				if err := yaml.Unmarshal(definitionYAML, &definition); err != nil {
					t.Fatal(err)
				}
				var object map[string]any
				if err := yaml.Unmarshal(example.workload, &object); err != nil {
					t.Fatal(err)
				}
				for _, template := range example.templates(object) {
					template["metadata"] = map[string]any{"labels": map[string]any{"keep": "existing"}}
					template["opaqueTemplate"] = map[string]any{"nested": []any{nil, false, float64(0)}}
					spec := template["spec"].(map[string]any)
					containers := spec["containers"].([]any)
					containers[0].(map[string]any)["opaqueContainer"] = map[string]any{"nested": []any{map[string]any{"keep": true}}}
					spec["containers"] = append(containers, map[string]any{
						"name": "sidecar", "image": "ghcr.io/example/sidecar:v1", "opaqueSidecar": []any{nil, false},
					})
				}
				if workflow == "null parent" {
					example.templates(object)[0]["metadata"] = nil
				}
				before := runtime.DeepCopyJSON(object)
				editor, err := tree.Open(t.Context(), &definition, &unstructured.Unstructured{Object: object})
				if err != nil {
					t.Fatal(err)
				}
				want := runtime.DeepCopyJSON(object)
				if workflow == "draft" || workflow == "legacy merge" {
					for _, template := range example.templates(want) {
						template["spec"].(map[string]any)["schedulerName"] = "test-scheduler"
						template["metadata"].(map[string]any)["labels"].(map[string]any)["app.kubernetes.io/managed-by"] = "karta"
					}
				}
				switch workflow {
				case "legacy merge":
					var writes []tree.Write
					for _, target := range example.targets {
						writes = append(writes, tree.Write{
							Component: target.Component, Instance: target.Instance, Field: target.Field,
							Value: map[string]any{
								"spec":     map[string]any{"schedulerName": "test-scheduler"},
								"metadata": map[string]any{"labels": map[string]any{"app.kubernetes.io/managed-by": "karta"}},
							},
							Options: resource.MutationOptions{PatchType: resource.PatchTypeMergePatch, Strategy: resource.Merge},
						})
					}
					err = editor.Mutate(t.Context(), writes...)
				case "late failure":
					targets := append([]tree.Target{}, example.targets...)
					targets = append(targets, tree.Target{Component: "missing", Field: tree.PodTemplateSpec})
					err = editTemplates(t.Context(), editor, targets, "test-scheduler")
				default:
					err = editTemplates(t.Context(), editor, example.targets, "test-scheduler")
				}
				if workflow == "late failure" || workflow == "null parent" {
					if err == nil {
						t.Fatal("invalid draft committed")
					}
				} else if err != nil {
					t.Fatal(err)
				}
				updated, err := editor.GetResource()
				if err != nil {
					t.Fatal(err)
				}
				if got := updated.(*unstructured.Unstructured).Object; !reflect.DeepEqual(got, want) {
					t.Fatalf("fields outside scheduler and managed-by changed\n got: %#v\nwant: %#v", got, want)
				}
				if !reflect.DeepEqual(object, before) {
					t.Fatal("editing changed the caller's workload")
				}
			})
		}
	}
}
