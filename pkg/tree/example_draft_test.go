// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/tree"
)

func ExampleBeginEdit() {
	ctx := context.Background()
	workload := &unstructured.Unstructured{}
	// KServe model shape with placeholder values, not a deployable model service.
	if err := json.Unmarshal([]byte(`{
	  "apiVersion":"serving.kserve.io/v1beta1", "kind":"InferenceService",
	  "metadata":{"name":"example-model"},
	  "spec":{"predictor":{"model":{
	    "image":"ghcr.io/example/inference:v1",
	    "storageUri":"s3://example-bucket/model", "modelFormat":{"name":"sklearn"}
	  }}}
	}`), &workload.Object); err != nil {
		panic(err)
	}
	editor, err := tree.Open(ctx, kartas.KServe(), workload)
	if err != nil {
		panic(err)
	}
	draft, err := tree.BeginEdit(ctx, editor)
	if err != nil {
		panic(err)
	}
	defer draft.Abort()
	model, err := draft.Target(ctx, tree.Target{Component: "predictor", Field: tree.Container})
	if err != nil {
		panic(err)
	}
	var view corev1.Container
	if err := model.ReadInto(&view); err != nil {
		panic(err)
	}
	fmt.Println("typed image:", view.Image)
	newImage := "ghcr.io/example/inference:v2" // An external caller value.
	if err := model.At("image").Set(newImage); err != nil {
		panic(err)
	}
	path, err := model.At("image").Path()
	if err != nil {
		panic(err)
	}
	fmt.Println("resolved leaf:", path)
	if err := draft.Commit(ctx); err != nil {
		panic(err)
	}
	result, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: "predictor", Field: tree.Container})
	if err != nil {
		panic(err)
	}
	value := result.Value.(map[string]any)
	fmt.Println("after:", value["image"])
	fmt.Println("kept storage:", value["storageUri"])
	fmt.Println("kept format:", value["modelFormat"].(map[string]any)["name"])
	// Output:
	// typed image: ghcr.io/example/inference:v1
	// resolved leaf: /spec/predictor/model/image
	// after: ghcr.io/example/inference:v2
	// kept storage: s3://example-bucket/model
	// kept format: sklearn
}

func ExampleCursor_MoveBefore() {
	ctx := context.Background()
	workload := &unstructured.Unstructured{}
	// Unknown fields make this a local preservation example, not a valid Deployment.
	if err := json.Unmarshal([]byte(`{
	  "apiVersion":"apps/v1", "kind":"Deployment", "metadata":{"name":"example"},
	  "spec":{"template":{"metadata":{"labels":{"team":"ml","owner":"alice"}},"spec":{
	    "containers":[
	      {"name":"api","image":"api:v1","operatorData":{"keep":true}},
	      {"name":"sidecar","image":"sidecar:v1","operatorData":{"keep":"also"}}
	    ]
	  }}}
	}`), &workload.Object); err != nil {
		panic(err)
	}
	editor, err := tree.Open(ctx, kartas.Deployment(), workload)
	if err != nil {
		panic(err)
	}
	draft, err := tree.BeginEdit(ctx, editor)
	if err != nil {
		panic(err)
	}
	defer draft.Abort()
	template, err := draft.Target(ctx, tree.Target{Component: "deployment", Field: tree.PodTemplateSpec})
	if err != nil {
		panic(err)
	}
	containers := template.At("spec", "containers")
	api, err := containers.Match("name", "api")
	if err != nil {
		panic(err)
	}
	if err := api.MoveBefore(nil); err != nil {
		panic(err)
	} // Move the whole raw item to the end.
	if err := api.At("image").Set("api:v2"); err != nil {
		panic(err)
	}
	if err := template.At("metadata", "labels", "owner").Remove(); err != nil {
		panic(err)
	}
	// Copy a value read from the CR into another field; Set also accepts outside values.
	team, exists, err := template.At("metadata", "labels", "team").Read()
	if err != nil || !exists {
		panic("team must exist")
	}
	if err := template.At("metadata", "labels", "example.com/copied-team").Set(team); err != nil {
		panic(err)
	}
	if err := draft.Commit(ctx); err != nil {
		panic(err)
	}
	result, err := editor.ResolveWriteTarget(ctx, tree.Target{Component: "deployment", Field: tree.PodTemplateSpec})
	if err != nil {
		panic(err)
	}
	value := result.Value.(map[string]any)
	items := value["spec"].(map[string]any)["containers"].([]any)
	for _, item := range items {
		container := item.(map[string]any)
		fmt.Println(container["name"], container["image"], container["operatorData"].(map[string]any)["keep"])
	}
	labels := value["metadata"].(map[string]any)["labels"].(map[string]any)
	_, hasOwner := labels["owner"]
	fmt.Println("owner present:", hasOwner)
	fmt.Println("copied team:", labels["example.com/copied-team"])
	// Output:
	// sidecar sidecar:v1 also
	// api api:v2 true
	// owner present: false
	// copied team: ml
}

func ExampleDraft_Commit() {
	ctx := context.Background()
	workload := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "batch/v1", "kind": "Job", "metadata": map[string]any{"name": "example"},
		"spec": map[string]any{"suspend": false, "template": map[string]any{"spec": map[string]any{
			"containers": []any{map[string]any{"name": "worker", "image": "worker:v1"}},
		}}},
	}}
	editor, err := tree.Open(ctx, kartas.BatchJob(), workload)
	if err != nil {
		panic(err)
	}
	draft, err := tree.BeginEdit(ctx, editor)
	if err != nil {
		panic(err)
	}
	if err := editor.Suspend(ctx); err != nil {
		panic(err)
	}
	// Even an unchanged draft cannot commit over an intervening editor mutation.
	fmt.Println("stale:", errors.Is(draft.Commit(ctx), tree.ErrStaleDraft))
	// Open a new draft and recompute the intended changes after a conflict.
	// Output:
	// stale: true
}
