// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package resource

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog/kartas"
)

func TestSDKTypedPodRootReplacementPreservesIdentity(t *testing.T) {
	for _, format := range []PatchType{PatchTypeMergePatch, PatchTypeJSONPatch} {
		t.Run(string(format), func(t *testing.T) {
			original := identityWritePod()
			before := runtime.DeepCopyJSON(original)
			accessor := sdkTestAccessor(t, original, MutationOptions{PatchType: format, Strategy: Replace})
			factory := NewComponentFactory(kartas.Pod(), accessor)
			component, err := factory.GetRootComponent()
			if err != nil {
				t.Fatal(err)
			}
			templates, err := component.GetPodTemplateSpec(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			template := templates[""]
			template.Spec.SchedulerName = "changed-scheduler"
			templates[""] = template
			inputBefore := template.DeepCopy()
			if err := component.UpdatePodTemplateSpec(t.Context(), templates); err == nil || !strings.Contains(err.Error(), "missing apiVersion") {
				t.Fatalf("typed PodTemplateSpec root replacement should reject lost identity, got %v", err)
			}
			if !reflect.DeepEqual(sdkTestObject(t, accessor), before) || !reflect.DeepEqual(original, before) {
				t.Fatal("rejected typed root replacement changed the workload")
			}
			inputAfter := templates[""]
			if !reflect.DeepEqual(&inputAfter, inputBefore) {
				t.Fatal("rejected typed root replacement changed the supplied template")
			}
			if _, err := factory.GetResource(); err != nil {
				t.Fatalf("rejected replacement left an invalid resource: %v", err)
			}
		})
	}
}

func TestSDKIdentityWritesRejectAtomically(t *testing.T) {
	for _, format := range []PatchType{PatchTypeMergePatch, PatchTypeJSONPatch} {
		for _, entry := range []string{"root write", "field write", "raw patch"} {
			for _, field := range []struct {
				path      []string
				generated bool
				wantError string
			}{
				{path: []string{"apiVersion"}, wantError: "missing apiVersion"},
				{path: []string{"kind"}, wantError: "missing kind"},
				{path: []string{"metadata"}, wantError: "missing metadata.name or metadata.generateName"},
				{path: []string{"metadata", "name"}, wantError: "missing metadata.name or metadata.generateName"},
				{path: []string{"metadata", "generateName"}, generated: true, wantError: "missing metadata.name or metadata.generateName"},
			} {
				for _, value := range []struct {
					name  string
					value any
				}{
					{name: "removed"},
					{name: "empty", value: ""},
					{name: "boolean", value: false},
					{name: "number", value: float64(3)},
				} {
					t.Run(string(format)+"/"+entry+"/"+strings.Join(field.path, ".")+"/"+value.name, func(t *testing.T) {
						original := identityWritePod()
						if field.generated {
							original["metadata"] = map[string]any{"generateName": "example-"}
						}
						before := runtime.DeepCopyJSON(original)
						accessor := sdkTestAccessor(t, original)
						path := "/" + strings.Join(field.path, "/")
						input := value.value
						switch entry {
						case "root write":
							root := runtime.DeepCopyJSON(original)
							root["spec"].(map[string]any)["schedulerName"] = "changed-scheduler"
							if value.value == nil {
								unstructured.RemoveNestedField(root, field.path...)
							} else if err := unstructured.SetNestedField(root, value.value, field.path...); err != nil {
								t.Fatal(err)
							}
							path, input = "", root
						case "raw patch":
							if format == PatchTypeJSONPatch {
								operation := map[string]any{"op": "replace", "path": path, "value": value.value}
								if value.value == nil {
									operation = map[string]any{"op": "remove", "path": path}
								}
								input = []any{
									map[string]any{"op": "replace", "path": "/spec/schedulerName", "value": "changed-scheduler"},
									operation,
								}
							} else {
								patch := map[string]any{"spec": map[string]any{"schedulerName": "changed-scheduler"}}
								if err := unstructured.SetNestedField(patch, value.value, field.path...); err != nil {
									t.Fatal(err)
								}
								input = patch
							}
						}
						inputBefore := runtime.DeepCopyJSONValue(input)
						var err error
						if entry == "raw patch" {
							err = accessor.ApplyPatch(t.Context(), format, input)
						} else {
							err = accessor.WriteValues(t.Context(), v1alpha1.ComponentDefinition{},
								&v1alpha1.ValueAccessor{PathWrite: ptr.To(path)}, []any{input},
								MutationOptions{PatchType: format, Strategy: Replace})
						}
						if err == nil || !strings.Contains(err.Error(), field.wantError) {
							t.Fatalf("invalid identity write should fail with %q, got %v", field.wantError, err)
						}
						if !reflect.DeepEqual(sdkTestObject(t, accessor), before) || !reflect.DeepEqual(original, before) {
							t.Fatal("rejected identity write changed the workload")
						}
						if !reflect.DeepEqual(input, inputBefore) {
							t.Fatal("rejected identity write changed the supplied input")
						}
					})
				}
			}
		}
	}
}

func TestSDKIdentityGuardPermitsValidWrites(t *testing.T) {
	for _, format := range []PatchType{PatchTypeMergePatch, PatchTypeJSONPatch} {
		for _, entry := range []string{"root write", "narrow write", "raw patch"} {
			for _, source := range []string{"named workload", "generated workload", "generic JSON"} {
				t.Run(string(format)+"/"+entry+"/"+source, func(t *testing.T) {
					original := identityWritePod()
					switch source {
					case "generated workload":
						original["metadata"] = map[string]any{"generateName": "example-"}
					case "generic JSON":
						delete(original, "apiVersion")
						delete(original, "kind")
						delete(original, "metadata")
					}
					before := runtime.DeepCopyJSON(original)
					accessor := sdkTestAccessor(t, original)
					expected := runtime.DeepCopyJSON(original)
					expected["spec"].(map[string]any)["schedulerName"] = "changed-scheduler"
					var err error
					switch entry {
					case "root write":
						err = accessor.WriteValues(t.Context(), v1alpha1.ComponentDefinition{},
							&v1alpha1.ValueAccessor{PathWrite: ptr.To("")}, []any{expected},
							MutationOptions{PatchType: format, Strategy: Replace})
					case "narrow write":
						err = accessor.WriteValues(t.Context(), v1alpha1.ComponentDefinition{},
							&v1alpha1.ValueAccessor{PathWrite: ptr.To("/spec/schedulerName")}, []any{"changed-scheduler"},
							MutationOptions{PatchType: format, Strategy: Replace})
					case "raw patch":
						var patch any = map[string]any{"spec": map[string]any{"schedulerName": "changed-scheduler"}}
						if format == PatchTypeJSONPatch {
							patch = []any{map[string]any{"op": "replace", "path": "", "value": expected}}
						}
						err = accessor.ApplyPatch(t.Context(), format, patch)
					}
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(sdkTestObject(t, accessor), expected) {
						t.Fatal("successful write changed more than the selected scheduler")
					}
					if !reflect.DeepEqual(original, before) {
						t.Fatal("successful write changed the caller's workload")
					}
				})
			}
		}
	}
}

func identityWritePod() map[string]any {
	return map[string]any{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]any{"name": "example-pod", "labels": map[string]any{"team": "example"}},
		"spec": map[string]any{
			"schedulerName": "original-scheduler",
			"containers":    []any{map[string]any{"name": "app", "image": "ghcr.io/example/app:v1"}},
		},
	}
}
