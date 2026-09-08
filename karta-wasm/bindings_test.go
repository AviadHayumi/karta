// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

//go:build js && wasm

package main

import (
	"encoding/json"
	"slices"
	"syscall/js"
	"testing"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/types"
)

func TestJsBuildTree(t *testing.T) {
	definitionJSON := marshal(t, types.ReactorKarta())
	workloadJSON := marshal(t, types.NewReactorObject())

	env := jsBuildTree(js.Value{}, []js.Value{js.ValueOf(definitionJSON), js.ValueOf(workloadJSON)}).(js.Value)
	if !env.Get("error").IsNull() {
		t.Fatalf("unexpected error: %s", env.Get("error").String())
	}

	var tree struct {
		Status   *struct{ Phases []string }
		Children []struct{ Name string }
	}
	if err := json.Unmarshal([]byte(env.Get("data").String()), &tree); err != nil {
		t.Fatalf("failed to unmarshal tree: %v", err)
	}
	if tree.Status == nil || len(tree.Status.Phases) != 1 || tree.Status.Phases[0] != "Running" {
		t.Fatalf("expected Status.Phases = [Running], got %#v", tree.Status)
	}
	if len(tree.Children) != 1 || tree.Children[0].Name != "service" {
		t.Fatalf("expected a single %q component, got %#v", "service", tree.Children)
	}
}

func TestJsBuildTree_WrongArgCount(t *testing.T) {
	env := jsBuildTree(js.Value{}, []js.Value{js.ValueOf("{}")}).(js.Value)

	if env.Get("error").IsNull() {
		t.Fatal("expected an error for a missing argument")
	}
}

func TestJsEvaluatePhases(t *testing.T) {
	definitionJSON := marshal(t, types.ReactorKarta())
	workloadJSON := marshal(t, types.NewReactorObject())

	env := jsEvaluatePhases(js.Value{}, []js.Value{js.ValueOf(definitionJSON), js.ValueOf(workloadJSON)}).(js.Value)
	if !env.Get("error").IsNull() {
		t.Fatalf("unexpected error: %s", env.Get("error").String())
	}

	var phases []string
	if err := json.Unmarshal([]byte(env.Get("data").String()), &phases); err != nil {
		t.Fatalf("failed to unmarshal phases: %v", err)
	}
	if len(phases) != 1 || phases[0] != "Running" {
		t.Fatalf("expected phases = [Running], got %#v", phases)
	}
}

// jsEvaluatePhases reaches the root status without building the tree, so it
// duplicates what tree.Build does with that status. This pins the two together.
func TestJsEvaluatePhases_MatchesTreeBuild(t *testing.T) {
	for _, tc := range []struct {
		name       string
		definition *v1alpha1.Karta
		workload   any
	}{
		{"reactor", types.ReactorKarta(), types.NewReactorObject()},
		{"pyflow", types.PyFlowKarta(), types.NewPyFlowObject()},
		{"milvus", types.MilvusKarta(), types.NewMilvusObject()},
		{"jobgroup", types.JobGroupKarta(), types.NewJobGroupObject()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			definitionJSON := marshal(t, tc.definition)
			workloadJSON := marshal(t, tc.workload)

			env := jsEvaluatePhases(js.Value{}, []js.Value{
				js.ValueOf(definitionJSON), js.ValueOf(workloadJSON),
			}).(js.Value)
			if !env.Get("error").IsNull() {
				t.Fatalf("unexpected error: %s", env.Get("error").String())
			}
			var phases []string
			if err := json.Unmarshal([]byte(env.Get("data").String()), &phases); err != nil {
				t.Fatalf("failed to unmarshal phases: %v", err)
			}

			treeEnv := jsBuildTree(js.Value{}, []js.Value{
				js.ValueOf(definitionJSON), js.ValueOf(workloadJSON),
			}).(js.Value)
			if !treeEnv.Get("error").IsNull() {
				t.Fatalf("unexpected error building the tree: %s", treeEnv.Get("error").String())
			}
			var built struct {
				Status *struct{ Phases []string }
			}
			if err := json.Unmarshal([]byte(treeEnv.Get("data").String()), &built); err != nil {
				t.Fatalf("failed to unmarshal tree: %v", err)
			}

			var want []string
			if built.Status != nil {
				want = built.Status.Phases
			}
			if !slices.Equal(phases, want) {
				t.Errorf("phases = %#v, tree.Build's Status.Phases = %#v", phases, want)
			}
		})
	}
}

func TestJsEvaluatePhases_WrongArgCount(t *testing.T) {
	env := jsEvaluatePhases(js.Value{}, nil).(js.Value)

	if env.Get("error").IsNull() {
		t.Fatal("expected an error for a missing argument")
	}
}

func TestJsListCatalog(t *testing.T) {
	env := jsListCatalog(js.Value{}, nil).(js.Value)
	if !env.Get("error").IsNull() {
		t.Fatalf("unexpected error: %s", env.Get("error").String())
	}

	var entries []map[string]any
	if err := json.Unmarshal([]byte(env.Get("data").String()), &entries); err != nil {
		t.Fatalf("failed to unmarshal catalog: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected the embedded catalog to be non-empty")
	}
}
