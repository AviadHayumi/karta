// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	celpkg "github.com/run-ai/karta/pkg/cel"
	"github.com/run-ai/karta/pkg/expression"
	"github.com/run-ai/karta/pkg/references"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
	"github.com/run-ai/karta/test/e2e/recorder"
)

type extractionField struct {
	name string
	via  *v1alpha1.ValueAccessor
	path []string
}

type extractionCoverage struct {
	events, components, instances, fields int
	catalogs                              map[string]bool
}

// All means every field declared by a Karta, including nil and zero values. It
// does not mean arbitrary operator fields that have no Karta accessor.
func TestEditableTreeExtractionReplay(t *testing.T) {
	ctx := context.Background()
	coverage := extractionCoverage{catalogs: map[string]bool{}}
	paths := replayPaths(t)
	for _, path := range paths {
		t.Run(strings.TrimPrefix(path, "../recorded_data/"), func(t *testing.T) {
			r, karta := replayDefinition(t, path)
			coverage.catalogs[filepath.Base(r.Recording().KartaFile)] = true
			events := 0
			for r.Next() {
				events++
				t.Run(fmt.Sprintf("%02d-%s", events, r.State()), func(t *testing.T) {
					reader := &recordedReader{objects: r.References()}
					factory := resource.NewComponentFactoryFromObject(karta, r.Object(), resource.WithReferenceReader(reader))
					editable, err := tree.Open(ctx, karta, r.Object(), resource.WithReferenceReader(reader))
					if err != nil {
						t.Fatal(err)
					}
					snapshot := editable.Snapshot()
					if snapshot.Root == nil {
						t.Fatal("tree omitted the root component and its extracted fields")
					}
					if snapshot.Status == nil || !containsString(snapshot.Status.Phases, r.State()) {
						t.Fatalf("tree phases %v do not contain recorded state %q", snapshot.Status, r.State())
					}
					runner, err := independentRunner(karta, r.Object(), reader)
					if err != nil {
						t.Fatal(err)
					}
					defs := append([]v1alpha1.ComponentDefinition{karta.Spec.StructureDefinition.RootComponent}, karta.Spec.StructureDefinition.ChildComponents...)
					byParent := map[string][]v1alpha1.ComponentDefinition{}
					for _, def := range defs {
						if def.OwnerRef != nil {
							byParent[*def.OwnerRef] = append(byParent[*def.OwnerRef], def)
						}
					}
					verifyExtractedNode(t, ctx, *snapshot.Root, karta.Spec.StructureDefinition.RootComponent, byParent, runner, factory, &coverage)
					if len(snapshot.Root.Instances) == 0 {
						t.Fatal("root has no instance")
					}
					if !reflect.DeepEqual(snapshot.Children, snapshot.Root.Instances[0].Children) {
						t.Fatal("legacy tree Children do not match the root instance's hierarchy")
					}
					coverage.events++
				})
			}
			if events == 0 {
				t.Fatal("recording contains no state events")
			}
		})
	}
	files, err := filepath.Glob(filepath.Join(repoRoot, "docs", "catalog", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for _, file := range files {
		if !coverage.catalogs[filepath.Base(file)] {
			missing = append(missing, filepath.Base(file))
		}
	}
	sort.Strings(missing)
	t.Logf("extraction coverage: %d recordings; %d catalog definitions; %d state events; %d component occurrences; %d instance occurrences; %d declared accessor values", len(paths), len(coverage.catalogs), coverage.events, coverage.components, coverage.instances, coverage.fields)
	t.Logf("catalog definitions without recorded fixtures: %v", missing)
}

func verifyExtractedNode(t *testing.T, ctx context.Context, node tree.ComponentNode, def v1alpha1.ComponentDefinition, children map[string][]v1alpha1.ComponentDefinition, runner expression.Runner, factory *resource.ComponentFactory, coverage *extractionCoverage) {
	t.Helper()
	if node.Name != def.Name {
		t.Fatalf("component name %q, want %q", node.Name, def.Name)
	}
	component, err := factory.GetComponent(def.Name)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(node.Kind, component.Kind()) || node.HasPodDefinition != component.HasPodDefinition() {
		t.Fatalf("component %s lost kind or pod-definition metadata", def.Name)
	}
	verifyRawStatus(t, ctx, def, runner, component)
	ids := independentInstanceIDs(t, ctx, def, runner)
	if len(node.Instances) != len(ids) {
		t.Fatalf("component %s has %d tree instances, want %d from CEL", def.Name, len(node.Instances), len(ids))
	}
	direct, err := component.GetExtractedInstances(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(direct) != len(ids) {
		t.Fatalf("component %s direct extraction omitted an instance", def.Name)
	}
	expected := make(map[string]*resource.ExtractedInstance, len(ids))
	for _, id := range ids {
		if _, duplicate := expected[id]; duplicate {
			t.Fatalf("component %s repeats instance ID %q", def.Name, id)
		}
		expected[id] = &resource.ExtractedInstance{}
	}
	for _, field := range declaredExtractionFields(def) {
		if field.via == nil || field.via.Expression == "" {
			continue
		}
		values, err := runner.EvaluateWithVariables(ctx, field.via.Expression, nil)
		if err != nil {
			t.Fatalf("component %s field %s CEL: %v", def.Name, field.name, err)
		}
		if def.InstanceIds != nil && def.InstanceIds.Expression != "" {
			if len(values) != 1 {
				t.Fatalf("component %s field %s did not return one CEL value", def.Name, field.name)
			}
			list, ok := values[0].([]any)
			if !ok {
				t.Fatalf("component %s field %s should return a per-instance list, got %T", def.Name, field.name, values[0])
			}
			values = list
		}
		if len(values) != len(ids) {
			t.Fatalf("component %s field %s returned %d values for %d IDs", def.Name, field.name, len(values), len(ids))
		}
		for i, id := range ids {
			setExpectedField(t, expected[id], field.path, values[i])
			coverage.fields++
		}
	}
	seen := map[string]bool{}
	for _, instance := range node.Instances {
		id := ""
		if instance.InstanceKey != nil {
			id = *instance.InstanceKey
		}
		want, found := expected[id]
		if !found || seen[id] {
			t.Fatalf("component %s has unexpected or repeated tree instance %q", def.Name, id)
		}
		seen[id] = true
		if !reflect.DeepEqual(instance.ExtractedInstance, want) {
			t.Fatalf("component %s instance %q tree extraction differs from independent CEL/typed projection\ngot: %s\nwant: %s", def.Name, id, jsonText(instance.ExtractedInstance), jsonText(want))
		}
		if got, ok := direct[id]; !ok || !reflect.DeepEqual(got, *want) {
			t.Fatalf("component %s instance %q direct extraction differs from CEL projection", def.Name, id)
		}
		if !reflect.DeepEqual(instance.Scale, want.Scale) {
			t.Fatalf("component %s instance %q omitted tree scale", def.Name, id)
		}
		childDefs := children[def.Name]
		if len(instance.Children) != len(childDefs) {
			t.Fatalf("component %s instance %q has %d children, want %d", def.Name, id, len(instance.Children), len(childDefs))
		}
		for i, childDef := range childDefs {
			verifyExtractedNode(t, ctx, instance.Children[i], childDef, children, runner, factory, coverage)
		}
		coverage.instances++
	}
	coverage.components++
}

func verifyRawStatus(t *testing.T, ctx context.Context, def v1alpha1.ComponentDefinition, runner expression.Runner, component *resource.Component) {
	t.Helper()
	status, err := component.GetStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if def.StatusDefinition == nil {
		if status != nil {
			t.Fatalf("component %s extracted an undeclared status", def.Name)
		}
		return
	}
	if status == nil {
		t.Fatalf("component %s omitted its declared status", def.Name)
	}
	if phase := def.StatusDefinition.PhaseDefinition; phase != nil && phase.Expression != "" {
		values, err := runner.EvaluateWithVariables(ctx, phase.Expression, nil)
		if err != nil || len(values) != 1 {
			t.Fatalf("component %s phase expression: values=%v error=%v", def.Name, values, err)
		}
		// The public extractor projects a declared null phase into string's
		// zero value. Undeclared phases remain nil.
		var expected string
		if err := normalizedJSONValue(values[0], &expected); err != nil {
			t.Fatal(err)
		}
		if status.Phase == nil || *status.Phase != expected {
			t.Fatalf("component %s phase=%v, want typed CEL value %q", def.Name, status.Phase, expected)
		}
	}
	conditions := def.StatusDefinition.ConditionsDefinition
	if conditions == nil || conditions.Expression == "" {
		if len(status.Conditions) != 0 {
			t.Fatalf("component %s extracted undeclared conditions", def.Name)
		}
		return
	}
	values, err := runner.EvaluateWithVariables(ctx, conditions.Expression, nil)
	if err != nil || len(values) != 1 {
		t.Fatalf("component %s conditions expression: values=%v error=%v", def.Name, values, err)
	}
	var raw []map[string]any
	if err := normalizedJSONValue(values[0], &raw); err != nil {
		t.Fatal(err)
	}
	if len(status.Conditions) != len(raw) {
		t.Fatalf("component %s extracted %d conditions, want %d", def.Name, len(status.Conditions), len(raw))
	}
	for i, source := range raw {
		projected := map[string]any{"type": source[conditions.TypeFieldName], "status": source[conditions.StatusFieldName]}
		if conditions.MessageFieldName != nil {
			projected["message"] = source[*conditions.MessageFieldName]
		}
		if conditions.ReasonFieldName != nil {
			projected["reason"] = source[*conditions.ReasonFieldName]
		}
		var expected resource.Condition
		if err := normalizedJSONValue(projected, &expected); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(status.Conditions[i], expected) {
			t.Fatalf("component %s condition %d differs from CEL projection", def.Name, i)
		}
	}
}

func declaredExtractionFields(def v1alpha1.ComponentDefinition) []extractionField {
	var fields []extractionField
	if spec := def.SpecDefinition; spec != nil {
		fields = append(fields,
			extractionField{"podTemplateSpec", spec.PodTemplateSpec, []string{"PodTemplateSpec"}},
			extractionField{"podSpec", spec.PodSpec, []string{"PodSpec"}},
			extractionField{"metadata", spec.Metadata, []string{"Metadata"}},
		)
		if fragments := spec.FragmentedPodSpecDefinition; fragments != nil {
			value := reflect.ValueOf(fragments).Elem()
			for i := 0; i < value.NumField(); i++ {
				field := value.Type().Field(i)
				via, _ := value.Field(i).Interface().(*v1alpha1.ValueAccessor)
				fields = append(fields, extractionField{"fragmented." + strings.Split(field.Tag.Get("json"), ",")[0], via, []string{"FragmentedPodSpec", field.Name}})
			}
		}
	}
	if scale := def.ScaleDefinition; scale != nil {
		fields = append(fields,
			extractionField{"scale.replicas", scale.Replicas, []string{"Scale", "Replicas"}},
			extractionField{"scale.minReplicas", scale.MinReplicas, []string{"Scale", "MinReplicas"}},
			extractionField{"scale.maxReplicas", scale.MaxReplicas, []string{"Scale", "MaxReplicas"}},
		)
	}
	return fields
}

func independentInstanceIDs(t *testing.T, ctx context.Context, def v1alpha1.ComponentDefinition, runner expression.Runner) []string {
	t.Helper()
	if def.InstanceIds == nil || def.InstanceIds.Expression == "" {
		return []string{""}
	}
	values, err := runner.Evaluate(ctx, def.InstanceIds.Expression)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(values))
	for i, value := range values {
		var ok bool
		ids[i], ok = value.(string)
		if !ok {
			t.Fatalf("component %s instance ID is %T, want string", def.Name, value)
		}
	}
	return ids
}

func setExpectedField(t *testing.T, instance *resource.ExtractedInstance, path []string, value any) {
	t.Helper()
	parent := reflect.ValueOf(instance).Elem().FieldByName(path[0])
	if parent.IsNil() {
		parent.Set(reflect.New(parent.Type().Elem()))
	}
	target := parent.Interface()
	if len(path) == 2 {
		target = parent.Elem().FieldByName(path[1]).Addr().Interface()
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("project field %v into the public Go type: %v", path, err)
	}
}

func independentRunner(karta *v1alpha1.Karta, object resource.KubernetesObject, reader *recordedReader) (expression.Runner, error) {
	variables := make([]celpkg.NamedExpression, 0, len(karta.Spec.Variables))
	for _, variable := range karta.Spec.Variables {
		variables = append(variables, celpkg.NamedExpression{Name: variable.Name, Expression: variable.Expression})
	}
	return celpkg.NewRunnerWithVariables(object, variables, celpkg.WithReferenceProvider(func(ctx context.Context) (map[string]any, error) {
		resolved, err := references.Resolve(ctx, reader, karta, object)
		if err != nil {
			return nil, err
		}
		return resolved.Bindings()
	}))
}

func replayPaths(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(recordedGlob)
	if err != nil || len(paths) == 0 {
		t.Fatalf("recordings are required: count=%d, error=%v", len(paths), err)
	}
	return paths
}

func replayDefinition(t *testing.T, path string) (*recorder.Reader, *v1alpha1.Karta) {
	t.Helper()
	r, err := recorder.OpenRecording(path)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(r.Recording().KartaFile)
	data, err := os.ReadFile(filepath.Join(repoRoot, "docs", "catalog", name))
	if err != nil {
		t.Fatal(err)
	}
	karta := &v1alpha1.Karta{}
	if err := yaml.UnmarshalStrict(data, karta); err != nil {
		t.Fatal(err)
	}
	return r, karta
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func jsonText(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return err.Error()
	}
	return string(data)
}
