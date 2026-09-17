// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package v1alpha1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/tree"
)

// Examples and karta-verify currently use permissive yaml.Unmarshal. A validator
// cannot report removed fields after decoding has already discarded them.
func TestMigrationGapPermissiveDecodeDiscardsLegacyWrites(t *testing.T) {
	for _, fixture := range []struct {
		name         string
		spec         string
		unknownField string
		writeError   string
	}{
		{
			name: "old CEL patches and patchStrategy",
			spec: `{"podTemplateSpec": {
  "expression": "object.spec.template",
  "patchStrategy": "Replace",
  "patches": [{"patchType": "MergePatch", "expression": "{'spec': {'template': value}}"}]
}}`,
			unknownField: "patchStrategy", writeError: "read-only",
		},
		{
			name: "old jq podTemplateSpecPath", spec: `{"podTemplateSpecPath": ".spec.template"}`,
			unknownField: "podTemplateSpecPath", writeError: `no field "podTemplateSpec"`,
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			input := []byte(fmt.Sprintf(`{
  "apiVersion": "run.ai/v1alpha1", "kind": "Karta", "metadata": {"name": "example-deployment"},
  "spec": {"structureDefinition": {"rootComponent": {
    "name": "deployment", "kind": {"group": "apps", "version": "v1", "kind": "Deployment"},
    "statusDefinition": {}, "specDefinition": %s
  }}}
}`, fixture.spec))
			for _, mode := range []string{"ordinary JSON", "ordinary YAML"} {
				t.Run(mode, func(t *testing.T) {
					definition := &v1alpha1.Karta{}
					if mode == "ordinary JSON" {
						if err := json.Unmarshal(input, definition); err != nil {
							t.Fatal(err)
						}
					} else {
						if err := yaml.Unmarshal(input, definition); err != nil {
							t.Fatal(err)
						}
					}
					if err := v1alpha1.NewKartaValidator(definition).Validate(); err != nil {
						t.Fatalf("the current validator unexpectedly detected discarded legacy fields: %v", err)
					}
					accessor := definition.Spec.StructureDefinition.RootComponent.SpecDefinition.PodTemplateSpec
					if fixture.writeError == "read-only" {
						if accessor == nil || accessor.Expression != "object.spec.template" || accessor.PathWrite != nil || accessor.PathWriteExpression != "" {
							t.Fatalf("old CEL should retain only its read expression, got %#v", accessor)
						}
					} else if accessor != nil {
						t.Fatalf("old jq field should be discarded, got %#v", accessor)
					}
					workload := &unstructured.Unstructured{Object: map[string]any{
						"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]any{"name": "example-api"},
						"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
							"schedulerName": "default-scheduler", "containers": []any{map[string]any{"name": "main", "image": "ghcr.io/example/api:v1"}},
						}}},
					}}
					ctx := context.Background()
					editor, err := tree.Open(ctx, definition, workload)
					if err != nil {
						t.Fatal(err)
					}
					before, err := editor.GetResource()
					if err != nil {
						t.Fatal(err)
					}
					beforeTree := editor.Snapshot()
					err = editor.Mutate(ctx, tree.Write{Component: "deployment", Field: tree.PodTemplateSpec,
						Value: map[string]any{"spec": map[string]any{"schedulerName": "batch-scheduler"}}})
					if err == nil || !strings.Contains(err.Error(), fixture.writeError) {
						t.Fatalf("expected lost write capability %q, got %v", fixture.writeError, err)
					}
					after, err := editor.GetResource()
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(beforeTree, editor.Snapshot()) {
						t.Fatal("failed legacy-definition write changed workload or extraction")
					}
				})
			}
			t.Run("strict decoding rejects legacy field", func(t *testing.T) {
				decoder := json.NewDecoder(strings.NewReader(string(input)))
				decoder.DisallowUnknownFields()
				var definition v1alpha1.Karta
				err := decoder.Decode(&definition)
				if err == nil || !strings.Contains(err.Error(), `unknown field "`+fixture.unknownField+`"`) {
					t.Fatalf("strict JSON decode should identify legacy field %q, got %v", fixture.unknownField, err)
				}
				if err := yaml.UnmarshalStrict(input, &definition); err == nil {
					t.Fatal("strict YAML decode accepted removed legacy fields")
				}
			})
		})
	}
}
