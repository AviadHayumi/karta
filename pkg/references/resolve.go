// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package references

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	celpkg "github.com/run-ai/karta/pkg/cel"
	"github.com/run-ai/karta/pkg/expression"
)

// Resolve fetches every reference the definition declares. The name and label expressions run
// against the workload alone - a reference cannot address through another reference - and a
// namespaced reference always resolves in the workload's namespace. When the reader implements
// PermissionChecker, each fetch is preceded by a permission check so a denial names the
// reference and the missing verb.
func Resolve(ctx context.Context, reader ResourceReader, karta *v1alpha1.Karta, workload any) (ResolvedReferences, error) {
	if reader == nil {
		return nil, errors.New("resolve references: reader is nil")
	}
	if karta == nil {
		return nil, errors.New("resolve references: karta is nil")
	}

	runner, err := celpkg.NewRunner(workload)
	if err != nil {
		return nil, fmt.Errorf("resolve references: %w", err)
	}
	namespace, err := workloadNamespace(ctx, runner)
	if err != nil {
		return nil, fmt.Errorf("resolve references: %w", err)
	}

	resolved := make(ResolvedReferences, len(karta.Spec.StructureDefinition.References))
	for _, ref := range karta.Spec.StructureDefinition.References {
		value, err := resolveOne(ctx, reader, runner, ref, namespace)
		if err != nil {
			return nil, fmt.Errorf("reference %q: %w", ref.Name, err)
		}
		resolved[ref.Name] = value
	}

	return resolved, nil
}

func resolveOne(ctx context.Context, reader ResourceReader,
	runner expression.Runner, ref v1alpha1.ResourceReference, namespace string) (ReferenceValue, error) {
	checker, _ := reader.(PermissionChecker)
	gvk := schema.GroupVersionKind{Group: ref.GVK.Group, Version: ref.GVK.Version, Kind: ref.GVK.Kind}
	switch {
	case ref.Lookup != nil:
		name, err := evaluateString(ctx, runner, ref.Lookup.NameExpression)
		if err != nil {
			return ReferenceValue{}, fmt.Errorf("nameExpression: %w", err)
		}
		if checker != nil {
			if err := checker.CheckRead(ctx, gvk, namespace, "get"); err != nil {
				return ReferenceValue{}, fmt.Errorf("the reader may not get %s %q: %w", gvk, name, err)
			}
		}
		object, err := reader.Get(ctx, gvk, namespace, name)
		if errors.Is(err, ErrNotFound) {
			return ReferenceValue{}, nil
		}
		if err != nil {
			return ReferenceValue{}, err
		}

		return LookupValue(object), nil

	case ref.List != nil:
		selector, err := buildSelector(ctx, runner, ref.List)
		if err != nil {
			return ReferenceValue{}, err
		}
		if checker != nil {
			if err := checker.CheckRead(ctx, gvk, namespace, "list"); err != nil {
				return ReferenceValue{}, fmt.Errorf("the reader may not list %s: %w", gvk, err)
			}
		}
		items, err := reader.List(ctx, gvk, ListQuery{Namespace: namespace, Selector: selector})
		if err != nil {
			return ReferenceValue{}, err
		}

		return ListValue(items), nil
	}

	return ReferenceValue{}, errors.New("neither lookup nor list is set")
}

// evaluateString evaluates one expression against the workload and requires a non-empty string
// result. The whole-value method is used so a list result cannot masquerade as a scalar through
// stream spreading.
func evaluateString(ctx context.Context, runner expression.Runner, expr string) (string, error) {
	results, err := runner.EvaluateWithVariables(ctx, expr, map[string]any{})
	if err != nil {
		return "", err
	}
	if len(results) != 1 {
		return "", fmt.Errorf("expected a single result, got %d", len(results))
	}
	value, ok := results[0].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("expected a non-empty string, got %v", results[0])
	}

	return value, nil
}

// buildSelector converts the structured selector into a Kubernetes label selector, resolving
// every expression-sourced value against the workload.
func buildSelector(ctx context.Context, runner expression.Runner, list *v1alpha1.ListReference) (labels.Selector, error) {
	selector := labels.NewSelector()
	for key, value := range list.MatchLabels {
		resolved, err := labelValue(ctx, runner, value)
		if err != nil {
			return nil, fmt.Errorf("matchLabels[%s]: %w", key, err)
		}
		requirement, err := labels.NewRequirement(key, selection.Equals, []string{resolved})
		if err != nil {
			return nil, fmt.Errorf("matchLabels[%s]: %w", key, err)
		}
		selector = selector.Add(*requirement)
	}
	for _, req := range list.MatchExpressions {
		op, err := operatorFor(req.Operator)
		if err != nil {
			return nil, fmt.Errorf("matchExpressions[%s]: %w", req.Key, err)
		}
		values := make([]string, 0, len(req.Values))
		for _, value := range req.Values {
			resolved, err := labelValue(ctx, runner, value)
			if err != nil {
				return nil, fmt.Errorf("matchExpressions[%s]: %w", req.Key, err)
			}
			values = append(values, resolved)
		}
		requirement, err := labels.NewRequirement(req.Key, op, values)
		if err != nil {
			return nil, fmt.Errorf("matchExpressions[%s]: %w", req.Key, err)
		}
		selector = selector.Add(*requirement)
	}

	return selector, nil
}

func labelValue(ctx context.Context, runner expression.Runner, value v1alpha1.LabelValue) (string, error) {
	switch {
	case value.Value != nil:
		return *value.Value, nil
	case value.Expression != nil:
		return evaluateString(ctx, runner, *value.Expression)
	}

	return "", errors.New("neither value nor expression is set")
}

func operatorFor(op v1alpha1.LabelSelectorOperator) (selection.Operator, error) {
	switch op {
	case v1alpha1.LabelSelectorOpIn:
		return selection.In, nil
	case v1alpha1.LabelSelectorOpNotIn:
		return selection.NotIn, nil
	case v1alpha1.LabelSelectorOpExists:
		return selection.Exists, nil
	case v1alpha1.LabelSelectorOpDoesNotExist:
		return selection.DoesNotExist, nil
	}

	return "", fmt.Errorf("unknown operator %q", op)
}

// workloadNamespace reads metadata.namespace from the workload. Empty is allowed: a
// cluster-scoped workload resolves cluster-scoped references, and a namespaced reader decides
// what an empty namespace means.
func workloadNamespace(ctx context.Context, runner expression.Runner) (string, error) {
	results, err := runner.EvaluateWithVariables(ctx, `object[?"metadata"][?"namespace"].orValue("")`, map[string]any{})
	if err != nil {
		return "", err
	}
	if len(results) != 1 {
		return "", fmt.Errorf("expected a single namespace result, got %d", len(results))
	}
	namespace, ok := results[0].(string)
	if !ok {
		return "", fmt.Errorf("metadata.namespace is not a string: %v", results[0])
	}

	return namespace, nil
}
