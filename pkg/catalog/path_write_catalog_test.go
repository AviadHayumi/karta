// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package catalog

import (
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog/kartas"
	celpkg "github.com/run-ai/karta/pkg/cel"
	"github.com/run-ai/karta/pkg/expression"
)

func TestCatalogPathWriteSerialization(t *testing.T) {
	for _, k := range List() {
		t.Run(k.Name, func(t *testing.T) {
			g := NewWithT(t)
			data, err := MarshalYAML(k)
			g.Expect(err).NotTo(HaveOccurred())
			for _, field := range []string{"patches:", "patchType:", "patchStrategy:", "suspendActions:", "resumeActions:"} {
				g.Expect(string(data)).NotTo(ContainSubstring(field))
			}
		})
	}

	t.Run("root pointer remains distinct from a read-only accessor", func(t *testing.T) {
		g := NewWithT(t)
		root := kartas.Pod().Spec.StructureDefinition.RootComponent
		g.Expect(root.SpecDefinition.PodTemplateSpec.PathWrite).NotTo(BeNil())
		g.Expect(*root.SpecDefinition.PodTemplateSpec.PathWrite).To(BeEmpty())
		data, err := json.Marshal(root.SpecDefinition.PodTemplateSpec)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(string(data)).To(ContainSubstring(`"pathWrite":""`))
		g.Expect(root.ScaleDefinition.Replicas.PathWrite).To(BeNil())
		g.Expect(root.ScaleDefinition.Replicas.PathWriteExpression).To(BeEmpty())
		data, err = json.Marshal(root.ScaleDefinition.Replicas)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(string(data)).NotTo(ContainSubstring("pathWrite"))
	})
}

func TestCatalogDynamoMapPathWrites(t *testing.T) {
	g := NewWithT(t)
	k := kartas.Dynamo()
	component := k.Spec.StructureDefinition.ChildComponents[0]
	fields := component.SpecDefinition.FragmentedPodSpecDefinition
	runner := catalogPathRunner(t, k, map[string]any{
		"spec": map[string]any{"services": map[string]any{
			"worker/~1": map[string]any{"labels": map[string]any{"instance": "worker/~1"}},
			"alpha":     map[string]any{"labels": map[string]any{"instance": "alpha"}},
		}},
	})
	instances, err := runner.Evaluate(t.Context(), component.InstanceIds.Expression)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(instances).To(Equal([]any{"alpha", "worker/~1"}))
	labels, err := runner.Evaluate(t.Context(), fields.Labels.Expression)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(labels).To(Equal([]any{
		map[string]any{"instance": "alpha"},
		map[string]any{"instance": "worker/~1"},
	}))

	for _, tc := range []struct {
		name     string
		accessor *v1alpha1.ValueAccessor
		suffix   string
	}{
		{"scheduler", fields.SchedulerName, "/extraPodSpec/schedulerName"},
		{"labels", fields.Labels, "/labels"},
		{"annotations", fields.Annotations, "/annotations"},
		{"resources", fields.Resources, "/resources"},
		{"resource claims", fields.ResourceClaims, "/extraPodSpec/resourceClaims"},
		{"pod affinity", fields.PodAffinity, "/extraPodSpec/affinity/podAffinity"},
		{"node affinity", fields.NodeAffinity, "/extraPodSpec/affinity/nodeAffinity"},
		{"container", fields.Container, "/extraPodSpec/mainContainer"},
		{"priority class", fields.PriorityClassName, "/extraPodSpec/priorityClassName"},
		{"image", fields.Image, "/extraPodSpec/mainContainer/image"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for index, instance := range instances {
				escaped := []string{"alpha", "worker~1~01"}[index]
				expectCatalogPath(t, runner, tc.accessor, instance, index, "/spec/services/"+escaped+tc.suffix)
			}
		})
	}
}

func TestCatalogListPathWritesFollowInstanceOrder(t *testing.T) {
	for _, tc := range []struct {
		karta     *v1alpha1.Karta
		component string
		object    string
		path      string
	}{
		{
			karta: kartas.DynamoV1beta1(), component: "component",
			object: `{"spec":{"components":[{"name":"zeta/~"},{"name":"alpha","podTemplate":{"metadata":{"labels":{"instance":"alpha"}}}}]}}`,
			path:   "/spec/components/%d/podTemplate/metadata/labels",
		},
		{
			karta: kartas.GrovePodCliqueSet(), component: "clique",
			object: `{"spec":{"template":{"cliques":[{"name":"zeta/~"},{"name":"alpha","labels":{"instance":"alpha"}}]}}}`,
			path:   "/spec/template/cliques/%d/labels",
		},
		{
			karta: kartas.Raycluster(), component: "worker",
			object: `{"spec":{"workerGroupSpecs":[{"groupName":"zeta/~"},{"groupName":"alpha","template":{"instance":"alpha"}}]}}`,
			path:   "/spec/workerGroupSpecs/%d/template",
		},
		{
			karta: kartas.Rayjob(), component: "worker",
			object: `{"spec":{"rayClusterSpec":{"workerGroupSpecs":[{"groupName":"zeta/~"},{"groupName":"alpha","template":{"instance":"alpha"}}]}}}`,
			path:   "/spec/rayClusterSpec/workerGroupSpecs/%d/template",
		},
		{
			karta: kartas.RayService(), component: "worker",
			object: `{"spec":{"rayClusterConfig":{"workerGroupSpecs":[{"groupName":"zeta/~"},{"groupName":"alpha","template":{"instance":"alpha"}}]}}}`,
			path:   "/spec/rayClusterConfig/workerGroupSpecs/%d/template",
		},
		{
			karta: kartas.Jobset(), component: "replicatedjob",
			object: `{"spec":{"replicatedJobs":[{"name":"zeta/~"},{"name":"alpha","template":{"spec":{"template":{"instance":"alpha"}}}}]}}`,
			path:   "/spec/replicatedJobs/%d/template/spec/template",
		},
	} {
		t.Run(tc.karta.Name, func(t *testing.T) {
			g := NewWithT(t)
			var object map[string]any
			g.Expect(json.Unmarshal([]byte(tc.object), &object)).To(Succeed())
			runner := catalogPathRunner(t, tc.karta, object)
			var component *v1alpha1.ComponentDefinition
			for i := range tc.karta.Spec.StructureDefinition.ChildComponents {
				candidate := &tc.karta.Spec.StructureDefinition.ChildComponents[i]
				if candidate.Name == tc.component {
					component = candidate
					break
				}
			}
			g.Expect(component).NotTo(BeNil())
			instances, err := runner.Evaluate(t.Context(), component.InstanceIds.Expression)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(instances).To(Equal([]any{"zeta/~", "alpha"}))
			accessor := component.SpecDefinition.PodTemplateSpec
			if fragmented := component.SpecDefinition.FragmentedPodSpecDefinition; fragmented != nil {
				accessor = fragmented.Labels
			}
			values, err := runner.Evaluate(t.Context(), accessor.Expression)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(values).To(Equal([]any{nil, map[string]any{"instance": "alpha"}}))
			for index, instance := range instances {
				expectCatalogPath(t, runner, accessor, instance, index, fmt.Sprintf(tc.path, index))
			}
		})
	}
}

func TestCatalogKServeContainerPaths(t *testing.T) {
	k := kartas.KServe()
	fields := k.Spec.StructureDefinition.ChildComponents[0].SpecDefinition.FragmentedPodSpecDefinition
	NewWithT(t).Expect(fields.Image).To(BeNil())
	for _, tc := range []struct {
		key     string
		escaped string
	}{
		{"model", "model"},
		{"sklearn", "sklearn"},
		{"custom/~1", "custom~1~01"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			g := NewWithT(t)
			container := map[string]any{"storageUri": "s3://example/model", "image": "ghcr.io/example/inference:v1.2.3"}
			runner := catalogPathRunner(t, k, map[string]any{
				"spec": map[string]any{"predictor": map[string]any{
					tc.key:        container,
					"annotations": map[string]any{"example.com/note": "preserved"},
				}},
			})
			value, err := runner.Evaluate(t.Context(), fields.Container.Expression)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(value).To(Equal([]any{container}))
			expectCatalogPath(t, runner, fields.Container, nil, 0, "/spec/predictor/"+tc.escaped)
		})
	}

	t.Run("missing image retains a writable container location", func(t *testing.T) {
		g := NewWithT(t)
		runner := catalogPathRunner(t, k, map[string]any{
			"spec": map[string]any{"predictor": map[string]any{
				"model": map[string]any{"storageUri": "s3://example/model"},
			}},
		})
		value, err := runner.Evaluate(t.Context(), fields.Container.Expression)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(value).To(Equal([]any{map[string]any{"storageUri": "s3://example/model"}}))
		expectCatalogPath(t, runner, fields.Container, nil, 0, "/spec/predictor/model")
	})

	t.Run("ambiguous candidates have no writable container location", func(t *testing.T) {
		g := NewWithT(t)
		first := map[string]any{"storageUri": "s3://example/first", "image": "ghcr.io/example/first:v1"}
		second := map[string]any{"storageUri": "s3://example/second", "image": "ghcr.io/example/second:v1"}
		runner := catalogPathRunner(t, k, map[string]any{
			"spec": map[string]any{"predictor": map[string]any{"model": first, "sklearn": second}},
		})
		value, err := runner.Evaluate(t.Context(), fields.Container.Expression)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(value).To(Equal([]any{first, second}))
		expectCatalogPath(t, runner, fields.Container, nil, 0, nil)
	})

	for _, tc := range []struct {
		name   string
		object string
	}{
		{"missing spec", `{}`},
		{"empty predictor", `{"spec":{"predictor":{}}}`},
		{"custom containers", `{"spec":{"predictor":{"containers":[{"name":"custom","image":"ghcr.io/example/inference:v1.2.3"}]}}}`},
		{"null storage URI", `{"spec":{"predictor":{"model":{"storageUri":null}}}}`},
	} {
		t.Run("missing container key/"+tc.name, func(t *testing.T) {
			g := NewWithT(t)
			var source map[string]any
			g.Expect(json.Unmarshal([]byte(tc.object), &source)).To(Succeed())
			runner := catalogPathRunner(t, k, source)
			value, err := runner.Evaluate(t.Context(), fields.Container.Expression)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(value).To(Equal([]any{nil}))
			expectCatalogPath(t, runner, fields.Container, nil, 0, nil)
		})
	}
}

func catalogPathRunner(t *testing.T, k *v1alpha1.Karta, object map[string]any) expression.Runner {
	t.Helper()
	variables := make([]celpkg.NamedExpression, len(k.Spec.Variables))
	for i, variable := range k.Spec.Variables {
		variables[i] = celpkg.NamedExpression{Name: variable.Name, Expression: variable.Expression}
	}
	runner, err := celpkg.NewRunnerWithVariables(object, variables)
	NewWithT(t).Expect(err).NotTo(HaveOccurred())
	return runner
}

func expectCatalogPath(t *testing.T, runner expression.Runner, accessor *v1alpha1.ValueAccessor, instance any, index int, want any) {
	t.Helper()
	g := NewWithT(t)
	g.Expect(accessor).NotTo(BeNil())
	g.Expect(accessor.PathWrite).To(BeNil())
	g.Expect(accessor.PathWriteExpression).NotTo(BeEmpty())
	path, err := runner.EvaluateWithVariables(t.Context(), accessor.PathWriteExpression, map[string]any{
		"instance": instance,
		"index":    index,
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(path).To(Equal([]any{want}))
}
