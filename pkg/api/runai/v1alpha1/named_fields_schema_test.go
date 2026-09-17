// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package v1alpha1

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/cel-go/cel"
	"sigs.k8s.io/yaml"
)

// Exercise generated rules as well as the Go validator so regeneration cannot
// silently omit the name restrictions or accessor validation.
func TestNamedFieldsGeneratedSchema(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "charts", "karta", "crds", "run.ai_kartas.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var crd map[string]any
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatal(err)
	}
	version := crd["spec"].(map[string]any)["versions"].([]any)[0].(map[string]any)
	schema := version["schema"].(map[string]any)["openAPIV3Schema"].(map[string]any)
	for _, name := range []string{"spec", "structureDefinition"} {
		schema = schema["properties"].(map[string]any)[name].(map[string]any)
	}
	properties := schema["properties"].(map[string]any)
	for _, name := range []string{"rootComponent", "childComponents"} {
		t.Run(name, func(t *testing.T) {
			component := properties[name].(map[string]any)
			if name == "childComponents" {
				component = component["items"].(map[string]any)
			}
			fields := component["properties"].(map[string]any)["fields"].(map[string]any)
			if fields["type"] != "object" {
				t.Fatal("custom fields must have an object schema")
			}
			accessor := fields["additionalProperties"].(map[string]any)
			componentRules := namedFieldSchemaRules(t, component)
			accessorRules := namedFieldSchemaRules(t, accessor)
			for _, field := range []string{"exampleos", "schedulerName", "custom field/a~b.c", "",
				"podTemplateSpec", "podSpec", "metadata", "fragmented.schedulerName", "fragmented.labels",
				"fragmented.annotations", "fragmented.resources", "fragmented.resourceClaims", "fragmented.podAffinity",
				"fragmented.nodeAffinity", "fragmented.containers", "fragmented.container", "fragmented.priorityClassName",
				"fragmented.image", "scale.replicas", "scale.minReplicas", "scale.maxReplicas", "suspend"} {
				valid := field == "exampleos" || field == "schedulerName" || field == "custom field/a~b.c"
				input := map[string]any{"fields": map[string]any{field: map[string]any{"pathWrite": "/d/d/c"}}}
				if got := namedFieldSchemaAccepts(t, componentRules, input); got != valid {
					t.Errorf("generated name validation for %q = %v, want %v", field, got, valid)
				}
			}
			if !namedFieldSchemaAccepts(t, componentRules, map[string]any{}) {
				t.Fatal("existing components without named fields must remain valid")
			}
			for _, input := range []map[string]any{
				{"expression": "object.d.d.c"},
				{"pathWrite": ""},
				{"pathWriteExpression": `"/d/d/c"`},
				{"pathWrite": "/d/d/c", "pathWriteExpression": `"/d/d/c"`},
			} {
				_, static := input["pathWrite"]
				_, computed := input["pathWriteExpression"]
				if got, want := namedFieldSchemaAccepts(t, accessorRules, input), !static || !computed; got != want {
					t.Errorf("generated accessor validation for %v = %v, want %v", input, got, want)
				}
			}
		})
	}
}

func namedFieldSchemaRules(t *testing.T, schema map[string]any) []cel.Program {
	t.Helper()
	rules, ok := schema["x-kubernetes-validations"].([]any)
	if !ok || len(rules) == 0 {
		t.Fatal("generated schema has no validation rules")
	}
	env, err := cel.NewEnv(cel.Variable("self", cel.MapType(cel.StringType, cel.DynType)))
	if err != nil {
		t.Fatal(err)
	}
	programs := make([]cel.Program, 0, len(rules))
	for _, raw := range rules {
		rule := raw.(map[string]any)["rule"].(string)
		ast, issues := env.Compile(rule)
		if issues.Err() != nil {
			t.Fatalf("compile generated rule %q: %v", rule, issues.Err())
		}
		program, err := env.Program(ast)
		if err != nil {
			t.Fatal(err)
		}
		programs = append(programs, program)
	}
	return programs
}

func namedFieldSchemaAccepts(t *testing.T, programs []cel.Program, input map[string]any) bool {
	t.Helper()
	for _, program := range programs {
		result, _, err := program.Eval(map[string]any{"self": input})
		if err != nil {
			t.Fatal(err)
		}
		if result.Value() != true {
			return false
		}
	}
	return true
}
