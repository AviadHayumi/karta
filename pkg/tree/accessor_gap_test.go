// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

type accessorGapCase struct {
	name          string
	definition    *v1alpha1.Karta
	workload      *unstructured.Unstructured
	component     string
	accessor      tree.Field
	accessorPath  string
	schedulerPath []string
}

func accessorGapCases(t *testing.T) []accessorGapCase {
	t.Helper()
	pod := safetyDeployment()
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	pod.Object["spec"] = pod.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"]
	kserve := syntheticCatalogInput(t, `{
  "apiVersion": "serving.kserve.io/v1beta1", "kind": "InferenceService",
  "metadata": {"name": "example-model"},
  "spec": {
    "predictor": {
      "schedulerName": "default-scheduler",
      "model": {"image": "ghcr.io/example/inference:v1", "storageUri": "s3://example-bucket/model", "modelFormat": {"name": "sklearn"}}
    },
    "transformer": {
      "schedulerName": "default-scheduler",
      "containers": [{"name": "transformer", "image": "ghcr.io/example/transformer:v1"}]
    }
  }
}`)
	return []accessorGapCase{
		{name: "Deployment template", definition: kartas.Deployment(), workload: safetyDeployment(), component: "deployment",
			accessor: tree.PodTemplateSpec, accessorPath: "/spec/template", schedulerPath: []string{"spec", "template", "spec", "schedulerName"}},
		{name: "Pod whole-object template", definition: kartas.Pod(), workload: pod, component: "pod",
			accessor: tree.PodTemplateSpec, accessorPath: "", schedulerPath: []string{"spec", "schedulerName"}},
		{name: "KServe predictor fragmented field", definition: kartas.KServe(), workload: kserve.DeepCopy(), component: "predictor",
			accessor: tree.SchedulerName, accessorPath: "/spec/predictor/schedulerName", schedulerPath: []string{"spec", "predictor", "schedulerName"}},
		{name: "KServe transformer PodSpec", definition: kartas.KServe(), workload: kserve.DeepCopy(), component: "transformer",
			accessor: tree.PodSpec, accessorPath: "/spec/transformer", schedulerPath: []string{"spec", "transformer", "schedulerName"}},
	}
}

// An open field name is writable only when the component declares its accessor.
func TestAccessorGapUnknownCustomFieldIsRejectedAtomically(t *testing.T) {
	for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		t.Run(string(format), func(t *testing.T) {
			ctx := context.Background()
			editor, err := tree.Open(ctx, kartas.Deployment(), safetyDeployment())
			if err != nil {
				t.Fatal(err)
			}
			beforeObject, beforeTree := safetyResource(t, editor), editor.Snapshot()
			const customField tree.Field = "exampleos"
			_, err = editor.ResolveWriteTarget(ctx, tree.Target{Component: "deployment", Field: customField})
			if err == nil || !strings.Contains(err.Error(), `no field "exampleos"`) {
				t.Fatalf("unknown field resolution should fail explicitly, got %v", err)
			}
			t.Logf("undeclared field rejected: %v", err)
			options := resource.MutationOptions{PatchType: format, Strategy: resource.Merge}
			err = editor.Mutate(ctx,
				tree.Write{Component: "deployment", Field: tree.PodTemplateSpec,
					Value: map[string]any{"spec": map[string]any{"schedulerName": "batch-scheduler"}}, Options: options},
				tree.Write{Component: "deployment", Field: customField, Value: "new", Options: options},
			)
			if err == nil || !strings.Contains(err.Error(), `no field "exampleos"`) {
				t.Fatalf("unknown field mutation should fail explicitly, got %v", err)
			}
			if !reflect.DeepEqual(beforeObject, safetyResource(t, editor)) || !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
				t.Fatal("unknown field caused a partial write or changed extracted values")
			}
		})
	}
}

// SchedulerName currently resolves only an explicitly declared fragmented field.
// It does not descend through a PodTemplateSpec or PodSpec accessor.
func TestAccessorGapSchedulerNameCharacterization(t *testing.T) {
	for _, fixture := range accessorGapCases(t) {
		t.Run(fixture.name, func(t *testing.T) {
			for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
				t.Run(string(format), func(t *testing.T) {
					ctx := context.Background()
					editor, err := tree.Open(ctx, fixture.definition, fixture.workload)
					if err != nil {
						t.Fatal(err)
					}
					beforeObject, beforeTree := safetyResource(t, editor), editor.Snapshot()
					target, resolveErr := editor.ResolveWriteTarget(ctx, tree.Target{Component: fixture.component, Field: tree.SchedulerName})
					writeErr := editor.Mutate(ctx, tree.Write{Component: fixture.component, Field: tree.SchedulerName,
						Value: "batch-scheduler", Options: resource.MutationOptions{PatchType: format, Strategy: resource.Merge}})
					if fixture.accessor != tree.SchedulerName {
						for _, err := range []error{resolveErr, writeErr} {
							if err == nil || !strings.Contains(err.Error(), `no field "fragmented.schedulerName"`) {
								t.Fatalf("missing logical-field fallback should fail explicitly, got %v", err)
							}
						}
						t.Logf("current missing capability: %v", resolveErr)
						if !reflect.DeepEqual(beforeObject, safetyResource(t, editor)) || !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
							t.Fatal("unsupported scheduler write changed the workload or extraction")
						}
						return
					}
					if resolveErr != nil || writeErr != nil {
						t.Fatalf("declared scheduler failed: resolve=%v mutate=%v", resolveErr, writeErr)
					}
					if target.Path != fixture.accessorPath || !target.Exists || target.Value != "default-scheduler" {
						t.Fatalf("scheduler target = %+v, want path %q and initial value", target, fixture.accessorPath)
					}
					if err := unstructured.SetNestedField(beforeObject, "batch-scheduler", fixture.schedulerPath...); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(beforeObject, safetyResource(t, editor)) {
						t.Fatal("scheduler write changed neighboring fields")
					}
					updated, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: fixture.component, Field: tree.SchedulerName})
					if err != nil || updated.Value != "batch-scheduler" {
						t.Fatalf("updated scheduler read-back = %+v, error = %v", updated, err)
					}
				})
			}
		})
	}
}

func TestAccessorGapSchedulerViaDeclaredContainerAccessor(t *testing.T) {
	for _, fixture := range accessorGapCases(t) {
		if fixture.accessor == tree.SchedulerName {
			continue
		}
		t.Run(fixture.name, func(t *testing.T) {
			for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
				t.Run(string(format), func(t *testing.T) {
					ctx := context.Background()
					editor, err := tree.Open(ctx, fixture.definition, fixture.workload)
					if err != nil {
						t.Fatal(err)
					}
					target, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: fixture.component, Field: fixture.accessor})
					if err != nil || target.Path != fixture.accessorPath || !target.Exists {
						t.Fatalf("declared accessor target = %+v, error = %v; want %q", target, err, fixture.accessorPath)
					}
					beforeObject := safetyResource(t, editor)
					// The caller knows the standard PodSpec/PodTemplateSpec shape,
					// but does not embed the operator's path in the mutation call.
					input := map[string]any{"schedulerName": "batch-scheduler"}
					if fixture.accessor == tree.PodTemplateSpec {
						input = map[string]any{"spec": input}
					}
					if err := editor.Mutate(ctx, tree.Write{Component: fixture.component, Field: fixture.accessor,
						Value: input, Options: resource.MutationOptions{PatchType: format, Strategy: resource.Merge}}); err != nil {
						t.Fatal(err)
					}
					if err := unstructured.SetNestedField(beforeObject, "batch-scheduler", fixture.schedulerPath...); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(beforeObject, safetyResource(t, editor)) {
						t.Fatal("partial scheduler write changed identity, containers, labels, or other neighboring fields")
					}
					updated, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: fixture.component, Field: fixture.accessor})
					if err != nil {
						t.Fatal(err)
					}
					relativePath := []string{"schedulerName"}
					if fixture.accessor == tree.PodTemplateSpec {
						relativePath = []string{"spec", "schedulerName"}
					}
					got, found, err := unstructured.NestedString(updated.Value.(map[string]any), relativePath...)
					if err != nil || !found || got != "batch-scheduler" {
						t.Fatalf("scheduler read-back = %q, found = %v, error = %v", got, found, err)
					}
				})
			}
		})
	}
}

func TestAccessorGapCanonicalFieldCanHideArbitraryStoragePath(t *testing.T) {
	for _, mode := range []string{"pathWrite", "pathWriteExpression"} {
		t.Run(mode, func(t *testing.T) {
			for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
				t.Run(string(format), func(t *testing.T) {
					definition, workload := kartas.Deployment(), safetyDeployment()
					accessor := &v1alpha1.ValueAccessor{Expression: "object.d.d.c", PathWrite: ptr.To("/d/d/c")}
					if mode == "pathWriteExpression" {
						accessor.PathWrite = nil
						accessor.PathWriteExpression = `"/d/" + object.destination + "/c"`
					}
					definition.Spec.StructureDefinition.RootComponent.SpecDefinition = &v1alpha1.SpecDefinition{
						FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{SchedulerName: accessor},
					}
					workload.Object["destination"] = "d"
					workload.Object["d"] = map[string]any{"d": map[string]any{"c": "default-scheduler", "neighbor": "keep"}}
					ctx := context.Background()
					editor, err := tree.Open(ctx, definition, workload)
					if err != nil {
						t.Fatal(err)
					}
					beforeObject := safetyResource(t, editor)
					target, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: "deployment", Field: tree.SchedulerName})
					if err != nil || target.Path != "/d/d/c" || !target.Exists || target.Value != "default-scheduler" {
						t.Fatalf("canonical field target = %+v, error = %v", target, err)
					}
					if err := editor.Mutate(ctx, tree.Write{Component: "deployment", Field: tree.SchedulerName,
						Value: "batch-scheduler", Options: resource.MutationOptions{PatchType: format, Strategy: resource.Merge}}); err != nil {
						t.Fatal(err)
					}
					if err := unstructured.SetNestedField(beforeObject, "batch-scheduler", "d", "d", "c"); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(beforeObject, safetyResource(t, editor)) {
						t.Fatal("canonical field write changed unrelated data")
					}
					if got := editor.Snapshot().Root.Instances[0].ExtractedInstance.FragmentedPodSpec.SchedulerName; got != "batch-scheduler" {
						t.Fatalf("extracted canonical scheduler = %q, want batch-scheduler", got)
					}
				})
			}
		})
	}
}

// A whole-container write resolves the named main container, not the first item.
func TestAccessorGapDynamoMainContainerWrite(t *testing.T) {
	for _, field := range []tree.Field{tree.Container} {
		t.Run(string(field), func(t *testing.T) {
			for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
				t.Run(string(format), func(t *testing.T) {
					workload := syntheticCatalogInput(t, `{
  "apiVersion": "nvidia.com/v1beta1", "kind": "DynamoGraphDeployment",
  "metadata": {"name": "example-graph"},
  "spec": {"components": [{
    "name": "worker", "replicas": 1,
    "podTemplate": {"spec": {"containers": [
      {"name": "sidecar", "image": "ghcr.io/example/helper:v1"},
      {"name": "main", "image": "ghcr.io/example/worker:v1", "env": [{"name": "MODE", "value": "serve"}]}
    ]}}
  }]}
}`)
					ctx := context.Background()
					editor, err := tree.Open(ctx, kartas.DynamoV1beta1(), workload)
					if err != nil {
						t.Fatal(err)
					}
					beforeTree := editor.Snapshot()
					if len(beforeTree.Children) != 1 || beforeTree.Children[0].Name != "component" || len(beforeTree.Children[0].Instances) != 1 {
						t.Fatalf("unexpected Dynamo component extraction: %#v", beforeTree.Children)
					}
					instance := beforeTree.Children[0].Instances[0]
					if instance.InstanceKey == nil || *instance.InstanceKey != "worker" || instance.ExtractedInstance == nil || instance.ExtractedInstance.FragmentedPodSpec == nil {
						t.Fatalf("unexpected Dynamo worker extraction: %#v", instance)
					}
					container := instance.ExtractedInstance.FragmentedPodSpec.Container
					if container == nil || container.Name != "main" || container.Image != "ghcr.io/example/worker:v1" || len(container.Env) != 1 || container.Env[0].Value != "serve" {
						t.Fatalf("main container was not correctly read before mutation: %#v", container)
					}
					target, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: "component", Instance: "worker", Field: field})
					if err != nil || target.Path != "/spec/components/0/podTemplate/spec/containers/1" || !target.Exists {
						t.Fatalf("main container target = %+v, error = %v", target, err)
					}
					err = editor.Mutate(ctx, tree.Write{Component: "component", Instance: "worker", Field: field,
						Value:   map[string]any{"image": "ghcr.io/example/worker:v2"},
						Options: resource.MutationOptions{PatchType: format, Strategy: resource.Merge},
					})
					if err != nil {
						t.Fatalf("main container write failed: %v", err)
					}
					expected := workload.DeepCopy()
					containers := expected.Object["spec"].(map[string]any)["components"].([]any)[0].(map[string]any)["podTemplate"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)
					containers[1].(map[string]any)["image"] = "ghcr.io/example/worker:v2"
					assertSyntheticCatalogObject(t, editor, expected)
					if container.Image != "ghcr.io/example/worker:v1" {
						t.Fatal("container write changed the previous snapshot")
					}
					updated := editor.Snapshot().Children[0].Instances[0].ExtractedInstance.FragmentedPodSpec
					if updated.Container.Image != "ghcr.io/example/worker:v2" || updated.Image != updated.Container.Image || len(updated.Container.Env) != 1 {
						t.Fatalf("container write was not extracted or removed its environment: %#v", updated)
					}
				})
			}
		})
	}
}
