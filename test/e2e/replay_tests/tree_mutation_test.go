// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package replay

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
	"github.com/run-ai/karta/test/e2e/recorder"
)

// These inputs and updates match the existing mutation-golden replay. Comparing
// both SDK entry points does not replace or regenerate those original goldens.
func TestEditableTreeMutationReplay(t *testing.T) {
	ctx := context.Background()
	paths := replayPaths(t)
	variants := []resource.MutationOptions{
		{PatchType: resource.PatchTypeMergePatch, Strategy: resource.Merge},
		{PatchType: resource.PatchTypeMergePatch, Strategy: resource.Replace},
		{PatchType: resource.PatchTypeJSONPatch, Strategy: resource.Merge},
		{PatchType: resource.PatchTypeJSONPatch, Strategy: resource.Replace},
	}
	var comparisons, protectedRootReplacements, suspendResume int
	for _, options := range variants {
		t.Run(string(options.PatchType)+"/"+string(options.Strategy), func(t *testing.T) {
			for _, path := range paths {
				t.Run(strings.TrimPrefix(path, "../recorded_data/"), func(t *testing.T) {
					r, karta := replayDefinition(t, path)
					cr, reader := replayInput(t, r)
					opts := []resource.FactoryOption{resource.WithReferenceReader(reader), resource.WithMutationOptions(options)}
					factory := resource.NewComponentFactoryFromObject(karta, cr, opts...)
					editable, err := tree.Open(ctx, karta, cr, opts...)
					if err != nil {
						t.Fatal(err)
					}
					root, err := factory.GetRootComponent()
					if err != nil {
						t.Fatal(err)
					}
					children, err := factory.GetChildComponents()
					if err != nil {
						t.Fatal(err)
					}
					writes := 0
					for _, component := range append([]*resource.Component{root}, children...) {
						def := component.Definition()
						if def.SpecDefinition == nil {
							continue
						}
						if writable(def.SpecDefinition.PodTemplateSpec) {
							values, err := component.GetPodTemplateSpec(ctx)
							if err != nil {
								t.Fatal(err)
							}
							batch := make([]tree.Write, 0, len(values))
							for id, value := range values {
								value.Spec.SchedulerName = goldenScheduler
								values[id] = value
								batch = append(batch, tree.Write{Component: component.Name(), Instance: id, Field: tree.PodTemplateSpec, Value: value, Options: options})
							}
							before := jsonText(mustTreeResource(t, editable))
							beforeSnapshot := editable.Snapshot()
							updateErr := component.UpdatePodTemplateSpec(ctx, values)
							if cr.GetObjectKind().GroupVersionKind().Kind == "Pod" && options.Strategy == resource.Replace {
								if updateErr == nil || !strings.Contains(updateErr.Error(), "missing apiVersion") {
									t.Fatalf("component accepted an invalid Pod root replacement: %v", updateErr)
								}
								after, err := factory.GetResource()
								if err != nil || jsonText(after) != before {
									t.Fatalf("rejected component root replacement changed the workload: %v", err)
								}
								if err := editable.Mutate(ctx, batch...); err == nil || !strings.Contains(err.Error(), "missing apiVersion") {
									t.Fatalf("tree accepted an invalid Pod root replacement: %v", err)
								}
								if jsonText(mustTreeResource(t, editable)) != before || !reflect.DeepEqual(beforeSnapshot, editable.Snapshot()) {
									t.Fatal("rejected tree root replacement changed workload or extraction")
								}
								protectedRootReplacements++
								return
							}
							if updateErr != nil {
								t.Fatal(updateErr)
							}
							if !compareTreeMutation(t, ctx, editable, factory, batch) {
								t.Fatal("unexpected invalid-object guard outside Pod root replacement")
							}
							writes += len(batch)
						}
						if writable(def.SpecDefinition.PodSpec) {
							values, err := component.GetPodSpec(ctx)
							if err != nil {
								t.Fatal(err)
							}
							batch := make([]tree.Write, 0, len(values))
							for id, value := range values {
								value.SchedulerName = goldenScheduler
								values[id] = value
								batch = append(batch, tree.Write{Component: component.Name(), Instance: id, Field: tree.PodSpec, Value: value, Options: options})
							}
							if err := component.UpdatePodSpec(ctx, values); err != nil {
								t.Fatal(err)
							}
							if !compareTreeMutation(t, ctx, editable, factory, batch) {
								t.Fatal("pod spec update unexpectedly made the workload invalid")
							}
							writes += len(batch)
						}
						if fragmented := def.SpecDefinition.FragmentedPodSpecDefinition; fragmented != nil && (writable(fragmented.SchedulerName) || writable(fragmented.NodeAffinity)) {
							current, err := component.GetFragmentedPodSpec(ctx)
							if err != nil {
								t.Fatal(err)
							}
							values := make(map[string]resource.FragmentedPodSpec, len(current))
							var batch []tree.Write
							for id := range current {
								fragment := resource.FragmentedPodSpec{}
								if writable(fragmented.SchedulerName) {
									fragment.SchedulerName = goldenScheduler
									batch = append(batch, tree.Write{Component: component.Name(), Instance: id, Field: tree.SchedulerName, Value: goldenScheduler, Options: options})
								}
								if writable(fragmented.NodeAffinity) {
									fragment.NodeAffinity = goldenNodeAffinity
									batch = append(batch, tree.Write{Component: component.Name(), Instance: id, Field: tree.NodeAffinity, Value: goldenNodeAffinity, Options: options})
								}
								values[id] = fragment
							}
							if err := component.UpdateFragmentedPodSpec(ctx, values); err != nil {
								t.Fatal(err)
							}
							if !compareTreeMutation(t, ctx, editable, factory, batch) {
								t.Fatal("fragmented update unexpectedly made the workload invalid")
							}
							writes += len(batch)
						}
					}
					if editable.IsSuspendable() != root.HasSuspendDefinition() {
						t.Fatal("tree suspend capability differs from the catalog definition")
					}
					if editable.IsSuspendable() {
						if err := root.Suspend(ctx); err != nil {
							t.Fatal(err)
						}
						if err := editable.Suspend(ctx); err != nil {
							t.Fatal(err)
						}
						compareTreeResource(t, editable, factory)
						if err := root.Resume(ctx); err != nil {
							t.Fatal(err)
						}
						if err := editable.Resume(ctx); err != nil {
							t.Fatal(err)
						}
						compareTreeResource(t, editable, factory)
						suspendResume++
						writes++
					}
					if writes == 0 {
						t.Fatal("recording exercised no mutations")
					}
					comparisons++
				})
			}
		})
	}
	if protectedRootReplacements == 0 {
		t.Fatal("no recorded Pod exercised the atomic invalid-root replacement guard")
	}
	t.Logf("mutation parity: %d recordings x %d policies = %d cases; %d matched valid legacy results; %d rejected unsafe Pod root replacements atomically in both APIs; %d suspend/resume round trips", len(paths), len(variants), len(paths)*len(variants), comparisons, protectedRootReplacements, suspendResume)
}

// The older mutation replay covers only a subset of fields. Exercise every
// declared write accessor individually, so labels, resources, scale and dynamic
// container destinations do not disappear behind the scheduler-only examples.
func TestEditableTreeEveryDeclaredWriteReplay(t *testing.T) {
	ctx := context.Background()
	var mutations, unavailable, fields, instances int
	for _, path := range replayPaths(t) {
		t.Run(strings.TrimPrefix(path, "../recorded_data/"), func(t *testing.T) {
			r, karta := replayDefinition(t, path)
			cr, reader := replayInput(t, r)
			factory := resource.NewComponentFactoryFromObject(karta, cr, resource.WithReferenceReader(reader))
			defs := append([]v1alpha1.ComponentDefinition{karta.Spec.StructureDefinition.RootComponent}, karta.Spec.StructureDefinition.ChildComponents...)
			for _, def := range defs {
				component, err := factory.GetComponent(def.Name)
				if err != nil {
					t.Fatal(err)
				}
				ids, err := component.GetInstanceIds(ctx)
				if err != nil {
					t.Fatal(err)
				}
				for _, field := range declaredExtractionFields(def) {
					if !writable(field.via) {
						continue
					}
					fields++
					for _, id := range ids {
						t.Run(def.Name+"/"+id+"/"+field.name, func(t *testing.T) {
							instances++
							editable, err := tree.Open(ctx, karta, cr, resource.WithReferenceReader(reader))
							if err != nil {
								t.Fatal(err)
							}
							target := tree.Target{Component: def.Name, Instance: id, Field: tree.Field(field.name)}
							resolved, err := editable.ResolveWriteTarget(ctx, target)
							if err != nil {
								// The failed KServe fixture has no recognized predictor
								// model. It must reject the write, not invent a model.
								if karta.Name != "serving-kserve-io-inferenceservice-v1beta1" || def.Name != "predictor" || field.name != "fragmented.container" || !strings.Contains(err.Error(), "must return one JSON Pointer string, got <nil>") {
									t.Fatalf("unexpected path-resolution error: %v", err)
								}
								before := jsonText(mustTreeResource(t, editable))
								if err := editable.Mutate(ctx, tree.Write{Component: def.Name, Instance: id, Field: target.Field, Value: map[string]any{"image": "ghcr.io/example/new:v2"}}); err == nil {
									t.Fatal("mutation accepted an unresolved destination")
								}
								if got := jsonText(mustTreeResource(t, editable)); got != before {
									t.Fatal("failed unresolved write changed the workload")
								}
								unavailable++
								t.Logf("optional target unavailable: %v", err)
								return
							}
							value := replayWriteValue(t, field.name)
							if err := editable.Mutate(ctx, tree.Write{Component: def.Name, Instance: id, Field: target.Field, Value: value}); err != nil {
								t.Fatalf("write %s: %v", resolved.Path, err)
							}
							after, err := editable.ResolveWriteTarget(ctx, target)
							if err != nil || !after.Exists || after.Path != resolved.Path {
								t.Fatalf("written target disappeared or moved: before=%+v after=%+v error=%v", resolved, after, err)
							}
							if !containsJSONValue(after.Value, value) {
								t.Fatalf("target %s does not contain the caller's value: got=%s supplied=%s", after.Path, jsonText(after.Value), jsonText(value))
							}
							// Rebuild extraction independently from the final object.
							// The tree must publish refreshed data, not stale nodes.
							final := mustTreeResource(t, editable)
							fresh, err := tree.Build(ctx, resource.NewComponentFactoryFromObject(karta, final, resource.WithReferenceReader(reader)))
							if err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(editable.Snapshot(), fresh) {
								t.Fatal("editable tree contains stale extracted values after mutation")
							}
							mutations++
						})
					}
				}
			}
		})
	}
	if mutations == 0 {
		t.Fatal("no declared write accessors were exercised")
	}
	t.Logf("declared-write coverage: %d accessor occurrences; %d instance targets; %d successful mutations; %d absent dynamic targets rejected atomically", fields, instances, mutations, unavailable)
}

func compareTreeMutation(t *testing.T, ctx context.Context, editable tree.Editable, factory *resource.ComponentFactory, batch []tree.Write) bool {
	t.Helper()
	before := jsonText(mustTreeResource(t, editable))
	_, baselineErr := factory.GetResource()
	err := editable.Mutate(ctx, batch...)
	if baselineErr != nil {
		if err == nil {
			t.Fatalf("tree published an invalid Kubernetes object: %v", baselineErr)
		}
		if after := jsonText(mustTreeResource(t, editable)); after != before {
			t.Fatal("failed root replacement changed the editable workload")
		}
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	compareTreeResource(t, editable, factory)
	return true
}

func compareTreeResource(t *testing.T, editable tree.Editable, factory *resource.ComponentFactory) {
	t.Helper()
	want, err := factory.GetResource()
	if err != nil {
		t.Fatal(err)
	}
	got := mustTreeResource(t, editable)
	if jsonText(got) != jsonText(want) {
		t.Fatalf("tree mutation differs from the existing SDK\ngot: %s\nwant: %s", jsonText(got), jsonText(want))
	}
}

func mustTreeResource(t *testing.T, editable tree.Editable) resource.KubernetesObject {
	t.Helper()
	object, err := editable.GetResource()
	if err != nil {
		t.Fatal(err)
	}
	return object
}

func replayInput(t *testing.T, recording *recorder.Reader) (resource.KubernetesObject, *recordedReader) {
	t.Helper()
	var object resource.KubernetesObject
	var reader *recordedReader
	for recording.Next() {
		object = recording.Object()
		reader = &recordedReader{objects: recording.References()}
		if recording.State() == "running" {
			break
		}
	}
	if object == nil {
		t.Fatal("recording has no workload object")
	}
	return object, reader
}

func writable(via *v1alpha1.ValueAccessor) bool {
	return via != nil && (via.PathWrite != nil || via.PathWriteExpression != "")
}

func replayWriteValue(t *testing.T, name string) any {
	t.Helper()
	switch name {
	case "podTemplateSpec":
		return map[string]any{"spec": map[string]any{"schedulerName": goldenScheduler}}
	case "podSpec":
		return map[string]any{"schedulerName": goldenScheduler}
	case "metadata":
		return map[string]any{"labels": map[string]any{"karta.example/replay": "updated"}}
	case "fragmented.schedulerName":
		return goldenScheduler
	case "fragmented.labels", "fragmented.annotations":
		return map[string]any{"karta.example/replay": "updated"}
	case "fragmented.resources":
		return map[string]any{"requests": map[string]any{"cpu": "2"}}
	case "fragmented.resourceClaims":
		return []any{map[string]any{"name": "replay-claim", "resourceClaimName": "example-claim"}}
	case "fragmented.podAffinity":
		return map[string]any{"requiredDuringSchedulingIgnoredDuringExecution": []any{map[string]any{"topologyKey": "kubernetes.io/hostname"}}}
	case "fragmented.nodeAffinity":
		return goldenNodeAffinity
	case "fragmented.containers":
		return []corev1.Container{{Name: "replay", Image: "ghcr.io/example/replay:v2"}}
	case "fragmented.container":
		return map[string]any{"image": "ghcr.io/example/replay:v2"}
	case "fragmented.priorityClassName":
		return "replay-priority"
	case "fragmented.image":
		return "ghcr.io/example/replay:v2"
	case "scale.replicas", "scale.minReplicas", "scale.maxReplicas":
		return int32(3)
	default:
		t.Fatalf("no example value for declared field %q", name)
		return nil
	}
}

func containsJSONValue(actual, supplied any) bool {
	var got, want any
	if err := normalizedJSONValue(actual, &got); err != nil {
		return false
	}
	if err := normalizedJSONValue(supplied, &want); err != nil {
		return false
	}
	return containsNormalizedJSON(got, want)
}

func normalizedJSONValue(value, target any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func containsNormalizedJSON(actual, supplied any) bool {
	if wantMap, ok := supplied.(map[string]any); ok {
		gotMap, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for key, value := range wantMap {
			got, found := gotMap[key]
			if !found || !containsNormalizedJSON(got, value) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(actual, supplied)
}
