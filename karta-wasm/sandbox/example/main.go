// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Command example shows how a host consumes the karta-wasm WASI door: compile
// the wasm once, then build a workload tree inside the sandbox.
//
//	go run ./example karta-wasi.wasm
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/run-ai/karta/karta-wasm/sandbox"
)

const podDefinition = `{"apiVersion":"run.ai/v1alpha1","kind":"Karta","metadata":{"name":"pod"},` +
	`"spec":{"structureDefinition":{"rootComponent":{"name":"pod","kind":{"group":"","version":"v1","kind":"Pod"},` +
	`"specDefinition":{"podTemplateSpecPath":"."},"statusDefinition":{"phaseDefinition":{"path":".status.phase"},` +
	`"statusMappings":{"running":[{"byPhase":"Running"}]}}}}}}`

const pod = `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"web"},"status":{"phase":"Running"}}`

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: example karta-wasi.wasm")
		os.Exit(1)
	}
	wasm, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	box, err := sandbox.New(ctx, wasm)
	if err != nil {
		panic(err)
	}
	defer box.Close(ctx)

	tree, err := box.BuildTree(ctx, podDefinition, pod)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(tree))
}
