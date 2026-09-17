// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package resource_test

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/cel"
	"github.com/run-ai/karta/pkg/resource"
)

func ExampleAccessor_WriteValues() {
	// This destination comes from Karta. The patch format and strategy do not.
	labels := &v1alpha1.ValueAccessor{PathWrite: ptr.To("/labels")}
	input := map[string]any{"team": "platform"}
	for _, strategy := range []resource.MergeStrategy{resource.Merge, resource.Replace} {
		runner, err := cel.NewRunner(map[string]any{
			"labels": map[string]any{"team": "ml", "owner": "alice"},
		})
		if err != nil {
			panic(err)
		}
		writer := resource.NewAccessor(runner)
		err = writer.WriteValues(context.Background(), v1alpha1.ComponentDefinition{}, labels,
			[]any{input}, resource.MutationOptions{
				PatchType: resource.PatchTypeMergePatch,
				Strategy:  strategy,
			})
		if err != nil {
			panic(err)
		}
		updated, err := writer.GetObject()
		if err != nil {
			panic(err)
		}
		encoded, err := json.Marshal(updated)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s: %s\n", strategy, encoded)
	}
	// Output:
	// Merge: {"labels":{"owner":"alice","team":"platform"}}
	// Replace: {"labels":{"team":"platform"}}
}

func ExampleAccessor_ResolveWriteTarget() {
	runner, err := cel.NewRunner(map[string]any{
		"spec": map[string]any{"services": map[string]any{
			"worker/a": map[string]any{"image": "api:v1"},
		}},
	})
	if err != nil {
		panic(err)
	}
	writer := resource.NewAccessor(runner)
	image := &v1alpha1.ValueAccessor{
		PathWriteExpression: `"/spec/services/" + instance.replace("~", "~0").replace("/", "~1") + "/image"`,
	}
	target, err := writer.ResolveWriteTarget(context.Background(), image, "worker/a", 0)
	if err != nil {
		panic(err)
	}
	fmt.Printf("path=%s exists=%t current=%s\n", target.Path, target.Exists, target.Value)
	// Output:
	// path=/spec/services/worker~1a/image exists=true current=api:v1
}

func ExampleAccessor_ApplyPatch() {
	runner, err := cel.NewRunner(map[string]any{
		"labels": map[string]any{"team": "ml"},
		"image":  "api:v1",
	})
	if err != nil {
		panic(err)
	}
	writer := resource.NewAccessor(runner)
	imageFromUser := "api:v9"
	// These operations are Go caller code, not Karta CR fields or CEL.
	err = writer.ApplyPatch(context.Background(), resource.PatchTypeJSONPatch, []map[string]any{
		{"op": "copy", "from": "/labels/team", "path": "/ownerTeam"},
		{"op": "replace", "path": "/image", "value": imageFromUser},
	})
	if err != nil {
		panic(err)
	}
	updated, err := writer.GetObject()
	if err != nil {
		panic(err)
	}
	encoded, err := json.Marshal(updated)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
	// Output:
	// {"image":"api:v9","labels":{"team":"ml"},"ownerTeam":"ml"}
}
