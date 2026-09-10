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

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
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

	// referencesProvider resolves the definition's references on first use. Nil means the
	// consumer provided nothing: an expression that reads references.<name> fails with
	// expression.ErrReferencesNotSupported.
	referencesProvider func(ctx context.Context) (map[string]any, error)
	refMu              sync.Mutex
	refValues          map[string]any
	mentions           map[string]bool

	mu     sync.Mutex
	source any

	once sync.Once
	data any
	err  error
}

// Option configures a runner.
type Option func(*runner)

// WithReferenceProvider makes references.<name> available to every expression. The provider runs
// on the first expression that mentions references and its successful result is memoized - the
// runner owns the returned map - so resolution is required by use and addressing stays stable
// across a multi-pass write.
func WithReferenceProvider(provider func(ctx context.Context) (map[string]any, error)) Option {
	return func(r *runner) { r.referencesProvider = provider }
}

// NamedExpression is a definition-level variable: evaluated in order before an expression runs,
// available to it as variables.<name>. The shape is the engine contract's, so a definition's
// variables convert once for either engine.
type NamedExpression = expression.NamedExpression

// NewRunner returns a Runner that evaluates source with CEL.
func NewRunner(source any, opts ...Option) (expression.Runner, error) {
	return NewRunnerWithVariables(source, nil, opts...)
}

// NewRunnerWithVariables returns a Runner whose expressions see the definition's named
// variables as variables.<name>, the way a ValidatingAdmissionPolicy composes expressions.
func NewRunnerWithVariables(source any, variables []NamedExpression, opts ...Option) (expression.Runner, error) {
	shared, err := Shared()
	if err != nil {
		return nil, err
	}

	r := &runner{evaluator: shared.(*evaluator), source: source, variables: variables}
	for _, opt := range opts {
		opt(r)
	}

	return r, nil
}

// referencesAny is the fallback scan for an expression that does not parse; the parse error
// itself surfaces at evaluation, so over-detecting here only costs an early fetch.
var referencesAny = regexp.MustCompile(`\breferences\b`)

// mentionsReferences reports whether the parsed expression contains a free identifier named
// references. Detection is syntactic, like the variables scan: a mention on a branch evaluation
// would not take still resolves. A field or string literal spelled "references" does not count,
// so a definition reading object.spec.references never triggers a fetch.
func (r *runner) mentionsReferences(expr string) bool {
	if cached, ok := r.mentions[expr]; ok {
		return cached
	}
	found := referencesFreeIdent(r.evaluator.env, expr)
	if r.mentions == nil {
		r.mentions = map[string]bool{}
	}
	r.mentions[expr] = found

	return found
}

func referencesFreeIdent(env *cel.Env, expr string) bool {
	parsed, issues := env.Parse(expr)
	if issues != nil && issues.Err() != nil {
		return referencesAny.MatchString(expr)
	}
	found := false
	celast.PreOrderVisit(parsed.NativeRep().Expr(), celast.NewExprVisitor(func(e celast.Expr) {
		if e.Kind() == celast.IdentKind && e.AsIdent() == "references" {
			found = true
		}
	}))

	return found
}

// needsReferences reports whether any of the expressions, or any of the definition variables the
// needed set selects, mentions the references binding.
func (r *runner) needsReferences(neededVars map[string]bool, expressions ...string) bool {
	for _, expr := range expressions {
		if r.mentionsReferences(expr) {
			return true
		}
	}
	for _, variable := range r.variables {
		if neededVars != nil && !neededVars[variable.Name] {
			continue
		}
		if r.mentionsReferences(variable.Expression) {
			return true
		}
	}

	return false
}

// resolveReferences resolves the definition's references and memoizes a successful result, so
// addressing stays stable across a multi-pass write. A failed resolution is not cached: a
// transient fetch error on one call must not poison every later one. The lock is deliberately
// held across the provider call - a single flight, so concurrent first readers cannot each hit
// the cluster.
func (r *runner) resolveReferences(ctx context.Context) (map[string]any, error) {
	if r.referencesProvider == nil {
		return nil, expression.ErrReferencesNotSupported
	}
	r.refMu.Lock()
	defer r.refMu.Unlock()
	if r.refValues != nil {
		return r.refValues, nil
	}
	values, err := r.referencesProvider(ctx)
	if err != nil {
		return nil, err
	}
	if values == nil {
		values = map[string]any{}
	}
	r.refValues = values

	return r.refValues, nil
}

// referencesFor returns the references binding for an evaluation: the resolved values when any
// involved expression mentions references, an empty map otherwise.
func (r *runner) referencesFor(ctx context.Context, neededVars map[string]bool, expressions ...string) (map[string]any, error) {
	if !r.needsReferences(neededVars, expressions...) {
		return map[string]any{}, nil
	}

	return r.resolveReferences(ctx)
}

// ResolveVariables evaluates the definition's variables against the CURRENT document and returns
// them, so a caller can freeze addressing across a multi-pass write: every path is computed
// against the document as it stood before the first write landed. When expressions are given,
// only the variables they reference (transitively) are resolved.
func (r *runner) ResolveVariables(ctx context.Context, expressions ...string) (map[string]any, error) {
	object, err := r.GetObject()
	if err != nil {
		return nil, err
	}

	needed := neededVariables(r.variables, expressions)
	refs, err := r.referencesFor(ctx, needed, expressions...)
	if err != nil {
		return nil, err
	}

	return r.resolveVariables(ctx, object, needed, refs)
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
func (r *runner) resolveVariables(ctx context.Context, object any, needed map[string]bool, refs map[string]any) (map[string]any, error) {
	resolved := make(map[string]any, len(r.variables))
	for _, variable := range r.variables {
		if needed != nil && !needed[variable.Name] {
			continue
		}
		out, err := r.evaluator.EvaluateWithVariables(ctx, variable.Expression, object,
			map[string]any{"variables": resolved, "references": refs})
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

// EvaluateWithVariables evaluates a CEL expression with value and instance bound. The result is
// always a single entry holding the whole value - a list result is never spread. Used by patch
// writes; never a path.
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
	neededVars := neededVariables(r.variables, []string{expression})
	// The references binding is runner-managed, never caller-supplied. With a frozen variables
	// map the definition variables never re-run, so only the expression itself decides whether
	// references are needed.
	scanScope := neededVars
	_, frozen := converted["variables"]
	if frozen {
		scanScope = map[string]bool{}
	}
	refs, err := r.referencesFor(ctx, scanScope, expression)
	if err != nil {
		return nil, err
	}
	converted["references"] = refs
	if !frozen {
		resolved, err := r.resolveVariables(ctx, object, neededVars, refs)
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

	needed := neededVariables(r.variables, []string{expression})
	refs, err := r.referencesFor(ctx, needed, expression)
	if err != nil {
		return nil, err
	}
	resolved, err := r.resolveVariables(ctx, object, needed, refs)
	if err != nil {
		return nil, err
	}
	out, err := r.evaluator.EvaluateWithVariables(ctx, expression, object,
		map[string]any{"variables": resolved, "references": refs})
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
