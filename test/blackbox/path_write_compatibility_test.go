// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package blackbox

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

func compatibilityDeployment() map[string]any {
	return map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": "example"},
		"spec": map[string]any{
			"replicas": float64(1),
			"template": map[string]any{
				"metadata": map[string]any{"labels": map[string]any{"team": "ml", "owner": "alice"}},
				"spec": map[string]any{
					"schedulerName": "batch-scheduler",
					"nodeSelector":  map[string]any{"zone": "east"},
					"extensionData": "outside-the-typed-view",
					"containers": []any{map[string]any{
						"name": "api", "image": "ghcr.io/example/api:v1",
						"resources": map[string]any{
							"limits": map[string]any{"cpu": "2"}, "requests": map[string]any{"cpu": "1"},
						},
					}},
				},
			},
		},
	}
}

// These characterize compatibility gaps, not desired clearing semantics. The
// matching historical tests directly invoke the old setter without mergeIntent.
func TestTypedSetterOmissionCompatibility(t *testing.T) {
	cases := []struct {
		name string
		edit func(*corev1.PodTemplateSpec)
		path []string
	}{
		{"remove_owner", func(p *corev1.PodTemplateSpec) { p.Labels = map[string]string{"team": "platform"} }, []string{"metadata", "labels", "owner"}},
		{"empty_labels", func(p *corev1.PodTemplateSpec) { p.Labels = map[string]string{} }, []string{"metadata", "labels"}},
		{"nil_labels", func(p *corev1.PodTemplateSpec) { p.Labels = nil }, []string{"metadata", "labels"}},
		{"empty_scheduler", func(p *corev1.PodTemplateSpec) { p.Spec.SchedulerName = "" }, []string{"spec", "schedulerName"}},
		{"nil_node_selector", func(p *corev1.PodTemplateSpec) { p.Spec.NodeSelector = nil }, []string{"spec", "nodeSelector"}},
	}
	for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		for _, strategy := range []resource.MergeStrategy{resource.Merge, resource.Replace} {
			for _, test := range cases {
				t.Run(string(format)+"/"+string(strategy)+"/"+test.name, func(t *testing.T) {
					ctx := context.Background()
					factory, accessor := newCatalogPathFactory(t, kartas.Deployment(), compatibilityDeployment(), resource.MutationOptions{PatchType: format, Strategy: strategy})
					component, err := factory.GetComponent("deployment")
					if err != nil {
						t.Fatal(err)
					}
					templates, err := component.GetPodTemplateSpec(ctx)
					if err != nil {
						t.Fatal(err)
					}
					value := templates[""]
					test.edit(&value)
					templates[""] = value
					if err := component.UpdatePodTemplateSpec(ctx, templates); err != nil {
						t.Fatal(err)
					}
					after, err := accessor.GetObject()
					if err != nil {
						t.Fatal(err)
					}
					template := after["spec"].(map[string]any)["template"].(map[string]any)
					_, exists, err := unstructured.NestedFieldNoCopy(template, test.path...)
					if err != nil || exists != (strategy == resource.Merge) {
						t.Fatalf("omitted field %v exists=%v, strategy=%s, err=%v", test.path, exists, strategy, err)
					}
					_, unknownKept, err := unstructured.NestedFieldNoCopy(template, "spec", "extensionData")
					if err != nil || unknownKept != (strategy == resource.Merge) {
						t.Fatal("Replace clearing tradeoff changed: broad typed replacement also removes unknown fields")
					}
					if _, err := factory.GetResource(); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestTreeExplicitDeletionPreservesUnmentionedFields(t *testing.T) {
	ctx := context.Background()
	before := compatibilityDeployment()
	editor, err := tree.Open(ctx, kartas.Deployment(), &unstructured.Unstructured{Object: before})
	if err != nil {
		t.Fatal(err)
	}
	err = editor.Mutate(ctx, tree.Write{
		Component: "deployment", Field: tree.PodTemplateSpec,
		Value: map[string]any{
			"metadata": map[string]any{"labels": map[string]any{"owner": nil, "team": "platform"}},
			"spec":     map[string]any{"schedulerName": nil, "nodeSelector": nil},
		},
		Options: resource.MutationOptions{PatchType: resource.PatchTypeMergePatch, Strategy: resource.Merge},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := editor.GetResource()
	if err != nil {
		t.Fatal(err)
	}
	after := result.(*unstructured.Unstructured).Object
	template := after["spec"].(map[string]any)["template"].(map[string]any)
	for _, path := range [][]string{{"metadata", "labels", "owner"}, {"spec", "schedulerName"}, {"spec", "nodeSelector"}} {
		if _, exists, err := unstructured.NestedFieldNoCopy(template, path...); err != nil || exists {
			t.Fatalf("explicit deletion failed at %v: exists=%v err=%v", path, exists, err)
		}
	}
	if got := template["metadata"].(map[string]any)["labels"]; !reflect.DeepEqual(got, map[string]any{"team": "platform"}) {
		t.Fatalf("labels = %#v", got)
	}
	spec := template["spec"].(map[string]any)
	beforeSpec := before["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
	for _, key := range []string{"containers", "extensionData"} {
		if !reflect.DeepEqual(spec[key], beforeSpec[key]) {
			t.Errorf("unmentioned %s changed", key)
		}
	}
	if !reflect.DeepEqual(before, compatibilityDeployment()) {
		t.Fatal("tree edited caller-owned input")
	}
}

func TestTypedArrayRoundTripStillDropsUnknownElementFields(t *testing.T) {
	for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		t.Run(string(format), func(t *testing.T) {
			ctx := context.Background()
			before := compatibilityDeployment()
			spec := before["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
			container := spec["containers"].([]any)[0].(map[string]any)
			container["extensionData"] = "lost-by-typed-array-replacement"
			factory, accessor := newCatalogPathFactory(t, kartas.Deployment(), before, resource.MutationOptions{PatchType: format})
			component, err := factory.GetComponent("deployment")
			if err != nil {
				t.Fatal(err)
			}
			templates, err := component.GetPodTemplateSpec(ctx)
			if err != nil {
				t.Fatal(err)
			}
			value := templates[""]
			value.Spec.Containers[0].Image = "ghcr.io/example/api:v2"
			value.Spec.Containers[0].Resources.Limits = nil
			templates[""] = value
			if err := component.UpdatePodTemplateSpec(ctx, templates); err != nil {
				t.Fatal(err)
			}
			after, err := accessor.GetObject()
			if err != nil {
				t.Fatal(err)
			}
			spec = after["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
			container = spec["containers"].([]any)[0].(map[string]any)
			if container["image"] != "ghcr.io/example/api:v2" {
				t.Fatal("image write failed")
			}
			if _, exists := container["extensionData"]; exists {
				t.Fatal("typed array round-trip behavior changed; revisit compatibility classification")
			}
			resources := container["resources"].(map[string]any)
			if _, exists := resources["limits"]; exists {
				t.Fatal("array replacement should remove omitted limits")
			}
			if !reflect.DeepEqual(resources["requests"], map[string]any{"cpu": "1"}) || spec["extensionData"] != "outside-the-typed-view" {
				t.Fatal("expected sibling preservation is incorrect")
			}
		})
	}
}

// Replace owns exactly its target subtree. Synthetic unknown fields below test
// preservation; they are not a claim that Deployment admission accepts them.
func TestReplaceScopeLabelsVersusWholeTemplate(t *testing.T) {
	for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		for _, scope := range []string{"narrow labels map", "broad typed template", "broad partial template"} {
			t.Run(string(format)+"/"+scope, func(t *testing.T) {
				ctx := context.Background()
				before := compatibilityDeployment()
				template := before["spec"].(map[string]any)["template"].(map[string]any)
				template["extensionMap"] = map[string]any{"setting": "keep"}
				template["metadata"].(map[string]any)["annotations"] = map[string]any{"example.com/note": "keep"}
				expected := runtime.DeepCopyJSON(before)
				options := resource.MutationOptions{PatchType: format, Strategy: resource.Replace}
				factory, accessor := newCatalogPathFactory(t, kartas.Deployment(), before, options)
				component, err := factory.GetComponent("deployment")
				if err != nil {
					t.Fatal(err)
				}
				definition := component.Definition()
				writer, err := factory.PathWriter()
				if err != nil {
					t.Fatal(err)
				}
				target, err := writer.ResolveWriteTarget(ctx, definition.SpecDefinition.PodTemplateSpec, "", 0)
				if err != nil || target.Path != "/spec/template" || !target.Exists {
					t.Fatalf("template target = %+v, error = %v", target, err)
				}
				var input any
				switch scope {
				case "narrow labels map":
					// No Deployment-specific path is embedded in the write. The
					// suffix is the standard PodTemplateSpec metadata/labels shape.
					labelsPath := target.Path + "/metadata/labels"
					input = map[string]any{"team": "platform"}
					if err := writer.WriteValues(ctx, definition, &v1alpha1.ValueAccessor{PathWrite: &labelsPath}, []any{input}, options); err != nil {
						t.Fatal(err)
					}
					expected["spec"].(map[string]any)["template"].(map[string]any)["metadata"].(map[string]any)["labels"] = map[string]any{"team": "platform"}
				case "broad typed template":
					templates, err := component.GetPodTemplateSpec(ctx)
					if err != nil {
						t.Fatal(err)
					}
					value := templates[""]
					value.Labels = map[string]string{"team": "platform"}
					templates[""] = value
					input = value
					if err := component.UpdatePodTemplateSpec(ctx, templates); err != nil {
						t.Fatal(err)
					}
					encoded, err := json.Marshal(value)
					if err != nil {
						t.Fatal(err)
					}
					var typedTemplate map[string]any
					if err := json.Unmarshal(encoded, &typedTemplate); err != nil {
						t.Fatal(err)
					}
					expected["spec"].(map[string]any)["template"] = typedTemplate
				case "broad partial template":
					input = map[string]any{"metadata": map[string]any{"labels": map[string]any{"team": "platform"}}}
					if err := writer.WriteValues(ctx, definition, definition.SpecDefinition.PodTemplateSpec, []any{input}, options); err != nil {
						t.Fatal(err)
					}
					expected["spec"].(map[string]any)["template"] = input
				}
				after, err := accessor.GetObject()
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(expected, after) {
					t.Fatalf("Replace affected a different scope:\n got %#v\nwant %#v", after, expected)
				}
				afterTemplate := after["spec"].(map[string]any)["template"].(map[string]any)
				labels := afterTemplate["metadata"].(map[string]any)["labels"].(map[string]any)
				if _, ownerKept := labels["owner"]; ownerKept || labels["team"] != "platform" {
					t.Fatalf("Replace should remove owner at every tested scope: %#v", labels)
				}
				_, unknownKept := afterTemplate["extensionMap"]
				if unknownKept != (scope == "narrow labels map") {
					t.Fatalf("unknown template map preservation = %v at scope %q", unknownKept, scope)
				}
				_, containersKept, err := unstructured.NestedFieldNoCopy(afterTemplate, "spec", "containers")
				if err != nil || containersKept != (scope != "broad partial template") {
					t.Fatalf("containers preservation = %v at scope %q, error = %v", containersKept, scope, err)
				}
				// GetResource checks the Kubernetes envelope, not Deployment
				// admission. Even the incomplete broad partial result passes it.
				if _, err := factory.GetResource(); err != nil {
					t.Fatal(err)
				}
				for _, sample := range []struct {
					name  string
					value any
				}{{"before", before}, {"input", input}, {"after", after}} {
					encoded, err := json.Marshal(sample.value)
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("%s: %s", sample.name, encoded)
				}
			})
		}
	}
}
