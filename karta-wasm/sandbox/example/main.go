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
	"fmt"
	"os"

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
	wasm := mustRead(os.Args[1])
	definitionJSON := toJSON(mustRead(os.Args[2]))
	workloadJSON := toJSON(mustRead(os.Args[3]))

	ctx := context.Background()
	box, err := sandbox.New(ctx, wasm)
	if err != nil {
		panic(err)
	}
	defer box.Close(ctx)

	// 1. build the tree, in the sandbox
	workloadTree := parse(box.BuildTree(ctx, definitionJSON, workloadJSON))

	// 2. parse it: the status, then every component and its instances
	fmt.Println("status:", workloadTree.Status.Phases)
	walk(workloadTree.Children, "  ")

	// 3. suspend, then resume. offline this flips the workload's suspend
	// INTENT (the definition's suspendActions, e.g. .spec.suspend) - the
	// Suspended phase itself appears once the operator reports it in status.
	suspended := parseRaw(box.Suspend(ctx, definitionJSON, workloadJSON))
	fmt.Println("after suspend : .spec.suspend =", field(suspended, "spec", "suspend"))

	resumed := parseRaw(box.Resume(ctx, definitionJSON, string(suspended)))
	fmt.Println("after resume  : .spec.suspend =", field(resumed, "spec", "suspend"))

	// 5. the low-level write door : set one field, read the object back
	mutated := parseRaw(box.SetField(ctx, workloadJSON, `.metadata.labels.team`, `"ml"`))
	var object map[string]any
	json.Unmarshal(mutated, &object)
	labels := object["metadata"].(map[string]any)["labels"]
	fmt.Println("after setField .metadata.labels.team=ml ->", labels)
}

func field(raw json.RawMessage, path ...string) any {
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		panic(err)
	}
	var v any = object
	for _, key := range path {
		v = v.(map[string]any)[key]
	}
	return v
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

func parse(out []byte, err error) *tree.WorkloadTree {
	raw := parseRaw(out, err)
	var t tree.WorkloadTree
	if err := json.Unmarshal(raw, &t); err != nil {
		panic(err)
	}
	return &t
}

func parseRaw(out []byte, err error) json.RawMessage {
	if err != nil {
		panic(err)
	}
	var env envelope
	if err := json.Unmarshal(out, &env); err != nil {
		panic(err)
	}
	if env.Error != "" {
		panic(env.Error)
	}
	return env.Data
}

func mustRead(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return b
}

func toJSON(y []byte) string {
	j, err := yaml.YAMLToJSON(y)
	if err != nil {
		panic(err)
	}
	return string(j)
}
