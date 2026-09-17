// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package resource

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/expression"
)

type testNamedFieldReader struct {
	ComponentAccessor
	fields []map[string]any
	ids    []string
}

func (r testNamedFieldReader) ExtractFields(context.Context, v1alpha1.ComponentDefinition) ([]map[string]any, error) {
	return r.fields, nil
}

func (r testNamedFieldReader) ExtractInstanceIds(context.Context, v1alpha1.ComponentDefinition) ([]string, error) {
	return r.ids, nil
}

func TestNamedFieldsSingleInstance(t *testing.T) {
	definition := v1alpha1.ComponentDefinition{Fields: map[string]v1alpha1.ValueAccessor{
		"scalar":    {Expression: `object.spec.count`},
		"list":      {Expression: `object.spec.items`},
		"object":    {Expression: `object.spec.settings`},
		"null":      {Expression: `null`},
		"emptyList": {Expression: `[]`},
		"emptyMap":  {Expression: `{}`},
		"writeOnly": {PathWrite: ptr.To("/spec/writeOnly")},
	}}
	accessor := sdkTestAccessor(t, map[string]any{"spec": map[string]any{
		"count": 3,
		"items": []any{"a", nil, map[string]any{"active": true}},
		"settings": map[string]any{
			"labels": map[string]any{"team": "platform"},
		},
	}})
	component := &Component{definition: definition, accessor: accessor}
	got, err := component.GetFields(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]any{"": {
		"scalar":    float64(3),
		"list":      []any{"a", nil, map[string]any{"active": true}},
		"object":    map[string]any{"labels": map[string]any{"team": "platform"}},
		"null":      nil,
		"emptyList": []any{},
		"emptyMap":  map[string]any{},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %#v, want %#v", got, want)
	}
}

func TestNamedFieldsMultipleInstances(t *testing.T) {
	definition := v1alpha1.ComponentDefinition{
		InstanceIds: &v1alpha1.ValueAccessor{Expression: `object.spec.groups.map(g, g.name)`},
		Fields: map[string]v1alpha1.ValueAccessor{
			"port":     {Expression: `object.spec.groups.map(g, g.port)`},
			"items":    {Expression: `object.spec.groups.map(g, g.items)`},
			"settings": {Expression: `object.spec.groups.map(g, g.settings)`},
		},
	}
	accessor := sdkTestAccessor(t, map[string]any{"spec": map[string]any{
		"groups": []any{
			map[string]any{"name": "worker", "port": 8080, "items": []any{"first"}, "settings": nil},
			map[string]any{"name": "head", "port": 9090, "items": []any{"second", "third"}, "settings": map[string]any{"ready": true}},
		},
	}})
	component := &Component{definition: definition, accessor: accessor}
	got, err := component.GetFields(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]any{
		"worker": {"port": float64(8080), "items": []any{"first"}, "settings": nil},
		"head":   {"port": float64(9090), "items": []any{"second", "third"}, "settings": map[string]any{"ready": true}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %#v, want %#v", got, want)
	}
}

func TestNamedFieldsWithoutReadableDefinitions(t *testing.T) {
	for _, fields := range []map[string]v1alpha1.ValueAccessor{
		nil,
		{},
		{"writeOnly": {PathWrite: ptr.To("/spec/value")}},
	} {
		definition := v1alpha1.ComponentDefinition{Fields: fields}
		accessor := NewAccessor(expression.NewMockRunner(gomock.NewController(t)))
		values, err := accessor.ExtractFields(context.Background(), definition)
		if err != nil || values != nil {
			t.Fatalf("ExtractFields = %#v, %v; want nil, nil", values, err)
		}
		component := &Component{definition: definition, accessor: NewMockComponentAccessor(gomock.NewController(t))}
		got, err := component.GetFields(context.Background())
		if err != nil || got != nil {
			t.Fatalf("GetFields = %#v, %v; want nil, nil", got, err)
		}
	}
}

func TestNamedFieldsEmptyInstances(t *testing.T) {
	component := &Component{
		definition: v1alpha1.ComponentDefinition{
			InstanceIds: &v1alpha1.ValueAccessor{Expression: `[]`},
			Fields:      map[string]v1alpha1.ValueAccessor{"value": {Expression: `[]`}},
		},
		accessor: sdkTestAccessor(t, map[string]any{}),
	}
	got, err := component.GetFields(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("GetFields = %#v, %v; want no instances", got, err)
	}
}

func TestNamedFieldsErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		ids        string
		expression string
		wantError  string
	}{
		{name: "too few values", ids: `["a", "b"]`, expression: `[1]`, wantError: `field "value": instance ids count (2) does not match results count (1)`},
		{name: "too many values", ids: `["a"]`, expression: `[1, 2]`, wantError: `results count (2)`},
		{name: "duplicate ids", ids: `["a", "a"]`, expression: `[1, 2]`, wantError: `duplicate instance id "a"`},
		{name: "empty id", ids: `[""]`, expression: `[1]`, wantError: `nonempty instance ids`},
		{name: "scalar field with one instance", ids: `["a"]`, expression: `1`, wantError: `must return a list`},
		{name: "null field with one instance", ids: `["a"]`, expression: `null`, wantError: `must return a list`},
		{name: "scalar ids", ids: `"a"`, expression: `[]`, wantError: `failed to get instance ids`},
		{name: "null ids", ids: `null`, expression: `[]`, wantError: `failed to get instance ids`},
		{name: "numeric id", ids: `[1]`, expression: `[1]`, wantError: `instance ids must be strings`},
		{name: "null id", ids: `[null]`, expression: `[1]`, wantError: `instance ids must be strings`},
		{name: "object id", ids: `[{"name": "a"}]`, expression: `[1]`, wantError: `instance ids must be strings`},
		{name: "field expression", expression: `object.missing.value`, wantError: `failed to extract field "value"`},
		{name: "id expression", ids: `object.missing.ids`, expression: `[1]`, wantError: `failed to get instance ids`},
	} {
		t.Run(test.name, func(t *testing.T) {
			definition := v1alpha1.ComponentDefinition{Fields: map[string]v1alpha1.ValueAccessor{
				"value": {Expression: test.expression},
			}}
			if test.ids != "" {
				definition.InstanceIds = &v1alpha1.ValueAccessor{Expression: test.ids}
			}
			got, err := sdkTestAccessor(t, map[string]any{}).ExtractFields(context.Background(), definition)
			if err == nil || !strings.Contains(err.Error(), test.wantError) || got != nil {
				t.Fatalf("ExtractFields = %#v, %v; want error containing %q", got, err, test.wantError)
			}
		})
	}
}

func TestNamedFieldsRunnerFailures(t *testing.T) {
	ctx := context.Background()
	definition := v1alpha1.ComponentDefinition{Fields: map[string]v1alpha1.ValueAccessor{
		"value": {Expression: "read"},
	}}
	sentinel := errors.New("evaluation failed")
	for _, test := range []struct {
		name    string
		results []any
		err     error
	}{
		{name: "no result"},
		{name: "multiple results", results: []any{"a", "b"}},
		{name: "invalid JSON value", results: []any{make(chan int)}},
		{name: "evaluation error", err: sentinel},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := expression.NewMockRunner(gomock.NewController(t))
			runner.EXPECT().EvaluateWithVariables(ctx, "read", nil).Return(test.results, test.err)
			got, err := NewAccessor(runner).ExtractFields(ctx, definition)
			if err == nil || got != nil {
				t.Fatalf("ExtractFields = %#v, %v; want an error", got, err)
			}
			if test.err != nil && !errors.Is(err, sentinel) {
				t.Fatalf("error %v does not wrap %v", err, sentinel)
			}
		})
	}
}

func TestNamedFieldsDetachedFromRunner(t *testing.T) {
	ctx := context.Background()
	source := map[string]any{"items": []any{map[string]any{"name": "original"}}}
	runner := expression.NewMockRunner(gomock.NewController(t))
	runner.EXPECT().EvaluateWithVariables(ctx, "ids", nil).Return([]any{[]any{"a", "b"}}, nil)
	runner.EXPECT().EvaluateWithVariables(ctx, "read", nil).Return([]any{[]any{source, source}}, nil)
	definition := v1alpha1.ComponentDefinition{
		InstanceIds: &v1alpha1.ValueAccessor{Expression: "ids"},
		Fields:      map[string]v1alpha1.ValueAccessor{"value": {Expression: "read"}},
	}
	got, err := NewAccessor(runner).ExtractFields(ctx, definition)
	if err != nil {
		t.Fatal(err)
	}
	got[0]["value"].(map[string]any)["items"].([]any)[0].(map[string]any)["name"] = "changed"
	if source["items"].([]any)[0].(map[string]any)["name"] != "original" {
		t.Fatal("field mutation changed the runner's value")
	}
	if !reflect.DeepEqual(got[1]["value"], source) {
		t.Fatal("field mutation changed another instance's value")
	}
	source["items"].([]any)[0].(map[string]any)["name"] = "runner changed"
	if got[1]["value"].(map[string]any)["items"].([]any)[0].(map[string]any)["name"] != "original" {
		t.Fatal("runner mutation changed extracted fields")
	}
}

func TestNamedFieldsOptionalReaderValidation(t *testing.T) {
	definition := v1alpha1.ComponentDefinition{
		InstanceIds: &v1alpha1.ValueAccessor{Expression: `["a", "b"]`},
		Fields:      map[string]v1alpha1.ValueAccessor{"value": {Expression: `[1, 2]`}},
	}
	for _, test := range []struct {
		name     string
		accessor ComponentAccessor
		want     string
	}{
		{name: "unsupported", accessor: NewMockComponentAccessor(gomock.NewController(t)), want: "does not support named field reads"},
		{name: "missing results", accessor: testNamedFieldReader{ids: []string{"a", "b"}}, want: "results count (0)"},
		{name: "duplicate ids", accessor: testNamedFieldReader{ids: []string{"a", "a"}, fields: []map[string]any{{}, {}}}, want: "duplicate instance id"},
		{name: "empty id", accessor: testNamedFieldReader{ids: []string{""}, fields: []map[string]any{{}}}, want: "nonempty instance ids"},
		{name: "invalid JSON value", accessor: testNamedFieldReader{ids: []string{"a"}, fields: []map[string]any{{"value": make(chan int)}}}, want: "failed to copy fields"},
	} {
		t.Run(test.name, func(t *testing.T) {
			component := &Component{definition: definition, accessor: test.accessor}
			got, err := component.GetFields(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.want) || got != nil {
				t.Fatalf("GetFields = %#v, %v; want error containing %q", got, err, test.want)
			}
		})
	}
}

func TestNamedFieldsOptionalReaderDetachment(t *testing.T) {
	source := map[string]any{"count": 3, "items": []any{map[string]any{"name": "original"}}}
	component := &Component{
		definition: v1alpha1.ComponentDefinition{
			InstanceIds: &v1alpha1.ValueAccessor{Expression: `["a", "b"]`},
			Fields:      map[string]v1alpha1.ValueAccessor{"value": {Expression: `[]`}},
		},
		accessor: testNamedFieldReader{
			ids:    []string{"a", "b"},
			fields: []map[string]any{{"value": source}, {"value": source}},
		},
	}
	got, err := component.GetFields(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first := got["a"]["value"].(map[string]any)
	second := got["b"]["value"].(map[string]any)
	if count, ok := first["count"].(float64); !ok || count != 3 {
		t.Fatalf("custom reader count = %#v, want float64(3)", first["count"])
	}
	first["items"].([]any)[0].(map[string]any)["name"] = "changed"
	if source["items"].([]any)[0].(map[string]any)["name"] != "original" {
		t.Fatal("field mutation changed the optional reader's value")
	}
	if second["items"].([]any)[0].(map[string]any)["name"] != "original" {
		t.Fatal("field mutation changed another instance's value")
	}
	source["items"].([]any)[0].(map[string]any)["name"] = "reader changed"
	if second["items"].([]any)[0].(map[string]any)["name"] != "original" {
		t.Fatal("optional reader mutation changed extracted fields")
	}
}

func TestNamedFieldsExtractedInstancesRefresh(t *testing.T) {
	ctx := context.Background()
	via := v1alpha1.ValueAccessor{Expression: `object.spec.value`, PathWrite: ptr.To("/spec/value")}
	definition := v1alpha1.ComponentDefinition{Fields: map[string]v1alpha1.ValueAccessor{"value": via}}
	accessor := sdkTestAccessor(t, map[string]any{"spec": map[string]any{"value": map[string]any{"name": "before"}}})
	component := &Component{definition: definition, accessor: accessor}
	before, err := component.GetExtractedInstances(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := accessor.WriteValues(ctx, definition, &via, []any{map[string]any{"name": "after"}}, MutationOptions{}); err != nil {
		t.Fatal(err)
	}
	after, err := component.GetExtractedInstances(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := before[""].Fields["value"].(map[string]any)["name"]; got != "before" {
		t.Fatalf("previous extraction changed to %v", got)
	}
	if got := after[""].Fields["value"].(map[string]any)["name"]; got != "after" {
		t.Fatalf("refreshed fields = %v, want after", got)
	}
}
