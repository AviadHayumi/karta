// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package references resolves a definition's resource references: the named pointers a Karta
// declares to other cluster resources, exposed to its expressions as references.<name>.
//
// The package owns the reading contract but no client. A consumer with a cluster adapts the
// client it already holds to ResourceReader; a consumer without one implements the two methods
// over its own store. Karta itself never imports a Kubernetes client.
package references

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ErrNotFound is returned by a ResourceReader's Get when the object does not exist. A lookup
// that finds nothing is not an error by itself: the reference stays unbound and only an
// expression that reads it fails.
var ErrNotFound = errors.New("resource not found")

// ListQuery describes which objects List returns. New capabilities (field selectors, limits)
// are added as fields; construct it with field names so additions stay compatible.
type ListQuery struct {
	Namespace string
	Selector  labels.Selector
}

// ResourceReader reads cluster resources for reference resolution. Two methods, plain
// arguments, a sentinel not-found error: a cluster consumer bridges its client with one small
// adapter, a non-cluster consumer implements the methods over its own data.
type ResourceReader interface {
	// Get returns the object's content, or ErrNotFound if it does not exist.
	Get(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error)
	// List returns every object of gvk matching the query.
	List(ctx context.Context, gvk schema.GroupVersionKind, query ListQuery) ([]unstructured.Unstructured, error)
}

// PermissionChecker is optionally implemented by a ResourceReader that can answer whether the
// caller may read a kind before trying. When the reader implements it, Resolve asks before
// every fetch and a denial surfaces as an error naming the reference and the missing verb,
// instead of a bare API failure mid-resolution.
type PermissionChecker interface {
	// CheckRead returns nil when verb ("get" or "list") is allowed on gvk in namespace, and an
	// error explaining what is missing otherwise.
	CheckRead(ctx context.Context, gvk schema.GroupVersionKind, namespace, verb string) error
}

// ReferenceValue is the fetched value of one reference: exactly one of Object or List is set.
// Build values with LookupValue and ListValue, which canonicalize the empty cases; the zero
// value means a lookup that found nothing (the reference stays unbound, so only an expression
// that reads it fails). When both fields are set, Object wins.
type ReferenceValue struct {
	// Object is set for a lookup; nil when the object was not found.
	Object *unstructured.Unstructured
	// List is set for a list; empty (never nil) when nothing matched.
	List []unstructured.Unstructured
}

// LookupValue is the resolved value of a lookup reference; pass nil for a miss.
func LookupValue(object *unstructured.Unstructured) ReferenceValue {
	return ReferenceValue{Object: object}
}

// ListValue is the resolved value of a list reference; a nil slice canonicalizes to empty, so
// an empty match binds as an empty list rather than staying unbound.
func ListValue(items []unstructured.Unstructured) ReferenceValue {
	if items == nil {
		items = []unstructured.Unstructured{}
	}

	return ReferenceValue{List: items}
}

// ResolvedReferences maps each reference name to its fetched value.
type ResolvedReferences map[string]ReferenceValue

// Bindings converts the resolved values into the shape the expression engine binds as
// references.<name>: a lookup is its object content, a list is a list of object contents
// (empty when nothing matched). A lookup that found nothing is absent from the map, so plain
// access fails on use and optional access can supply a default. Values are normalized through
// JSON so a referenced object has the same value domain as the workload document, whether it
// came from a live cluster or a recording; a value that cannot round-trip is an error naming
// the reference.
func (r ResolvedReferences) Bindings() (map[string]any, error) {
	out := make(map[string]any, len(r))
	for name, value := range r {
		switch {
		case value.Object != nil:
			normalized, err := normalize(value.Object.Object)
			if err != nil {
				return nil, fmt.Errorf("reference %q: %w", name, err)
			}
			out[name] = normalized
		case value.List != nil:
			items := make([]any, 0, len(value.List))
			for _, item := range value.List {
				normalized, err := normalize(item.Object)
				if err != nil {
					return nil, fmt.Errorf("reference %q: %w", name, err)
				}
				items = append(items, normalized)
			}
			out[name] = items
		}
	}

	return out, nil
}

// normalize round-trips a value through JSON, so numbers and structures carry the same Go types
// the engine reads from the workload document.
func normalize(value map[string]any) (any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("normalize referenced object: %w", err)
	}
	var out any
	if err := json.Unmarshal(encoded, &out); err != nil {
		return nil, fmt.Errorf("normalize referenced object: %w", err)
	}

	return out, nil
}
