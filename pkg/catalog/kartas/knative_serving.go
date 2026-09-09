// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// KnativeServing returns the built-in Karta for the Knative Serving Service
// workload (serving.knative.dev/v1).
func KnativeServing() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "serving-knative-dev-service-v1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "minScale", Expression: `([dyn(object[?"spec"][?"template"][?"metadata"][?"annotations"][?"autoscaling.knative.dev/min-scale"].orValue(null))].filter(v, v != null && v != false) + [1])[0]`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "knativeservice",
					Kind: &v1alpha1.GroupVersionKind{Group: "serving.knative.dev", Version: "v1", Kind: "Service"},
					StatusDefinition: &v1alpha1.StatusDefinition{
						ConditionsDefinition: &v1alpha1.ConditionsDefinition{
							Expression:       `object[?"status"][?"conditions"].orValue(null)`,
							TypeFieldName:    "type",
							StatusFieldName:  "status",
							MessageFieldName: ptr.To("message"),
						},
						StatusMappings: v1alpha1.StatusMappings{
							Running: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Ready", Status: ptr.To("True")}}}},
							// Deploying: Ready is Unknown while the revision, route, and ingress
							// come up (reasons OutOfDate, RevisionMissing, IngressNotConfigured,
							// Uninitialized). Failed once Ready settles False.
							Initializing: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Ready", Status: ptr.To("Unknown")}}}},
							Failed:       []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Ready", Status: ptr.To("False")}}}},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "revision",
						Kind:     &v1alpha1.GroupVersionKind{Group: "serving.knative.dev", Version: "v1", Kind: "Revision"},
						OwnerRef: ptr.To("knativeservice"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodTemplateSpec: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"template"].orValue(null)`, Patch: `{"spec": {"template": value}}`, Replace: true},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							MinReplicas: &v1alpha1.ValueAccessor{Expression: `variables.minScale`},
							MaxReplicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"template"][?"metadata"][?"annotations"][?"autoscaling.knative.dev/max-scale"].orValue(null)`},
						},
					},
				},
				AdditionalChildKinds: []v1alpha1.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
			Instructions: v1alpha1.OptimizationInstructions{
				GangScheduling: &v1alpha1.GangSchedulingInstruction{
					PodGroups: []v1alpha1.PodGroupDefinition{{
						Name: "revision",
						Members: []v1alpha1.PodGroupMemberDefinition{{
							ComponentName:      "revision",
							GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"serving.knative.dev/revision"].orValue(null)`},
						}},
					}},
				},
			},
		},
	}
}
