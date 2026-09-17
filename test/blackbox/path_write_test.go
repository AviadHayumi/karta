// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package blackbox

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog/kartas"
	celpkg "github.com/run-ai/karta/pkg/cel"
	"github.com/run-ai/karta/pkg/resource"
)

func newCatalogPathFactory(t *testing.T, karta *v1alpha1.Karta, object map[string]any, options resource.MutationOptions) (*resource.ComponentFactory, *resource.Accessor) {
	t.Helper()
	g := NewWithT(t)
	variables := make([]celpkg.NamedExpression, len(karta.Spec.Variables))
	for i, variable := range karta.Spec.Variables {
		variables[i] = celpkg.NamedExpression{Name: variable.Name, Expression: variable.Expression}
	}
	runner, err := celpkg.NewRunnerWithVariables(object, variables)
	g.Expect(err).NotTo(HaveOccurred())
	accessor := resource.NewAccessor(runner, options)
	return resource.NewComponentFactory(karta, accessor), accessor
}

func TestCatalogKServePathWrites(t *testing.T) {
	for _, test := range []struct {
		name       string
		options    resource.MutationOptions
		value      resource.FragmentedPodSpec
		keepFields bool
	}{
		{
			name:       "default merge preserves framework fields on a broad container write",
			value:      resource.FragmentedPodSpec{Container: &corev1.Container{Image: "ghcr.io/example/inference:v2"}},
			keepFields: true,
		},
		{
			// This characterizes an explicit Replace hazard, not a safe update guarantee.
			name:    "known hazard broad container replace drops framework fields",
			options: resource.MutationOptions{Strategy: resource.Replace},
			value:   resource.FragmentedPodSpec{Container: &corev1.Container{Image: "ghcr.io/example/inference:v2"}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			object := map[string]any{
				"apiVersion": "serving.kserve.io/v1beta1", "kind": "InferenceService",
				"metadata": map[string]any{"name": "example"},
				"spec": map[string]any{"predictor": map[string]any{
					"model": map[string]any{
						"storageUri": "s3://example-models/model", "runtime": "example-runtime",
						"image": "ghcr.io/example/inference:v1",
					},
					"minReplicas": float64(2),
				}},
			}
			factory, accessor := newCatalogPathFactory(t, kartas.KServe(), object, test.options)
			predictor, err := factory.GetComponent("predictor")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(predictor.UpdateFragmentedPodSpec(context.Background(), map[string]resource.FragmentedPodSpec{"": test.value})).To(Succeed())
			updated, err := accessor.GetObject()
			g.Expect(err).NotTo(HaveOccurred())
			predictorObject := updated["spec"].(map[string]any)["predictor"].(map[string]any)
			model := predictorObject["model"].(map[string]any)
			g.Expect(model).To(HaveKeyWithValue("image", "ghcr.io/example/inference:v2"))
			g.Expect(predictorObject).To(HaveKeyWithValue("minReplicas", float64(2)))
			if test.keepFields {
				g.Expect(model).To(HaveKeyWithValue("storageUri", "s3://example-models/model"))
				g.Expect(model).To(HaveKeyWithValue("runtime", "example-runtime"))
			} else {
				g.Expect(model).NotTo(HaveKey("storageUri"))
				g.Expect(model).NotTo(HaveKey("runtime"))
			}
		})
	}
}

func TestCatalogPodRootPathWrites(t *testing.T) {
	for _, test := range []struct {
		name    string
		options resource.MutationOptions
	}{
		{name: "default merge preserves object envelope and partial label keys"},
		{
			// Rejecting typed root replacement intentionally replaces the former identity-loss hazard.
			name:    "merge patch typed root replacement rejects missing identity",
			options: resource.MutationOptions{Strategy: resource.Replace},
		},
		{
			name:    "json patch typed root replacement rejects missing identity",
			options: resource.MutationOptions{PatchType: resource.PatchTypeJSONPatch, Strategy: resource.Replace},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()
			object := map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{"name": "example", "labels": map[string]any{"existing": "retained"}},
				"spec":     map[string]any{"containers": []any{map[string]any{"name": "main", "image": "ghcr.io/example/worker:v1"}}},
				"status":   map[string]any{"phase": "Running"},
			}
			before := runtime.DeepCopyJSON(object)
			factory, accessor := newCatalogPathFactory(t, kartas.Pod(), object, test.options)
			pod, err := factory.GetRootComponent()
			g.Expect(err).NotTo(HaveOccurred())
			templates, err := pod.GetPodTemplateSpec(ctx)
			g.Expect(err).NotTo(HaveOccurred())
			template := templates[""]
			template.Labels = map[string]string{"added": "new"}
			templates[""] = template
			writeErr := pod.UpdatePodTemplateSpec(ctx, templates)
			updated, err := accessor.GetObject()
			g.Expect(err).NotTo(HaveOccurred())
			labels := updated["metadata"].(map[string]any)["labels"].(map[string]any)
			if test.options.Strategy == resource.Replace {
				g.Expect(writeErr).To(MatchError(ContainSubstring("missing apiVersion")))
				g.Expect(updated).To(Equal(before))
			} else {
				g.Expect(writeErr).NotTo(HaveOccurred())
				g.Expect(labels).To(HaveKeyWithValue("added", "new"))
				g.Expect(updated).To(HaveKeyWithValue("apiVersion", "v1"))
				g.Expect(updated).To(HaveKeyWithValue("kind", "Pod"))
				g.Expect(updated).To(HaveKeyWithValue("status", map[string]any{"phase": "Running"}))
				g.Expect(labels).To(HaveKeyWithValue("existing", "retained"))
			}
			g.Expect(object).To(Equal(before))
			_, err = factory.GetResource()
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestCatalogKServeConflictingImageRoundTripLeavesObjectUnchanged(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	object := map[string]any{
		"apiVersion": "serving.kserve.io/v1beta1", "kind": "InferenceService",
		"metadata": map[string]any{"name": "example"},
		"spec": map[string]any{"predictor": map[string]any{
			"model": map[string]any{
				"storageUri": "s3://example-models/model", "runtime": "example-runtime",
				"modelFormat": map[string]any{"name": "example-format"},
				"image":       "ghcr.io/example/inference:v1",
			},
		}},
	}
	factory, accessor := newCatalogPathFactory(t, kartas.KServe(), object, resource.MutationOptions{})
	predictor, err := factory.GetComponent("predictor")
	g.Expect(err).NotTo(HaveOccurred())
	fragments, err := predictor.GetFragmentedPodSpec(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	fragment := fragments[""]
	g.Expect(fragment.Container).NotTo(BeNil())
	g.Expect(fragment.Container.Image).To(Equal("ghcr.io/example/inference:v1"))
	fragment.Image = "ghcr.io/example/inference:v2"
	fragments[""] = fragment
	g.Expect(predictor.UpdateFragmentedPodSpec(ctx, fragments)).NotTo(Succeed())
	updated, err := accessor.GetObject()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(updated).To(Equal(object))
}

func TestCatalogKServeContainerImageRoundTripPreservesFrameworkFields(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	object := map[string]any{
		"apiVersion": "serving.kserve.io/v1beta1", "kind": "InferenceService",
		"metadata": map[string]any{"name": "example"},
		"spec": map[string]any{"predictor": map[string]any{
			"model": map[string]any{
				"storageUri": "s3://example-models/model", "runtime": "example-runtime",
				"modelFormat": map[string]any{"name": "example-format"},
				"image":       "ghcr.io/example/inference:v1",
			},
		}},
	}
	factory, accessor := newCatalogPathFactory(t, kartas.KServe(), object, resource.MutationOptions{})
	predictor, err := factory.GetComponent("predictor")
	g.Expect(err).NotTo(HaveOccurred())
	fragments, err := predictor.GetFragmentedPodSpec(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	fragment := fragments[""]
	g.Expect(fragment.Image).To(BeEmpty())
	g.Expect(fragment.Container).NotTo(BeNil())
	g.Expect(fragment.Container.Image).To(Equal("ghcr.io/example/inference:v1"))
	fragment.Container.Image = "ghcr.io/example/inference:v2"
	fragments[""] = fragment
	g.Expect(predictor.UpdateFragmentedPodSpec(ctx, fragments)).To(Succeed())
	updated, err := accessor.GetObject()
	g.Expect(err).NotTo(HaveOccurred())
	model := updated["spec"].(map[string]any)["predictor"].(map[string]any)["model"].(map[string]any)
	g.Expect(model).To(HaveKeyWithValue("image", "ghcr.io/example/inference:v2"))
	g.Expect(model).To(HaveKeyWithValue("storageUri", "s3://example-models/model"))
	g.Expect(model).To(HaveKeyWithValue("runtime", "example-runtime"))
	g.Expect(model).To(HaveKeyWithValue("modelFormat", map[string]any{"name": "example-format"}))
}

func TestSDKKServeNarrowImageWritePreservesFrameworkFields(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	object := map[string]any{
		"apiVersion": "serving.kserve.io/v1beta1", "kind": "InferenceService",
		"metadata": map[string]any{"name": "example"},
		"spec": map[string]any{"predictor": map[string]any{
			"model": map[string]any{
				"storageUri": "s3://example-models/model", "runtime": "example-runtime",
				"modelFormat": map[string]any{"name": "example-format"},
				"image":       "ghcr.io/example/inference:v1",
			},
		}},
	}
	factory, accessor := newCatalogPathFactory(t, kartas.KServe(), object, resource.MutationOptions{})
	predictor, err := factory.GetComponent("predictor")
	g.Expect(err).NotTo(HaveOccurred())
	definition := predictor.Definition()
	container := definition.SpecDefinition.FragmentedPodSpecDefinition.Container
	target, err := accessor.ResolveWriteTarget(ctx, container, "", 0)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(accessor.WriteValues(ctx, definition,
		&v1alpha1.ValueAccessor{PathWrite: ptr.To(target.Path + "/image")},
		[]any{"ghcr.io/example/inference:v2"}, resource.MutationOptions{Strategy: resource.Replace})).To(Succeed())
	updated, err := accessor.GetObject()
	g.Expect(err).NotTo(HaveOccurred())
	model := updated["spec"].(map[string]any)["predictor"].(map[string]any)["model"].(map[string]any)
	g.Expect(model).To(Equal(map[string]any{
		"storageUri": "s3://example-models/model", "runtime": "example-runtime",
		"modelFormat": map[string]any{"name": "example-format"},
		"image":       "ghcr.io/example/inference:v2",
	}))
}

func TestCatalogKServeMissingContainerTargetLeavesObjectUnchanged(t *testing.T) {
	g := NewWithT(t)
	object := map[string]any{
		"apiVersion": "serving.kserve.io/v1beta1", "kind": "InferenceService",
		"metadata": map[string]any{"name": "example"},
		"spec":     map[string]any{"predictor": map[string]any{"minReplicas": float64(2)}},
	}
	factory, accessor := newCatalogPathFactory(t, kartas.KServe(), object, resource.MutationOptions{})
	predictor, err := factory.GetComponent("predictor")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(predictor.UpdateFragmentedPodSpec(context.Background(), map[string]resource.FragmentedPodSpec{
		"": {Container: &corev1.Container{Image: "ghcr.io/example/inference:v2"}},
	})).NotTo(Succeed())
	updated, err := accessor.GetObject()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(updated).To(Equal(object))
}

func TestCatalogRayPathWritesFollowInstanceIDsAfterReorder(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	groups := []any{}
	for _, name := range []string{"alpha", "beta"} {
		groups = append(groups, map[string]any{
			"groupName": name, "replicas": float64(2),
			"template": map[string]any{"spec": map[string]any{
				"schedulerName": "original-" + name,
				"containers":    []any{map[string]any{"name": "worker", "image": "ghcr.io/example/worker:v1"}},
			}},
		})
	}
	object := map[string]any{
		"apiVersion": "ray.io/v1", "kind": "RayCluster",
		"metadata": map[string]any{"name": "example"},
		"spec":     map[string]any{"workerGroupSpecs": groups},
	}
	factory, accessor := newCatalogPathFactory(t, kartas.Raycluster(), object, resource.MutationOptions{})
	worker, err := factory.GetComponent("worker")
	g.Expect(err).NotTo(HaveOccurred())
	templates, err := worker.GetPodTemplateSpec(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	for id, template := range templates {
		template.Spec.SchedulerName = "updated-" + id
		templates[id] = template
	}
	g.Expect(accessor.ApplyPatch(ctx, resource.PatchTypeJSONPatch, []any{map[string]any{
		"op": "move", "from": "/spec/workerGroupSpecs/0", "path": "/spec/workerGroupSpecs/1",
	}})).To(Succeed())
	g.Expect(worker.UpdatePodTemplateSpec(ctx, templates)).To(Succeed())
	updated, err := accessor.GetObject()
	g.Expect(err).NotTo(HaveOccurred())
	updatedGroups := updated["spec"].(map[string]any)["workerGroupSpecs"].([]any)
	g.Expect(updatedGroups).To(HaveLen(2))
	for i, name := range []string{"beta", "alpha"} {
		group := updatedGroups[i].(map[string]any)
		g.Expect(group).To(HaveKeyWithValue("groupName", name))
		g.Expect(group).To(HaveKeyWithValue("replicas", float64(2)))
		spec := group["template"].(map[string]any)["spec"].(map[string]any)
		g.Expect(spec).To(HaveKeyWithValue("schedulerName", "updated-"+name))
	}
}
