// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

//go:build wasip1

// Command karta-wasm is the WASI door: one request on stdin
// ({definition, workload}), the workload tree on stdout. It calls the same
// core the browser door calls - no separate guest, no op protocol.
package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/run-ai/karta/karta-wasm/core"
)

type request struct {
	Definition string `json:"definition"`
	Workload   string `json:"workload"`
}

type response struct {
	Data  any    `json:"data"`
	Error string `json:"error,omitempty"`
}

func main() {
	var req request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		writeError(err)
		return
	}
	workloadTree, err := core.BuildTree(context.Background(), req.Definition, req.Workload)
	if err != nil {
		writeError(err)
		return
	}
	_ = json.NewEncoder(os.Stdout).Encode(response{Data: workloadTree})
}

func writeError(err error) {
	_ = json.NewEncoder(os.Stdout).Encode(response{Error: err.Error()})
	os.Exit(1)
}
