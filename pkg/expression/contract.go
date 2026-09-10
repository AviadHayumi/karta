// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package expression holds the engine contract the CEL engine implements. The resource layer
// depends only on the contract, never on the engine package directly.
//
// A Runner is bound to one workload document and is not safe for concurrent use.
package expression

import (
	"context"
	"errors"
)

// ErrReferencesNotSupported is returned when an expression requires the references binding but
// the consumer configured no way to resolve it - neither resolved values nor a reader. A consumer
// that has not implemented resolution rejects such definitions loudly instead of failing with a
// cryptic expression error.
var ErrReferencesNotSupported = errors.New("an expression reads references but the consumer provided no resolved references and no reader")

//go:generate go run go.uber.org/mock/mockgen -source=contract.go -destination=runner_mock.go -package=expression Runner

// Evaluator reads values from the workload document.
type Evaluator interface {
	// Evaluate evaluates an expression against the document. The results have a stream shape:
	// an expression yielding a list is spread into one entry per element.
	Evaluate(ctx context.Context, expression string) ([]any, error)
	// GetObject returns the document as plain Go types.
	GetObject() (any, error)
}

// Assigner replaces the workload document after a patch was applied to a copy.
type Assigner interface {
	// Assign replaces the document. Writes go through patches - an expression constructs the
	// change and shared code applies it - so the only location a runner assigns is the root.
	Assign(ctx context.Context, expression string, value any) error
}

// VariableEvaluator evaluates expressions with caller bindings and the definition's named
// variables, so a patch can be constructed around the value karta computed.
type VariableEvaluator interface {
	// EvaluateWithVariables evaluates expression with value, instance and index bound (null when
	// absent) alongside the definition's variables. Unlike Evaluate, the result keeps its whole
	// value: always exactly one entry, and a list result is never spread. A caller that already resolved the variables passes them
	// under the "variables" key to freeze addressing across a multi-pass write.
	EvaluateWithVariables(ctx context.Context, expression string, vars map[string]any) ([]any, error)
	// ResolveVariables evaluates the definition's variables against the current document. When
	// expressions are given, only the variables they syntactically reference (transitively) are
	// resolved, so one variable's failure cannot poison an expression that never mentions it.
	ResolveVariables(ctx context.Context, expressions ...string) (map[string]any, error)
}

// NamedExpression is a definition-level variable: evaluated in order before an expression runs,
// available to it as variables.<name>.
type NamedExpression struct {
	Name       string
	Expression string
}

// Runner is the full engine contract. The engine implements all of it, so a caller never asks
// which capabilities a runner has.
type Runner interface {
	Evaluator
	Assigner
	VariableEvaluator
}
