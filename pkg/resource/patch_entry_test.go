// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package resource

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/cel"
)

func sdkTestAccessor(t *testing.T, object map[string]any, options ...MutationOptions) *Accessor {
	t.Helper()
	runner, err := cel.NewRunner(object)
	if err != nil {
		t.Fatal(err)
	}
	return NewAccessor(runner, options...)
}

func sdkTestObject(t *testing.T, accessor *Accessor) map[string]any {
	t.Helper()
	object, err := accessor.GetObject()
	if err != nil {
		t.Fatal(err)
	}
	return object
}

func TestSDKWritePolicies(t *testing.T) {
	t.Run("default merges and creates map parents", func(t *testing.T) {
		g := NewWithT(t)
		accessor := sdkTestAccessor(t, map[string]any{"spec": map[string]any{"keep": true}})
		g.Expect(accessor.WriteValues(context.Background(), v1alpha1.ComponentDefinition{},
			&v1alpha1.ValueAccessor{PathWrite: ptr.To("/spec")},
			[]any{map[string]any{"new": true}}, MutationOptions{})).To(Succeed())
		g.Expect(accessor.WriteValues(context.Background(), v1alpha1.ComponentDefinition{},
			&v1alpha1.ValueAccessor{PathWrite: ptr.To("/metadata/labels/app")},
			[]any{"example"}, MutationOptions{})).To(Succeed())
		g.Expect(sdkTestObject(t, accessor)).To(Equal(map[string]any{
			"spec":     map[string]any{"keep": true, "new": true},
			"metadata": map[string]any{"labels": map[string]any{"app": "example"}},
		}))
	})
	for _, format := range []PatchType{PatchTypeMergePatch, PatchTypeJSONPatch} {
		for _, strategy := range []MergeStrategy{Merge, Replace} {
			t.Run(string(format)+"/"+string(strategy), func(t *testing.T) {
				g := NewWithT(t)
				accessor := sdkTestAccessor(t, map[string]any{
					"spec": map[string]any{
						"keep": "sibling",
						"template": map[string]any{
							"keep":   "target",
							"nested": map[string]any{"keep": "nested", "update": "old"},
							"items":  []any{"old", "removed"},
						},
					},
				})
				incoming := map[string]any{
					"nested": map[string]any{"update": "new"},
					"items":  []any{"new"},
				}
				via := &v1alpha1.ValueAccessor{PathWrite: ptr.To("/spec/template")}
				g.Expect(accessor.WriteValues(context.Background(), v1alpha1.ComponentDefinition{}, via,
					[]any{incoming}, MutationOptions{PatchType: format, Strategy: strategy})).To(Succeed())

				expected := map[string]any{
					"nested": map[string]any{"update": "new"},
					"items":  []any{"new"},
				}
				if strategy == Merge {
					expected["keep"] = "target"
					expected["nested"] = map[string]any{"keep": "nested", "update": "new"}
				}
				g.Expect(sdkTestObject(t, accessor)).To(Equal(map[string]any{
					"spec": map[string]any{"keep": "sibling", "template": expected},
				}))
				incoming["items"] = []any{"mutated after write"}
				g.Expect(sdkTestObject(t, accessor)["spec"].(map[string]any)["template"]).To(Equal(expected))
			})
		}
	}
}

func TestSDKExplicitNullAndEmptyValues(t *testing.T) {
	for _, format := range []PatchType{PatchTypeMergePatch, PatchTypeJSONPatch} {
		for _, strategy := range []MergeStrategy{Merge, Replace} {
			for _, test := range []struct {
				name  string
				input any
			}{
				{name: "null", input: nil},
				{name: "empty map", input: map[string]any{}},
				{name: "empty array", input: []any{}},
				{name: "nested null", input: map[string]any{"keep": nil}},
			} {
				t.Run(string(format)+"/"+string(strategy)+"/"+test.name, func(t *testing.T) {
					g := NewWithT(t)
					accessor := sdkTestAccessor(t, map[string]any{
						"target": map[string]any{"keep": "old"}, "sibling": true,
					})
					g.Expect(accessor.WriteValues(context.Background(), v1alpha1.ComponentDefinition{},
						&v1alpha1.ValueAccessor{PathWrite: ptr.To("/target")}, []any{test.input},
						MutationOptions{PatchType: format, Strategy: strategy})).To(Succeed())
					object := sdkTestObject(t, accessor)
					g.Expect(object["sibling"]).To(BeTrue())
					switch test.name {
					case "null":
						if format == PatchTypeMergePatch {
							g.Expect(object).NotTo(HaveKey("target"))
						} else {
							g.Expect(object).To(HaveKeyWithValue("target", BeNil()))
						}
					case "empty map":
						expected := map[string]any{}
						if strategy == Merge {
							expected["keep"] = "old"
						}
						g.Expect(object["target"]).To(Equal(expected))
					case "empty array":
						g.Expect(object["target"]).To(Equal([]any{}))
					case "nested null":
						expected := map[string]any{}
						if format == PatchTypeJSONPatch {
							expected["keep"] = nil
						}
						g.Expect(object["target"]).To(Equal(expected))
					}
				})
			}
		}
	}
}

func TestSDKRootWrites(t *testing.T) {
	for _, format := range []PatchType{PatchTypeMergePatch, PatchTypeJSONPatch} {
		for _, strategy := range []MergeStrategy{Merge, Replace} {
			t.Run(string(format)+"/"+string(strategy), func(t *testing.T) {
				g := NewWithT(t)
				accessor := sdkTestAccessor(t, map[string]any{"keep": true, "replace": "old"})
				via := &v1alpha1.ValueAccessor{PathWrite: ptr.To("")}
				options := MutationOptions{PatchType: format, Strategy: strategy}
				g.Expect(accessor.WriteValues(context.Background(), v1alpha1.ComponentDefinition{}, via,
					[]any{map[string]any{"replace": "new"}}, options)).To(Succeed())
				expected := map[string]any{"replace": "new"}
				if strategy == Merge {
					expected["keep"] = true
				}
				g.Expect(sdkTestObject(t, accessor)).To(Equal(expected))
				for _, invalid := range []any{nil, false, "scalar", []any{}} {
					g.Expect(accessor.WriteValues(context.Background(), v1alpha1.ComponentDefinition{}, via,
						[]any{invalid}, options)).NotTo(Succeed())
					g.Expect(sdkTestObject(t, accessor)).To(Equal(expected))
				}
			})
		}
	}
}

func TestSDKResolveWriteTargets(t *testing.T) {
	t.Run("dynamic variables references and escaping", func(t *testing.T) {
		g := NewWithT(t)
		object := map[string]any{"spec": map[string]any{
			"services": map[string]any{"a~/b": map[string]any{"image": "old"}},
			"selected": "services",
		}}
		runner, err := cel.NewRunnerWithVariables(object,
			[]cel.NamedExpression{{Name: "section", Expression: `object.spec.selected`}},
			cel.WithReferenceProvider(func(context.Context) (map[string]any, error) {
				return map[string]any{"field": "image"}, nil
			}))
		g.Expect(err).NotTo(HaveOccurred())
		accessor := NewAccessor(runner)
		via := &v1alpha1.ValueAccessor{PathWriteExpression: `"/spec/" + variables.section + "/" + instance.replace("~", "~0").replace("/", "~1") + "/" + references.field`}
		target, err := accessor.ResolveWriteTarget(context.Background(), via, "a~/b", 3)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(target).To(Equal(WriteTarget{Path: "/spec/services/a~0~1b/image", Exists: true, Value: "old"}))
	})

	t.Run("existing null missing root and detached values", func(t *testing.T) {
		g := NewWithT(t)
		accessor := sdkTestAccessor(t, map[string]any{"present": nil, "nested": map[string]any{"keep": true}})
		for _, test := range []struct {
			path   string
			exists bool
		}{
			{path: "/present", exists: true},
			{path: "/missing", exists: false},
			{path: "", exists: true},
		} {
			target, err := accessor.ResolveWriteTarget(context.Background(),
				&v1alpha1.ValueAccessor{PathWrite: ptr.To(test.path)}, "", 0)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(target.Exists).To(Equal(test.exists))
			if test.path == "" {
				target.Value.(map[string]any)["nested"].(map[string]any)["keep"] = false
			}
		}
		g.Expect(sdkTestObject(t, accessor)["nested"]).To(Equal(map[string]any{"keep": true}))
	})

	for _, test := range []struct {
		name string
		via  *v1alpha1.ValueAccessor
	}{
		{name: "missing accessor"},
		{name: "read-only", via: &v1alpha1.ValueAccessor{Expression: `object.spec`}},
		{name: "both locations", via: &v1alpha1.ValueAccessor{PathWrite: ptr.To("/spec"), PathWriteExpression: `"/spec"`}},
		{name: "dot path", via: &v1alpha1.ValueAccessor{PathWrite: ptr.To(".spec")}},
		{name: "bad escape", via: &v1alpha1.ValueAccessor{PathWrite: ptr.To("/spec/~2")}},
		{name: "trailing escape", via: &v1alpha1.ValueAccessor{PathWrite: ptr.To("/spec/a~")}},
		{name: "null expression", via: &v1alpha1.ValueAccessor{PathWriteExpression: `null`}},
		{name: "number expression", via: &v1alpha1.ValueAccessor{PathWriteExpression: `42`}},
		{name: "list expression", via: &v1alpha1.ValueAccessor{PathWriteExpression: `["/spec"]`}},
		{name: "map expression", via: &v1alpha1.ValueAccessor{PathWriteExpression: `{"path": "/spec"}`}},
		{name: "evaluation failure", via: &v1alpha1.ValueAccessor{PathWriteExpression: `object.missing.path`}},
		{name: "value unavailable", via: &v1alpha1.ValueAccessor{PathWriteExpression: `value == null ? "/spec/a" : "/spec/b"`}},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			accessor := sdkTestAccessor(t, map[string]any{"spec": map[string]any{"keep": true}})
			_, err := accessor.ResolveWriteTarget(context.Background(), test.via, "", 0)
			g.Expect(err).To(HaveOccurred())
			g.Expect(sdkTestObject(t, accessor)).To(Equal(map[string]any{"spec": map[string]any{"keep": true}}))
		})
	}
}

func TestSDKArrayTargetsPreserveOtherItems(t *testing.T) {
	t.Run("null item retains array position for JSONPatch", func(t *testing.T) {
		g := NewWithT(t)
		original := map[string]any{"items": []any{"first", "second"}}
		accessor := sdkTestAccessor(t, original)
		via := &v1alpha1.ValueAccessor{PathWrite: ptr.To("/items/0")}
		g.Expect(accessor.WriteValues(context.Background(), v1alpha1.ComponentDefinition{}, via,
			[]any{nil}, MutationOptions{PatchType: PatchTypeMergePatch})).NotTo(Succeed())
		g.Expect(sdkTestObject(t, accessor)).To(Equal(original))
		g.Expect(accessor.WriteValues(context.Background(), v1alpha1.ComponentDefinition{}, via,
			[]any{nil}, MutationOptions{PatchType: PatchTypeJSONPatch})).To(Succeed())
		g.Expect(sdkTestObject(t, accessor)).To(Equal(map[string]any{"items": []any{nil, "second"}}))
	})
	for _, format := range []PatchType{PatchTypeMergePatch, PatchTypeJSONPatch} {
		t.Run(string(format)+"/null field within array item", func(t *testing.T) {
			g := NewWithT(t)
			accessor := sdkTestAccessor(t, map[string]any{"items": []any{
				map[string]any{"name": "first", "image": "old-first", "keep": true},
				map[string]any{"name": "second", "image": "old-second"},
			}})
			g.Expect(accessor.WriteValues(context.Background(), v1alpha1.ComponentDefinition{},
				&v1alpha1.ValueAccessor{PathWrite: ptr.To("/items/0/image")}, []any{nil},
				MutationOptions{PatchType: format, Parents: RequireParents})).To(Succeed())
			first := map[string]any{"name": "first", "keep": true}
			if format == PatchTypeJSONPatch {
				first["image"] = nil
			}
			g.Expect(sdkTestObject(t, accessor)).To(Equal(map[string]any{"items": []any{
				first,
				map[string]any{"name": "second", "image": "old-second"},
			}}))
		})
		for _, strategy := range []MergeStrategy{Merge, Replace} {
			t.Run(string(format)+"/"+string(strategy), func(t *testing.T) {
				g := NewWithT(t)
				accessor := sdkTestAccessor(t, map[string]any{"spec": map[string]any{"items": []any{
					map[string]any{"name": "z", "image": "old-z", "keep": "z"},
					map[string]any{"name": "a", "image": "old-a", "keep": "a"},
				}}})
				definition := v1alpha1.ComponentDefinition{InstanceIds: &v1alpha1.ValueAccessor{
					Expression: `object.spec.items.map(item, item.name)`,
				}}
				g.Expect(accessor.WriteValues(context.Background(), definition,
					&v1alpha1.ValueAccessor{PathWriteExpression: `"/spec/items/" + string(index) + "/image"`},
					[]any{"new-z", "new-a"}, MutationOptions{PatchType: format, Strategy: strategy})).To(Succeed())
				g.Expect(sdkTestObject(t, accessor)).To(Equal(map[string]any{"spec": map[string]any{"items": []any{
					map[string]any{"name": "z", "image": "new-z", "keep": "z"},
					map[string]any{"name": "a", "image": "new-a", "keep": "a"},
				}}}))
			})
		}
	}
}

func TestSDKArrayParentPolicies(t *testing.T) {
	for _, format := range []PatchType{PatchTypeMergePatch, PatchTypeJSONPatch} {
		for _, policy := range []ParentPolicy{CreateMapParents, RequireParents} {
			for _, parent := range []string{"missing", "null"} {
				t.Run(string(format)+"/"+string(policy)+"/"+parent, func(t *testing.T) {
					g := NewWithT(t)
					second := map[string]any{"name": "second", "keep": "second"}
					if parent == "null" {
						second["template"] = nil
					}
					original := map[string]any{"items": []any{
						map[string]any{
							"name": "first", "keep": "first",
							"template": map[string]any{"spec": map[string]any{"image": "old", "keep": true}},
						},
						second,
						map[string]any{"name": "untouched", "keep": "third"},
					}}
					accessor := sdkTestAccessor(t, original)
					definition := v1alpha1.ComponentDefinition{InstanceIds: &v1alpha1.ValueAccessor{
						Expression: `["first", "second"]`,
					}}
					err := accessor.WriteValues(context.Background(), definition,
						&v1alpha1.ValueAccessor{PathWriteExpression: `"/items/" + string(index) + "/template/spec/image"`},
						[]any{"new-first", "new-second"}, MutationOptions{PatchType: format, Parents: policy})
					if policy == RequireParents {
						g.Expect(err).To(MatchError(ContainSubstring(`write "/items/1/template/spec/image": missing parent`)))
						g.Expect(sdkTestObject(t, accessor)).To(Equal(original))
						return
					}
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(sdkTestObject(t, accessor)).To(Equal(map[string]any{"items": []any{
						map[string]any{
							"name": "first", "keep": "first",
							"template": map[string]any{"spec": map[string]any{"image": "new-first", "keep": true}},
						},
						map[string]any{
							"name": "second", "keep": "second",
							"template": map[string]any{"spec": map[string]any{"image": "new-second"}},
						},
						map[string]any{"name": "untouched", "keep": "third"},
					}}))
				})
			}
		}
	}
}

func TestSDKParentPolicies(t *testing.T) {
	t.Run("deleting an absent child does not turn a null parent into a map", func(t *testing.T) {
		for _, policy := range []ParentPolicy{CreateMapParents, RequireParents} {
			g := NewWithT(t)
			original := map[string]any{"spec": nil, "keep": true}
			accessor := sdkTestAccessor(t, original)
			g.Expect(accessor.WriteValues(context.Background(), v1alpha1.ComponentDefinition{},
				&v1alpha1.ValueAccessor{PathWrite: ptr.To("/spec/absent")}, []any{nil},
				MutationOptions{Parents: policy})).To(Succeed())
			g.Expect(sdkTestObject(t, accessor)).To(Equal(original))
		}
	})
	for _, test := range []struct {
		name     string
		object   map[string]any
		path     string
		policy   ParentPolicy
		succeeds bool
	}{
		{name: "create missing maps", object: map[string]any{}, path: "/spec/template/image", policy: CreateMapParents, succeeds: true},
		{name: "require missing maps", object: map[string]any{}, path: "/spec/template/image", policy: RequireParents},
		{name: "create under null", object: map[string]any{"spec": nil}, path: "/spec/image", policy: CreateMapParents, succeeds: true},
		{name: "require null parent", object: map[string]any{"spec": nil}, path: "/spec/image", policy: RequireParents},
		{name: "cannot traverse scalar", object: map[string]any{"spec": true}, path: "/spec/image", policy: CreateMapParents},
		{name: "cannot invent array", object: map[string]any{}, path: "/spec/items/0/image", policy: CreateMapParents},
		{name: "numeric existing map key", object: map[string]any{"spec": map[string]any{"0": map[string]any{}}}, path: "/spec/0/image", policy: RequireParents, succeeds: true},
		{name: "out of bounds", object: map[string]any{"items": []any{map[string]any{}}}, path: "/items/1/image", policy: CreateMapParents},
		{name: "leading zero index", object: map[string]any{"items": []any{map[string]any{}}}, path: "/items/00/image", policy: CreateMapParents},
		{name: "append is not target", object: map[string]any{"items": []any{map[string]any{}}}, path: "/items/-", policy: CreateMapParents},
		{name: "negative index", object: map[string]any{"items": []any{map[string]any{}}}, path: "/items/-1", policy: CreateMapParents},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			accessor := sdkTestAccessor(t, test.object)
			via := &v1alpha1.ValueAccessor{PathWrite: ptr.To(test.path)}
			err := accessor.WriteValues(context.Background(), v1alpha1.ComponentDefinition{}, via,
				[]any{"new"}, MutationOptions{Parents: test.policy})
			if test.succeeds {
				g.Expect(err).NotTo(HaveOccurred())
				target, err := accessor.ResolveWriteTarget(context.Background(), via, "", 0)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(target.Value).To(Equal("new"))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(sdkTestObject(t, accessor)).To(Equal(test.object))
			}
		})
	}
}

func TestSDKBatchWriteRollback(t *testing.T) {
	t.Run("all destinations resolve before the first write", func(t *testing.T) {
		g := NewWithT(t)
		accessor := sdkTestAccessor(t, map[string]any{"spec": map[string]any{"route": "old"}})
		definition := v1alpha1.ComponentDefinition{InstanceIds: &v1alpha1.ValueAccessor{Expression: `["a", "b"]`}}
		via := &v1alpha1.ValueAccessor{PathWriteExpression: `index == 0 ? "/spec/route" : "/spec/" + object.spec.route`}
		g.Expect(accessor.WriteValues(context.Background(), definition, via,
			[]any{"new", true}, MutationOptions{})).To(Succeed())
		g.Expect(sdkTestObject(t, accessor)).To(Equal(map[string]any{
			"spec": map[string]any{"route": "new", "old": true},
		}))
	})
	for _, test := range []struct {
		name          string
		ids           string
		path          string
		values        []any
		errorContains string
	}{
		{name: "duplicate instance IDs", ids: `["a", "a"]`, path: `"/spec/" + instance`, values: []any{true, false}},
		{name: "duplicate targets", ids: `["a", "b"]`, path: `"/spec/target"`, values: []any{true, false}},
		{name: "overlapping targets", ids: `["a", "b"]`, path: `index == 0 ? "/spec/template" : "/spec/template/image"`, values: []any{map[string]any{}, "new"}},
		{name: "count mismatch", ids: `["a", "b"]`, path: `"/spec/" + instance`, values: []any{true}},
		{name: "later target error", ids: `["a", "b"]`, path: `index == 0 ? dyn("/spec/a") : null`, values: []any{true, false}, errorContains: "target 1: pathWriteExpression must return one JSON Pointer string"},
		{name: "later unencodable value", ids: `["a", "b"]`, path: `"/spec/" + instance`, values: []any{true, make(chan int)}},
		{name: "later missing parent", ids: `["a", "b"]`, path: `index == 0 ? "/spec/a" : "/missing/parent/b"`, values: []any{true, false}},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			original := map[string]any{"spec": map[string]any{"keep": true}}
			accessor := sdkTestAccessor(t, original)
			definition := v1alpha1.ComponentDefinition{InstanceIds: &v1alpha1.ValueAccessor{Expression: test.ids}}
			err := accessor.WriteValues(context.Background(), definition,
				&v1alpha1.ValueAccessor{PathWriteExpression: test.path}, test.values,
				MutationOptions{Parents: RequireParents})
			g.Expect(err).To(HaveOccurred())
			if test.errorContains != "" {
				g.Expect(err).To(MatchError(ContainSubstring(test.errorContains)))
			}
			g.Expect(sdkTestObject(t, accessor)).To(Equal(original))
		})
	}
	t.Run("cancelled context and invalid options", func(t *testing.T) {
		g := NewWithT(t)
		original := map[string]any{"keep": true}
		accessor := sdkTestAccessor(t, original)
		via := &v1alpha1.ValueAccessor{PathWrite: ptr.To("/new")}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		g.Expect(accessor.WriteValues(ctx, v1alpha1.ComponentDefinition{}, via, []any{true}, MutationOptions{})).NotTo(Succeed())
		for _, options := range []MutationOptions{
			{PatchType: "unknown"}, {Strategy: "unknown"}, {Parents: "unknown"},
		} {
			g.Expect(accessor.WriteValues(context.Background(), v1alpha1.ComponentDefinition{}, via, []any{true}, options)).NotTo(Succeed())
		}
		g.Expect(sdkTestObject(t, accessor)).To(Equal(original))
	})
}

func TestSDKBooleanSuspendResume(t *testing.T) {
	for _, format := range []PatchType{PatchTypeMergePatch, PatchTypeJSONPatch} {
		t.Run(string(format), func(t *testing.T) {
			g := NewWithT(t)
			accessor := sdkTestAccessor(t, map[string]any{"spec": map[string]any{"keep": true}},
				MutationOptions{PatchType: format, Strategy: Replace})
			definition := v1alpha1.ComponentDefinition{SuspendDefinition: &v1alpha1.SuspendDefinition{
				PathWrite: ptr.To("/spec/suspend"),
			}}
			g.Expect(accessor.ApplySuspendActions(context.Background(), definition)).To(Succeed())
			g.Expect(sdkTestObject(t, accessor)["spec"]).To(Equal(map[string]any{"keep": true, "suspend": true}))
			g.Expect(accessor.ApplyResumeActions(context.Background(), definition)).To(Succeed())
			g.Expect(sdkTestObject(t, accessor)["spec"]).To(Equal(map[string]any{"keep": true, "suspend": false}))

			definition.InstanceIds = &v1alpha1.ValueAccessor{Expression: `["a", "b"]`}
			definition.SuspendDefinition = &v1alpha1.SuspendDefinition{PathWriteExpression: `"/spec/" + instance + "/suspend"`}
			g.Expect(accessor.ApplySuspendActions(context.Background(), definition)).To(Succeed())
			g.Expect(accessor.ApplyResumeActions(context.Background(), definition)).To(Succeed())
			spec := sdkTestObject(t, accessor)["spec"].(map[string]any)
			g.Expect(spec["a"]).To(Equal(map[string]any{"suspend": false}))
			g.Expect(spec["b"]).To(Equal(map[string]any{"suspend": false}))
		})
	}
}

func TestSDKCustomPatchOperations(t *testing.T) {
	g := NewWithT(t)
	accessor := sdkTestAccessor(t, map[string]any{
		"spec": map[string]any{"source": "copied", "keep": true, "remove": true},
	})
	operations := []any{
		map[string]any{"op": "test", "path": "/spec/source", "value": "copied"},
		map[string]any{"op": "copy", "from": "/spec/source", "path": "/spec/copied"},
		map[string]any{"op": "move", "from": "/spec/copied", "path": "/spec/moved"},
		map[string]any{"op": "replace", "path": "/spec/source", "value": "new"},
		map[string]any{"op": "add", "path": "/metadata/labels/state", "value": "ready"},
		map[string]any{"op": "remove", "path": "/spec/remove"},
	}
	g.Expect(accessor.ApplyPatch(context.Background(), PatchTypeJSONPatch, operations)).To(Succeed())
	expected := map[string]any{
		"spec":     map[string]any{"source": "new", "moved": "copied", "keep": true},
		"metadata": map[string]any{"labels": map[string]any{"state": "ready"}},
	}
	g.Expect(sdkTestObject(t, accessor)).To(Equal(expected))
	g.Expect(accessor.ApplyPatch(context.Background(), PatchTypeJSONPatch, []any{
		map[string]any{"op": "add", "path": "/temporary/parent/value", "value": true},
		map[string]any{"op": "test", "path": "/spec/source", "value": "wrong"},
	})).NotTo(Succeed())
	g.Expect(sdkTestObject(t, accessor)).To(Equal(expected))
	for _, invalid := range []any{
		map[string]any{"spec": true},
		[]any{"not an operation"},
		[]any{map[string]any{"path": "/spec"}},
	} {
		g.Expect(accessor.ApplyPatch(context.Background(), PatchTypeJSONPatch, invalid)).NotTo(Succeed())
		g.Expect(sdkTestObject(t, accessor)).To(Equal(expected))
	}
	g.Expect(accessor.ApplyPatch(context.Background(), PatchTypeMergePatch, map[string]any{
		"spec": map[string]any{"source": nil},
	})).To(Succeed())
	delete(expected["spec"].(map[string]any), "source")
	g.Expect(sdkTestObject(t, accessor)).To(Equal(expected))
	g.Expect(accessor.ApplyPatch(context.Background(), PatchTypeMergePatch, nil)).NotTo(Succeed())
	g.Expect(sdkTestObject(t, accessor)).To(Equal(expected))
}
