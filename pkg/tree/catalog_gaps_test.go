// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

// These small synthetic objects cover catalog entries without recorded e2e
// fixtures. They are not operator recordings or deployment-ready manifests.
func syntheticCatalogInput(t *testing.T, input string) *unstructured.Unstructured {
	t.Helper()
	object := &unstructured.Unstructured{}
	if err := json.Unmarshal([]byte(input), &object.Object); err != nil {
		t.Fatal(err)
	}
	return object
}

func syntheticCatalogComponents(t *testing.T, definition *v1alpha1.Karta, snapshot *tree.WorkloadTree) map[string]tree.ComponentNode {
	t.Helper()
	if snapshot.Root == nil || snapshot.Root.Name != definition.Spec.StructureDefinition.RootComponent.Name {
		t.Fatal("catalog root was not extracted")
	}
	if snapshot.Status == nil || !reflect.DeepEqual(snapshot.Status.Phases, []string{"Running"}) {
		t.Fatalf("normalized status = %#v, want Running", snapshot.Status)
	}
	found := make(map[string]tree.ComponentNode)
	var walk func(tree.ComponentNode)
	walk = func(component tree.ComponentNode) {
		found[component.Name] = component
		if len(component.Instances) == 0 {
			t.Fatalf("component %s has no extracted instances", component.Name)
		}
		for _, instance := range component.Instances {
			if instance.ExtractedInstance == nil {
				t.Fatalf("component %s has an empty extraction", component.Name)
			}
			for _, child := range instance.Children {
				walk(child)
			}
		}
	}
	walk(*snapshot.Root)
	expected := append([]v1alpha1.ComponentDefinition{definition.Spec.StructureDefinition.RootComponent}, definition.Spec.StructureDefinition.ChildComponents...)
	if len(found) != len(expected) {
		t.Fatalf("extracted %d components, catalog declares %d", len(found), len(expected))
	}
	for _, component := range expected {
		if _, ok := found[component.Name]; !ok {
			t.Fatalf("catalog component %s is missing", component.Name)
		}
	}
	return found
}

func assertSyntheticCatalogObject(t *testing.T, editor tree.Editable, expected *unstructured.Unstructured) {
	t.Helper()
	actual, err := editor.GetResource()
	if err != nil {
		t.Fatal(err)
	}
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	if string(actualJSON) != string(expectedJSON) {
		t.Fatalf("mutation changed fields outside the expected edits:\n got %s\nwant %s", actualJSON, expectedJSON)
	}
}

func TestSyntheticCatalogNIMCacheReadOnly(t *testing.T) {
	ctx := context.Background()
	definition := kartas.NIMCache()
	workload := syntheticCatalogInput(t, `{
  "apiVersion": "apps.nvidia.com/v1alpha1", "kind": "NIMCache",
  "metadata": {"name": "example-cache"},
  "spec": {"resources": {"cpu": "500m", "memory": "1Gi"}, "extension": "keep"},
  "status": {"state": "InProgress"}
}`)
	editor, err := tree.Open(ctx, definition, workload)
	if err != nil {
		t.Fatal(err)
	}
	nodes := syntheticCatalogComponents(t, definition, editor.Snapshot())
	instance := nodes["nimcache"].Instances[0]
	if instance.InstanceKey != nil || instance.Scale == nil || instance.Scale.Replicas == nil || *instance.Scale.Replicas != 1 {
		t.Fatalf("NIMCache instance or replica extraction = %#v", instance)
	}
	fragment := instance.ExtractedInstance.FragmentedPodSpec
	if fragment == nil || fragment.Resources == nil || fragment.Resources.Requests.Cpu().String() != "500m" || fragment.Resources.Requests.Memory().String() != "1Gi" {
		t.Fatalf("NIMCache resource extraction = %#v", fragment)
	}
	beforeTree := editor.Snapshot()
	for _, field := range []tree.Field{tree.Resources, tree.Replicas} {
		_, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: "nimcache", Field: field})
		if err == nil || !strings.Contains(err.Error(), "read-only") {
			t.Fatalf("%s should have no writable path, got %v", field, err)
		}
		err = editor.Mutate(ctx, tree.Write{Component: "nimcache", Field: field, Value: 2})
		if err == nil || !strings.Contains(err.Error(), "read-only") {
			t.Fatalf("read-only %s write was not rejected: %v", field, err)
		}
	}
	if !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
		t.Fatal("rejected writes changed the extracted tree")
	}
	assertSyntheticCatalogObject(t, editor, workload)
}

func TestSyntheticCatalogDynamoV1beta1(t *testing.T) {
	for _, patchType := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		t.Run(string(patchType), func(t *testing.T) {
			ctx := context.Background()
			definition := kartas.DynamoV1beta1()
			workload := syntheticCatalogInput(t, `{
  "apiVersion": "nvidia.com/v1beta1", "kind": "DynamoGraphDeployment",
  "metadata": {"name": "example-graph"},
  "spec": {"extension": "keep", "components": [
    {"name": "worker", "replicas": 2, "multinode": {"nodeCount": 3}, "extension": "keep",
     "podTemplate": {
       "metadata": {"labels": {"team": "ml", "owner": "alice"}, "annotations": {"example.com/note": "keep"}},
       "spec": {
         "schedulerName": "default-scheduler", "priorityClassName": "batch",
         "resourceClaims": [{"name": "gpu", "resourceClaimName": "example-claim"}],
         "affinity": {
           "podAffinity": {"requiredDuringSchedulingIgnoredDuringExecution": [{"topologyKey": "kubernetes.io/hostname", "labelSelector": {"matchLabels": {"app": "backend"}}}]},
           "nodeAffinity": {"requiredDuringSchedulingIgnoredDuringExecution": {"nodeSelectorTerms": [{"matchExpressions": [{"key": "region", "operator": "In", "values": ["west"]}]}]}}
         },
         "containers": [{"name": "sidecar", "image": "ghcr.io/example/helper:v1"}, {"name": "main", "image": "ghcr.io/example/worker:v1", "env": [{"name": "MODE", "value": "serve"}]}]
       }
     }},
    {"name": "frontend", "replicas": 1, "podTemplate": {"spec": {"schedulerName": "default-scheduler", "containers": [{"name": "main", "image": "ghcr.io/example/frontend:v1"}]}}}
  ]},
  "status": {"state": "successful"}
}`)
			editor, err := tree.Open(ctx, definition, workload)
			if err != nil {
				t.Fatal(err)
			}
			nodes := syntheticCatalogComponents(t, definition, editor.Snapshot())
			instances := nodes["component"].Instances
			if len(instances) != 2 || instances[0].InstanceKey == nil || *instances[0].InstanceKey != "frontend" || instances[1].InstanceKey == nil || *instances[1].InstanceKey != "worker" {
				t.Fatalf("Dynamo instance extraction = %#v", instances)
			}
			worker := instances[1]
			if worker.Scale == nil || worker.Scale.Replicas == nil || *worker.Scale.Replicas != 6 {
				t.Fatalf("worker replicas = %#v, want 2 * 3", worker.Scale)
			}
			fragment := worker.ExtractedInstance.FragmentedPodSpec
			if fragment == nil || fragment.SchedulerName != "default-scheduler" || fragment.PriorityClassName != "batch" || fragment.Image != "ghcr.io/example/worker:v1" {
				t.Fatalf("Dynamo scalar extraction = %#v", fragment)
			}
			if fragment.Container == nil || fragment.Container.Name != "main" || fragment.Container.Image != fragment.Image || len(fragment.Container.Env) != 1 || fragment.Container.Env[0].Value != "serve" {
				t.Fatalf("named main container was not extracted: %#v", fragment.Container)
			}
			if fragment.Labels["owner"] != "alice" || fragment.Annotations["example.com/note"] != "keep" || len(fragment.ResourceClaims) != 1 || fragment.ResourceClaims[0].ResourceClaimName == nil || *fragment.ResourceClaims[0].ResourceClaimName != "example-claim" {
				t.Fatalf("Dynamo metadata or resource-claim extraction = %#v", fragment)
			}
			if fragment.PodAffinity == nil || len(fragment.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution) != 1 || fragment.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey != "kubernetes.io/hostname" {
				t.Fatalf("pod affinity was not extracted: %#v", fragment.PodAffinity)
			}
			if fragment.NodeAffinity == nil || fragment.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil || len(fragment.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) != 1 || fragment.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0] != "west" {
				t.Fatalf("node affinity was not extracted: %#v", fragment.NodeAffinity)
			}
			for field, suffix := range map[tree.Field]string{
				tree.SchedulerName: "spec/schedulerName", tree.LabelsField: "metadata/labels", tree.Annotations: "metadata/annotations",
				tree.ResourceClaims: "spec/resourceClaims", tree.PodAffinity: "spec/affinity/podAffinity", tree.NodeAffinity: "spec/affinity/nodeAffinity", tree.PriorityClassName: "spec/priorityClassName",
				tree.Image: "spec/containers/1/image", tree.Container: "spec/containers/1",
			} {
				target, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: "component", Instance: "worker", Field: field})
				if err != nil || !target.Exists || target.Path != "/spec/components/0/podTemplate/"+suffix {
					t.Fatalf("%s target = %+v, error = %v", field, target, err)
				}
			}
			options := resource.MutationOptions{PatchType: patchType, Strategy: resource.Merge}
			if err := editor.Mutate(ctx,
				tree.Write{Component: "component", Instance: "worker", Field: tree.SchedulerName, Value: "batch-scheduler", Options: options},
				tree.Write{Component: "component", Instance: "worker", Field: tree.LabelsField, Value: map[string]any{"team": "platform"}, Options: options},
				tree.Write{Component: "component", Instance: "worker", Field: tree.Image, Value: "ghcr.io/example/worker:v2", Options: options},
			); err != nil {
				t.Fatal(err)
			}
			after := syntheticCatalogComponents(t, definition, editor.Snapshot())["component"].Instances[1].ExtractedInstance.FragmentedPodSpec
			if after.SchedulerName != "batch-scheduler" || after.Labels["team"] != "platform" || after.Labels["owner"] != "alice" || after.Image != "ghcr.io/example/worker:v2" || after.Container == nil || after.Container.Image != after.Image {
				t.Fatalf("Dynamo mutation read-back = %#v", after)
			}
			expected := workload.DeepCopy()
			podTemplate := expected.Object["spec"].(map[string]any)["components"].([]any)[0].(map[string]any)["podTemplate"].(map[string]any)
			podTemplate["spec"].(map[string]any)["schedulerName"] = "batch-scheduler"
			podTemplate["metadata"].(map[string]any)["labels"].(map[string]any)["team"] = "platform"
			podTemplate["spec"].(map[string]any)["containers"].([]any)[1].(map[string]any)["image"] = "ghcr.io/example/worker:v2"
			assertSyntheticCatalogObject(t, editor, expected)
		})
	}
}

func TestSyntheticCatalogRayService(t *testing.T) {
	for _, patchType := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		t.Run(string(patchType), func(t *testing.T) {
			ctx := context.Background()
			definition := kartas.RayService()
			workload := syntheticCatalogInput(t, `{
  "apiVersion": "ray.io/v1", "kind": "RayService",
  "metadata": {"name": "example-service"},
  "spec": {"extension": "keep", "rayClusterConfig": {
    "headGroupSpec": {"template": {"metadata": {"labels": {"owner": "alice"}}, "spec": {"containers": [{"name": "head", "image": "ghcr.io/example/ray:v1"}]}}},
    "workerGroupSpecs": [
      {"groupName": "gpu", "replicas": 2, "template": {"extension": "keep", "metadata": {"labels": {"team": "ml"}}, "spec": {"schedulerName": "default-scheduler", "containers": [{"name": "worker", "image": "ghcr.io/example/ray:v1"}]}}},
      {"groupName": "cpu", "replicas": 3, "template": {"spec": {"schedulerName": "default-scheduler", "containers": [{"name": "worker", "image": "ghcr.io/example/ray:v1"}]}}}
    ]
  }},
  "status": {"conditions": [{"type": "Ready", "status": "True"}]}
}`)
			editor, err := tree.Open(ctx, definition, workload)
			if err != nil {
				t.Fatal(err)
			}
			nodes := syntheticCatalogComponents(t, definition, editor.Snapshot())
			head := nodes["head"].Instances[0]
			if head.InstanceKey != nil || head.Scale == nil || head.Scale.Replicas == nil || *head.Scale.Replicas != 1 || head.ExtractedInstance.PodTemplateSpec == nil || head.ExtractedInstance.PodTemplateSpec.Spec.Containers[0].Name != "head" {
				t.Fatalf("RayService head extraction = %#v", head)
			}
			workers := nodes["worker"].Instances
			if len(workers) != 2 || workers[0].InstanceKey == nil || *workers[0].InstanceKey != "cpu" || workers[1].InstanceKey == nil || *workers[1].InstanceKey != "gpu" {
				t.Fatalf("RayService worker extraction = %#v", workers)
			}
			for i, replicas := range []int32{3, 2} {
				instance := workers[i]
				if instance.Scale == nil || instance.Scale.Replicas == nil || *instance.Scale.Replicas != replicas || instance.ExtractedInstance.PodTemplateSpec == nil || instance.ExtractedInstance.PodTemplateSpec.Spec.SchedulerName != "default-scheduler" || instance.ExtractedInstance.PodTemplateSpec.Spec.Containers[0].Image != "ghcr.io/example/ray:v1" {
					t.Fatalf("RayService worker %d extraction = %#v", i, instance)
				}
			}
			for selector, path := range map[tree.Target]string{
				{Component: "head", Field: tree.PodTemplateSpec}:                    "/spec/rayClusterConfig/headGroupSpec/template",
				{Component: "worker", Instance: "gpu", Field: tree.PodTemplateSpec}: "/spec/rayClusterConfig/workerGroupSpecs/0/template",
				{Component: "worker", Instance: "cpu", Field: tree.PodTemplateSpec}: "/spec/rayClusterConfig/workerGroupSpecs/1/template",
			} {
				target, err := editor.ResolveWriteTarget(ctx, selector)
				if err != nil || !target.Exists || target.Path != path {
					t.Fatalf("%+v target = %+v, error = %v", selector, target, err)
				}
			}
			options := resource.MutationOptions{PatchType: patchType, Strategy: resource.Merge}
			if err := editor.Mutate(ctx,
				tree.Write{Component: "head", Field: tree.PodTemplateSpec, Value: map[string]any{"metadata": map[string]any{"labels": map[string]any{"team": "platform"}}}, Options: options},
				tree.Write{Component: "worker", Instance: "gpu", Field: tree.PodTemplateSpec, Value: map[string]any{"spec": map[string]any{"schedulerName": "batch-scheduler"}}, Options: options},
			); err != nil {
				t.Fatal(err)
			}
			after := syntheticCatalogComponents(t, definition, editor.Snapshot())
			headLabels := after["head"].Instances[0].ExtractedInstance.PodTemplateSpec.Labels
			if headLabels["team"] != "platform" || headLabels["owner"] != "alice" || after["worker"].Instances[1].ExtractedInstance.PodTemplateSpec.Spec.SchedulerName != "batch-scheduler" || after["worker"].Instances[0].ExtractedInstance.PodTemplateSpec.Spec.SchedulerName != "default-scheduler" {
				t.Fatal("RayService mutation was not extracted on the selected head/gpu instances")
			}
			expected := workload.DeepCopy()
			config := expected.Object["spec"].(map[string]any)["rayClusterConfig"].(map[string]any)
			config["headGroupSpec"].(map[string]any)["template"].(map[string]any)["metadata"].(map[string]any)["labels"].(map[string]any)["team"] = "platform"
			config["workerGroupSpecs"].([]any)[0].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["schedulerName"] = "batch-scheduler"
			assertSyntheticCatalogObject(t, editor, expected)
		})
	}
}
