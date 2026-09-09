// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cel

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/google/cel-go/common/types/ref"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/run-ai/karta/pkg/expression"
)

// runner evaluates a Karta definition with CEL. It satisfies the
// expression.Runner contract, so the accessor, the status engine and the mutation paths are unchanged
// - only the language a definition is written in differs.
//
// A path expression cannot be a CEL expression, because CEL evaluates to a value and cannot be
// assigned to. Paths are therefore resolved to concrete locations (see path.go) and read or written
// against those, while CEL evaluates the predicates and the boolean matchers.
type runner struct {
	evaluator *evaluator
	variables []NamedExpression

	mu     sync.Mutex
	source any

	once sync.Once
	data any
	err  error
}

// NamedExpression is a definition-level variable: evaluated in order before an expression runs,
// available to it as variables.<name>. The shape is the engine contract's, so a definition's
// variables convert once for either engine.
type NamedExpression = expression.NamedExpression

// NewRunner returns a Runner that evaluates source with CEL.
func NewRunner(source any) (expression.Runner, error) {
	return NewRunnerWithVariables(source, nil)
}

// NewRunnerWithVariables returns a Runner whose expressions see the definition's named
// variables as variables.<name>, the way a ValidatingAdmissionPolicy composes expressions.
func NewRunnerWithVariables(source any, variables []NamedExpression) (expression.Runner, error) {
	shared, err := Shared()
	if err != nil {
		return nil, err
	}

	return &runner{evaluator: shared.(*evaluator), source: source, variables: variables}, nil
}

// ResolveVariables evaluates the definition's variables against the CURRENT document and returns
// them, so a caller can freeze addressing across a multi-pass write - every
// path before the first write landed. When expressions are given, only the variables they
// reference (transitively) are resolved.
func (r *runner) ResolveVariables(ctx context.Context, expressions ...string) (map[string]any, error) {
	object, err := r.GetObject()
	if err != nil {
		return nil, err
	}

	return r.resolveVariables(ctx, object, neededVariables(r.variables, expressions))
}

// variableRef matches both static spellings a variable reference has: variables.name and
// variables["name"]. variableAny finds every mention of the variables binding, so a dynamic
// access the static scan cannot name falls back to resolving everything.
var (
	variableRef = regexp.MustCompile(`\bvariables(?:\.(\w+)|\["([^"]+)"\])`)
	variableAny = regexp.MustCompile(`\bvariables\b`)
)

// neededVariables returns the set of variable names the expressions statically reference, closed
// transitively over the variables' own expressions. Nil means every variable: no expression
// context, or a mention of variables the static scan cannot name.
func neededVariables(variables []NamedExpression, expressions []string) map[string]bool {
	if len(expressions) == 0 {
		return nil
	}
	for _, expr := range expressions {
		if len(variableAny.FindAllString(expr, -1)) != len(variableRef.FindAllString(expr, -1)) {
			return nil
		}
	}
	byName := make(map[string]string, len(variables))
	for _, v := range variables {
		byName[v.Name] = v.Expression
	}
	needed := map[string]bool{}
	queue := append([]string{}, expressions...)
	for len(queue) > 0 {
		expr := queue[0]
		queue = queue[1:]
		for _, m := range variableRef.FindAllStringSubmatch(expr, -1) {
			name := m[1] + m[2]
			if needed[name] {
				continue
			}
			needed[name] = true
			if next, ok := byName[name]; ok {
				queue = append(queue, next)
			}
		}
	}

	return needed
}

// resolveVariables evaluates the definition's variables in order against the current document.
// A later variable sees the earlier ones, exactly like upstream variable composition. Selection
// is syntactic, not lazy: only the variables the expression mentions run, so one variable's
// failure cannot poison an expression that never references it, but a mentioned variable runs
// even on a branch evaluation would never take.
func (r *runner) resolveVariables(ctx context.Context, object any, needed map[string]bool) (map[string]any, error) {
	resolved := make(map[string]any, len(r.variables))
	for _, variable := range r.variables {
		if needed != nil && !needed[variable.Name] {
			continue
		}
		out, err := r.evaluator.EvaluateWithVariables(ctx, variable.Expression, object,
			map[string]any{"variables": resolved})
		if err != nil {
			return nil, fmt.Errorf("variable %q: %w", variable.Name, err)
		}
		value, err := native(out)
		if err != nil {
			return nil, fmt.Errorf("variable %q: %w", variable.Name, err)
		}
		resolved[variable.Name] = value
	}

	return resolved, nil
}

// EvaluateWithVariables evaluates a CEL expression with value and instance bound, returning the
// results in the stream shape Evaluate uses. Used by patch writes; never a path.
func (r *runner) EvaluateWithVariables(ctx context.Context, expression string, vars map[string]any) ([]any, error) {
	object, err := r.GetObject()
	if err != nil {
		return nil, err
	}
	converted := map[string]any{"value": nil, "instance": nil, "index": nil}
	for name, v := range vars {
		primitive, err := toPrimitive(v)
		if err != nil {
			return nil, err
		}
		converted[name] = primitive
	}
	if _, frozen := converted["variables"]; !frozen {
		resolved, err := r.resolveVariables(ctx, object, neededVariables(r.variables, []string{expression}))
		if err != nil {
			return nil, err
		}
		converted["variables"] = resolved
	}
	out, err := r.evaluator.EvaluateWithVariables(ctx, expression, object, converted)
	if err != nil {
		return nil, err
	}
	native, err := native(out)
	if err != nil {
		return nil, err
	}

	return []any{native}, nil
}

// Language names the expression language this runner evaluates. The accessor uses it to route a
// matcher whose definition is CEL at the spec level rather than on the matcher itself.
// GetObject returns the workload as plain Go types.
func (r *runner) GetObject() (any, error) {
	r.once.Do(func() {
		encoded, err := json.Marshal(r.source)
		if err != nil {
			r.err = fmt.Errorf("encode object: %w", err)

			return
		}
		r.err = json.Unmarshal(encoded, &r.data)
	})

	return r.data, r.err
}

// Evaluate runs expression against the workload. A boolean matcher is evaluated as CEL; a path
// expression yields the value at each location it selects.
func (r *runner) Evaluate(ctx context.Context, expression string) ([]any, error) {
	object, err := r.GetObject()
	if err != nil {
		return nil, err
	}

	resolved, err := r.resolveVariables(ctx, object, neededVariables(r.variables, []string{expression}))
	if err != nil {
		return nil, err
	}
	out, err := r.evaluator.EvaluateWithVariables(ctx, expression, object, map[string]any{"variables": resolved})
	if err != nil {
		return nil, err
	}

	// A list result is spread into a stream shape - one result per element.
	value, err := native(out)
	if err != nil {
		return nil, err
	}
	if list, ok := value.([]any); ok {
		return list, nil
	}

	return []any{value}, nil
}

// native converts a CEL result into plain Go types. A result that cannot round-trip into a JSON
// value is an error: leaking a CEL-native container would hand the two engines different value
// domains.
func native(out ref.Val) (any, error) {
	converted, err := out.ConvertToNative(reflect.TypeOf(&structpb.Value{}))
	if err != nil {
		return nil, fmt.Errorf("convert CEL result to a JSON value: %w", err)
	}
	value, ok := converted.(*structpb.Value)
	if !ok {
		return nil, fmt.Errorf("unexpected CEL native result %T", converted)
	}

	return value.AsInterface(), nil
}

// Assign replaces the document. A CEL definition writes through patches - an expression
// constructs the change and shared Go code applies it - so the only location a CEL runner
// ever assigns is the root.
func (r *runner) Assign(ctx context.Context, expression string, value any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.TrimSpace(expression) != "." {
		return fmt.Errorf("a CEL definition cannot assign to %q: writes are patches, not paths", expression)
	}
	// GetObject's once must fire before data is replaced, or a later call resets it from the
	// source. Replacing the pointer leaves any object handed out earlier untouched.
	if _, err := r.GetObject(); err != nil {
		return err
	}
	converted, err := toPrimitive(value)
	if err != nil {
		return err
	}
	r.data = converted

	return nil
}

func toPrimitive(value any) (any, error) {
	switch value.(type) {
	case nil, bool, string:
		return value, nil
	}
	// Numbers and structures round-trip through JSON so that a value written by either engine has
	// the same Go type, and a caller comparing objects sees no difference.
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("convert value: %w", err)
	}
	var out any
	if err := json.Unmarshal(encoded, &out); err != nil {
		return nil, fmt.Errorf("convert value: %w", err)
	}

	return out, nil
}
