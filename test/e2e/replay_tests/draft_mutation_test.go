// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package replay

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	jsonpatch "gopkg.in/evanphx/json-patch.v4"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

type draftReplayEdit struct {
	name     string
	keys     []string
	listItem bool
	value    any
}

// Drafts exercise scalar edits within retained raw targets and existing list
// items. Affinity/resource structure replacement remains covered by the legacy
// declared-write replay; this test does not claim to edit every accessor family.
func TestDraftMutationReplay(t *testing.T) {
	ctx := context.Background()
	paths := replayPaths(t)
	var mutations, missingParents, unresolved, targets, omittedFamilies int
	for _, path := range paths {
		t.Run(strings.TrimPrefix(path, "../recorded_data/"), func(t *testing.T) {
			r, karta := replayDefinition(t, path)
			original, reader := replayInput(t, r)
			factory := resource.NewComponentFactoryFromObject(karta, original, resource.WithReferenceReader(reader))
			writer, err := factory.PathWriter()
			if err != nil {
				t.Fatal(err)
			}
			defs := append([]v1alpha1.ComponentDefinition{karta.Spec.StructureDefinition.RootComponent}, karta.Spec.StructureDefinition.ChildComponents...)
			beforeMutations := mutations
			for _, def := range defs {
				component, err := factory.GetComponent(def.Name)
				if err != nil {
					t.Fatal(err)
				}
				ids, err := component.GetInstanceIds(ctx)
				if err != nil {
					t.Fatal(err)
				}
				fields := declaredExtractionFields(def)
				if suspend := def.SuspendDefinition; suspend != nil {
					fields = append(fields, extractionField{name: string(tree.SuspendField), via: &v1alpha1.ValueAccessor{PathWrite: suspend.PathWrite, PathWriteExpression: suspend.PathWriteExpression}})
				}
				for _, field := range fields {
					if !writable(field.via) {
						continue
					}
					edits := draftReplayEdits(field.name)
					if len(edits) == 0 {
						omittedFamilies++
						continue
					}
					for index, id := range ids {
						for _, edit := range edits {
							t.Run(def.Name+"/"+id+"/"+field.name+"/"+edit.name, func(t *testing.T) {
								targets++
								editor, err := tree.Open(ctx, karta, original, resource.WithReferenceReader(reader))
								if err != nil {
									t.Fatal(err)
								}
								before := jsonText(mustTreeResource(t, editor))
								beforeSnapshot := editor.Snapshot()
								draft, err := tree.BeginEdit(ctx, editor, tree.WithEditParents(resource.RequireParents))
								if err != nil {
									t.Fatal(err)
								}
								defer draft.Abort()
								target := tree.Target{Component: def.Name, Instance: id, Field: tree.Field(field.name)}
								resolved, resolveErr := writer.ResolveWriteTarget(ctx, field.via, id, index)
								cursor, targetErr := draft.Target(ctx, target)
								if resolveErr != nil {
									if karta.Name != "serving-kserve-io-inferenceservice-v1beta1" || def.Name != "predictor" || field.name != "fragmented.container" {
										t.Fatalf("unexpected catalog target failure: %v", resolveErr)
									}
									if targetErr == nil || draft.Commit(ctx) == nil {
										t.Fatal("draft accepted an unresolved model destination")
									}
									assertDraftReplayUnchanged(t, editor, before, beforeSnapshot)
									unresolved++
									return
								}
								if targetErr == nil {
									value, exists, err := cursor.Read()
									if err != nil || exists != resolved.Exists || !reflect.DeepEqual(value, resolved.Value) {
										t.Fatalf("draft raw read differs from the independently resolved target: %v", err)
									}
								}
								value := edit.value
								if strings.HasPrefix(field.name, "scale.") {
									if current, ok := resolved.Value.(float64); ok {
										value = current + 1
									}
								} else if field.name == string(tree.SuspendField) {
									value = resolved.Value != true
								}
								pointer := resolved.Path
								for _, key := range edit.keys {
									pointer += "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
								}
								if edit.listItem {
									pointer += "/0/image"
								}
								// RFC 6902 add changes one leaf and requires its parent.
								// This oracle is independent of Draft's retained-node engine.
								expected, expectedErr := draftReplayExpected(before, pointer, value)
								selected, editErr := draftReplayCursor(cursor, targetErr, edit)
								if editErr == nil {
									editErr = selected.Set(value)
								}
								if expectedErr != nil {
									if editErr == nil || draft.Commit(ctx) == nil {
										t.Fatalf("RequireParents accepted an absent or non-object parent at %s", pointer)
									}
									assertDraftReplayUnchanged(t, editor, before, beforeSnapshot)
									missingParents++
									return
								}
								if editErr != nil {
									t.Fatalf("edit %s: %v", pointer, editErr)
								}
								if expected == before {
									t.Fatal("replay must perform a non-no-op edit")
								}
								if err := draft.Commit(ctx); err != nil {
									t.Fatalf("commit %s: %v", pointer, err)
								}
								final := mustTreeResource(t, editor)
								if jsonText(final) != expected {
									t.Fatalf("draft changed raw fields beyond the intended leaf %s", pointer)
								}
								if jsonText(original) != before {
									t.Fatal("draft changed caller-owned recording input")
								}
								fresh, err := tree.Build(ctx, resource.NewComponentFactoryFromObject(karta, final, resource.WithReferenceReader(reader)))
								if err != nil || !reflect.DeepEqual(editor.Snapshot(), fresh) {
									t.Fatalf("draft published stale extraction: %v", err)
								}
								readback, err := tree.BeginEdit(ctx, editor)
								if err != nil {
									t.Fatal(err)
								}
								defer readback.Abort()
								cursor, err = readback.Target(ctx, target)
								selected, err = draftReplayCursor(cursor, err, edit)
								if err != nil {
									t.Fatal(err)
								}
								got, exists, err := selected.Read()
								if err != nil || !exists || jsonText(got) != jsonText(value) {
									t.Fatalf("committed draft readback = %v, exists=%v, error=%v", got, exists, err)
								}
								mutations++
							})
						}
					}
				}
			}
			if mutations == beforeMutations {
				t.Fatal("recording exercised no successful draft mutations")
			}
		})
	}
	if missingParents == 0 || unresolved == 0 {
		t.Fatal("recordings must exercise both strict-parent and unresolved-target rejection")
	}
	t.Logf("draft coverage: %d recordings; %d attempted target edits; %d exact raw-object mutations and readbacks; %d strict-parent refusals; %d unresolved dynamic targets; %d declared accessor occurrences delegated to legacy structural-write coverage", len(paths), targets, mutations, missingParents, unresolved, omittedFamilies)
}

func draftReplayEdits(field string) []draftReplayEdit {
	const image = "ghcr.io/example/draft-replay:v2"
	switch field {
	case "podTemplateSpec":
		return []draftReplayEdit{{name: "scheduler", keys: []string{"spec", "schedulerName"}, value: goldenScheduler}, {name: "container-image", keys: []string{"spec", "containers"}, listItem: true, value: image}}
	case "podSpec":
		return []draftReplayEdit{{name: "scheduler", keys: []string{"schedulerName"}, value: goldenScheduler}, {name: "container-image", keys: []string{"containers"}, listItem: true, value: image}}
	case "metadata":
		return []draftReplayEdit{{name: "label", keys: []string{"labels", "karta.example/replay"}, value: "updated"}}
	case "fragmented.labels", "fragmented.annotations":
		return []draftReplayEdit{{name: "entry", keys: []string{"karta.example/replay"}, value: "updated"}}
	case "fragmented.schedulerName":
		return []draftReplayEdit{{name: "scheduler", value: goldenScheduler}}
	case "fragmented.priorityClassName":
		return []draftReplayEdit{{name: "priority", value: "draft-replay-priority"}}
	case "fragmented.image":
		return []draftReplayEdit{{name: "image", value: image}}
	case "fragmented.container":
		return []draftReplayEdit{{name: "image", keys: []string{"image"}, value: image}}
	case "fragmented.containers":
		return []draftReplayEdit{{name: "container-image", listItem: true, value: image}}
	case "scale.replicas", "scale.minReplicas", "scale.maxReplicas":
		return []draftReplayEdit{{name: "replicas", value: float64(7)}}
	case "suspend":
		return []draftReplayEdit{{name: "toggle", value: true}}
	default:
		return nil
	}
}

func draftReplayCursor(cursor *tree.Cursor, err error, edit draftReplayEdit) (*tree.Cursor, error) {
	if err != nil {
		return nil, err
	}
	cursor = cursor.At(edit.keys...)
	if edit.listItem {
		items, err := cursor.Items()
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			return cursor.Match("name", "draft-replay-absent")
		}
		cursor = items[0].At("image")
	}
	return cursor, nil
}

func draftReplayExpected(before, path string, value any) (string, error) {
	encoded, err := json.Marshal([]map[string]any{{"op": "add", "path": path, "value": value}})
	if err != nil {
		return "", err
	}
	patch, err := jsonpatch.DecodePatch(encoded)
	if err != nil {
		return "", err
	}
	result, err := patch.Apply([]byte(before))
	if err != nil {
		return "", err
	}
	var normalized any
	if err := json.Unmarshal(result, &normalized); err != nil {
		return "", err
	}
	return jsonText(normalized), nil
}

func assertDraftReplayUnchanged(t *testing.T, editor tree.Editable, before string, snapshot *tree.WorkloadTree) {
	t.Helper()
	if jsonText(mustTreeResource(t, editor)) != before || !reflect.DeepEqual(editor.Snapshot(), snapshot) {
		t.Fatal("rejected draft changed the workload or extraction")
	}
}
