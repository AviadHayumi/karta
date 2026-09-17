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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/references"
)

func forkFixture() (*v1alpha1.Karta, *unstructured.Unstructured) {
	definition := &v1alpha1.Karta{Spec: v1alpha1.KartaSpec{
		StructureDefinition: v1alpha1.StructureDefinition{
			RootComponent: v1alpha1.ComponentDefinition{
				Name: "example",
				SpecDefinition: &v1alpha1.SpecDefinition{
					FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
						Labels: &v1alpha1.ValueAccessor{Expression: "object.spec.labels", PathWrite: ptr.To("/spec/labels")},
					},
				},
				SuspendDefinition: &v1alpha1.SuspendDefinition{PathWrite: ptr.To("/spec/suspend")},
			},
		},
	}}
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1", "kind": "Example",
		"metadata": map[string]any{"name": "demo"},
		"spec": map[string]any{
			"labels":  map[string]any{"team": "ml", "owner": "alice"},
			"suspend": false,
			"left":    map[string]any{"image": "api:v1"},
			"right":   map[string]any{"image": "api:v2"},
		},
	}}
	return definition, object
}

func TestFactoryForkDetachesCurrentObjectAndKarta(t *testing.T) {
	ctx := context.Background()
	definition, object := forkFixture()
	factory := NewComponentFactoryFromObject(definition, object)
	original, err := factory.GetRootComponent()
	if err != nil {
		t.Fatal(err)
	}
	if err := original.Suspend(ctx); err != nil {
		t.Fatal(err)
	}
	fork, err := factory.Fork()
	if err != nil {
		t.Fatal(err)
	}
	staged, err := fork.GetRootComponent()
	if err != nil {
		t.Fatal(err)
	}
	if err := staged.Resume(ctx); err != nil {
		t.Fatal(err)
	}
	if err := staged.UpdateFragmentedPodSpec(ctx, map[string]FragmentedPodSpec{"": {Labels: map[string]string{"team": "platform"}}}); err != nil {
		t.Fatal(err)
	}
	parentObject, err := factory.GetResource()
	if err != nil {
		t.Fatal(err)
	}
	parentSpec := parentObject.(*unstructured.Unstructured).Object["spec"].(map[string]any)
	if parentSpec["suspend"] != true || parentSpec["labels"].(map[string]any)["team"] != "ml" {
		t.Fatalf("staged writes changed the source factory: %#v", parentSpec)
	}
	stagedObject, err := fork.GetResource()
	if err != nil {
		t.Fatal(err)
	}
	stagedSpec := stagedObject.(*unstructured.Unstructured).Object["spec"].(map[string]any)
	if stagedSpec["suspend"] != false || stagedSpec["labels"].(map[string]any)["team"] != "platform" {
		t.Fatalf("staged writes were not applied: %#v", stagedSpec)
	}
	if object.Object["spec"].(map[string]any)["suspend"] != false {
		t.Fatal("factory mutated its caller's input object")
	}
	fork.GetKarta().Spec.StructureDefinition.RootComponent.SpecDefinition.FragmentedPodSpecDefinition.Labels.Expression = "'changed'"
	if factory.GetKarta().Spec.StructureDefinition.RootComponent.SpecDefinition.FragmentedPodSpecDefinition.Labels.Expression != "object.spec.labels" {
		t.Fatal("fork shares its definition with the source factory")
	}
}

func TestFactoryForkPreservesMutationOptions(t *testing.T) {
	definition, object := forkFixture()
	options := MutationOptions{PatchType: PatchTypeJSONPatch, Strategy: Replace, Parents: RequireParents}
	factory := NewComponentFactoryFromObject(definition, object, WithMutationOptions(options))
	fork, err := factory.Fork()
	if err != nil {
		t.Fatal(err)
	}
	if fork.MutationOptions() != options {
		t.Fatalf("fork lost mutation options: %+v", fork.MutationOptions())
	}
	component, err := fork.GetRootComponent()
	if err != nil {
		t.Fatal(err)
	}
	if err := component.UpdateFragmentedPodSpec(context.Background(), map[string]FragmentedPodSpec{"": {Labels: map[string]string{"team": "platform"}}}); err != nil {
		t.Fatal(err)
	}
	updated, err := fork.GetResource()
	if err != nil {
		t.Fatal(err)
	}
	labels := updated.(*unstructured.Unstructured).Object["spec"].(map[string]any)["labels"]
	if !reflect.DeepEqual(labels, map[string]any{"team": "platform"}) {
		t.Fatalf("fork did not preserve Replace strategy: %#v", labels)
	}
}

func TestFactoryForkRejectsCustomAccessor(t *testing.T) {
	definition, _ := forkFixture()
	factory := NewComponentFactory(definition, NewMockComponentAccessor(gomock.NewController(t)))
	if _, err := factory.Fork(); err == nil || !strings.Contains(err.Error(), "custom-accessor") {
		t.Fatalf("expected unsupported custom-accessor error, got %v", err)
	}
	if _, err := factory.PathWriter(); err == nil {
		t.Fatal("custom accessor without path methods was accepted")
	}
	if factory.MutationOptions() != (MutationOptions{}) {
		t.Fatal("custom factory should have no configured mutation defaults")
	}
}

func TestFactoryForkRejectsInvalidCurrentWorkload(t *testing.T) {
	definition, object := forkFixture()
	// An invalid starting object remains a fork error; SDK writes reject corruption before publication.
	delete(object.Object, "apiVersion")
	before := object.DeepCopy()
	factory := NewComponentFactoryFromObject(definition, object)
	if _, err := factory.Fork(); err == nil || !strings.Contains(err.Error(), "missing apiVersion") {
		t.Fatalf("expected invalid-workload error, got %v", err)
	}
	current, err := factory.accessor.GetObject()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(current, before.Object) || !reflect.DeepEqual(object, before) {
		t.Fatal("rejected fork changed the invalid starting object")
	}
}

func referenceForkFixture() (*v1alpha1.Karta, *unstructured.Unstructured, *v1alpha1.ValueAccessor) {
	definition, object := forkFixture()
	definition.Spec.StructureDefinition.References = []v1alpha1.ResourceReference{{
		Name: "config", GVK: v1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
		Lookup: &v1alpha1.LookupReference{NameExpression: "'settings'"},
	}}
	via := &v1alpha1.ValueAccessor{PathWriteExpression: `"/spec/" + references.config.data.target`}
	return definition, object, via
}

func TestFactoryForkPreservesSuppliedReferences(t *testing.T) {
	definition, object, via := referenceForkFixture()
	config := &unstructured.Unstructured{Object: map[string]any{"data": map[string]any{"target": "left"}}}
	resolved := references.ResolvedReferences{"config": references.NewLookupValue(config)}
	factory := NewComponentFactoryFromObject(definition, object, WithReferences(resolved))
	config.Object["data"].(map[string]any)["target"] = "right"
	delete(resolved, "config")
	fork, err := factory.Fork()
	if err != nil {
		t.Fatal(err)
	}
	for _, current := range []*ComponentFactory{factory, fork} {
		writer, err := current.PathWriter()
		if err != nil {
			t.Fatal(err)
		}
		target, err := writer.ResolveWriteTarget(context.Background(), via, "", 0)
		if err != nil {
			t.Fatal(err)
		}
		if target.Path != "/spec/left" || !target.Exists {
			t.Fatalf("supplied references were not snapshotted: %+v", target)
		}
	}
}

func TestFactoryForkSharesLazyReaderSnapshot(t *testing.T) {
	definition, object, via := referenceForkFixture()
	object.Object["spec"].(map[string]any)["replicas"] = int32(3)
	reader := &countingReader{runtime: &unstructured.Unstructured{Object: map[string]any{
		"data": map[string]any{"target": "left"},
	}}}
	factory := NewComponentFactoryFromObject(definition, object, WithReferenceReader(reader))
	fork, err := factory.Fork()
	if err != nil {
		t.Fatal(err)
	}
	if reader.gets != 0 {
		t.Fatal("fork eagerly read references")
	}
	for _, current := range []*ComponentFactory{fork, factory} {
		writer, err := current.PathWriter()
		if err != nil {
			t.Fatal(err)
		}
		target, err := writer.ResolveWriteTarget(context.Background(), via, "", 0)
		if err != nil {
			t.Fatal(err)
		}
		if target.Path != "/spec/left" {
			t.Fatalf("reference target changed between forks: %+v", target)
		}
		reader.runtime.Object["data"].(map[string]any)["target"] = "right"
	}
	if reader.gets != 1 {
		t.Fatalf("reference snapshot fetched %d times, want 1", reader.gets)
	}
}

func TestFactoryForkRetriesFailedReferenceRead(t *testing.T) {
	definition, object, via := referenceForkFixture()
	reader := &countingReader{
		runtime: &unstructured.Unstructured{Object: map[string]any{"data": map[string]any{"target": "left"}}},
		denied:  errors.New("temporary denial"),
	}
	factory := NewComponentFactoryFromObject(definition, object, WithReferenceReader(reader))
	writer, err := factory.PathWriter()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.ResolveWriteTarget(context.Background(), via, "", 0); err == nil {
		t.Fatal("expected the first reference read to fail")
	}
	reader.denied = nil
	fork, err := factory.Fork()
	if err != nil {
		t.Fatal(err)
	}
	writer, err = fork.PathWriter()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.ResolveWriteTarget(context.Background(), via, "", 0); err != nil {
		t.Fatalf("failed reference resolution was cached: %v", err)
	}
}

func TestComponentSuspendableCapability(t *testing.T) {
	for _, supported := range []bool{true, false} {
		name := "unsupported"
		if supported {
			name = "supported"
		}
		t.Run(name, func(t *testing.T) {
			definition, object := forkFixture()
			if !supported {
				definition.Spec.StructureDefinition.RootComponent.SuspendDefinition = nil
			}
			factory := NewComponentFactoryFromObject(definition, object)
			component, err := factory.GetRootComponent()
			if err != nil {
				t.Fatal(err)
			}
			var capability Suspendable = component
			if capability.IsSuspendable() != supported || component.HasSuspendDefinition() != supported {
				t.Fatal("incorrect suspension capability")
			}
			if err := capability.Suspend(context.Background()); err != nil {
				t.Fatal(err)
			}
			updated, err := factory.GetResource()
			if err != nil {
				t.Fatal(err)
			}
			if got := updated.(*unstructured.Unstructured).Object["spec"].(map[string]any)["suspend"]; got != supported {
				t.Fatalf("suspend state = %v, want %v", got, supported)
			}
			if err := capability.Resume(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
