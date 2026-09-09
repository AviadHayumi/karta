// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// KServe returns the built-in Karta for the KServe InferenceService workload
// (serving.kserve.io/v1beta1).
func KServe() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "serving-kserve-io-inferenceservice-v1beta1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{{
				// The predictor holds the container as one of its own values, under a key that
				// varies by flavor (model, sklearn, pytorch ...). This finds that key once, and
				// the container read and write both build on it.
				Name:       "containerKey",
				Expression: `(([dyn(object[?"spec"][?"predictor"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, string(k)).sort().filter(k, type(object.spec.predictor[k]) == map && "storageUri" in object.spec.predictor[k] && object.spec.predictor[k]["storageUri"] != null && object.spec.predictor[k]["storageUri"] != false) + [""])[0]`,
			}},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "inferenceservice",
					Kind: &v1alpha1.GroupVersionKind{Group: "serving.kserve.io", Version: "v1beta1", Kind: "InferenceService"},
					StatusDefinition: &v1alpha1.StatusDefinition{
						ConditionsDefinition: &v1alpha1.ConditionsDefinition{
							Expression:       `object[?"status"][?"conditions"].orValue(null)`,
							TypeFieldName:    "type",
							StatusFieldName:  "status",
							MessageFieldName: ptr.To("message"),
						},
						StatusMappings: v1alpha1.StatusMappings{
							Running: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{
								{Type: "PredictorReady", Status: ptr.To("True")},
								{Type: "RoutesReady", Status: ptr.To("True")},
								{Type: "LatestDeploymentReady", Status: ptr.To("True")},
							}}},
							// Deploying: Ready is not yet decided (absent early, then Unknown while
							// the predictor, routes, and ingress come up). Failed is the specific
							// all-False pattern below, where Ready is False, so this stays disjoint.
							Initializing: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `!([dyn(object.?status.?conditions.orValue(null))].filter(v, type(v) == list) + [[]])[0].exists(c, c.?type.orValue(null) == "Ready" && (c.?status.orValue(null) == "True" || c.?status.orValue(null) == "False"))`,
								ExpectedResult: "true",
							}}},
							Failed: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{
								{Type: "PredictorReady", Status: ptr.To("False")},
								{Type: "PredictorConfigurationReady", Status: ptr.To("False")},
								{Type: "RoutesReady", Status: ptr.To("False")},
							}}},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "predictor",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("inferenceservice"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"predictor"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"predictor": {"schedulerName": value}}}`, Replace: true},
								Labels:        &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"predictor"][?"labels"].orValue(null)`, Patch: `{"spec": {"predictor": {"labels": value}}}`, Replace: true},
								Annotations:   &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"predictor"][?"annotations"].orValue(null)`, Patch: `{"spec": {"predictor": {"annotations": value}}}`, Replace: true},
								PodAffinity:   &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"predictor"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"predictor": {"affinity": {"podAffinity": value}}}}`, Replace: true},
								NodeAffinity:  &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"predictor"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"predictor": {"affinity": {"nodeAffinity": value}}}}`, Replace: true},
								Containers:    &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"predictor"][?"containers"].orValue(null)`, Patch: `{"spec": {"predictor": {"containers": value}}}`, Replace: true},
								Container: &v1alpha1.ValueAccessor{
									Expression: `variables.containerKey != "" ? object.spec.predictor[variables.containerKey] : null`,
									Patch:      `variables.containerKey != "" ? {"spec": {"predictor": {variables.containerKey: value}}} : {}`,
									Replace:    true,
								},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"predictor"][?"priorityClassName"].orValue(null)`, Patch: `{"spec": {"predictor": {"priorityClassName": value}}}`, Replace: true},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							MinReplicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"predictor"][?"minReplicas"].orValue(null)`},
							MaxReplicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"predictor"][?"maxReplicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"component"].orValue(null)`,
								Value:      ptr.To("predictor"),
							},
						},
					},
					{
						Name:     "transformer",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("inferenceservice"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodSpec:  &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"transformer"].orValue(null)`, Patch: `{"spec": {"transformer": value}}`, Replace: true},
							Metadata: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"transformer"].orValue(null)`, Patch: `{"spec": {"transformer": value}}`, Replace: true},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							MinReplicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"transformer"][?"minReplicas"].orValue(null)`},
							MaxReplicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"transformer"][?"maxReplicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"component"].orValue(null)`,
								Value:      ptr.To("transformer"),
							},
						},
					},
				},
			},
			Instructions: v1alpha1.OptimizationInstructions{
				GangScheduling: &v1alpha1.GangSchedulingInstruction{
					PodGroups: []v1alpha1.PodGroupDefinition{{
						Name: "service",
						Members: []v1alpha1.PodGroupMemberDefinition{
							{
								ComponentName:      "predictor",
								GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"serving.kserve.io/inferenceservice"].orValue(null)`},
							},
							{
								ComponentName:      "transformer",
								GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"serving.kserve.io/inferenceservice"].orValue(null)`},
							},
						},
					}},
				},
			},
		},
	}
}
