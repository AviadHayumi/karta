// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package resource

import (
	"context"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/expression"
)

type fragmentedTrackingRunner struct {
	expression.Runner
	assignments         int
	instanceExpression  string
	instanceEvaluations int
}

func (r *fragmentedTrackingRunner) Assign(ctx context.Context, expression string, value any) error {
	r.assignments++
	return r.Runner.Assign(ctx, expression, value)
}

func (r *fragmentedTrackingRunner) EvaluateWithVariables(ctx context.Context, expression string, vars map[string]any) ([]any, error) {
	if expression == r.instanceExpression {
		r.instanceEvaluations++
	}
	return r.Runner.EvaluateWithVariables(ctx, expression, vars)
}

func TestFragmentedWritesRejectOverlaps(t *testing.T) {
	for _, format := range []PatchType{PatchTypeMergePatch, PatchTypeJSONPatch} {
		for _, strategy := range []MergeStrategy{Merge, Replace} {
			for _, test := range []struct {
				name      string
				image     string
				container string
			}{
				{name: "same target", image: "/spec/main", container: "/spec/main"},
				{name: "later ancestor", image: "/spec/main/image", container: "/spec/main"},
				{name: "earlier ancestor", image: "/spec/main", container: "/spec/main/child"},
				{name: "root overlaps all", image: "", container: "/spec/main"},
				{name: "escaped key ancestor", image: "/spec/a~1b/image", container: "/spec/a~1b"},
				{name: "escaped tilde ancestor", image: "/spec/a~01b/image", container: "/spec/a~01b"},
			} {
				t.Run(string(format)+"/"+string(strategy)+"/"+test.name, func(t *testing.T) {
					accessor := sdkTestAccessor(t, map[string]any{"spec": map[string]any{
						"main": map[string]any{"image": "old", "child": map[string]any{}},
					}}, MutationOptions{PatchType: format, Strategy: strategy})
					runner := &fragmentedTrackingRunner{Runner: accessor.runner}
					accessor.runner = runner
					before, err := jsonCopy(sdkTestObject(t, accessor))
					if err != nil {
						t.Fatal(err)
					}
					definition := v1alpha1.ComponentDefinition{SpecDefinition: &v1alpha1.SpecDefinition{
						FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
							Image:     &v1alpha1.ValueAccessor{PathWrite: ptr.To(test.image)},
							Container: &v1alpha1.ValueAccessor{PathWrite: ptr.To(test.container)},
						},
					}}
					err = accessor.UpdateFragmentedPodSpec(context.Background(), definition, []FragmentedPodSpec{{
						Image: "new", Container: &corev1.Container{Image: "old"},
					}})
					if err == nil || !strings.Contains(err.Error(), "overlapping fragmented writes") ||
						!strings.Contains(err.Error(), "image") || !strings.Contains(err.Error(), "container") ||
						!strings.Contains(err.Error(), "select only one") {
						t.Fatalf("got error %v, want named overlap error with a recovery instruction", err)
					}
					if runner.assignments != 0 || !reflect.DeepEqual(before, sdkTestObject(t, accessor)) {
						t.Fatalf("rejected mutation published %d writes or changed the object", runner.assignments)
					}
				})
			}
		}
	}
}

func TestFragmentedWritesDisjointPointerSegments(t *testing.T) {
	for _, test := range []struct {
		name      string
		scheduler string
		image     string
	}{
		{name: "sibling fields", scheduler: "/spec/main/schedulerName", image: "/spec/main/image"},
		{name: "shared text prefix", scheduler: "/spec/a", image: "/spec/ab"},
		{name: "slash in a key is not a separator", scheduler: "/spec/a~1b", image: "/spec/a/b"},
		{name: "escaped tilde is not an escaped slash", scheduler: "/spec/a~01b", image: "/spec/a~1b"},
	} {
		t.Run(test.name, func(t *testing.T) {
			accessor := sdkTestAccessor(t, map[string]any{"spec": map[string]any{"keep": true}})
			runner := &fragmentedTrackingRunner{Runner: accessor.runner}
			accessor.runner = runner
			definition := v1alpha1.ComponentDefinition{SpecDefinition: &v1alpha1.SpecDefinition{
				FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
					SchedulerName: &v1alpha1.ValueAccessor{PathWrite: ptr.To(test.scheduler)},
					Image:         &v1alpha1.ValueAccessor{PathWrite: ptr.To(test.image)},
				},
			}}
			if err := accessor.UpdateFragmentedPodSpec(context.Background(), definition, []FragmentedPodSpec{{SchedulerName: "batch", Image: "new"}}); err != nil {
				t.Fatal(err)
			}
			for path, want := range map[string]any{test.scheduler: "batch", test.image: "new", "/spec/keep": true} {
				target, err := accessor.ResolveWriteTarget(context.Background(), &v1alpha1.ValueAccessor{PathWrite: ptr.To(path)}, "", 0)
				if err != nil || target.Value != want {
					t.Fatalf("%s = %#v, %v; want %#v", path, target.Value, err, want)
				}
			}
			if runner.assignments != 1 {
				t.Fatalf("published %d writes, want one", runner.assignments)
			}
		})
	}
}

func TestFragmentedWritesFreezeComputedPaths(t *testing.T) {
	accessor := sdkTestAccessor(t, map[string]any{"spec": map[string]any{
		"selected": "a", "a": map[string]any{"image": "old-a"}, "b": map[string]any{"image": "old-b"},
	}})
	definition := v1alpha1.ComponentDefinition{SpecDefinition: &v1alpha1.SpecDefinition{
		FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
			SchedulerName: &v1alpha1.ValueAccessor{PathWrite: ptr.To("/spec/selected")},
			Image:         &v1alpha1.ValueAccessor{PathWriteExpression: `"/spec/" + object.spec.selected + "/image"`},
		},
	}}
	if err := accessor.UpdateFragmentedPodSpec(context.Background(), definition, []FragmentedPodSpec{{SchedulerName: "b", Image: "new"}}); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"spec": map[string]any{
		"selected": "b", "a": map[string]any{"image": "new"}, "b": map[string]any{"image": "old-b"},
	}}
	if got := sdkTestObject(t, accessor); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want targets frozen before the first field changed: %#v", got, want)
	}
}

func TestFragmentedWritesMultipleInstances(t *testing.T) {
	for _, overlap := range []bool{false, true} {
		t.Run(map[bool]string{false: "disjoint", true: "cross-field overlap"}[overlap], func(t *testing.T) {
			accessor := sdkTestAccessor(t, map[string]any{"spec": map[string]any{"items": []any{
				map[string]any{"schedulerName": "a", "image": "old-a"},
				map[string]any{"schedulerName": "b", "image": "old-b"},
			}}})
			idsExpression := `object.spec.items.map(item, item.schedulerName)`
			runner := &fragmentedTrackingRunner{Runner: accessor.runner, instanceExpression: idsExpression}
			accessor.runner = runner
			imageExpression := `"/spec/items/" + string(index) + "/image"`
			if overlap {
				imageExpression = `"/spec/items/" + string(1 - int(index)) + "/schedulerName"`
			}
			definition := v1alpha1.ComponentDefinition{
				InstanceIds: &v1alpha1.ValueAccessor{Expression: idsExpression},
				SpecDefinition: &v1alpha1.SpecDefinition{FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
					SchedulerName: &v1alpha1.ValueAccessor{PathWriteExpression: `"/spec/items/" + string(index) + "/schedulerName"`},
					Image:         &v1alpha1.ValueAccessor{PathWriteExpression: imageExpression},
				}},
			}
			before, err := jsonCopy(sdkTestObject(t, accessor))
			if err != nil {
				t.Fatal(err)
			}
			err = accessor.UpdateFragmentedPodSpec(context.Background(), definition, []FragmentedPodSpec{
				{SchedulerName: "changed-a", Image: "new-a"}, {SchedulerName: "changed-b", Image: ""},
			})
			if runner.instanceEvaluations != 1 {
				t.Fatalf("instance IDs evaluated %d times, want once", runner.instanceEvaluations)
			}
			if overlap {
				if err == nil || !strings.Contains(err.Error(), "overlapping fragmented writes") || runner.assignments != 0 || !reflect.DeepEqual(before, sdkTestObject(t, accessor)) {
					t.Fatalf("overlap was not rejected without publication: error=%v assignments=%d", err, runner.assignments)
				}
				return
			}
			if err != nil || runner.assignments != 1 {
				t.Fatalf("write failed or did not publish once: error=%v assignments=%d", err, runner.assignments)
			}
			want := map[string]any{"spec": map[string]any{"items": []any{
				map[string]any{"schedulerName": "changed-a", "image": "new-a"},
				map[string]any{"schedulerName": "changed-b", "image": ""},
			}}}
			if got := sdkTestObject(t, accessor); !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v, want %#v (an active field retains all per-instance values)", got, want)
			}
		})
	}
}

func TestFragmentedWritesFailuresDoNotPublish(t *testing.T) {
	for _, test := range []struct {
		name    string
		image   *v1alpha1.ValueAccessor
		options MutationOptions
		cancel  bool
	}{
		{name: "read-only field", image: &v1alpha1.ValueAccessor{Expression: `object.image`}},
		{name: "undeclared field"},
		{name: "null computed path", image: &v1alpha1.ValueAccessor{PathWriteExpression: `null`}},
		{name: "invalid pointer", image: &v1alpha1.ValueAccessor{PathWrite: ptr.To("/bad~key")}},
		{name: "later missing parent", image: &v1alpha1.ValueAccessor{PathWrite: ptr.To("/missing/image")}, options: MutationOptions{Parents: RequireParents}},
		{name: "invalid policy", image: &v1alpha1.ValueAccessor{PathWrite: ptr.To("/image")}, options: MutationOptions{Strategy: "wrong"}},
		{name: "cancelled context", image: &v1alpha1.ValueAccessor{PathWrite: ptr.To("/image")}, cancel: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := map[string]any{"schedulerName": "old", "image": "old"}
			accessor := sdkTestAccessor(t, before, test.options)
			runner := &fragmentedTrackingRunner{Runner: accessor.runner}
			accessor.runner = runner
			definition := v1alpha1.ComponentDefinition{SpecDefinition: &v1alpha1.SpecDefinition{
				FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
					SchedulerName: &v1alpha1.ValueAccessor{PathWrite: ptr.To("/schedulerName")}, Image: test.image,
				},
			}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if test.cancel {
				cancel()
			}
			err := accessor.UpdateFragmentedPodSpec(ctx, definition, []FragmentedPodSpec{{SchedulerName: "new", Image: "new"}})
			if err == nil || runner.assignments != 0 || !reflect.DeepEqual(before, sdkTestObject(t, accessor)) {
				t.Fatalf("failed mutation was published: error=%v assignments=%d", err, runner.assignments)
			}
		})
	}
}

func TestFragmentedWritesEmptyValuesRemainNoOp(t *testing.T) {
	accessor := sdkTestAccessor(t, map[string]any{"keep": true}, MutationOptions{Strategy: "invalid but unused"})
	runner := &fragmentedTrackingRunner{Runner: accessor.runner}
	accessor.runner = runner
	definition := v1alpha1.ComponentDefinition{SpecDefinition: &v1alpha1.SpecDefinition{
		FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{},
	}}
	for _, fragments := range [][]FragmentedPodSpec{nil, {{}}, {{Labels: map[string]string{}, Containers: []corev1.Container{}}}} {
		if err := accessor.UpdateFragmentedPodSpec(context.Background(), definition, fragments); err != nil {
			t.Fatal(err)
		}
	}
	if runner.assignments != 0 || !reflect.DeepEqual(sdkTestObject(t, accessor), map[string]any{"keep": true}) {
		t.Fatal("empty fields should not write or require write accessors")
	}
}
