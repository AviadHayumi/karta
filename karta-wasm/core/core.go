// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package core provides Karta operations shared by the browser and WASI doors,
// keeping both transports as thin input and output adapters.
package core

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog"
	"github.com/run-ai/karta/pkg/jq/execution"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

// DecodeDefinition parses a Karta definition from JSON.
func DecodeDefinition(definitionJSON string) (*v1alpha1.Karta, error) {
	var definition v1alpha1.Karta
	if err := json.Unmarshal([]byte(definitionJSON), &definition); err != nil {
		return nil, fmt.Errorf("failed to unmarshal definition: %w", err)
	}
	return &definition, nil
}

// DecodeWorkload parses a workload from JSON into an unstructured object, so
// fields from any Kubernetes resource type survive.
func DecodeWorkload(workloadJSON string) (*unstructured.Unstructured, error) {
	var workload map[string]any
	if err := json.Unmarshal([]byte(workloadJSON), &workload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workload: %w", err)
	}
	return &unstructured.Unstructured{Object: workload}, nil
}

// BuildTree builds the workload tree from the definition and workload,
// including the root status. tree.Build validates the definition first.
func BuildTree(ctx context.Context, definitionJSON, workloadJSON string) (*tree.WorkloadTree, error) {
	definition, err := DecodeDefinition(definitionJSON)
	if err != nil {
		return nil, err
	}
	workload, err := DecodeWorkload(workloadJSON)
	if err != nil {
		return nil, err
	}
	componentFactory := resource.NewComponentFactoryFromObject(definition, workload)
	return tree.Build(ctx, componentFactory)
}

// SetField applies a single field write to the workload: it assigns value at
// the jq path (a plain field path like .spec.schedulerName) and returns the
// mutated object as JSON. This is the low-level write door; the typed,
// capability-checked UpdatePodTemplate is the higher-level way.
func SetField(ctx context.Context, workloadJSON, path string, value any) ([]byte, error) {
	workload, err := DecodeWorkload(workloadJSON)
	if err != nil {
		return nil, err
	}
	runner := execution.NewDefaultRunner(workload.Object)
	if err := runner.Assign(ctx, path, value); err != nil {
		return nil, fmt.Errorf("assign %s: %w", path, err)
	}
	object, err := runner.GetObject()
	if err != nil {
		return nil, err
	}
	return json.Marshal(object)
}

// Suspend applies every component's suspend actions to the workload and
// returns the mutated object as JSON.
func Suspend(ctx context.Context, definitionJSON, workloadJSON string) ([]byte, error) {
	return applySuspendActions(ctx, definitionJSON, workloadJSON, func(ctx context.Context, component *resource.Component) error {
		return component.Suspend(ctx)
	})
}

// Resume applies every component's resume actions to the workload and returns
// the mutated object as JSON.
func Resume(ctx context.Context, definitionJSON, workloadJSON string) ([]byte, error) {
	return applySuspendActions(ctx, definitionJSON, workloadJSON, func(ctx context.Context, component *resource.Component) error {
		return component.Resume(ctx)
	})
}

func applySuspendActions(ctx context.Context, definitionJSON, workloadJSON string, apply func(context.Context, *resource.Component) error) ([]byte, error) {
	definition, err := DecodeDefinition(definitionJSON)
	if err != nil {
		return nil, err
	}
	workload, err := DecodeWorkload(workloadJSON)
	if err != nil {
		return nil, err
	}
	factory := resource.NewComponentFactoryFromObject(definition, workload)
	root, err := factory.GetRootComponent()
	if err != nil {
		return nil, err
	}
	children, err := factory.GetChildComponents()
	if err != nil {
		return nil, err
	}
	for _, component := range append(children, root) {
		if !component.HasSuspendDefinition() {
			continue
		}
		if err := apply(ctx, component); err != nil {
			return nil, err
		}
	}
	object, err := factory.GetResource()
	if err != nil {
		return nil, err
	}
	return json.Marshal(object)
}

// ListCatalog returns the Karta definitions embedded at build time.
func ListCatalog() []*v1alpha1.Karta {
	return catalog.List()
}
