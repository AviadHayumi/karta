// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

//go:build js && wasm

package main

import (
	"context"
	"fmt"
	"syscall/js"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

func jsBuildTree(_ js.Value, args []js.Value) any {
	if len(args) != 2 {
		return encodeEnvelope(nil, fmt.Errorf("kartaBuildTree: expected 2 arguments, got %d", len(args)))
	}
	definition, err := decodeDefinition(args[0].String())
	if err != nil {
		return encodeEnvelope(nil, err)
	}
	workload, err := decodeWorkload(args[1].String())
	if err != nil {
		return encodeEnvelope(nil, err)
	}

	factory := resource.NewComponentFactoryFromObject(definition, workload)
	workloadTree, err := tree.Build(context.Background(), factory)
	return encodeEnvelope(workloadTree, err)
}

func jsEvaluatePhases(_ js.Value, args []js.Value) any {
	if len(args) != 2 {
		return encodeEnvelope(nil, fmt.Errorf("kartaEvaluatePhases: expected 2 arguments, got %d", len(args)))
	}
	definition, err := decodeDefinition(args[0].String())
	if err != nil {
		return encodeEnvelope(nil, err)
	}
	workload, err := decodeWorkload(args[1].String())
	if err != nil {
		return encodeEnvelope(nil, err)
	}

	if err := v1alpha1.NewKartaValidator(definition).Validate(); err != nil {
		return encodeEnvelope(nil, fmt.Errorf("invalid karta: %w", err))
	}

	factory := resource.NewComponentFactoryFromObject(definition, workload)
	rootComponent, err := factory.GetRootComponent()
	if err != nil {
		return encodeEnvelope(nil, fmt.Errorf("failed to get root component: %w", err))
	}

	status, err := rootComponent.GetStatus(context.Background())
	if err != nil {
		return encodeEnvelope(nil, fmt.Errorf("failed to get root status: %w", err))
	}
	if status == nil {
		return encodeEnvelope([]string{}, nil)
	}

	phases := make([]string, len(status.MatchedStatuses))
	for i, matched := range status.MatchedStatuses {
		phases[i] = string(matched)
	}
	return encodeEnvelope(phases, nil)
}

func jsListCatalog(js.Value, []js.Value) any {
	return encodeEnvelope(catalog.List(), nil)
}
