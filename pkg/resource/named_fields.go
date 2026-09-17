// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package resource

import (
	"context"
	"fmt"
	"slices"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// NamedFieldReader optionally extends component accessors with named field reads.
// Results follow instance order, or contain one entry for a single-instance component.
type NamedFieldReader interface {
	ExtractFields(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]map[string]any, error)
}

var _ NamedFieldReader = (*Accessor)(nil)

// ExtractFields evaluates readable named fields and returns detached JSON values.
// Fields without an expression are write-only and do not appear in the results.
func (a *Accessor) ExtractFields(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]map[string]any, error) {
	names := readableFieldNames(definition)
	if len(names) == 0 {
		return nil, nil
	}

	instanceIDs := []string{""}
	if instancedComponent(definition) {
		values, err := a.extractFieldValues(ctx, definition.InstanceIds, true)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", errGetInstanceIds, err)
		}
		instanceIDs = make([]string, len(values))
		for i, value := range values {
			id, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("instance ids must be strings, got %T at index %d", value, i)
			}
			instanceIDs[i] = id
		}
	}
	if err := validateFieldInstanceIDs(instanceIDs, instancedComponent(definition)); err != nil {
		return nil, err
	}

	fields := make([]map[string]any, len(instanceIDs))
	for i := range fields {
		fields[i] = make(map[string]any, len(names))
	}
	for _, name := range names {
		via := definition.Fields[name]
		values, err := a.extractFieldValues(ctx, &via, instancedComponent(definition))
		if err != nil {
			return nil, fmt.Errorf("failed to extract field %q: %w", name, err)
		}
		if len(values) != len(instanceIDs) {
			return nil, fmt.Errorf("field %q: instance ids count (%d) does not match results count (%d)", name, len(instanceIDs), len(values))
		}
		for i, value := range values {
			copy, err := jsonCopy(value)
			if err != nil {
				return nil, fmt.Errorf("failed to copy field %q for instance %q: %w", name, instanceIDs[i], err)
			}
			fields[i][name] = copy
		}
	}
	return fields, nil
}

func (a *Accessor) extractFieldValues(ctx context.Context, via *v1alpha1.ValueAccessor, instanced bool) ([]any, error) {
	var values []any
	if err := extractVia(ctx, via, false, a.runner, &values); err != nil {
		return nil, err
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("named field expression must return exactly one value, got %d", len(values))
	}
	if !instanced {
		return values, nil
	}
	list, ok := values[0].([]any)
	if !ok {
		return nil, fmt.Errorf("named field expression must return a list for an instanced component, got %T", values[0])
	}
	return list, nil
}

// GetFields extracts readable named fields mapped by instance ID.
// A single-instance component uses the empty string as its instance ID.
func (c *Component) GetFields(ctx context.Context) (map[string]map[string]any, error) {
	if len(readableFieldNames(c.definition)) == 0 {
		return nil, nil
	}
	reader, ok := c.accessor.(NamedFieldReader)
	if !ok {
		return nil, fmt.Errorf("component %q accessor does not support named field reads", c.name)
	}
	fields, err := reader.ExtractFields(ctx, c.definition)
	if err != nil {
		return nil, fmt.Errorf("failed to extract fields: %w", err)
	}
	instanceIDs, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetInstanceIds, err)
	}
	if err := validateFieldInstanceIDs(instanceIDs, instancedComponent(c.definition)); err != nil {
		return nil, err
	}
	result, err := zipWithInstanceIds(instanceIDs, fields)
	if err != nil {
		return nil, err
	}
	for _, id := range instanceIDs {
		if result[id] == nil {
			continue
		}
		copy, err := jsonCopy(result[id])
		if err != nil {
			return nil, fmt.Errorf("failed to copy fields for instance %q: %w", id, err)
		}
		result[id] = copy.(map[string]any)
	}
	return result, nil
}

func readableFieldNames(definition v1alpha1.ComponentDefinition) []string {
	names := make([]string, 0, len(definition.Fields))
	for name, via := range definition.Fields {
		if via.Expression != "" {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

func validateFieldInstanceIDs(instanceIDs []string, instanced bool) error {
	seen := make(map[string]struct{}, len(instanceIDs))
	for _, id := range instanceIDs {
		if instanced && id == "" {
			return fmt.Errorf("named fields require nonempty instance ids")
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("duplicate instance id %q for named fields", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}
