// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package main

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/run-ai/karta/pkg/references"
)

// clientReader adapts a controller-runtime reader to karta's references.ResourceReader, so a
// controller passes the client it already holds:
//
//	factory := resource.NewComponentFactoryFromObject(karta, workload,
//	    resource.WithReferenceReader(&clientReader{reader: mgr.GetAPIReader()}))
//
// The one line every adapter must get right is the not-found translation: karta's contract is
// the client-agnostic references.ErrNotFound, never an apierrors status error.
type clientReader struct {
	reader client.Reader
}

func (r *clientReader) Get(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(gvk)
	err := r.reader.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, object)
	if apierrors.IsNotFound(err) {
		return nil, references.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return object, nil
}

func (r *clientReader) List(ctx context.Context, gvk schema.GroupVersionKind, query references.ListQuery) ([]unstructured.Unstructured, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)
	err := r.reader.List(ctx, list,
		client.InNamespace(query.Namespace),
		client.MatchingLabelsSelector{Selector: query.Selector})
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}
