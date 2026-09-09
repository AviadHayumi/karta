// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package resource

import (
	"encoding/json"
	"testing"
)

func TestMergePatch(t *testing.T) {
	cases := []struct{ name, live, patch, want string }{
		{"maps merge recursively",
			`{"spec":{"replicas":2,"paused":false}}`,
			`{"spec":{"paused":true}}`,
			`{"spec":{"paused":true,"replicas":2}}`},
		{"a list replaces , not appends",
			`{"spec":{"containers":[{"name":"a"},{"name":"b"}]}}`,
			`{"spec":{"containers":[{"name":"c"}]}}`,
			`{"spec":{"containers":[{"name":"c"}]}}`},
		{"null removes the field",
			`{"metadata":{"labels":{"keep":"1","drop":"2"}}}`,
			`{"metadata":{"labels":{"drop":null}}}`,
			`{"metadata":{"labels":{"keep":"1"}}}`},
		{"a scalar replaces a map",
			`{"spec":{"x":{"deep":true}}}`,
			`{"spec":{"x":7}}`,
			`{"spec":{"x":7}}`},
		{"a map lands on a missing field",
			`{"spec":{}}`,
			`{"spec":{"new":{"a":1}}}`,
			`{"spec":{"new":{"a":1}}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var live, patch, want any
			mustJSON(t, c.live, &live)
			mustJSON(t, c.patch, &patch)
			mustJSON(t, c.want, &want)
			got, _ := json.Marshal(mergePatch(live, patch))
			expected, _ := json.Marshal(want)
			if string(got) != string(expected) {
				t.Fatalf("got %s, want %s", got, expected)
			}
		})
	}
}

func mustJSON(t *testing.T, s string, into *any) {
	t.Helper()
	if err := json.Unmarshal([]byte(s), into); err != nil {
		t.Fatal(err)
	}
}

func TestApplyConstructedPatchAddParents(t *testing.T) {
	live := map[string]any{"spec": map[string]any{"template": map[string]any{"cliques": []any{
		map[string]any{"name": "leader", "spec": map[string]any{"podSpec": map[string]any{
			"containers": []any{map[string]any{"name": "c", "image": "i"}},
		}}},
	}}}}
	ops := []any{map[string]any{
		"op":    "add",
		"path":  "/spec/template/cliques/0/spec/podSpec/affinity/nodeAffinity",
		"value": map[string]any{"requiredDuringSchedulingIgnoredDuringExecution": map[string]any{}},
	}}
	out, err := applyConstructedPatch(live, ops)
	if err != nil {
		t.Fatalf("add under a missing affinity parent: %v", err)
	}
	clique := out.(map[string]any)["spec"].(map[string]any)["template"].(map[string]any)["cliques"].([]any)[0].(map[string]any)
	affinity, ok := clique["spec"].(map[string]any)["podSpec"].(map[string]any)["affinity"].(map[string]any)
	if !ok {
		t.Fatal("the affinity parent was not created")
	}
	if _, ok := affinity["nodeAffinity"]; !ok {
		t.Fatal("nodeAffinity was not written")
	}

	badIndex := []any{map[string]any{"op": "add", "path": "/spec/template/cliques/3/spec/affinity", "value": map[string]any{}}}
	if _, err := applyConstructedPatch(live, badIndex); err == nil {
		t.Fatal("an out-of-range list index must stay an error")
	}
	if _, stray := live["spec"].(map[string]any)["template"].(map[string]any)["cliques"].([]any)[0].(map[string]any)["spec"].(map[string]any)["affinity"]; stray {
		t.Fatal("a failed patch must not leave created parents in the live document")
	}

	wholeElement := map[string]any{"spec": map[string]any{}}
	elementAdd := []any{map[string]any{"op": "add", "path": "/spec/containers/0", "value": map[string]any{"name": "c"}}}
	if _, err := applyConstructedPatch(wholeElement, elementAdd); err == nil {
		t.Fatal("adding element 0 of a missing list must stay an error, never build a map keyed 0")
	}

	missingList := map[string]any{"spec": map[string]any{}}
	numericParent := []any{map[string]any{"op": "add", "path": "/spec/containers/0/image", "value": "img"}}
	if _, err := applyConstructedPatch(missingList, numericParent); err == nil {
		t.Fatal("a numeric segment under a missing list must stay an error, never become a map key")
	}
	if _, stray := missingList["spec"].(map[string]any)["containers"]; stray {
		t.Fatal("the missing list must not be created at all")
	}

	escaped := map[string]any{"metadata": map[string]any{}}
	ops2 := []any{map[string]any{"op": "add", "path": "/metadata/a~1b/leaf", "value": "v"}}
	out2, err := applyConstructedPatch(escaped, ops2)
	if err != nil {
		t.Fatalf("escaped pointer segment: %v", err)
	}
	parent, ok := out2.(map[string]any)["metadata"].(map[string]any)["a/b"].(map[string]any)
	if !ok || parent["leaf"] != "v" {
		t.Fatalf("the ~1 segment must unescape to a literal slash, got %v", out2)
	}
}
