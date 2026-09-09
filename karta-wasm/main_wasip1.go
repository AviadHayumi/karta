// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

//go:build wasip1

// Command karta-wasm is the WASI door: one request on stdin, the result on
// stdout. It calls the same core the browser door calls - no separate guest.
//
//	{"op":"buildTree","definition":"<json>","workload":"<json>"}
//	{"op":"setField","workload":"<json>","path":".spec.schedulerName","value":"kai"}
//	{"op":"suspend","definition":"<json>","workload":"<json>"}
//	{"op":"resume","definition":"<json>","workload":"<json>"}
package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/run-ai/karta/karta-wasm/core"
)

type request struct {
	Op         string `json:"op"`
	Definition string `json:"definition,omitempty"`
	Workload   string `json:"workload"`
	Path       string `json:"path,omitempty"`
	Value      any    `json:"value,omitempty"`
}

type response struct {
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

func main() {
	var req request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fail(err)
	}
	ctx := context.Background()

	switch req.Op {
	case "buildTree":
		workloadTree, err := core.BuildTree(ctx, req.Definition, req.Workload)
		if err != nil {
			fail(err)
		}
		emit(workloadTree)
	case "suspend":
		object, err := core.Suspend(ctx, req.Definition, req.Workload)
		if err != nil {
			fail(err)
		}
		emitRaw(object)
	case "resume":
		object, err := core.Resume(ctx, req.Definition, req.Workload)
		if err != nil {
			fail(err)
		}
		emitRaw(object)
	case "setField":
		object, err := core.SetField(ctx, req.Workload, req.Path, req.Value)
		if err != nil {
			fail(err)
		}
		emitRaw(object)
	default:
		emitError("unknown op " + req.Op)
	}
}

func emit(v any) {
	data, _ := json.Marshal(v)
	emitRaw(data)
}
func emitRaw(data []byte) {
	_ = json.NewEncoder(os.Stdout).Encode(response{Data: data})
}
func emitError(msg string) {
	_ = json.NewEncoder(os.Stdout).Encode(response{Error: msg})
	os.Exit(1)
}
func fail(err error) { emitError(err.Error()) }
