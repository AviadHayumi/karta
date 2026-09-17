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

func namedFieldsFixture(value any) (*v1alpha1.Karta, *unstructured.Unstructured) {
	definition, workload := kartas.Deployment(), safetyDeployment()
	definition.Spec.Variables = nil
	definition.Spec.StructureDefinition.ChildComponents = nil
	root := &definition.Spec.StructureDefinition.RootComponent
	root.SpecDefinition, root.ScaleDefinition = nil, nil
	root.StatusDefinition = &v1alpha1.StatusDefinition{}
	root.Fields = map[string]v1alpha1.ValueAccessor{
		"exampleos": {Expression: `object.d.d[?"c"].orValue(null)`, PathWrite: ptr.To("/d/d/c")},
	}
	workload.Object["d"] = map[string]any{"d": map[string]any{"c": value, "neighbor": "keep"}, "neighbor": "keep"}
	return definition, workload
}

func TestNamedFieldsWritePoliciesAndValues(t *testing.T) {
	for _, format := range []resource.PatchType{resource.PatchTypeJSONPatch, resource.PatchTypeMergePatch} {
		for _, strategy := range []resource.MergeStrategy{resource.Merge, resource.Replace} {
			for _, test := range []struct {
				name   string
				before any
				write  any
				merged any
			}{
				{name: "string", before: "old", write: "new", merged: "new"},
				{name: "boolean", before: false, write: true, merged: true},
				{name: "number", before: float64(1), write: float64(2), merged: float64(2)},
				{name: "map", before: map[string]any{"old": "keep", "nested": map[string]any{"left": "keep", "right": "old"}},
					write:  map[string]any{"nested": map[string]any{"right": "new"}},
					merged: map[string]any{"old": "keep", "nested": map[string]any{"left": "keep", "right": "new"}}},
				{name: "list", before: []any{"old", "removed"}, write: []any{"new"}, merged: []any{"new"}},
				{name: "null", before: "old", write: nil, merged: nil},
			} {
				t.Run(string(format)+"/"+string(strategy)+"/"+test.name, func(t *testing.T) {
					ctx := context.Background()
					definition, workload := namedFieldsFixture(test.before)
					editor, err := tree.Open(ctx, definition, workload)
					if err != nil {
						t.Fatal(err)
					}
					target, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: "deployment", Field: tree.Field("exampleos")})
					if err != nil || target.Path != "/d/d/c" || !target.Exists || !reflect.DeepEqual(target.Value, test.before) {
						t.Fatalf("initial named target = %+v, error = %v", target, err)
					}
					before := safetyResource(t, editor)
					if err := editor.Mutate(ctx, tree.Write{Component: "deployment", Field: tree.Field("exampleos"), Value: test.write,
						Options: resource.MutationOptions{PatchType: format, Strategy: strategy}}); err != nil {
						t.Fatal(err)
					}
					want := test.write
					if strategy == resource.Merge {
						want = test.merged
					}
					exists := test.write != nil || format == resource.PatchTypeJSONPatch
					if exists {
						if err := unstructured.SetNestedField(before, want, "d", "d", "c"); err != nil {
							t.Fatal(err)
						}
					} else {
						unstructured.RemoveNestedField(before, "d", "d", "c")
					}
					if !reflect.DeepEqual(before, safetyResource(t, editor)) {
						t.Fatal("named-field write changed values outside its selected subtree or used the wrong merge strategy")
					}
					target, err = editor.ResolveWriteTarget(ctx, tree.Target{Component: "deployment", Field: tree.Field("exampleos")})
					if err != nil || target.Exists != exists || !reflect.DeepEqual(target.Value, want) {
						t.Fatalf("updated named target = %+v, error = %v; want %#v, exists %v", target, err, want, exists)
					}
					got, found := editor.Snapshot().Root.Instances[0].ExtractedInstance.Fields["exampleos"]
					if !found || !reflect.DeepEqual(got, want) {
						t.Fatalf("refreshed named field = %#v, found = %v; want %#v", got, found, want)
					}
				})
			}
		}
	}
}

func TestNamedFieldsSnapshotsAndInputsAreDetached(t *testing.T) {
	ctx := context.Background()
	definition, workload := namedFieldsFixture(map[string]any{"items": []any{map[string]any{"name": "original"}}})
	editor, err := tree.Open(ctx, definition, workload)
	if err != nil {
		t.Fatal(err)
	}
	beforeObject, beforeTree := safetyResource(t, editor), editor.Snapshot()
	via := definition.Spec.StructureDefinition.RootComponent.Fields["exampleos"]
	*via.PathWrite = "/wrong"
	definition.Spec.StructureDefinition.RootComponent.Fields["exampleos"] = v1alpha1.ValueAccessor{Expression: `"wrong"`}
	workload.Object["d"].(map[string]any)["d"].(map[string]any)["c"] = "wrong"
	snapshot := editor.Snapshot()
	snapshot.Root.Instances[0].ExtractedInstance.Fields["exampleos"].(map[string]any)["items"].([]any)[0].(map[string]any)["name"] = "wrong"
	snapshot.Root.Instances[0].ExtractedInstance.Fields["added"] = "wrong"
	target, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: "deployment", Field: tree.Field("exampleos")})
	if err != nil || target.Path != "/d/d/c" {
		t.Fatalf("caller definition changed target: %+v, error = %v", target, err)
	}
	target.Value.(map[string]any)["items"].([]any)[0].(map[string]any)["name"] = "wrong"
	if !reflect.DeepEqual(beforeObject, safetyResource(t, editor)) || !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
		t.Fatal("caller-owned named data leaked into the editor")
	}
	input := map[string]any{"items": []any{map[string]any{"name": "updated"}}}
	if err := editor.Mutate(ctx, tree.Write{Component: "deployment", Field: tree.Field("exampleos"), Value: input}); err != nil {
		t.Fatal(err)
	}
	input["items"].([]any)[0].(map[string]any)["name"] = "wrong"
	got := editor.Snapshot().Root.Instances[0].ExtractedInstance.Fields["exampleos"].(map[string]any)["items"].([]any)[0].(map[string]any)["name"]
	if got != "updated" {
		t.Fatalf("mutating write input changed refreshed field to %v", got)
	}
	if got := beforeTree.Root.Instances[0].ExtractedInstance.Fields["exampleos"].(map[string]any)["items"].([]any)[0].(map[string]any)["name"]; got != "original" {
		t.Fatalf("writing changed previous snapshot to %v", got)
	}
}

func TestNamedFieldsComputedPathsFollowStableInstancesAfterReorder(t *testing.T) {
	ctx := context.Background()
	definition, workload := namedFieldsFixture("root")
	definition.Spec.Variables = []v1alpha1.Variable{{Name: "prefix", Expression: `"/groups/"`}}
	definition.Spec.StructureDefinition.RootComponent.Fields["groups"] = v1alpha1.ValueAccessor{
		Expression: "object.groups", PathWrite: ptr.To("/groups"),
	}
	definition.Spec.StructureDefinition.ChildComponents = []v1alpha1.ComponentDefinition{{
		Name: "worker", OwnerRef: ptr.To("deployment"),
		InstanceIds: &v1alpha1.ValueAccessor{Expression: "object.groups.map(g, g.name)"},
		PodSelector: &v1alpha1.PodSelector{ComponentInstanceSelector: &v1alpha1.ComponentInstanceSelector{Expression: `object.metadata.labels["group"]`}},
		Fields: map[string]v1alpha1.ValueAccessor{
			"exampleos": {
				Expression:          "object.groups.map(g, g.value)",
				PathWriteExpression: `object.groups[index].name == instance ? variables.prefix + string(index) + "/value" : dyn(null)`,
			},
		},
	}}
	groups := []any{map[string]any{"name": "zeta", "value": "zeta-old"}, map[string]any{"name": "alpha", "value": "alpha-old"}}
	workload.Object["groups"] = groups
	editor, err := tree.Open(ctx, definition, workload)
	if err != nil {
		t.Fatal(err)
	}
	instances := editor.Snapshot().Children[0].Instances
	if *instances[0].InstanceKey != "alpha" || instances[0].ExtractedInstance.Fields["exampleos"] != "alpha-old" {
		t.Fatalf("read list was not aligned to stable instance IDs: %#v", instances)
	}
	for _, step := range []struct{ path, value string }{{"/groups/0/value", "first"}, {"/groups/1/value", "second"}} {
		target, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: "worker", Instance: "zeta", Field: tree.Field("exampleos")})
		if err != nil || target.Path != step.path {
			t.Fatalf("stable instance target = %+v, error = %v; want %s", target, err, step.path)
		}
		before := safetyResource(t, editor)
		if err := editor.Mutate(ctx, tree.Write{Component: "worker", Instance: "zeta", Field: tree.Field("exampleos"), Value: step.value}); err != nil {
			t.Fatal(err)
		}
		current := safetyResource(t, editor)["groups"].([]any)
		for i, group := range current {
			if group.(map[string]any)["name"] == "zeta" {
				before["groups"].([]any)[i].(map[string]any)["value"] = step.value
			}
		}
		if !reflect.DeepEqual(before, safetyResource(t, editor)) {
			t.Fatal("computed path changed a different instance")
		}
		instances = editor.Snapshot().Children[0].Instances
		if instances[0].ExtractedInstance.Fields["exampleos"] != "alpha-old" || instances[1].ExtractedInstance.Fields["exampleos"] != step.value {
			t.Fatal("refreshed values were attached to the wrong stable instance")
		}
		if step.value == "first" {
			if err := editor.Mutate(ctx, tree.Write{Component: "deployment", Field: tree.Field("groups"), Value: []any{current[1], current[0]}}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestNamedFieldsRejectedWritesAreAtomic(t *testing.T) {
	for _, mode := range []string{"read-only", "undefined", "invalid selector", "null selector", "missing selector", "invalid later write", "custom overlap", "custom ancestor overlap", "builtin overlap", "read failure"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			definition, workload := namedFieldsFixture("old")
			root := &definition.Spec.StructureDefinition.RootComponent
			root.Fields["other"] = v1alpha1.ValueAccessor{Expression: "object.d.d.neighbor", PathWrite: ptr.To("/d/d/neighbor")}
			writes := []tree.Write{{Component: "deployment", Field: tree.Field("exampleos"), Value: "updated"},
				{Component: "deployment", Field: tree.Field("other"), Value: "updated"}}
			wantError := ""
			switch mode {
			case "read-only":
				root.Fields["other"] = v1alpha1.ValueAccessor{Expression: "object.d.d.neighbor"}
				wantError = "read-only"
			case "undefined":
				delete(root.Fields, "other")
				wantError = `no field "other"`
			case "invalid selector", "null selector", "missing selector":
				selector := map[string]string{"invalid selector": `"/d/~2"`, "null selector": "null", "missing selector": "object.missing"}[mode]
				root.Fields["other"] = v1alpha1.ValueAccessor{Expression: "object.d.d.neighbor", PathWriteExpression: selector}
			case "invalid later write":
				writes[1].Options.PatchType = "invalid"
				wantError = "patch type"
			case "custom overlap", "custom ancestor overlap":
				path := "/d/d/c"
				if mode == "custom ancestor overlap" {
					path = "/d/d"
				}
				root.Fields["other"] = v1alpha1.ValueAccessor{Expression: "object.d.d.neighbor", PathWrite: &path}
				wantError = "overlapping"
			case "builtin overlap":
				root.SpecDefinition = &v1alpha1.SpecDefinition{PodSpec: &v1alpha1.ValueAccessor{Expression: "{}", PathWrite: ptr.To("/d/d")}}
				writes[1].Field, writes[1].Value = tree.PodSpec, map[string]any{}
				wantError = "overlapping"
			case "read failure":
				root.Fields["exampleos"] = v1alpha1.ValueAccessor{Expression: `object.d.d.c + "!"`, PathWrite: ptr.To("/d/d/c")}
				writes[0].Value = map[string]any{"bad": "type"}
				wantError = "extract"
			}
			editor, err := tree.Open(ctx, definition, workload)
			if err != nil {
				t.Fatal(err)
			}
			beforeObject, beforeTree := safetyResource(t, editor), editor.Snapshot()
			if mode == "read-only" || mode == "undefined" || strings.HasSuffix(mode, "selector") {
				_, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: "deployment", Field: writes[1].Field})
				if err == nil || !strings.Contains(err.Error(), wantError) {
					t.Fatalf("invalid named target resolution error = %v; want %q", err, wantError)
				}
			}
			if err := editor.Mutate(ctx, writes...); err == nil || !strings.Contains(err.Error(), wantError) {
				t.Fatalf("invalid transaction error = %v; want %q", err, wantError)
			}
			if !reflect.DeepEqual(beforeObject, safetyResource(t, editor)) || !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
				t.Fatal("failed named-field transaction published a partial workload or extraction")
			}
		})
	}
}

func TestNamedFieldsNamesAreScopedToComponents(t *testing.T) {
	ctx := context.Background()
	definition, workload := namedFieldsFixture("root-old")
	definition.Spec.StructureDefinition.ChildComponents = []v1alpha1.ComponentDefinition{{
		Name: "child", OwnerRef: ptr.To("deployment"), Fields: map[string]v1alpha1.ValueAccessor{
			"exampleos": {Expression: "object.d.d.neighbor", PathWrite: ptr.To("/d/d/neighbor")},
		},
	}}
	definition.Spec.StructureDefinition.RootComponent.Fields["schedulerName"] = v1alpha1.ValueAccessor{
		Expression: "object.spec.template.spec.schedulerName", PathWrite: ptr.To("/spec/template/spec/schedulerName"),
	}
	editor, err := tree.Open(ctx, definition, workload)
	if err != nil {
		t.Fatal(err)
	}
	if err := editor.Mutate(ctx,
		tree.Write{Component: "deployment", Field: tree.Field("exampleos"), Value: "root-new"},
		tree.Write{Component: "child", Field: tree.Field("exampleos"), Value: "child-new"},
		tree.Write{Component: "deployment", Field: tree.Field("schedulerName"), Value: "batch-scheduler"},
	); err != nil {
		t.Fatal(err)
	}
	snapshot := editor.Snapshot()
	if snapshot.Root.Instances[0].ExtractedInstance.Fields["exampleos"] != "root-new" || snapshot.Children[0].Instances[0].ExtractedInstance.Fields["exampleos"] != "child-new" {
		t.Fatal("same-name fields from separate components collided")
	}
	if got := snapshot.Root.Instances[0].ExtractedInstance.Fields["schedulerName"]; got != "batch-scheduler" {
		t.Fatalf("unqualified schedulerName did not use its named accessor: %v", got)
	}
}
