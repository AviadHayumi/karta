// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/references"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

func safetyDeployment() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": "example-api"},
		"spec": map[string]any{
			"replicas": int64(3),
			"template": map[string]any{
				"metadata": map[string]any{"labels": map[string]any{"team": "ml", "owner": "alice"}},
				"spec": map[string]any{
					"schedulerName": "default-scheduler",
					"containers": []any{map[string]any{
						"name": "api", "image": "ghcr.io/example/api:v1",
						"env": []any{map[string]any{"name": "MODE", "value": "production"}},
					}},
				},
			},
		},
	}}
}

func safetyResource(t *testing.T, editor tree.Editable) map[string]any {
	t.Helper()
	object, err := editor.GetResource()
	if err != nil {
		t.Fatal(err)
	}
	return object.(*unstructured.Unstructured).Object
}

func TestEditableSafetyOpenInputs(t *testing.T) {
	t.Run("typed nil workload", func(t *testing.T) {
		var workload *unstructured.Unstructured
		if _, err := tree.Open(context.Background(), kartas.Deployment(), workload); err == nil {
			t.Fatal("typed nil workload was accepted")
		}
	})
	for _, number := range []any{int(3), int32(3), int64(3)} {
		t.Run(fmt.Sprintf("unstructured %T", number), func(t *testing.T) {
			defer func() {
				if caught := recover(); caught != nil {
					t.Fatalf("Open panicked for JSON-serializable Go number %T: %v", number, caught)
				}
			}()
			workload := safetyDeployment()
			workload.Object["spec"].(map[string]any)["replicas"] = number
			editor, err := tree.Open(context.Background(), kartas.Deployment(), workload)
			if err != nil {
				t.Fatal(err)
			}
			if got := *editor.Snapshot().Root.Instances[0].Scale.Replicas; got != 3 {
				t.Fatalf("replicas = %d, want 3", got)
			}
		})
	}
}

func TestEditableSafetySnapshotsAndInputsAreDetached(t *testing.T) {
	ctx := context.Background()
	definition, workload := kartas.Deployment(), safetyDeployment()
	editor, err := tree.Open(ctx, definition, workload)
	if err != nil {
		t.Fatal(err)
	}
	beforeObject, beforeTree := safetyResource(t, editor), editor.Snapshot()
	definition.Spec.StructureDefinition.RootComponent.SpecDefinition.PodTemplateSpec.PathWrite = ptr.To("/wrong")
	workload.Object["spec"].(map[string]any)["replicas"] = int64(999)
	snapshot := editor.Snapshot()
	snapshot.Root.Kind.Kind = "Changed"
	*snapshot.Root.Instances[0].Scale.Replicas = 999
	*snapshot.Root.Instances[0].ExtractedInstance.Scale.Replicas = 999
	template := snapshot.Root.Instances[0].ExtractedInstance.PodTemplateSpec
	template.Labels["owner"] = "changed"
	template.Spec.Containers[0].Env[0].Value = "changed"
	snapshot.Children[0].Name = "changed"
	returned := safetyResource(t, editor)
	returned["metadata"].(map[string]any)["name"] = "changed"
	target, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: "deployment", Field: tree.PodTemplateSpec})
	if err != nil {
		t.Fatal(err)
	}
	if target.Path != "/spec/template" {
		t.Fatalf("caller definition mutation changed target: %+v", target)
	}
	target.Value.(map[string]any)["spec"].(map[string]any)["schedulerName"] = "changed"
	if !reflect.DeepEqual(beforeObject, safetyResource(t, editor)) || !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
		t.Fatal("mutating a caller input, returned object, target, or snapshot leaked into the editor")
	}
}

func TestEditableSafetyFailedWritesRollbackObjectAndExtraction(t *testing.T) {
	for _, mode := range []string{"later invalid option", "invalid extraction", "overlapping paths"} {
		t.Run(mode, func(t *testing.T) {
			definition := kartas.Deployment()
			definition.Spec.StructureDefinition.RootComponent.ScaleDefinition.Replicas.PathWrite = ptr.To("/spec/replicas")
			editor, err := tree.Open(context.Background(), definition, safetyDeployment())
			if err != nil {
				t.Fatal(err)
			}
			beforeObject, beforeTree := safetyResource(t, editor), editor.Snapshot()
			writes := []tree.Write{{Component: "deployment", Field: tree.PodTemplateSpec,
				Value: map[string]any{"spec": map[string]any{"schedulerName": "changed"}}}}
			switch mode {
			case "later invalid option":
				writes = append(writes, tree.Write{Component: "deployment", Field: tree.Replicas, Value: 8,
					Options: resource.MutationOptions{PatchType: "invalid"}})
			case "invalid extraction":
				writes[0].Value = map[string]any{"spec": map[string]any{"containers": "not an array"}}
			case "overlapping paths":
				writes = append(writes, writes[0])
			}
			if err := editor.Mutate(context.Background(), writes...); err == nil {
				t.Fatal("invalid mutation was accepted")
			}
			if !reflect.DeepEqual(beforeObject, safetyResource(t, editor)) || !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
				t.Fatal("failed transaction published partial data or changed the extracted tree")
			}
		})
	}
}

func TestEditableSafetyRootReplacementDoesNotPublishInvalidWorkload(t *testing.T) {
	object := safetyDeployment()
	object.SetAPIVersion("v1")
	object.SetKind("Pod")
	object.Object["spec"] = object.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"]
	editor, err := tree.Open(context.Background(), kartas.Pod(), object)
	if err != nil {
		t.Fatal(err)
	}
	before := safetyResource(t, editor)
	err = editor.Mutate(context.Background(), tree.Write{Component: "pod", Field: tree.PodTemplateSpec,
		Value:   map[string]any{"spec": map[string]any{"schedulerName": "changed"}},
		Options: resource.MutationOptions{Strategy: resource.Replace},
	})
	if err == nil || !strings.Contains(err.Error(), "missing apiVersion") {
		t.Fatalf("expected root replacement validation error, got %v", err)
	}
	if !reflect.DeepEqual(before, safetyResource(t, editor)) {
		t.Fatal("invalid replacement was published")
	}
}

func safetyDynamicDefinition() (*v1alpha1.Karta, *unstructured.Unstructured) {
	definition := kartas.Deployment()
	definition.Spec.StructureDefinition.ChildComponents = nil
	definition.Spec.StructureDefinition.RootComponent.ScaleDefinition = nil
	definition.Spec.StructureDefinition.RootComponent.SpecDefinition = &v1alpha1.SpecDefinition{
		FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
			SchedulerName: &v1alpha1.ValueAccessor{Expression: "object.spec.destination", PathWrite: ptr.To("/spec/destination")},
			Image: &v1alpha1.ValueAccessor{Expression: "object.spec[object.spec.destination].image",
				PathWriteExpression: `"/spec/" + object.spec.destination + "/image"`},
		},
	}
	object := safetyDeployment()
	object.Object["spec"] = map[string]any{
		"destination": "left",
		"left":        map[string]any{"image": "api:left"},
		"right":       map[string]any{"image": "api:right"},
	}
	return definition, object
}

func TestEditableSafetyDestinationsFreezeBeforeFirstWrite(t *testing.T) {
	definition, object := safetyDynamicDefinition()
	editor, err := tree.Open(context.Background(), definition, object)
	if err != nil {
		t.Fatal(err)
	}
	if err := editor.Mutate(context.Background(),
		tree.Write{Component: "deployment", Field: tree.SchedulerName, Value: "right"},
		tree.Write{Component: "deployment", Field: tree.Image, Value: "api:new"},
	); err != nil {
		t.Fatal(err)
	}
	spec := safetyResource(t, editor)["spec"].(map[string]any)
	if spec["left"].(map[string]any)["image"] != "api:new" || spec["right"].(map[string]any)["image"] != "api:right" {
		t.Fatalf("the first write redirected a later target: %#v", spec)
	}
	target, err := editor.ResolveWriteTarget(context.Background(), tree.Target{Component: "deployment", Field: tree.Image})
	if err != nil || target.Path != "/spec/right/image" {
		t.Fatalf("fresh resolution should use updated selector, got %+v, %v", target, err)
	}
	if got := editor.Snapshot().Root.Instances[0].ExtractedInstance.FragmentedPodSpec.Image; got != "api:right" {
		t.Fatalf("snapshot was not re-extracted after selector change: %q", got)
	}
}

type safetyReferenceReader struct {
	object *unstructured.Unstructured
	reads  int
}

func (r *safetyReferenceReader) Get(context.Context, schema.GroupVersionKind, string, string) (*unstructured.Unstructured, error) {
	r.reads++
	return r.object, nil
}

func (*safetyReferenceReader) List(context.Context, schema.GroupVersionKind, references.ListQuery) ([]unstructured.Unstructured, error) {
	return nil, nil
}

func TestEditableSafetyReferenceTargetDoesNotDriftAcrossTransactions(t *testing.T) {
	definition, object := safetyDynamicDefinition()
	definition.Spec.StructureDefinition.References = []v1alpha1.ResourceReference{{
		Name: "config", GVK: v1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
		Lookup: &v1alpha1.LookupReference{NameExpression: "'example-config'"},
	}}
	definition.Spec.StructureDefinition.RootComponent.SpecDefinition.FragmentedPodSpecDefinition.Image.PathWriteExpression = `"/spec/" + references.config.data.target + "/image"`
	reader := &safetyReferenceReader{object: &unstructured.Unstructured{Object: map[string]any{"data": map[string]any{"target": "left"}}}}
	editor, err := tree.Open(context.Background(), definition, object, resource.WithReferenceReader(reader))
	if err != nil {
		t.Fatal(err)
	}
	if reader.reads != 0 {
		t.Fatal("Open fetched references that no read expression needs")
	}
	if _, err := editor.ResolveWriteTarget(context.Background(), tree.Target{Component: "deployment", Field: tree.Image}); err != nil {
		t.Fatal(err)
	}
	reader.object.Object["data"].(map[string]any)["target"] = "right"
	for _, image := range []string{"api:v2", "api:v3"} {
		if err := editor.Mutate(context.Background(), tree.Write{Component: "deployment", Field: tree.Image, Value: image}); err != nil {
			t.Fatal(err)
		}
	}
	spec := safetyResource(t, editor)["spec"].(map[string]any)
	if reader.reads != 1 || spec["left"].(map[string]any)["image"] != "api:v3" || spec["right"].(map[string]any)["image"] != "api:right" {
		t.Fatalf("reference snapshot drifted: reads=%d, spec=%#v", reader.reads, spec)
	}
}

func TestEditableSafetyFactoryOptionsAndNullSemantics(t *testing.T) {
	for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		t.Run(string(format), func(t *testing.T) {
			definition := kartas.Deployment()
			definition.Spec.StructureDefinition.RootComponent.ScaleDefinition.Replicas.PathWrite = ptr.To("/spec/replicas")
			editor, err := tree.Open(context.Background(), definition, safetyDeployment(), resource.WithMutationOptions(resource.MutationOptions{
				PatchType: format, Strategy: resource.Replace, Parents: resource.RequireParents,
			}))
			if err != nil {
				t.Fatal(err)
			}
			if err := editor.Mutate(context.Background(), tree.Write{Component: "deployment", Field: tree.Replicas, Value: nil}); err != nil {
				t.Fatal(err)
			}
			spec := safetyResource(t, editor)["spec"].(map[string]any)
			value, exists := spec["replicas"]
			if exists != (format == resource.PatchTypeJSONPatch) || value != nil {
				t.Fatalf("factory patch format lost: replicas value=%v exists=%v", value, exists)
			}
			// Override only strategy: the selected format/parent policy stay inherited.
			if err := editor.Mutate(context.Background(), tree.Write{Component: "deployment", Field: tree.PodTemplateSpec,
				Value:   map[string]any{"metadata": map[string]any{"labels": map[string]any{"team": "platform"}}},
				Options: resource.MutationOptions{Strategy: resource.Merge},
			}); err != nil {
				t.Fatal(err)
			}
			if got := editor.Snapshot().Root.Instances[0].ExtractedInstance.PodTemplateSpec.Labels["owner"]; got != "alice" {
				t.Fatal("per-write Merge override did not preserve the other label")
			}
		})
	}
}

func TestEditableSafetyConcurrentReadsAndWrites(t *testing.T) {
	editor, err := tree.Open(context.Background(), kartas.Deployment(), safetyDeployment())
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for i := 0; i < 12; i++ {
		workers.Go(func() {
			if err := editor.Mutate(context.Background(), tree.Write{Component: "deployment", Field: tree.PodTemplateSpec,
				Value: map[string]any{"spec": map[string]any{"schedulerName": "batch"}},
			}); err != nil {
				t.Error(err)
			}
			_ = editor.Snapshot()
			if _, err := editor.GetResource(); err != nil {
				t.Error(err)
			}
			if _, err := editor.ResolveWriteTarget(context.Background(), tree.Target{Component: "deployment", Field: tree.PodTemplateSpec}); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
}

func TestEditableSafetyUnsupportedSuspensionAndCanceledWrite(t *testing.T) {
	editor, err := tree.Open(context.Background(), kartas.Deployment(), safetyDeployment())
	if err != nil {
		t.Fatal(err)
	}
	if editor.IsSuspendable() {
		t.Fatal("Deployment incorrectly advertises native suspension")
	}
	if err := editor.Suspend(context.Background()); !errors.Is(err, tree.ErrNotSuspendable) {
		t.Fatalf("unsupported Suspend returned %v", err)
	}
	if err := editor.Resume(context.Background()); !errors.Is(err, tree.ErrNotSuspendable) {
		t.Fatalf("unsupported Resume returned %v", err)
	}
	before := safetyResource(t, editor)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := editor.Mutate(ctx, tree.Write{Component: "deployment", Field: tree.PodTemplateSpec, Value: nil}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled write returned %v", err)
	}
	if !reflect.DeepEqual(before, safetyResource(t, editor)) {
		t.Fatal("canceled write changed the workload")
	}
}

func TestEditableSafetyAncestorOverlapIsRejected(t *testing.T) {
	definition, object := safetyDynamicDefinition()
	fragment := definition.Spec.StructureDefinition.RootComponent.SpecDefinition.FragmentedPodSpecDefinition
	fragment.Container = &v1alpha1.ValueAccessor{Expression: "object.spec.left", PathWrite: ptr.To("/spec/left")}
	editor, err := tree.Open(context.Background(), definition, object)
	if err != nil {
		t.Fatal(err)
	}
	before := safetyResource(t, editor)
	err = editor.Mutate(context.Background(),
		tree.Write{Component: "deployment", Field: tree.Container, Value: map[string]any{"image": "api:v2"}},
		tree.Write{Component: "deployment", Field: tree.Image, Value: "api:v3"},
	)
	if err == nil || !strings.Contains(err.Error(), "overlapping write targets") {
		t.Fatalf("ancestor and child targets were not rejected: %v", err)
	}
	if !reflect.DeepEqual(before, safetyResource(t, editor)) {
		t.Fatal("overlapping writes changed the workload")
	}
}

func TestEditableSafetyEscapedLiteralKeyDoesNotOverlapNestedKey(t *testing.T) {
	definition, object := safetyDynamicDefinition()
	fragment := definition.Spec.StructureDefinition.RootComponent.SpecDefinition.FragmentedPodSpecDefinition
	fragment.SchedulerName = &v1alpha1.ValueAccessor{Expression: `object.spec.keys["acme.io/team"]`, PathWrite: ptr.To("/spec/keys/acme.io~1team")}
	fragment.Image = &v1alpha1.ValueAccessor{Expression: `object.spec.keys["acme.io"].team`, PathWrite: ptr.To("/spec/keys/acme.io/team")}
	object.Object["spec"] = map[string]any{"keys": map[string]any{
		"acme.io/team": "literal-old", "acme.io": map[string]any{"team": "nested-old"},
	}}
	editor, err := tree.Open(context.Background(), definition, object)
	if err != nil {
		t.Fatal(err)
	}
	if err := editor.Mutate(context.Background(),
		tree.Write{Component: "deployment", Field: tree.SchedulerName, Value: "literal-new"},
		tree.Write{Component: "deployment", Field: tree.Image, Value: "nested-new"},
	); err != nil {
		t.Fatalf("distinct literal-slash and nested keys were treated as overlapping: %v", err)
	}
	keys := safetyResource(t, editor)["spec"].(map[string]any)["keys"].(map[string]any)
	if keys["acme.io/team"] != "literal-new" || keys["acme.io"].(map[string]any)["team"] != "nested-new" {
		t.Fatalf("incorrect escaped destination writes: %#v", keys)
	}
}

func TestEditableSafetyDuplicateInstanceIDsAreRejected(t *testing.T) {
	object := safetyDeployment()
	object.SetAPIVersion("ray.io/v1")
	object.SetKind("RayCluster")
	object.Object["spec"] = map[string]any{"workerGroupSpecs": []any{
		map[string]any{"groupName": "gpu"},
		map[string]any{"groupName": "gpu"},
	}}
	if _, err := tree.Open(context.Background(), kartas.Raycluster(), object); err == nil || !strings.Contains(err.Error(), "duplicate instance id") {
		t.Fatalf("duplicate stable instance IDs were not rejected: %v", err)
	}
}

func TestEditableSafetyFragmentedSnapshotsAreDeeplyDetached(t *testing.T) {
	definition, object := safetyDynamicDefinition()
	definition.Spec.StructureDefinition.RootComponent.SpecDefinition.FragmentedPodSpecDefinition = &v1alpha1.FragmentedPodSpecDefinition{
		Resources:      &v1alpha1.ValueAccessor{Expression: "object.spec.resources"},
		ResourceClaims: &v1alpha1.ValueAccessor{Expression: "object.spec.resourceClaims"},
		Containers:     &v1alpha1.ValueAccessor{Expression: "object.spec.containers"},
		Container:      &v1alpha1.ValueAccessor{Expression: "object.spec.containers[0]"},
		PodAffinity:    &v1alpha1.ValueAccessor{Expression: "object.spec.podAffinity"},
		NodeAffinity:   &v1alpha1.ValueAccessor{Expression: "object.spec.nodeAffinity"},
	}
	object.Object["spec"] = map[string]any{
		"resources":      map[string]any{"requests": map[string]any{"cpu": "123456789012345678901", "memory": "2Gi"}},
		"resourceClaims": []any{map[string]any{"name": "gpu", "resourceClaimName": "example-gpu-claim"}},
		"containers": []any{map[string]any{
			"name": "api", "image": "api:v1",
			"env": []any{map[string]any{"name": "POD_NAME", "valueFrom": map[string]any{"fieldRef": map[string]any{"fieldPath": "metadata.name"}}}},
		}},
		"podAffinity": map[string]any{"requiredDuringSchedulingIgnoredDuringExecution": []any{map[string]any{
			"topologyKey": "kubernetes.io/hostname", "labelSelector": map[string]any{"matchLabels": map[string]any{"app": "api"}},
		}}},
		"nodeAffinity": map[string]any{"requiredDuringSchedulingIgnoredDuringExecution": map[string]any{"nodeSelectorTerms": []any{map[string]any{
			"matchExpressions": []any{map[string]any{"key": "example.com/pool", "operator": "In", "values": []any{"gpu"}}},
		}}}},
	}
	editor, err := tree.Open(context.Background(), definition, object)
	if err != nil {
		t.Fatal(err)
	}
	before := editor.Snapshot()
	fragment := editor.Snapshot().Root.Instances[0].ExtractedInstance.FragmentedPodSpec
	cpu := fragment.Resources.Requests[corev1.ResourceCPU]
	cpu.AsDec().SetUnscaled(999)
	fragment.Resources.Requests[corev1.ResourceCPU] = cpu
	*fragment.ResourceClaims[0].ResourceClaimName = "changed"
	fragment.Containers[0].Env[0].ValueFrom.FieldRef.FieldPath = "changed"
	fragment.Container.Env[0].ValueFrom.FieldRef.FieldPath = "changed"
	fragment.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchLabels["app"] = "changed"
	fragment.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0] = "changed"
	if !reflect.DeepEqual(before, editor.Snapshot()) {
		t.Fatal("nested quantity, claim pointer, container env, or affinity data leaked through Snapshot")
	}
}
