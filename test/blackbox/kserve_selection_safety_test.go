// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package blackbox

import (
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

func TestKServeAmbiguousContainerRejectsReadsAndBatchWrites(t *testing.T) {
	for _, format := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		t.Run(string(format), func(t *testing.T) {
			// This diagnostic input deliberately violates the singular model selection contract.
			object := map[string]any{
				"apiVersion": "serving.kserve.io/v1beta1", "kind": "InferenceService",
				"metadata": map[string]any{"name": "ambiguous-model"},
				"spec": map[string]any{"predictor": map[string]any{
					"schedulerName": "original-scheduler",
					"model": map[string]any{
						"storageUri": "s3://example/first", "image": "ghcr.io/example/first:v1",
					},
					"sklearn": map[string]any{
						"storageUri": "s3://example/second", "image": "ghcr.io/example/second:v1",
					},
				}},
			}
			before := runtime.DeepCopyJSON(object)
			factory, accessor := newCatalogPathFactory(t, kartas.KServe(), object, resource.MutationOptions{PatchType: format})
			predictor, err := factory.GetComponent("predictor")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := predictor.GetFragmentedPodSpec(t.Context()); err == nil || !strings.Contains(err.Error(), "cannot unmarshal array") {
				t.Fatalf("ambiguous read must reject multiple containers, got %v", err)
			}
			if err := predictor.UpdateFragmentedPodSpec(t.Context(), map[string]resource.FragmentedPodSpec{
				"": {
					SchedulerName: "changed-scheduler",
					Container:     &corev1.Container{Image: "ghcr.io/example/changed:v2"},
				},
			}); err == nil {
				t.Fatal("batch containing an ambiguous container write succeeded")
			}
			updated, err := accessor.GetObject()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(updated, before) || !reflect.DeepEqual(object, before) {
				t.Fatalf("rejected batch changed the workload: got %#v, want %#v", updated, before)
			}

			if _, err := tree.Open(t.Context(), kartas.KServe(), &unstructured.Unstructured{Object: object}); err == nil || !strings.Contains(err.Error(), "cannot unmarshal array") {
				t.Fatalf("opening an ambiguous model must fail visibly, got %v", err)
			}
			if !reflect.DeepEqual(object, before) {
				t.Fatal("rejected tree open changed the caller's workload")
			}
		})
	}
}

func TestKServeNoModelRetainsReadCompatibilityAndRejectsWrite(t *testing.T) {
	object := map[string]any{
		"apiVersion": "serving.kserve.io/v1beta1", "kind": "InferenceService",
		"metadata": map[string]any{"name": "custom-container"},
		"spec": map[string]any{"predictor": map[string]any{
			"containers": []any{map[string]any{"name": "custom", "image": "ghcr.io/example/custom:v1"}},
		}},
	}
	before := runtime.DeepCopyJSON(object)
	editor, err := tree.Open(t.Context(), kartas.KServe(), &unstructured.Unstructured{Object: object})
	if err != nil {
		t.Fatal(err)
	}
	if err := editor.Mutate(t.Context(), tree.Write{
		Component: "predictor", Field: tree.Container,
		Value: map[string]any{"image": "ghcr.io/example/changed:v2", "storageUri": "s3://example/incoming"},
	}); err == nil {
		t.Fatal("incoming value supplied a missing model selection")
	}
	updated, err := editor.GetResource()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(updated.(*unstructured.Unstructured).Object, before) || !reflect.DeepEqual(object, before) {
		t.Fatal("unresolved model write changed the workload")
	}
}
