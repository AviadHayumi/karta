// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

//go:build js && wasm

package main

import (
	"context"
	"fmt"
	"syscall/js"

	"github.com/run-ai/karta/pkg/catalog"
	"github.com/run-ai/karta/pkg/instructions"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

type PodComponentMatch struct {
	PodIndex      int     `json:"podIndex"`
	ComponentName string  `json:"componentName"`
	InstanceKey   *string `json:"instanceKey,omitempty"`
}

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

func jsInferPodComponents(_ js.Value, args []js.Value) any {
	if len(args) != 3 {
		return encodeEnvelope(nil, fmt.Errorf("kartaInferPodComponents: expected 3 arguments, got %d", len(args)))
	}
	ctx := context.Background()

	definition, err := decodeDefinition(args[0].String())
	if err != nil {
		return encodeEnvelope(nil, err)
	}
	workload, err := decodeWorkload(args[1].String())
	if err != nil {
		return encodeEnvelope(nil, err)
	}
	pods, err := decodePods(args[2].String())
	if err != nil {
		return encodeEnvelope(nil, err)
	}

	factory := resource.NewComponentFactoryFromObject(definition, workload)
	summary, err := instructions.NewStructureSummary(definition)
	if err != nil {
		return encodeEnvelope(nil, fmt.Errorf("failed to build structure summary: %w", err))
	}

	matches := make([]PodComponentMatch, 0, len(pods))
	for i := range pods {
		querier := resource.NewPodQuerier(&pods[i])

		componentName, err := instructions.InferPodComponent(ctx, querier, summary)
		if err != nil {
			continue
		}

		instanceKey, err := instructions.InferPodComponentInstance(ctx, querier, componentName, factory)
		if err != nil {
			continue
		}

		matches = append(matches, PodComponentMatch{PodIndex: i, ComponentName: componentName, InstanceKey: instanceKey})
	}

	return encodeEnvelope(matches, nil)
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

	factory := resource.NewComponentFactoryFromObject(definition, workload)
	workloadTree, err := tree.Build(context.Background(), factory)
	if err != nil {
		return encodeEnvelope(nil, err)
	}
	if workloadTree.Status == nil {
		return encodeEnvelope([]string{}, nil)
	}
	return encodeEnvelope(workloadTree.Status.Phases, nil)
}

func jsListCatalog(js.Value, []js.Value) any {
	return encodeEnvelope(catalog.List(), nil)
}
