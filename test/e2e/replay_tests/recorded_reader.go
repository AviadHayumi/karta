// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package replay

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/references"
)

// recordedReader serves the referenced CRs a recording captured with one state, so a definition
// that declares references replays offline exactly as it would read a live cluster.
type recordedReader struct {
	objects []*unstructured.Unstructured
}

func (r *recordedReader) Get(_ context.Context, gvk kartav1alpha1.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
	for _, object := range r.objects {
		if !matchesGVK(object, gvk) || object.GetName() != name {
			continue
		}
		if object.GetNamespace() != "" && namespace != "" && object.GetNamespace() != namespace {
			continue
		}

		return object, nil
	}

	return nil, references.ErrNotFound
}

func (r *recordedReader) List(_ context.Context, gvk kartav1alpha1.GroupVersionKind, query references.ListQuery) ([]unstructured.Unstructured, error) {
	var out []unstructured.Unstructured
	for _, object := range r.objects {
		if !matchesGVK(object, gvk) {
			continue
		}
		if object.GetNamespace() != "" && query.Namespace != "" && object.GetNamespace() != query.Namespace {
			continue
		}
		if query.Selector != nil && !query.Selector.Matches(labels.Set(object.GetLabels())) {
			continue
		}
		out = append(out, *object)
	}

	return out, nil
}

func matchesGVK(object *unstructured.Unstructured, gvk kartav1alpha1.GroupVersionKind) bool {
	got := object.GroupVersionKind()

	return got == schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind}
}
