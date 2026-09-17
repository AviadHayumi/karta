// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// PatchType selects an SDK mutation format, not a Karta definition field.
type PatchType string

const (
	PatchTypeJSONPatch  PatchType = "JSONPatch"
	PatchTypeMergePatch PatchType = "MergePatch"
)

// MergeStrategy controls the ownership of the selected subtree.
type MergeStrategy string

const (
	Merge   MergeStrategy = "Merge"
	Replace MergeStrategy = "Replace"
)

// ParentPolicy controls missing object parents. Arrays are never manufactured.
type ParentPolicy string

const (
	RequireParents   ParentPolicy = "Require"
	CreateMapParents ParentPolicy = "CreateMaps"
)

// MutationOptions belong to the caller. Zero values use MergePatch, Merge, and
// CreateMaps. Merge preserves unspecified map keys; arrays are always replaced.
// JSONPatch keeps literal nulls. MergePatch uses RFC 7396 null-deletes semantics.
type MutationOptions struct {
	PatchType PatchType
	Strategy  MergeStrategy
	Parents   ParentPolicy
}

// WriteTarget describes a concrete location in the current document snapshot.
// Value is detached from the runner. Resolve again after changing the document.
type WriteTarget struct {
	Path   string
	Exists bool
	Value  any
}

type resolvedWrite struct {
	target WriteTarget
	parts  []string
	input  any
}

func (o MutationOptions) normalized() (MutationOptions, error) {
	if o.PatchType == "" {
		o.PatchType = PatchTypeMergePatch
	}
	if o.Strategy == "" {
		o.Strategy = Merge
	}
	if o.Parents == "" {
		o.Parents = CreateMapParents
	}
	if o.PatchType != PatchTypeJSONPatch && o.PatchType != PatchTypeMergePatch {
		return o, fmt.Errorf("unknown SDK patch type %q", o.PatchType)
	}
	if o.Strategy != Merge && o.Strategy != Replace {
		return o, fmt.Errorf("unknown SDK merge strategy %q", o.Strategy)
	}
	if o.Parents != RequireParents && o.Parents != CreateMapParents {
		return o, fmt.Errorf("unknown SDK parent policy %q", o.Parents)
	}
	return o, nil
}

// ResolveWriteTarget evaluates only the destination, never a patch or caller
// value. index must correspond to instance in the current instanceIds result.
func (a *Accessor) ResolveWriteTarget(ctx context.Context, via *v1alpha1.ValueAccessor, instance string, index int) (WriteTarget, error) {
	if via == nil || (via.PathWrite == nil && via.PathWriteExpression == "") {
		return WriteTarget{}, fmt.Errorf("the field has no write path and is read-only")
	}
	if via.PathWrite != nil && via.PathWriteExpression != "" {
		return WriteTarget{}, fmt.Errorf("pathWrite and pathWriteExpression are mutually exclusive")
	}
	var path string
	if via.PathWrite != nil {
		path = *via.PathWrite
	} else {
		result, err := a.runner.EvaluateWritePath(ctx, via.PathWriteExpression, map[string]any{"instance": instance, "index": index})
		if err != nil {
			return WriteTarget{}, fmt.Errorf("evaluate pathWriteExpression: %w", err)
		}
		if len(result) != 1 {
			return WriteTarget{}, fmt.Errorf("pathWriteExpression must return one JSON Pointer string")
		}
		var ok bool
		path, ok = result[0].(string)
		if !ok {
			return WriteTarget{}, fmt.Errorf("pathWriteExpression must return one JSON Pointer string, got %T", result[0])
		}
	}
	segments, err := pointerSegments(path)
	if err != nil {
		return WriteTarget{}, err
	}
	object, err := a.runner.GetObject()
	if err != nil {
		return WriteTarget{}, err
	}
	value, exists, err := readPointer(object, segments)
	if err != nil {
		return WriteTarget{}, err
	}
	if exists {
		value, err = jsonCopy(value)
		if err != nil {
			return WriteTarget{}, err
		}
	}
	return WriteTarget{Path: path, Value: value, Exists: exists}, nil
}

// WriteValues writes one value per component instance using caller-owned policy.
// Every target resolves before any changes are published. An error leaves the
// runner unchanged, including failures on later instances or invalid parents.
func (a *Accessor) WriteValues(ctx context.Context, def v1alpha1.ComponentDefinition, via *v1alpha1.ValueAccessor, values []any, options MutationOptions) error {
	options, err := options.normalized()
	if err != nil {
		return err
	}
	ids, err := a.resolveWriteInstances(ctx, def, len(values))
	if err != nil {
		return err
	}
	writes, err := a.resolveWrites(ctx, via, ids, values)
	if err != nil {
		return err
	}
	return a.applyResolvedWrites(ctx, writes, options)
}

func (a *Accessor) resolveWriteInstances(ctx context.Context, def v1alpha1.ComponentDefinition, count int) ([]string, error) {
	var ids []string
	var err error
	if def.InstanceIds != nil && def.InstanceIds.Expression != "" {
		ids, err = a.ExtractInstanceIds(ctx, def)
		if err != nil {
			return nil, fmt.Errorf("resolve write instances: %w", err)
		}
		if len(ids) != count {
			return nil, fmt.Errorf("got %d values for %d instances", count, len(ids))
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				return nil, fmt.Errorf("duplicate instance ID %q", id)
			}
			seen[id] = true
		}
	} else if count != 1 {
		return nil, fmt.Errorf("a single-instance field requires one value, got %d", count)
	}
	return ids, nil
}

func (a *Accessor) resolveWrites(ctx context.Context, via *v1alpha1.ValueAccessor, ids []string, values []any) ([]resolvedWrite, error) {
	writes := make([]resolvedWrite, len(values))
	var err error
	for i := range values {
		instance := ""
		if len(ids) > 0 {
			instance = ids[i]
		}
		writes[i].target, err = a.ResolveWriteTarget(ctx, via, instance, i)
		if err != nil {
			return nil, fmt.Errorf("target %d: %w", i, err)
		}
		writes[i].parts, err = pointerSegments(writes[i].target.Path)
		if err != nil {
			return nil, err
		}
		writes[i].input = values[i]
		for j := 0; j < i; j++ {
			if pointersOverlap(writes[i].parts, writes[j].parts) {
				return nil, fmt.Errorf("overlapping write targets %q and %q", writes[j].target.Path, writes[i].target.Path)
			}
		}
	}
	return writes, nil
}

func (a *Accessor) applyResolvedWrites(ctx context.Context, writes []resolvedWrite, options MutationOptions) error {
	live, err := a.runner.GetObject()
	if err != nil {
		return err
	}
	scratch, err := jsonCopy(live)
	if err != nil {
		return err
	}
	for i, write := range writes {
		if err := ctx.Err(); err != nil {
			return err
		}
		input, err := jsonCopy(write.input)
		if err != nil {
			return fmt.Errorf("value %d: %w", i, err)
		}
		current := write.target.Value
		if options.Strategy == Replace {
			current = nil
		}
		var next any
		if options.PatchType == PatchTypeMergePatch {
			next = mergePatch(current, input)
		} else if options.Strategy == Merge {
			next = mergeJSONValue(current, input)
		} else {
			next = input
		}
		remove := input == nil && options.PatchType == PatchTypeMergePatch && len(write.parts) > 0
		if options.PatchType == PatchTypeJSONPatch {
			scratch, err = writeJSONPointer(scratch, write.target.Path, write.parts, next, options.Parents)
		} else {
			scratch, err = writePointer(scratch, write.parts, next, remove, options.Parents)
		}
		if err != nil {
			return fmt.Errorf("write %q: %w", write.target.Path, err)
		}
	}
	if err := validateWriteResult(live, scratch); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.runner.Assign(ctx, ".", scratch)
}

// ApplyPatch is the SDK escape hatch for caller-built JSONPatch operations or a
// JSON Merge Patch document. It does not evaluate CEL. JSONPatch uses the existing
// Karta CreateMaps extension; target writes have an explicit parent policy.
func (a *Accessor) ApplyPatch(ctx context.Context, patchType PatchType, patch any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	patch, err := jsonCopy(patch)
	if err != nil {
		return err
	}
	if patchType != PatchTypeJSONPatch && patchType != PatchTypeMergePatch {
		return fmt.Errorf("unknown SDK patch type %q", patchType)
	}
	if patchType == PatchTypeJSONPatch {
		if _, _, err := checkConstructedPatch(patch, patchType); err != nil {
			return err
		}
	}
	live, err := a.runner.GetObject()
	if err != nil {
		return err
	}
	updated, err := applyConstructedPatch(live, patch, patchType)
	if err != nil {
		return err
	}
	if err := validateWriteResult(live, updated); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.runner.Assign(ctx, ".", updated)
}

func validateWriteResult(live, updated any) error {
	object, ok := updated.(map[string]any)
	if !ok {
		return fmt.Errorf("a workload root must remain an object")
	}
	// Generic JSON accessors need no Kubernetes envelope, but replacing a valid
	// workload with a smaller typed view must not destroy its required identity.
	original, ok := live.(map[string]any)
	if !ok || validateKubernetesObject(&unstructured.Unstructured{Object: original}) != nil {
		return nil
	}
	if err := validateKubernetesObject(&unstructured.Unstructured{Object: object}); err != nil {
		return fmt.Errorf("invalid mutated Kubernetes object: %w", err)
	}
	return nil
}

func jsonCopy(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var copy any
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, err
	}
	return copy, nil
}

func pointerSegments(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("write path %q must be an RFC 6901 JSON Pointer", path)
	}
	parts := strings.Split(path[1:], "/")
	if len(parts) > 128 {
		return nil, fmt.Errorf("write path exceeds 128 segments")
	}
	for i, part := range parts {
		for j := 0; j < len(part); j++ {
			if part[j] != '~' {
				continue
			}
			if j+1 >= len(part) || (part[j+1] != '0' && part[j+1] != '1') {
				return nil, fmt.Errorf("invalid JSON Pointer escape in %q", path)
			}
			j++
		}
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts, nil
}

func pointerIndex(segment string, length int) (int, error) {
	i, err := strconv.Atoi(segment)
	if err != nil || i < 0 || i >= length || strconv.Itoa(i) != segment {
		return 0, fmt.Errorf("%q is not an existing array index", segment)
	}
	return i, nil
}

func readPointer(doc any, parts []string) (any, bool, error) {
	current := doc
	for _, part := range parts {
		switch node := current.(type) {
		case map[string]any:
			var exists bool
			current, exists = node[part]
			if !exists {
				return nil, false, nil
			}
		case []any:
			i, err := pointerIndex(part, len(node))
			if err != nil {
				return nil, false, err
			}
			current = node[i]
		case nil:
			return nil, false, nil
		default:
			return nil, false, fmt.Errorf("cannot traverse %q through %T", part, node)
		}
	}
	return current, true, nil
}

func writePointer(doc any, parts []string, value any, remove bool, parents ParentPolicy) (any, error) {
	if len(parts) == 0 {
		return value, nil
	}
	if doc == nil {
		if remove {
			return nil, nil
		}
		if parents != CreateMapParents {
			return nil, fmt.Errorf("missing parent")
		}
		if _, err := strconv.Atoi(parts[0]); err == nil || parts[0] == "-" {
			return nil, fmt.Errorf("cannot infer a missing array parent")
		}
		doc = map[string]any{}
	}
	part := parts[0]
	switch node := doc.(type) {
	case map[string]any:
		if len(parts) == 1 {
			if remove {
				delete(node, part)
			} else {
				node[part] = value
			}
			return node, nil
		}
		child, exists := node[part]
		if !exists && remove {
			return node, nil
		}
		next, err := writePointer(child, parts[1:], value, remove, parents)
		if err != nil {
			return nil, err
		}
		node[part] = next
		return node, nil
	case []any:
		i, err := pointerIndex(part, len(node))
		if err != nil {
			return nil, err
		}
		if len(parts) == 1 && remove {
			return nil, fmt.Errorf("MergePatch cannot delete an array item; use SDK JSONPatch remove")
		}
		next, err := writePointer(node[i], parts[1:], value, remove, parents)
		if err != nil {
			return nil, err
		}
		node[i] = next
		return node, nil
	default:
		return nil, fmt.Errorf("cannot traverse %q through %T", part, doc)
	}
}

// writeJSONPointer builds a concrete RFC 6902 assignment. Merging, when selected,
// was computed by the SDK before this operation; JSONPatch never evaluates CEL.
func writeJSONPointer(doc any, path string, parts []string, value any, parents ParentPolicy) (any, error) {
	op := "add"
	if len(parts) > 0 {
		parentParts := parts[:len(parts)-1]
		last := parts[len(parts)-1]
		parent, exists, err := readPointer(doc, parentParts)
		if err != nil {
			return nil, err
		}
		if !exists || parent == nil {
			if parents != CreateMapParents {
				return nil, fmt.Errorf("missing parent")
			}
			if _, err := strconv.Atoi(last); err == nil || last == "-" {
				return nil, fmt.Errorf("cannot infer a missing array parent")
			}
			parent = map[string]any{}
			doc, err = writePointer(doc, parentParts, parent, false, parents)
			if err != nil {
				return nil, err
			}
		}
		switch node := parent.(type) {
		case []any:
			if _, err := pointerIndex(last, len(node)); err != nil {
				return nil, err
			}
			op = "replace"
		case map[string]any:
		default:
			return nil, fmt.Errorf("cannot traverse %q through %T", last, parent)
		}
	}
	return applyConstructedPatch(doc, []any{map[string]any{"op": op, "path": path, "value": value}}, PatchTypeJSONPatch)
}

func pointersOverlap(a, b []string) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// JSONPatch's Merge strategy is SDK sugar, not an RFC 6902 operation.
func mergeJSONValue(live, incoming any) any {
	patch, ok := incoming.(map[string]any)
	if !ok {
		return incoming
	}
	current, _ := live.(map[string]any)
	merged := make(map[string]any, len(current)+len(patch))
	for key, value := range current {
		merged[key] = value
	}
	for key, value := range patch {
		merged[key] = mergeJSONValue(merged[key], value)
	}
	return merged
}
