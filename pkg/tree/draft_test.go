// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

type unsupportedDraftEditor struct{ tree.Editable }

func TestDraftLeafPreservesCompleteRawListsAndDetachedReads(t *testing.T) {
	workload := safetyDeployment()
	pod := workload.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
	pod["containers"].([]any)[0].(map[string]any)["unknown"] = map[string]any{"nested": []any{nil, false, float64(0), map[string]any{"keep": true}}}
	pod["containers"] = append(pod["containers"].([]any), map[string]any{"name": "sidecar", "image": "sidecar:v1", "custom": []any{map[string]any{"retain": "all"}}})
	editor := draftEditor(t, kartas.Deployment(), workload)
	want := safetyResource(t, editor)
	draft := draftBegin(t, editor)
	template := draftTarget(t, draft, "deployment", tree.PodTemplateSpec)
	containers := template.At("spec", "containers")
	main, err := containers.Match("name", "api")
	draftOK(t, err)
	image := main.At("image")
	var typed corev1.Container
	draftOK(t, main.ReadInto(&typed))
	typed.Image = "typed mutation"
	typed.Env[0].Value = "typed mutation"
	raw, exists, err := main.Read()
	draftOK(t, err)
	if !exists {
		t.Fatal("existing container was absent")
	}
	raw.(map[string]any)["unknown"].(map[string]any)["nested"].([]any)[1] = true
	snapshot := draft.Snapshot()
	snapshot.Root.Instances[0].ExtractedInstance.PodTemplateSpec.Spec.Containers[0].Image = "snapshot mutation"
	draftOK(t, image.Set("api:v2"))
	read, exists, err := image.Read()
	draftOK(t, err)
	if !exists || read != "ghcr.io/example/api:v1" {
		t.Fatalf("read followed staged edit: %#v", read)
	}
	if got := draft.Snapshot().Root.Instances[0].ExtractedInstance.PodTemplateSpec.Spec.Containers[0].Image; got != "ghcr.io/example/api:v1" {
		t.Fatalf("snapshot was aliased or staged: %q", got)
	}
	want["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)["image"] = "api:v2"
	draftOK(t, draft.Commit(t.Context()))
	draftEqual(t, safetyResource(t, editor), want)
	if got := editor.Snapshot().Root.Instances[0].ExtractedInstance.PodTemplateSpec.Spec.Containers[0].Image; got != "api:v2" {
		t.Fatalf("commit did not publish extraction: %q", got)
	}
	if err := image.Set("closed"); !errors.Is(err, tree.ErrDraftClosed) {
		t.Fatalf("committed cursor remained usable: %v", err)
	}
}

func TestDraftKServePreservationAndOptionalParents(t *testing.T) {
	for _, flavor := range []string{"model", "sklearn", "escaped/model~"} {
		t.Run(flavor, func(t *testing.T) {
			editor := draftEditor(t, kartas.KServe(), draftKServe(flavor))
			want := safetyResource(t, editor)
			draft := draftBegin(t, editor)
			container := draftTarget(t, draft, "predictor", tree.Container)
			draftOK(t, container.At("image").Set("inference:v2"))
			draftOK(t, draft.Commit(t.Context()))
			want["spec"].(map[string]any)["predictor"].(map[string]any)[flavor].(map[string]any)["image"] = "inference:v2"
			draftEqual(t, safetyResource(t, editor), want)
		})
	}
	for _, create := range []bool{false, true} {
		t.Run(fmt.Sprintf("create=%v", create), func(t *testing.T) {
			editor := draftEditor(t, kartas.KServe(), draftKServe("model"))
			before := safetyResource(t, editor)
			var options []tree.EditOption
			if create {
				options = append(options, tree.WithEditParents(resource.CreateMapParents))
			}
			draft := draftBegin(t, editor, options...)
			transformer := draftTarget(t, draft, "transformer", tree.PodSpec)
			if _, exists, err := transformer.Read(); err != nil || exists {
				t.Fatalf("missing transformer read: exists %v, err %v", exists, err)
			}
			err := transformer.At("schedulerName").Set("batch-scheduler")
			if create {
				draftOK(t, err)
				draftOK(t, draft.Commit(t.Context()))
				before["spec"].(map[string]any)["transformer"] = map[string]any{"schedulerName": "batch-scheduler"}
			} else if err == nil || draft.Commit(t.Context()) == nil {
				t.Fatal("missing transformer parent was silently created")
			}
			draftEqual(t, safetyResource(t, editor), before)
		})
	}
	for _, existing := range []any{nil, "scalar", []any{}} {
		definition, workload := namedFieldsFixture(map[string]any{"parent": existing})
		editor := draftEditor(t, definition, workload)
		before := safetyResource(t, editor)
		draft := draftBegin(t, editor, tree.WithEditParents(resource.CreateMapParents))
		cursor := draftTarget(t, draft, "deployment", "exampleos")
		if err := cursor.At("parent", "child").Set("bad"); err == nil {
			t.Fatalf("CreateMapParents replaced existing %T parent", existing)
		}
		if draft.Commit(t.Context()) == nil {
			t.Fatal("wrong parent draft committed")
		}
		draftEqual(t, safetyResource(t, editor), before)
	}
}

func TestDraftDynamicTargetsResolveAgainstBaseline(t *testing.T) {
	definition, object := safetyDynamicDefinition()
	editor := draftEditor(t, definition, object)
	want := safetyResource(t, editor)
	draft := draftBegin(t, editor)
	destination := draftTarget(t, draft, "deployment", tree.SchedulerName)
	draftOK(t, destination.Set("right"))
	image := draftTarget(t, draft, "deployment", tree.Image)
	path, err := image.Path()
	draftOK(t, err)
	if path != "/spec/left/image" {
		t.Fatalf("prior edit redirected dynamic target: %q", path)
	}
	draftOK(t, image.Set("left:v2"))
	draftOK(t, draft.Commit(t.Context()))
	want["spec"].(map[string]any)["destination"] = "right"
	want["spec"].(map[string]any)["left"].(map[string]any)["image"] = "left:v2"
	draftEqual(t, safetyResource(t, editor), want)
	fresh := draftBegin(t, editor)
	path, err = draftTarget(t, fresh, "deployment", tree.Image).Path()
	draftOK(t, err)
	if path != "/spec/right/image" {
		t.Fatalf("fresh draft did not resolve latest destination: %q", path)
	}
	fresh.Abort()
}

func TestDraftValuesCopiesAndLiteralMapKeys(t *testing.T) {
	definition, workload := namedFieldsFixture(map[string]any{"null": nil, "zero": 0, "false": false, "empty": "", "remove": "old"})
	editor := draftEditor(t, definition, workload)
	want := safetyResource(t, editor)
	draft := draftBegin(t, editor, tree.WithEditParents(resource.CreateMapParents))
	value := draftTarget(t, draft, "deployment", "exampleos")
	if got, exists, err := value.At("null").Read(); err != nil || !exists || got != nil {
		t.Fatalf("literal null read: %#v, %v, %v", got, exists, err)
	}
	if got, exists, err := value.At("missing").Read(); err != nil || exists || got != nil {
		t.Fatalf("absent read: %#v, %v, %v", got, exists, err)
	}
	draftOK(t, value.At("missing").Set(nil))
	draftOK(t, value.At("zero").Set(0))
	draftOK(t, value.At("false").Set(false))
	draftOK(t, value.At("remove").Remove())
	draftOK(t, value.At("absent").Remove())
	key := value.At("new", "0", "example.com/~key")
	draftOK(t, key.Set("literal"))
	path, err := key.Path()
	draftOK(t, err)
	if path != "/d/d/c/new/0/example.com~1~0key" {
		t.Fatalf("literal key path = %q", path)
	}
	replacement := map[string]any{"slice": []any{map[string]any{"accepted": true}}}
	draftOK(t, value.At("copy").Replace(replacement))
	replacement["slice"].([]any)[0].(map[string]any)["accepted"] = false
	pointedValue := ptr.To("accepted scalar")
	draftOK(t, value.At("pointed").Set(pointedValue))
	*pointedValue = "changed scalar"
	draftOK(t, draft.Commit(t.Context()))
	expected := want["d"].(map[string]any)["d"].(map[string]any)["c"].(map[string]any)
	expected["missing"] = nil
	delete(expected, "remove")
	expected["new"] = map[string]any{"0": map[string]any{"example.com/~key": "literal"}}
	expected["copy"] = map[string]any{"slice": []any{map[string]any{"accepted": true}}}
	expected["pointed"] = "accepted scalar"
	draftEqual(t, safetyResource(t, editor), want)
}

func TestDraftIgnoredErrorsPoisonAndRollback(t *testing.T) {
	for _, failure := range []string{"set map", "set over map", "bad at", "bad items", "zero match", "duplicate match", "read absent", "read bad output", "overlap", "unknown target", "read-only"} {
		t.Run(failure, func(t *testing.T) {
			definition, workload := namedFieldsFixture(map[string]any{"ok": "old", "list": []any{map[string]any{"name": "duplicate"}, map[string]any{"name": "duplicate"}}})
			definition.Spec.StructureDefinition.RootComponent.Fields["readonly"] = v1alpha1.ValueAccessor{Expression: `object.d.d.c`}
			editor := draftEditor(t, definition, workload)
			before, snapshot := safetyResource(t, editor), editor.Snapshot()
			draft := draftBegin(t, editor)
			cursor := draftTarget(t, draft, "deployment", "exampleos")
			draftOK(t, cursor.At("ok").Set("staged"))
			var err error
			switch failure {
			case "set map":
				err = cursor.At("leaf").Set(map[string]any{})
			case "set over map":
				err = cursor.Set(nil)
			case "bad at":
				cursor.At("ok", "bad")
				err = draft.Commit(t.Context())
			case "bad items":
				_, err = cursor.Items()
			case "zero match":
				_, err = cursor.At("list").Match("name", "absent")
			case "duplicate match":
				_, err = cursor.At("list").Match("name", "duplicate")
			case "read absent":
				var out any
				err = cursor.At("absent").ReadInto(&out)
			case "read bad output":
				err = cursor.ReadInto(nil)
			case "overlap":
				_, err = draft.Target(t.Context(), tree.Target{Component: "deployment", Field: "exampleos"})
			case "unknown target":
				_, err = draft.Target(t.Context(), tree.Target{Component: "absent", Field: "exampleos"})
			case "read-only":
				_, err = draft.Target(t.Context(), tree.Target{Component: "deployment", Field: "readonly"})
			}
			if err == nil || draft.Commit(t.Context()) == nil {
				t.Fatal("ignored error permitted publishing earlier edits")
			}
			if draft.Snapshot() != nil {
				t.Fatal("terminally failed draft returned a snapshot")
			}
			draftEqual(t, safetyResource(t, editor), before)
			draftEqual(t, editor.Snapshot(), snapshot)
		})
	}
}

func TestDraftStructuralOperationsKeepRawIdentity(t *testing.T) {
	definition, workload := namedFieldsFixture([]any{
		map[string]any{"id": "a", "value": "old", "opaque": []any{map[string]any{"keep": true}}},
		map[string]any{"id": "b", "opaque": "remove"},
		map[string]any{"id": "c", "opaque": "keep"},
	})
	editor := draftEditor(t, definition, workload)
	want := safetyResource(t, editor)
	draft := draftBegin(t, editor)
	list := draftTarget(t, draft, "deployment", "exampleos")
	items, err := list.Items()
	draftOK(t, err)
	if len(items) != 3 {
		t.Fatalf("Items returned %d handles", len(items))
	}
	leaf := items[0].At("value")
	draftOK(t, items[0].At("id").Set("renamed"))
	baselineMatch, err := list.Match("id", "a")
	draftOK(t, err)
	draftOK(t, baselineMatch.MoveBefore(nil))
	path, err := leaf.Path()
	draftOK(t, err)
	if path != "/d/d/c/2/value" {
		t.Fatalf("moved descendant path = %q", path)
	}
	draftOK(t, leaf.Set("new"))
	input := map[string]any{"id": "inserted", "unknown": []any{true}}
	inserted, err := list.InsertBefore(items[2], input)
	draftOK(t, err)
	input["unknown"].([]any)[0] = false
	draftOK(t, inserted.MoveBefore(items[1]))
	draftOK(t, items[1].Remove())
	draftOK(t, items[0].MoveBefore(items[2]))
	temporary, err := list.InsertBefore(nil, "temporary")
	draftOK(t, err)
	draftOK(t, temporary.Replace(map[string]any{"temporary": true}))
	draftOK(t, temporary.Remove())
	draftOK(t, draft.Commit(t.Context()))
	want["d"].(map[string]any)["d"].(map[string]any)["c"] = []any{
		map[string]any{"id": "inserted", "unknown": []any{true}},
		map[string]any{"id": "renamed", "value": "new", "opaque": []any{map[string]any{"keep": true}}},
		map[string]any{"id": "c", "opaque": "keep"},
	}
	draftEqual(t, safetyResource(t, editor), want)
}

func TestDraftInvalidatedAndForeignHandlesFail(t *testing.T) {
	for _, operation := range []string{"replace parent", "remove parent", "remove item", "insert child", "foreign move", "foreign insert"} {
		t.Run(operation, func(t *testing.T) {
			definition, workload := namedFieldsFixture(map[string]any{"list": []any{map[string]any{"leaf": "old"}}})
			editor := draftEditor(t, definition, workload)
			before := safetyResource(t, editor)
			draft := draftBegin(t, editor)
			parent := draftTarget(t, draft, "deployment", "exampleos")
			list := parent.At("list")
			items, err := list.Items()
			draftOK(t, err)
			leaf := items[0].At("leaf")
			var failed error
			switch operation {
			case "replace parent", "remove parent":
				if operation == "remove parent" {
					draftOK(t, parent.Remove())
				} else {
					draftOK(t, parent.Replace(map[string]any{}))
				}
				draftOK(t, parent.Replace(map[string]any{"list": []any{map[string]any{"leaf": "new"}}}))
				failed = leaf.Set("invalid")
			case "remove item":
				draftOK(t, items[0].Remove())
				failed = items[0].Replace(map[string]any{})
			case "insert child":
				inserted, err := list.InsertBefore(nil, map[string]any{"leaf": "new"})
				draftOK(t, err)
				failed = inserted.At("leaf").Set("invalid")
			case "foreign move", "foreign insert":
				other := draftBegin(t, editor)
				defer other.Abort()
				foreign, err := draftTarget(t, other, "deployment", "exampleos").At("list").Items()
				draftOK(t, err)
				if operation == "foreign move" {
					failed = items[0].MoveBefore(foreign[0])
				} else {
					_, failed = list.InsertBefore(foreign[0], "invalid")
				}
				if other.Snapshot() == nil {
					t.Fatal("foreign draft was poisoned by the caller's misuse")
				}
			}
			if failed == nil || draft.Commit(t.Context()) == nil {
				t.Fatal("invalid handle did not poison the draft")
			}
			draftEqual(t, safetyResource(t, editor), before)
		})
	}
}

func TestDraftStaleAcrossEveryPublication(t *testing.T) {
	for _, publication := range []string{"mutate", "suspend", "resume", "draft"} {
		t.Run(publication, func(t *testing.T) {
			workload := safetyDeployment()
			workload.SetAPIVersion("batch/v1")
			workload.SetKind("Job")
			workload.Object["spec"].(map[string]any)["suspend"] = publication == "resume"
			editor := draftEditor(t, kartas.BatchJob(), workload)
			draft := draftBegin(t, editor)
			draftOK(t, draftTarget(t, draft, "job", tree.PodTemplateSpec).At("spec", "schedulerName").Set("stale"))
			switch publication {
			case "mutate":
				draftOK(t, editor.Mutate(t.Context(), tree.Write{Component: "job", Field: tree.PodTemplateSpec, Value: map[string]any{"spec": map[string]any{"schedulerName": "published"}}}))
			case "suspend":
				draftOK(t, editor.Suspend(t.Context()))
			case "resume":
				draftOK(t, editor.Resume(t.Context()))
			case "draft":
				other := draftBegin(t, editor)
				draftOK(t, draftTarget(t, other, "job", tree.SuspendField).Set(true))
				draftOK(t, other.Commit(t.Context()))
			}
			before, snapshot := safetyResource(t, editor), editor.Snapshot()
			if err := draft.Commit(t.Context()); !errors.Is(err, tree.ErrStaleDraft) {
				t.Fatalf("intervening %s did not stale the draft: %v", publication, err)
			}
			draftEqual(t, safetyResource(t, editor), before)
			draftEqual(t, editor.Snapshot(), snapshot)
		})
	}
}

func TestDraftNoOpAndAbortDoNotInvalidateOtherDraft(t *testing.T) {
	editor := draftEditor(t, kartas.Deployment(), safetyDeployment())
	active := draftBegin(t, editor)
	draftOK(t, draftTarget(t, active, "deployment", tree.PodTemplateSpec).At("spec", "schedulerName").Set("active"))
	draftOK(t, draftBegin(t, editor).Commit(t.Context()))
	sameValue := draftBegin(t, editor)
	draftOK(t, draftTarget(t, sameValue, "deployment", tree.PodTemplateSpec).At("spec", "schedulerName").Set("default-scheduler"))
	draftOK(t, sameValue.Commit(t.Context()))
	aborted := draftBegin(t, editor)
	cursor := draftTarget(t, aborted, "deployment", tree.PodTemplateSpec)
	draftOK(t, cursor.At("spec", "schedulerName").Set("aborted"))
	aborted.Abort()
	aborted.Abort()
	if err := aborted.Commit(t.Context()); !errors.Is(err, tree.ErrDraftClosed) {
		t.Fatalf("aborted draft committed: %v", err)
	}
	if _, _, err := cursor.Read(); !errors.Is(err, tree.ErrDraftClosed) {
		t.Fatalf("aborted cursor remained valid: %v", err)
	}
	draftOK(t, editor.Mutate(t.Context()))
	if err := editor.Mutate(t.Context(), tree.Write{Component: "deployment", Field: tree.PodTemplateSpec,
		Value: map[string]any{"spec": map[string]any{"containers": "invalid extraction"}},
	}); err == nil {
		t.Fatal("invalid legacy mutation unexpectedly succeeded")
	}
	failed := draftBegin(t, editor)
	draftOK(t, draftTarget(t, failed, "deployment", tree.PodTemplateSpec).At("spec", "containers").Replace("invalid extraction"))
	if err := failed.Commit(t.Context()); err == nil {
		t.Fatal("invalid sibling draft unexpectedly committed")
	}
	draftOK(t, active.Commit(t.Context()))
}

func TestDraftTargetOverlapAndEscapedSiblingKeys(t *testing.T) {
	for _, overlap := range []bool{false, true} {
		t.Run(fmt.Sprintf("overlap=%v", overlap), func(t *testing.T) {
			definition, workload := namedFieldsFixture(map[string]any{"a/b": "literal", "a": map[string]any{"b": "nested"}})
			fields := definition.Spec.StructureDefinition.RootComponent.Fields
			fields["literal"] = v1alpha1.ValueAccessor{Expression: `object.d.d.c["a/b"]`, PathWrite: ptr.To("/d/d/c/a~1b")}
			fields["nested"] = v1alpha1.ValueAccessor{Expression: `object.d.d.c.a.b`, PathWrite: ptr.To("/d/d/c/a/b")}
			editor := draftEditor(t, definition, workload)
			want := safetyResource(t, editor)
			draft := draftBegin(t, editor)
			draftOK(t, draftTarget(t, draft, "deployment", "literal").Set("changed literal"))
			if overlap {
				if _, err := draft.Target(t.Context(), tree.Target{Component: "deployment", Field: "exampleos"}); err == nil {
					t.Fatal("ancestor target was accepted after descendant target")
				}
				if draft.Commit(t.Context()) == nil {
					t.Fatal("overlapping draft published")
				}
			} else {
				draftOK(t, draftTarget(t, draft, "deployment", "nested").Set("changed nested"))
				draftOK(t, draft.Commit(t.Context()))
				value := want["d"].(map[string]any)["d"].(map[string]any)["c"].(map[string]any)
				value["a/b"] = "changed literal"
				value["a"].(map[string]any)["b"] = "changed nested"
			}
			draftEqual(t, safetyResource(t, editor), want)
		})
	}
}

func TestDraftCommitRollbackAndCancellation(t *testing.T) {
	for _, failure := range []string{"extraction", "root", "canceled commit", "canceled target"} {
		t.Run(failure, func(t *testing.T) {
			definition, workload := kartas.Deployment(), safetyDeployment()
			component := "deployment"
			if failure == "root" {
				definition, component = kartas.Pod(), "pod"
				workload.SetAPIVersion("v1")
				workload.SetKind("Pod")
				workload.Object["spec"] = workload.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"]
			}
			editor := draftEditor(t, definition, workload)
			before, snapshot := safetyResource(t, editor), editor.Snapshot()
			draft := draftBegin(t, editor)
			cursor := draftTarget(t, draft, component, tree.PodTemplateSpec)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			var err error
			switch failure {
			case "extraction":
				draftOK(t, cursor.At("spec", "containers").Replace("not a list"))
				err = draft.Commit(t.Context())
			case "root":
				path, pathErr := cursor.Path()
				draftOK(t, pathErr)
				if path != "" {
					t.Fatalf("Pod root path = %q", path)
				}
				draftOK(t, cursor.Replace(map[string]any{"spec": map[string]any{}}))
				err = draft.Commit(t.Context())
			case "canceled commit":
				draftOK(t, cursor.At("spec", "schedulerName").Set("canceled"))
				err = draft.Commit(ctx)
			case "canceled target":
				_, err = draft.Target(ctx, tree.Target{Component: component, Field: tree.PodTemplateSpec})
			}
			if err == nil || draft.Commit(t.Context()) == nil {
				t.Fatal("failed transaction could publish")
			}
			draftEqual(t, safetyResource(t, editor), before)
			draftEqual(t, editor.Snapshot(), snapshot)
		})
	}
}

func TestDraftConcurrentCommitAndLegacyMutate(t *testing.T) {
	for iteration := 0; iteration < 12; iteration++ {
		editor := draftEditor(t, kartas.Deployment(), safetyDeployment())
		draft := draftBegin(t, editor)
		template := draftTarget(t, draft, "deployment", tree.PodTemplateSpec)
		main, err := template.At("spec", "containers").Match("name", "api")
		draftOK(t, err)
		draftOK(t, main.At("image").Set("draft:v2"))
		start := make(chan struct{})
		committed, mutated := make(chan error, 1), make(chan error, 1)
		go func() {
			<-start
			committed <- draft.Commit(t.Context())
		}()
		go func() {
			<-start
			mutated <- editor.Mutate(t.Context(), tree.Write{Component: "deployment", Field: tree.PodTemplateSpec, Value: map[string]any{"spec": map[string]any{"schedulerName": "legacy"}}})
		}()
		close(start)
		for range 3 {
			editor.Snapshot()
			if _, err := editor.GetResource(); err != nil {
				t.Fatal(err)
			}
		}
		draftOK(t, <-mutated)
		commitErr := <-committed
		if commitErr != nil && !errors.Is(commitErr, tree.ErrStaleDraft) {
			t.Fatalf("concurrent commit failed unexpectedly: %v", commitErr)
		}
		current := safetyResource(t, editor)["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
		if current["schedulerName"] != "legacy" {
			t.Fatal("draft overwrote concurrent legacy publication")
		}
		wantImage := "draft:v2"
		if commitErr != nil {
			wantImage = "ghcr.io/example/api:v1"
		}
		if image := current["containers"].([]any)[0].(map[string]any)["image"]; image != wantImage {
			t.Fatalf("commit result %v disagrees with raw publication: %#v", commitErr, image)
		}
		extracted := editor.Snapshot().Root.Instances[0].ExtractedInstance.PodTemplateSpec.Spec
		if extracted.SchedulerName != "legacy" || extracted.Containers[0].Image != wantImage {
			t.Fatal("concurrent publication split raw resource and extraction")
		}
	}
}

func TestDraftUnsupportedNilAndInvalidOptions(t *testing.T) {
	var typedNil *unsupportedDraftEditor
	for _, editor := range []tree.Editable{nil, typedNil, unsupportedDraftEditor{}} {
		if _, err := tree.BeginEdit(t.Context(), editor); !errors.Is(err, tree.ErrEditUnsupported) {
			t.Fatalf("unsupported editor %T returned %v", editor, err)
		}
	}
	editor := draftEditor(t, kartas.Deployment(), safetyDeployment())
	for _, option := range []tree.EditOption{nil, tree.WithEditParents("invalid"), tree.WithEditParents("")} {
		if _, err := tree.BeginEdit(t.Context(), editor, option); err == nil {
			t.Fatal("invalid edit option accepted")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := tree.BeginEdit(ctx, editor); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled BeginEdit returned %v", err)
	}
	var nilDraft *tree.Draft
	nilDraft.Abort()
	if nilDraft.Snapshot() != nil || !errors.Is(nilDraft.Commit(t.Context()), tree.ErrDraftClosed) {
		t.Fatal("nil draft did not fail cleanly")
	}
	var nilCursor *tree.Cursor
	if err := nilCursor.At("key").Set("value"); !errors.Is(err, tree.ErrDraftClosed) {
		t.Fatalf("nil cursor did not fail cleanly: %v", err)
	}
}

func draftEditor(t testing.TB, definition *v1alpha1.Karta, workload *unstructured.Unstructured) tree.Editable {
	t.Helper()
	editor, err := tree.Open(context.Background(), definition, workload)
	draftOK(t, err)
	return editor
}

func draftBegin(t testing.TB, editor tree.Editable, options ...tree.EditOption) *tree.Draft {
	t.Helper()
	draft, err := tree.BeginEdit(context.Background(), editor, options...)
	draftOK(t, err)
	return draft
}

func draftTarget(t testing.TB, draft *tree.Draft, component string, field tree.Field) *tree.Cursor {
	t.Helper()
	cursor, err := draft.Target(context.Background(), tree.Target{Component: component, Field: field})
	draftOK(t, err)
	return cursor
}

func draftOK(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func draftEqual(t testing.TB, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("published result mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func draftKServe(flavor string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "serving.kserve.io/v1beta1", "kind": "InferenceService",
		"metadata": map[string]any{"name": "draft-example"},
		"spec": map[string]any{"predictor": map[string]any{flavor: map[string]any{
			"image": "inference:v1", "storageUri": "s3://example-models/model",
			"modelFormat": map[string]any{"name": "sklearn", "version": "1"},
			"unknown":     []any{map[string]any{"nested": []any{nil, false, float64(0)}}},
		}}},
	}}
}

func BenchmarkDraftEdits(b *testing.B) {
	for _, count := range []int{1, 32} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			definition := kartas.Deployment()
			definition.Spec.StructureDefinition.RootComponent.Fields = map[string]v1alpha1.ValueAccessor{
				"extra": {Expression: `object.extra`, PathWrite: ptr.To("/extra")},
			}
			workload := safetyDeployment()
			extra := make(map[string]any, count)
			for index := range count {
				extra[fmt.Sprint(index)] = float64(0)
			}
			workload.Object["extra"] = runtime.DeepCopyJSON(extra)
			editor := draftEditor(b, definition, workload)
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				draft := draftBegin(b, editor)
				cursor := draftTarget(b, draft, "deployment", "extra")
				for key := range extra {
					draftOK(b, cursor.At(key).Set(index+1))
				}
				draftOK(b, draft.Commit(context.Background()))
			}
		})
	}
}

func BenchmarkDraftListEdit(b *testing.B) {
	for _, size := range []int{1, 256} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			items := make([]any, size)
			for index := range items {
				items[index] = map[string]any{"id": index, "image": "old", "unknown": map[string]any{"nested": []any{nil, false, "retain"}}}
			}
			definition, workload := namedFieldsFixture(items)
			editor := draftEditor(b, definition, workload)
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				draft := draftBegin(b, editor)
				list := draftTarget(b, draft, "deployment", "exampleos")
				item, err := list.Match("id", size/2)
				draftOK(b, err)
				draftOK(b, item.At("image").Set(fmt.Sprint(index)))
				draftOK(b, draft.Commit(context.Background()))
			}
		})
	}
}
