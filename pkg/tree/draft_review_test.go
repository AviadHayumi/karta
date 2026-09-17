// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree_test

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/tree"
)

func TestDraftReviewZeroValueSnapshot(t *testing.T) {
	var zero tree.Draft
	if snapshot := zero.Snapshot(); snapshot != nil {
		t.Fatalf("zero-value draft returned snapshot %#v", snapshot)
	}
	var missing *tree.Draft
	if snapshot := missing.Snapshot(); snapshot != nil {
		t.Fatalf("nil draft returned snapshot %#v", snapshot)
	}
}

func TestDraftReviewCopiesShareTerminalStateAndAnchors(t *testing.T) {
	for _, reason := range []string{"failed edit", "abort", "overlapping target"} {
		t.Run(reason, func(t *testing.T) {
			editor, before := draftReviewEditor(t)
			draft, err := tree.BeginEdit(t.Context(), editor)
			if err != nil {
				t.Fatal(err)
			}
			alias := *draft
			target := tree.Target{Component: "pod", Field: tree.PodTemplateSpec}
			root, err := draft.Target(t.Context(), target)
			if err != nil {
				t.Fatal(err)
			}
			if err := root.At("spec", "schedulerName").Set("changed-scheduler"); err != nil {
				t.Fatal(err)
			}
			switch reason {
			case "failed edit":
				if err := root.At("spec", "containers").Set("invalid-list-replacement"); err == nil {
					t.Fatal("scalar Set unexpectedly replaced a list")
				}
			case "abort":
				draft.Abort()
			case "overlapping target":
				if _, err := alias.Target(t.Context(), target); err == nil {
					t.Error("copied draft bypassed overlapping target rejection")
				}
			}
			if err := alias.Commit(t.Context()); err == nil {
				t.Error("copied draft published after a terminal operation")
			}
			if snapshot := draft.Snapshot(); snapshot != nil {
				t.Error("original draft remained usable after terminal operation through an alias")
			}
			after, err := editor.GetResource()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after.(*unstructured.Unstructured).Object, before.Object) {
				t.Fatal("copied draft published an incomplete edit intention")
			}
		})
	}
}

func TestDraftReviewCopiedCommitClosesEveryAlias(t *testing.T) {
	editor, _ := draftReviewEditor(t)
	draft, err := tree.BeginEdit(t.Context(), editor)
	if err != nil {
		t.Fatal(err)
	}
	alias := *draft
	root, err := draft.Target(t.Context(), tree.Target{Component: "pod", Field: tree.PodTemplateSpec})
	if err != nil {
		t.Fatal(err)
	}
	scheduler := root.At("spec", "schedulerName")
	if err := scheduler.Set("changed-scheduler"); err != nil {
		t.Fatal(err)
	}
	if err := alias.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Set("after-commit"); err == nil {
		t.Fatal("cursor remained usable after committing a copied draft")
	}
	if snapshot := draft.Snapshot(); snapshot != nil {
		t.Fatal("original draft remained readable after committing its copy")
	}
}

func TestDraftReviewInvalidUTF8KeyCannotOverwriteSibling(t *testing.T) {
	editor, before := draftReviewEditor(t)
	draft, err := tree.BeginEdit(t.Context(), editor)
	if err != nil {
		t.Fatal(err)
	}
	root, err := draft.Target(t.Context(), tree.Target{Component: "pod", Field: tree.PodTemplateSpec})
	if err != nil {
		t.Fatal(err)
	}
	err = root.At("metadata", "annotations", string([]byte{0xff})).Set("overwrite")
	if err == nil {
		err = draft.Commit(t.Context())
	}
	if err == nil {
		t.Error("invalid UTF-8 key was accepted and normalized into an existing sibling key")
	}
	after, err := editor.GetResource()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.(*unstructured.Unstructured).Object, before.Object) {
		t.Fatal("invalid UTF-8 key changed the untouched replacement-character annotation")
	}
}

func TestDraftReviewInvalidUTF8MatchCannotSelectAnotherIdentifier(t *testing.T) {
	type identifier string
	invalid := string([]byte{0xff})
	named := identifier(invalid)
	pointer := &named
	var boxed any = pointer
	var cycle any
	cycle = &cycle
	for _, selector := range []any{invalid, named, &invalid, pointer, &pointer, &boxed, cycle} {
		t.Run(fmt.Sprintf("%T", selector), func(t *testing.T) {
			_, original := draftReviewEditor(t)
			container := original.Object["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)
			container["selector"] = "\uFFFD"
			editor, err := tree.Open(t.Context(), kartas.Pod(), original)
			if err != nil {
				t.Fatal(err)
			}
			draft, err := tree.BeginEdit(t.Context(), editor)
			if err != nil {
				t.Fatal(err)
			}
			root, err := draft.Target(t.Context(), tree.Target{Component: "pod", Field: tree.PodTemplateSpec})
			if err != nil {
				t.Fatal(err)
			}
			if err := root.At("spec", "schedulerName").Set("staged"); err != nil {
				t.Fatal(err)
			}
			if _, err := root.At("spec", "containers").Match("selector", selector); err == nil {
				t.Fatal("invalid selector normalized into a different stored identifier")
			}
			if err := draft.Commit(t.Context()); err == nil {
				t.Fatal("ignored invalid selector error permitted publication")
			}
			after, err := editor.GetResource()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after.(*unstructured.Unstructured).Object, original.Object) {
				t.Fatal("invalid selector changed the raw workload")
			}
		})
	}
	valid := identifier("main")
	var null *identifier
	for _, selector := range []any{valid, &valid, null} {
		t.Run("valid/"+fmt.Sprintf("%T", selector), func(t *testing.T) {
			_, original := draftReviewEditor(t)
			original.Object["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)["nullable"] = nil
			editor, err := tree.Open(t.Context(), kartas.Pod(), original)
			if err != nil {
				t.Fatal(err)
			}
			draft, err := tree.BeginEdit(t.Context(), editor)
			if err != nil {
				t.Fatal(err)
			}
			defer draft.Abort()
			root, err := draft.Target(t.Context(), tree.Target{Component: "pod", Field: tree.PodTemplateSpec})
			if err != nil {
				t.Fatal(err)
			}
			key := "name"
			if selector == null {
				key = "nullable"
			}
			if _, err := root.At("spec", "containers").Match(key, selector); err != nil {
				t.Fatalf("valid named string selector rejected: %v", err)
			}
		})
	}
}

func draftReviewEditor(t *testing.T) (tree.Editable, *unstructured.Unstructured) {
	t.Helper()
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]any{
			"name": "review-pod", "annotations": map[string]any{"\uFFFD": "untouched"},
		},
		"spec": map[string]any{
			"schedulerName": "original-scheduler",
			"containers":    []any{map[string]any{"name": "main", "image": "ghcr.io/example/app:v1"}},
		},
	}}
	editor, err := tree.Open(t.Context(), kartas.Pod(), object)
	if err != nil {
		t.Fatal(err)
	}
	return editor, object
}
