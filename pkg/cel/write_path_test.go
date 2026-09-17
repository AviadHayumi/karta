// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cel

import (
	"context"
	"strings"
	"testing"
)

func TestWritePathRejectsValueThroughVariableLookups(t *testing.T) {
	for _, path := range []string{
		`variables.destination`,
		`variables["destination"]`,
		`variables['destination']`,
		`variables[object.selected]`,
		`variables.alias`,
	} {
		t.Run(path, func(t *testing.T) {
			runner, err := NewRunnerWithVariables(map[string]any{"selected": "destination"}, []NamedExpression{
				{Name: "destination", Expression: `value == null ? "/old" : "/new"`},
				{Name: "alias", Expression: `variables.destination`},
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = runner.EvaluateWritePath(context.Background(), path, map[string]any{"value": "outside"})
			if err == nil || !strings.Contains(err.Error(), "undeclared reference to 'value'") {
				t.Fatalf("expected forbidden value binding, got %v", err)
			}
		})
	}
}

func TestWritePathAllowsObjectValueAndLocalVariableNames(t *testing.T) {
	runner, err := NewRunner(map[string]any{"value": "/spec/image"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{`object.value`, `[object.value].map(value, value)[0]`} {
		result, err := runner.EvaluateWritePath(context.Background(), path, nil)
		if err != nil || len(result) != 1 || result[0] != "/spec/image" {
			t.Fatalf("%s: got %v, %v", path, result, err)
		}
	}
}
