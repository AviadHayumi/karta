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
	"errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// ErrNotFound is returned by a ResourceReader's Get when the object does not exist. A lookup
// that finds nothing is not an error by itself: the reference stays unbound and only an
// expression that reads it fails.
var ErrNotFound = errors.New("resource not found")

// ListQuery describes which objects List returns. New capabilities (field selectors, limits)
// are added as fields, so the interface never breaks.
type ListQuery struct {
	Namespace string
	Selector  labels.Selector
}

// ResourceReader reads cluster resources for reference resolution. Two methods, plain
// arguments, a sentinel not-found error: a cluster consumer bridges its client with one small
// adapter, a non-cluster consumer implements the methods over its own data.
type ResourceReader interface {
	// Get returns the object's content, or ErrNotFound if it does not exist.
	Get(ctx context.Context, gvk v1alpha1.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error)
	// List returns every object of gvk matching the query.
	List(ctx context.Context, gvk v1alpha1.GroupVersionKind, query ListQuery) ([]unstructured.Unstructured, error)
}

// PermissionChecker is optionally implemented by a ResourceReader that can answer whether the
// caller may read a kind before trying. When the reader implements it, Resolve asks before
// every fetch and a denial surfaces as an error naming the reference and the missing verb,
// instead of a bare API failure mid-resolution.
type PermissionChecker interface {
	// CanRead returns nil when verb ("get" or "list") is allowed on gvk in namespace, and an
	// error explaining what is missing otherwise.
	CanRead(ctx context.Context, gvk v1alpha1.GroupVersionKind, namespace, verb string) error
}

// ReferenceValue is the fetched value of one reference.
type ReferenceValue struct {
	// Object is set for a lookup; nil when the object was not found (the reference stays
	// unbound, so only an expression that reads it fails).
	Object *unstructured.Unstructured
	// List is set for a list; possibly empty.
	List []unstructured.Unstructured
}

// ResolvedReferences maps each reference name to its fetched value.
type ResolvedReferences map[string]ReferenceValue

// Bindings converts the resolved values into the shape the expression engine binds as
// references.<name>: a lookup is its object content, a list is a list of object contents.
// A lookup that found nothing is absent from the map, so plain access fails on use and
// optional access can supply a default.
func (r ResolvedReferences) Bindings() map[string]any {
	out := make(map[string]any, len(r))
	for name, value := range r {
		switch {
		case value.Object != nil:
			out[name] = value.Object.Object
		case value.List != nil:
			items := make([]any, 0, len(value.List))
			for _, item := range value.List {
				items = append(items, item.Object)
			}
			out[name] = items
		}
	}

	return out
}
