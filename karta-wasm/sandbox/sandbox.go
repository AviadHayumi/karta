// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package sandbox runs the karta-wasm WASI door under wazero, so karta reads a
// workload inside an isolated wasm instance instead of in the host process.
// The isolated instance is safer, and the cost is time: every call boots a
// fresh instance.
package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// instancePages bounds the memory a single guest instance may use.
// One wasm page is 64 KiB.
const instancePages = 2048 // 128 MiB

// Sandbox holds the compiled WASI door. It is safe for concurrent use: every
// call runs in its own fresh instance.
type Sandbox struct {
	runtime      wazero.Runtime
	compiled     wazero.CompiledModule
	moduleConfig wazero.ModuleConfig
}

type request struct {
	Op         string           `json:"op"`
	Definition string           `json:"definition,omitempty"`
	Workload   string           `json:"workload,omitempty"`
	Path       string           `json:"path,omitempty"`
	Value      *json.RawMessage `json:"value,omitempty"`
}

type guestResponse struct {
	Error string `json:"error"`
}

// New compiles the karta-wasm WASI door once. Reuse the returned Sandbox across
// calls; compilation is the expensive step.
func New(ctx context.Context, wasm []byte) (*Sandbox, error) {
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
		WithMemoryLimitPages(instancePages).
		WithCloseOnContextDone(true))
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("instantiate WASI: %w", err)
	}
	compiled, err := runtime.CompileModule(ctx, wasm)
	if err != nil {
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("compile guest: %w", err)
	}
	return &Sandbox{
		runtime:      runtime,
		compiled:     compiled,
		moduleConfig: wazero.NewModuleConfig().WithArgs("karta-wasi").WithName(""),
	}, nil
}

// Close releases the runtime.
func (s *Sandbox) Close(ctx context.Context) error { return s.runtime.Close(ctx) }

// BuildTree builds the workload tree in a fresh instance and returns the
// {data, error} envelope holding it.
func (s *Sandbox) BuildTree(ctx context.Context, definitionJSON, workloadJSON string) ([]byte, error) {
	return s.run(ctx, request{Op: "buildTree", Definition: definitionJSON, Workload: workloadJSON})
}

// SetField applies a single field write to the workload in a fresh instance
// and returns the {data, error} envelope holding the mutated object. value is
// JSON-encoded (a quoted string, a number, etc.).
func (s *Sandbox) SetField(ctx context.Context, workloadJSON, path, valueJSON string) ([]byte, error) {
	value := json.RawMessage(valueJSON)
	return s.run(ctx, request{Op: "setField", Workload: workloadJSON, Path: path, Value: &value})
}

// Suspend applies the definition's suspend actions in a fresh instance and
// returns the {data, error} envelope holding the mutated object.
func (s *Sandbox) Suspend(ctx context.Context, definitionJSON, workloadJSON string) ([]byte, error) {
	return s.run(ctx, request{Op: "suspend", Definition: definitionJSON, Workload: workloadJSON})
}

// Resume applies the definition's resume actions in a fresh instance and
// returns the {data, error} envelope holding the mutated object.
func (s *Sandbox) Resume(ctx context.Context, definitionJSON, workloadJSON string) ([]byte, error) {
	return s.run(ctx, request{Op: "resume", Definition: definitionJSON, Workload: workloadJSON})
}

// ListCatalog returns the definitions built into the wasm, in a fresh instance.
func (s *Sandbox) ListCatalog(ctx context.Context) ([]byte, error) {
	return s.run(ctx, request{Op: "listCatalog"})
}

func (s *Sandbox) run(ctx context.Context, req request) ([]byte, error) {
	requestJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var stdout, stderr bytes.Buffer
	config := s.moduleConfig.
		WithStdin(bytes.NewReader(requestJSON)).
		WithStdout(&stdout).
		WithStderr(&stderr)
	instance, err := s.runtime.InstantiateModule(ctx, s.compiled, config)
	if instance != nil {
		_ = instance.Close(ctx)
	}
	if err != nil {
		var response guestResponse
		if json.Unmarshal(stdout.Bytes(), &response) == nil && response.Error != "" {
			return stdout.Bytes(), nil
		}
		return stdout.Bytes(), fmt.Errorf("run guest: %w (stderr: %s)", err, stderr.String())
	}
	return stdout.Bytes(), nil
}
