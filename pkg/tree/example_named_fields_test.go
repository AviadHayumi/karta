// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree_test

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

func ExampleOpen_namedFields() {
	definition := &v1alpha1.Karta{Spec: v1alpha1.KartaSpec{
		StructureDefinition: v1alpha1.StructureDefinition{
			RootComponent: v1alpha1.ComponentDefinition{
				Name:             "app",
				Kind:             &v1alpha1.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "App"},
				StatusDefinition: &v1alpha1.StatusDefinition{},
				Fields: map[string]v1alpha1.ValueAccessor{
					"exampleos": {Expression: "object.d.d.c", PathWrite: ptr.To("/d/d/c")},
				},
			},
		},
	}}
	workload := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1", "kind": "App",
		"metadata": map[string]any{"name": "example-app"},
		"d":        map[string]any{"d": map[string]any{"c": "old", "neighbor": "keep"}},
	}}
	ctx := context.Background()
	editor, err := tree.Open(ctx, definition, workload)
	if err != nil {
		panic(err)
	}
	target, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: "app", Field: tree.Field("exampleos")})
	if err != nil {
		panic(err)
	}
	fmt.Println("target:", target.Path)
	fmt.Println("before:", editor.Snapshot().Root.Instances[0].ExtractedInstance.Fields["exampleos"])
	if err := editor.Mutate(ctx, tree.Write{
		Component: "app", Field: tree.Field("exampleos"), Value: "new",
		Options: resource.MutationOptions{PatchType: resource.PatchTypeJSONPatch, Strategy: resource.Replace},
	}); err != nil {
		panic(err)
	}
	fmt.Println("after:", editor.Snapshot().Root.Instances[0].ExtractedInstance.Fields["exampleos"])
	updated, err := editor.GetResource()
	if err != nil {
		panic(err)
	}
	values := updated.(*unstructured.Unstructured).Object["d"].(map[string]any)["d"].(map[string]any)
	fmt.Println("stored:", values["c"])
	fmt.Println("neighbor:", values["neighbor"])
	// Output:
	// target: /d/d/c
	// before: old
	// after: new
	// stored: new
	// neighbor: keep
}
