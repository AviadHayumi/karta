// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package catalog

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/tree"
)

func TestGroveReplicaExtractionPreservesInstanceCardinality(t *testing.T) {
	for _, tc := range []struct {
		name     string
		template map[string]any
		want     map[string]map[string]int32
	}{
		{
			name:     "absent cliques and scaling groups",
			template: map[string]any{},
			want:     map[string]map[string]int32{"clique": {}, "scalinggroup": {}},
		},
		{
			name: "explicitly empty lists",
			template: map[string]any{
				"cliques":                []any{},
				"podCliqueScalingGroups": []any{},
			},
			want: map[string]map[string]int32{"clique": {}, "scalinggroup": {}},
		},
		{
			name: "default each missing replica without dropping or reordering IDs",
			template: map[string]any{
				"cliques": []any{
					map[string]any{"name": "specified", "spec": map[string]any{"replicas": int64(3)}},
					map[string]any{"name": "missing", "spec": map[string]any{}},
					map[string]any{"name": "zero", "spec": map[string]any{"replicas": int64(0)}},
					map[string]any{"name": "null", "spec": map[string]any{"replicas": nil}},
				},
				"podCliqueScalingGroups": []any{
					map[string]any{"name": "specified", "replicas": int64(4)},
					map[string]any{"name": "missing"},
					map[string]any{"name": "zero", "replicas": int64(0)},
					map[string]any{"name": "null", "replicas": nil},
				},
			},
			want: map[string]map[string]int32{
				"clique":       {"specified": 6, "missing": 2, "zero": 0, "null": 2},
				"scalinggroup": {"specified": 8, "missing": 2, "zero": 0, "null": 2},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			editable, err := tree.Open(t.Context(), kartas.GrovePodCliqueSet(), groveExtractionObject(tc.template))
			if err != nil {
				t.Fatal(err)
			}
			snapshot := editable.Snapshot()
			if len(snapshot.Children) != len(tc.want) {
				t.Fatalf("got %d component nodes, want %d", len(snapshot.Children), len(tc.want))
			}
			for _, component := range snapshot.Children {
				want, found := tc.want[component.Name]
				if !found || len(component.Instances) != len(want) {
					t.Fatalf("component %s has %d instances, want %d", component.Name, len(component.Instances), len(want))
				}
				for _, instance := range component.Instances {
					if instance.InstanceKey == nil || instance.Scale == nil || instance.Scale.Replicas == nil {
						t.Fatalf("component %s omitted an instance ID or replica count", component.Name)
					}
					expected, found := want[*instance.InstanceKey]
					if !found || *instance.Scale.Replicas != expected {
						t.Fatalf("%s/%s replicas = %d, want %d", component.Name, *instance.InstanceKey, *instance.Scale.Replicas, expected)
					}
				}
			}
		})
	}
}

func TestGroveReplicaExtractionRejectsWrongTypes(t *testing.T) {
	for _, value := range []any{false, "three"} {
		for _, key := range []string{"cliques", "podCliqueScalingGroups"} {
			entry := map[string]any{"name": "invalid", "replicas": value}
			if key == "cliques" {
				entry = map[string]any{"name": "invalid", "spec": map[string]any{"replicas": value}}
			}
			_, err := tree.Open(t.Context(), kartas.GrovePodCliqueSet(), groveExtractionObject(map[string]any{key: []any{entry}}))
			if err == nil {
				t.Fatalf("accepted %s replicas of type %T", key, value)
			}
		}
	}
}

func groveExtractionObject(template map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "grove.io/v1alpha1", "kind": "PodCliqueSet",
		"metadata": map[string]any{"name": "example-grove"},
		"spec":     map[string]any{"replicas": int64(2), "template": template},
	}}
}
