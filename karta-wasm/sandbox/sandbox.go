// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package sandbox runs the karta-wasm WASI door under wazero, so karta reads a
// workload inside an isolated wasm instance instead of in the host process.
// The isolation is safer - the guest cannot touch host memory or state - and
// the cost is time: every call boots a fresh instance and runs interpreted.
package sandbox

import (
	"bytes"
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// instancePages bounds the memory a single guest instance may use.
// One wasm page is 64 KiB.
const instancePages = 2048 // 128 MiB

// Sandbox holds the compiled WASI door. It is safe for concurrent use: every
// BuildTree runs in its own fresh instance.
type Sandbox struct {
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
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
	return &Sandbox{runtime: runtime, compiled: compiled}, nil
}

// Close releases the runtime.
func (s *Sandbox) Close(ctx context.Context) error { return s.runtime.Close(ctx) }

// BuildTree runs the WASI door in a fresh instance: it writes
// {definition, workload} to the guest's stdin and returns the guest's stdout,
// the {data, error} envelope holding the workload tree.
func (s *Sandbox) BuildTree(ctx context.Context, definitionJSON, workloadJSON string) ([]byte, error) {
	request := fmt.Sprintf(`{"definition":%q,"workload":%q}`, definitionJSON, workloadJSON)
	var stdout, stderr bytes.Buffer
	config := wazero.NewModuleConfig().
		WithStdin(bytes.NewReader([]byte(request))).
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs("karta-wasi").
		WithName("")
	instance, err := s.runtime.InstantiateModule(ctx, s.compiled, config)
	if instance != nil {
		_ = instance.Close(ctx)
	}
	if err != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("run guest: %w (stderr: %s)", err, stderr.String())
	}
	return stdout.Bytes(), nil
}
