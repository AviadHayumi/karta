// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package resource

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	jsonpatch "gopkg.in/evanphx/json-patch.v4"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// mergePatch merges a constructed partial object into the live one, the way a
// MutatingAdmissionPolicy applyConfiguration mutation is merged: maps merge recursively, any other
// value replaces, and null removes the field. Both engines evaluate the patch expression; this
// merge is shared, so an apply cannot differ between them.
func mergePatch(live, patch any) any {
	patchMap, ok := patch.(map[string]any)
	if !ok {
		return patch
	}
	liveMap, ok := live.(map[string]any)
	if !ok {
		liveMap = map[string]any{}
	}

	merged := make(map[string]any, len(liveMap)+len(patchMap))
	for key, value := range liveMap {
		merged[key] = value
	}
	for key, value := range patchMap {
		if value == nil {
			delete(merged, key)

			continue
		}
		merged[key] = mergePatch(merged[key], value)
	}

	return merged
}

// ensureAddParents creates the missing map parents an add operation writes under.
// RFC 6902 add requires the parent container to exist, so without this a write into
// spec.podSpec.affinity.nodeAffinity fails on every workload that has no affinity yet. Only map
// segments are created; a numeric segment addresses an existing list element. The document passed
// in must be a scratch copy: parents created for a patch that later fails must not leak into the
// live document.
func ensureAddParents(doc any, path string) {
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	isIndex := func(segment string) bool {
		_, err := strconv.Atoi(segment)

		return err == nil || segment == "-"
	}
	current := doc
	for i, segment := range segments[:max(len(segments)-1, 0)] {
		// JSON pointer escaping, RFC 6901: ~1 is a literal slash, ~0 a literal tilde
		segment = strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
		switch node := current.(type) {
		case map[string]any:
			child, ok := node[segment]
			if !ok || child == nil {
				// a numeric or append segment - here or right below - addresses a list
				// element: a map keyed "0" is never right, so nothing is created and
				// the patch fails the way RFC 6902 defines
				if isIndex(segment) || isIndex(segments[i+1]) {
					return
				}
				child = map[string]any{}
				node[segment] = child
			}
			current = child
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(node) {
				return
			}
			current = node[index]
		default:
			return
		}
	}
}

// checkConstructedPatch checks what a patch expression built against the entry's
// declared patchType: a map for MergePatch, a list of RFC 6902 operations for
// JSONPatch. Anything else is an error, never a guess. An empty map or list means no
// change, reported through the empty result.
func checkConstructedPatch(constructed any, patchType v1alpha1.PatchType) (any, bool, error) {
	switch patchType {
	case v1alpha1.PatchTypeMergePatch:
		patch, ok := constructed.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("a MergePatch expression must build a map, got %T", constructed)
		}

		return patch, len(patch) == 0, nil
	case v1alpha1.PatchTypeJSONPatch:
		ops, ok := constructed.([]any)
		if !ok {
			return nil, false, fmt.Errorf("a JSONPatch expression must build a list of RFC 6902 operations, got %T", constructed)
		}
		for _, op := range ops {
			entry, isMap := op.(map[string]any)
			if !isMap {
				return nil, false, fmt.Errorf("a JSONPatch list must hold RFC 6902 operations, got %T", op)
			}
			if _, hasOp := entry["op"]; !hasOp {
				return nil, false, fmt.Errorf("a patch operation names an op, got %v", entry)
			}
		}

		return ops, len(ops) == 0, nil
	default:
		return nil, false, fmt.Errorf("unknown patch type %q", patchType)
	}
}

// applyConstructedPatch applies a checked patch by its declared type. A JSONPatch is
// applied as RFC 6902 operations - the mechanism a MutatingAdmissionPolicy jsonPatch
// mutation uses, and the one that can address a list element by index. A MergePatch
// merges, applyConfiguration style.
func applyConstructedPatch(live, constructed any, patchType v1alpha1.PatchType) (any, error) {
	if patchType != v1alpha1.PatchTypeJSONPatch {
		return mergePatch(live, constructed), nil
	}
	ops, ok := constructed.([]any)
	if !ok {
		return nil, fmt.Errorf("a JSONPatch expression must build a list of RFC 6902 operations, got %T", constructed)
	}
	liveJSON, err := json.Marshal(live)
	if err != nil {
		return nil, err
	}
	// The operations apply one at a time against a scratch copy: an add creates its missing map
	// parents just before it runs - not before earlier operations - and a failure anywhere
	// leaves the caller's document untouched.
	var scratch any
	if err := json.Unmarshal(liveJSON, &scratch); err != nil {
		return nil, err
	}
	for opIndex, op := range ops {
		entry := op.(map[string]any)
		if entry["op"] == "add" {
			if path, ok := entry["path"].(string); ok {
				ensureAddParents(scratch, path)
			}
		}
		opJSON, err := json.Marshal([]any{entry})
		if err != nil {
			return nil, err
		}
		patch, err := jsonpatch.DecodePatch(opJSON)
		if err != nil {
			return nil, fmt.Errorf("decode JSON patch: %w", err)
		}
		scratchJSON, err := json.Marshal(scratch)
		if err != nil {
			return nil, err
		}
		patched, err := patch.Apply(scratchJSON)
		if err != nil {
			return nil, fmt.Errorf("apply JSON patch operation %d (%v %v): %w", opIndex, entry["op"], entry["path"], err)
		}
		scratch = nil
		if err := json.Unmarshal(patched, &scratch); err != nil {
			return nil, err
		}
	}

	return scratch, nil
}
