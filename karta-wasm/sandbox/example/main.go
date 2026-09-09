// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Command example shows how a host consumes the karta-wasm WASI door, all
// inside the sandbox: build a workload tree, parse it by walking every
// (component, instance), then mutate a field and read the result back.
//
//	go run ./example <karta-wasi.wasm> <definition.yaml> <workload.yaml>
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/karta-wasm/sandbox"
	"github.com/run-ai/karta/pkg/tree"
)

func walk(components []tree.ComponentNode, indent string) {
	for _, component := range components {
		for _, instance := range component.Instances {
			id := "-"
			if instance.InstanceKey != nil {
				id = *instance.InstanceKey
			}
			fmt.Printf("%scomponent %-14s instance %-10s pods:%v\n", indent, component.Name, id, component.HasPodDefinition)
			walk(instance.Children, indent+"  ")
		}
	}
}

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: example <karta-wasi.wasm> <definition.yaml> <workload.yaml>")
		os.Exit(1)
	}
	if err := run(os.Args[1], os.Args[2], os.Args[3]); err != nil {
		fmt.Fprintln(os.Stderr, "example:", err)
		os.Exit(1)
	}
}

func run(wasmPath, definitionPath, workloadPath string) (returnErr error) {
	wasm, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("read wasm: %w", err)
	}
	definitionJSON, err := readYAMLAsJSON(definitionPath)
	if err != nil {
		return err
	}
	workloadJSON, err := readYAMLAsJSON(workloadPath)
	if err != nil {
		return err
	}

	ctx := context.Background()
	box, err := sandbox.New(ctx, wasm)
	if err != nil {
		return fmt.Errorf("create sandbox: %w", err)
	}
	defer func() {
		if err := box.Close(ctx); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("close sandbox: %w", err)
		}
	}()

	// 1. build the tree, in the sandbox
	treeJSON, err := parseRaw(box.BuildTree(ctx, definitionJSON, workloadJSON))
	if err != nil {
		return fmt.Errorf("build tree: %w", err)
	}
	var workloadTree tree.WorkloadTree
	if err := json.Unmarshal(treeJSON, &workloadTree); err != nil {
		return fmt.Errorf("decode workload tree: %w", err)
	}
	if workloadTree.Status == nil {
		return errors.New("workload tree has no status")
	}

	// 2. parse it: the status, then every component and its instances
	fmt.Println("status:", workloadTree.Status.Phases)
	walk(workloadTree.Children, "  ")

	// 3. suspend, then resume. offline this flips the workload's suspend
	// INTENT (the definition's suspendActions, e.g. .spec.suspend) - the
	// Suspended phase itself appears once the operator reports it in status.
	suspended, err := parseRaw(box.Suspend(ctx, definitionJSON, workloadJSON))
	if err != nil {
		return fmt.Errorf("suspend workload: %w", err)
	}
	suspendedValue, err := field(suspended, "spec", "suspend")
	if err != nil {
		return err
	}
	fmt.Println("after suspend : .spec.suspend =", suspendedValue)

	resumed, err := parseRaw(box.Resume(ctx, definitionJSON, string(suspended)))
	if err != nil {
		return fmt.Errorf("resume workload: %w", err)
	}
	resumedValue, err := field(resumed, "spec", "suspend")
	if err != nil {
		return err
	}
	fmt.Println("after resume  : .spec.suspend =", resumedValue)

	// 4. the low-level write door: set one field, read the object back
	mutated, err := parseRaw(box.SetField(ctx, workloadJSON, `.metadata.labels.team`, `"ml"`))
	if err != nil {
		return fmt.Errorf("set workload field: %w", err)
	}
	labels, err := field(mutated, "metadata", "labels")
	if err != nil {
		return err
	}
	fmt.Println("after setField .metadata.labels.team=ml ->", labels)
	return nil
}

func field(raw json.RawMessage, path ...string) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("decode workload: %w", err)
	}
	for _, key := range path {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("read field .%s: parent is %T, not an object", strings.Join(path, "."), value)
		}
		value, ok = object[key]
		if !ok {
			return nil, fmt.Errorf("field .%s not found", strings.Join(path, "."))
		}
	}
	return value, nil
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

func parseRaw(out []byte, runErr error) (json.RawMessage, error) {
	if runErr != nil {
		return nil, runErr
	}
	var env envelope
	if err := json.Unmarshal(out, &env); err != nil {
		return nil, fmt.Errorf("decode sandbox response: %w", err)
	}
	if env.Error != "" {
		return nil, errors.New(env.Error)
	}
	if len(env.Data) == 0 {
		return nil, errors.New("sandbox response has no data")
	}
	return env.Data, nil
}

func readYAMLAsJSON(path string) (string, error) {
	yamlBytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	jsonBytes, err := yaml.YAMLToJSON(yamlBytes)
	if err != nil {
		return "", fmt.Errorf("convert %s to JSON: %w", path, err)
	}
	return string(jsonBytes), nil
}
