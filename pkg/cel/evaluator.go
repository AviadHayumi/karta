// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package cel evaluates CEL status matchers against a workload.
//
// CEL is the expression language Kubernetes itself uses for CRD validation rules and admission
// policies, so a definition author already knows it. It is typed - comparing a list
// to a number is a compile or evaluation error rather than a silently true answer - and it cannot
// loop, so a matcher's cost is bounded.
package cel

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
)

// ObjectVar is the name the workload is bound to inside a CEL expression, matching the name
// Kubernetes admission policies use.
const ObjectVar = "object"

// Evaluator compiles and runs CEL expressions against a workload.
type Evaluator interface {
	// Evaluate runs expression against object and returns the result.
	Evaluate(ctx context.Context, expression string, object any) (ref.Val, error)
	// EvaluateWithVariables runs expression against object with the given bindings, so a caller
	// holding resolved definition variables can make them visible as variables.<name>.
	EvaluateWithVariables(ctx context.Context, expression string, object any, vars map[string]any) (ref.Val, error)
}

type evaluator struct {
	env *cel.Env

	mu       sync.RWMutex
	programs map[string]cel.Program
}

var (
	sharedOnce sync.Once
	shared     *evaluator
	sharedErr  error
)

// Shared returns the process-wide evaluator. Compiling a CEL program is expensive relative to
// running it, so programs are cached and reused across workloads.
func Shared() (Evaluator, error) {
	sharedOnce.Do(func() {
		shared, sharedErr = newEvaluator()
	})

	return shared, sharedErr
}

func newEvaluator() (*evaluator, error) {
	env, err := cel.NewEnv(
		cel.Variable(ObjectVar, cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("value", cel.DynType),
		cel.Variable("instance", cel.DynType),
		cel.Variable("index", cel.DynType),
		cel.Variable("variables", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("references", cel.MapType(cel.StringType, cel.DynType)),
		// Optional types give `object.status.?readyReplicas.orValue(0)`, the CEL spelling of
		// `.status.readyReplicas // 0`, so an absent field is a value rather than an error.
		cel.OptionalTypes(),
		// The list and string helpers Kubernetes also enables, so a definition can sort keys or
		// join names without extra builtins.
		ext.Lists(),
		ext.Strings(),
	)
	if err != nil {
		return nil, fmt.Errorf("build CEL environment: %w", err)
	}

	return &evaluator{env: env, programs: map[string]cel.Program{}}, nil
}

func (e *evaluator) Evaluate(ctx context.Context, expression string, object any) (ref.Val, error) {
	program, err := e.program(expression)
	if err != nil {
		return nil, err
	}

	out, _, err := program.ContextEval(ctx, map[string]any{ObjectVar: object})
	if err != nil {
		return nil, fmt.Errorf("evaluate CEL expression %q: %w", expression, err)
	}

	return out, nil
}

// EvaluateWithVariables evaluates expression with value and instance bound alongside the workload,
// so a patch can be constructed around the value karta computed.
func (e *evaluator) EvaluateWithVariables(ctx context.Context, expression string, object any, vars map[string]any) (ref.Val, error) {
	program, err := e.program(expression)
	if err != nil {
		return nil, err
	}

	activation := map[string]any{ObjectVar: object, "value": nil, "instance": nil, "index": nil}
	for name, v := range vars {
		activation[name] = v
	}
	out, _, err := program.ContextEval(ctx, activation)
	if err != nil {
		return nil, fmt.Errorf("evaluate CEL expression %q: %w", expression, err)
	}

	return out, nil
}

func (e *evaluator) program(expression string) (cel.Program, error) {
	e.mu.RLock()
	cached, ok := e.programs[expression]
	e.mu.RUnlock()
	if ok {
		return cached, nil
	}

	ast, issues := e.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compile CEL expression %q: %w", expression, issues.Err())
	}

	// Evaluation is charged per step and aborts once the budget is spent, so a runaway expression
	// fails fast instead of stalling a reconcile.
	program, err := e.env.Program(ast,
		cel.InterruptCheckFrequency(checkFrequency),
		cel.CostLimit(MaxCost),
	)
	if err != nil {
		return nil, fmt.Errorf("build CEL program %q: %w", expression, err)
	}

	e.mu.Lock()
	e.programs[expression] = program
	e.mu.Unlock()

	return program, nil
}

// checkFrequency is how often a running program checks the context for cancellation.
const checkFrequency = 100

// MaxCost is the evaluation budget for one matcher. A catalog matcher costs a few hundred; the
// ceiling is far above that, so an honest definition never notices it and a runaway expression
// is stopped.
const MaxCost = 1_000_000

// Validate reports whether expression compiles, so a bad definition fails admission rather than
// every reconcile.
func Validate(expression string) error {
	e, err := Shared()
	if err != nil {
		return err
	}
	_, err = e.(*evaluator).program(expression)

	return err
}
