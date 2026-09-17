// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package blackbox

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/resource"
)

func TestKServePathSelectionCharacterization(t *testing.T) {
	for _, fixture := range []struct {
		name       string
		key        string
		storageURI any
		omitURI    bool
		second     bool
		wantPath   string
	}{
		{name: "missing storageUri", key: "model", omitURI: true},
		{name: "null storageUri", key: "model"},
		{name: "false storageUri", key: "model", storageURI: false},
		{name: "empty string still matches", key: "model", storageURI: "", wantPath: "/spec/predictor/model"},
		{name: "model flavor", key: "model", storageURI: "s3://example-bucket/model", wantPath: "/spec/predictor/model"},
		{name: "sklearn flavor", key: "sklearn", storageURI: "s3://example-bucket/model", wantPath: "/spec/predictor/sklearn"},
		// This is a selector characterization, not a valid KServe schema claim.
		// Rejecting ambiguous shapes intentionally replaces the old sorted-first selection.
		{name: "two matches reject selection", key: "sklearn", storageURI: "s3://example-bucket/model", second: true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
				t.Run(string(format), func(t *testing.T) {
					model := map[string]any{
						"image": "ghcr.io/example/inference:v1", "modelFormat": map[string]any{"name": "sklearn"},
						"env":       []any{map[string]any{"name": "MODE", "value": "serve"}},
						"extension": map[string]any{"setting": "keep"},
					}
					if !fixture.omitURI {
						model["storageUri"] = fixture.storageURI
					}
					predictor := map[string]any{
						fixture.key: model, "minReplicas": float64(2),
						"containers": []any{map[string]any{"name": "sidecar", "image": "ghcr.io/example/helper:v1"}},
					}
					if fixture.second {
						predictor["model"] = runtime.DeepCopyJSONValue(model)
					}
					object := map[string]any{
						"apiVersion": "serving.kserve.io/v1beta1", "kind": "InferenceService", "metadata": map[string]any{"name": "example-model"},
						"spec": map[string]any{"predictor": predictor},
					}
					before := runtime.DeepCopyJSON(object)
					options := resource.MutationOptions{PatchType: format, Strategy: resource.Merge}
					factory, accessor := newCatalogPathFactory(t, kartas.KServe(), object, options)
					component, err := factory.GetComponent("predictor")
					if err != nil {
						t.Fatal(err)
					}
					definition := component.Definition()
					container := definition.SpecDefinition.FragmentedPodSpecDefinition.Container
					ctx := context.Background()
					target, resolveErr := accessor.ResolveWriteTarget(ctx, container, "", 0)
					if !reflect.DeepEqual(before, object) {
						t.Fatal("resolving a target changed the caller's workload")
					}
					if fixture.wantPath == "" {
						if resolveErr == nil || !strings.Contains(resolveErr.Error(), "must return one JSON Pointer string") {
							t.Fatalf("nonunique model should produce an unresolved destination, got %+v, %v", target, resolveErr)
						}
						err := accessor.WriteValues(ctx, definition, container, []any{map[string]any{
							"image": "ghcr.io/example/inference:v2", "storageUri": "s3://example-bucket/incoming",
						}}, options)
						if err == nil {
							t.Fatal("write to a nonunique model was accepted")
						}
						updated, err := accessor.GetObject()
						if err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(before, updated) || !reflect.DeepEqual(before, object) {
							t.Fatal("failed selector write changed the workload")
						}
						return
					}
					if resolveErr != nil || target.Path != fixture.wantPath || !target.Exists {
						t.Fatalf("selected target = %+v, error = %v; want %q", target, resolveErr, fixture.wantPath)
					}
					if target.Value.(map[string]any)["image"] != "ghcr.io/example/inference:v1" {
						t.Fatalf("wrong initial model value: %#v", target.Value)
					}
					if err := accessor.WriteValues(ctx, definition, &v1alpha1.ValueAccessor{PathWrite: ptr.To(target.Path + "/image")},
						[]any{"ghcr.io/example/inference:v2"}, options); err != nil {
						t.Fatal(err)
					}
					updated, err := accessor.GetObject()
					if err != nil {
						t.Fatal(err)
					}
					selectedKey := strings.TrimPrefix(fixture.wantPath, "/spec/predictor/")
					expected := runtime.DeepCopyJSON(before)
					expected["spec"].(map[string]any)["predictor"].(map[string]any)[selectedKey].(map[string]any)["image"] = "ghcr.io/example/inference:v2"
					if !reflect.DeepEqual(expected, updated) {
						t.Fatalf("narrow write changed storageUri, env, unknown fields, sidecars, or another model:\n got %#v\nwant %#v", updated, expected)
					}
					if !reflect.DeepEqual(before, object) {
						t.Fatal("narrow write mutated the caller's input object")
					}
				})
			}
		})
	}
}
