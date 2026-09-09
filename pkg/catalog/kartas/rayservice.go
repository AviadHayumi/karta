// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// RayService returns the built-in Karta for the RayService workload
// (ray.io/v1).
func RayService() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "ray-io-rayservice-v1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "workerGroups", Expression: `([dyn(object[?"spec"][?"rayClusterConfig"][?"workerGroupSpecs"].orValue(null))].filter(v, type(v) == list) + [[]])[0]`},
				{Name: "workerReplicas", Expression: `variables.workerGroups.map(x, dyn(x[?"replicas"].orValue(null))).filter(v, v != null && v != false)`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "rayservice",
					Kind: &v1alpha1.GroupVersionKind{Group: "ray.io", Version: "v1", Kind: "RayService"},
					StatusDefinition: &v1alpha1.StatusDefinition{
						ConditionsDefinition: &v1alpha1.ConditionsDefinition{
							Expression:       `object[?"status"][?"conditions"].orValue(null)`,
							TypeFieldName:    "type",
							StatusFieldName:  "status",
							ReasonFieldName:  ptr.To("reason"),
							MessageFieldName: ptr.To("message"),
						},
						StatusMappings: v1alpha1.StatusMappings{
							Running:      []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Ready", Status: ptr.To("True")}}}},
							Initializing: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Ready", Status: ptr.To("False"), Reason: ptr.To("Initializing")}}}},
							Degraded:     []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Ready", Status: ptr.To("False"), Reason: ptr.To("ZeroServeEndpoints")}}}},
							Failed: []v1alpha1.StatusMatcher{
								{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Ready", Status: ptr.To("False"), Reason: ptr.To("InitializingTimeout")}}},
								{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Ready", Status: ptr.To("False"), Reason: ptr.To("ValidationFailed")}}},
							},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "head",
						Kind:     &v1alpha1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
						OwnerRef: ptr.To("rayservice"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodTemplateSpec: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"rayClusterConfig"][?"headGroupSpec"][?"template"].orValue(null)`, Patch: `{"spec": {"rayClusterConfig": {"headGroupSpec": {"template": value}}}}`, Replace: true},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `1`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"ray.io/node-type"].orValue(null)`,
								Value:      ptr.To("head"),
							},
						},
					},
					{
						Name:     "worker",
						Kind:     &v1alpha1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
						OwnerRef: ptr.To("rayservice"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodTemplateSpec: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"rayClusterConfig"][?"workerGroupSpecs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"template"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/rayClusterConfig/workerGroupSpecs/" + string(index) + "/template", "value": value}]`},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `(variables.workerReplicas.size() > 0 ? variables.workerReplicas : [dyn(1)])`},
						},
						InstanceIds: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"rayClusterConfig"][?"workerGroupSpecs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"groupName"].orValue(null))`},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"ray.io/node-type"].orValue(null)`,
								Value:      ptr.To("worker"),
							},
							ComponentInstanceSelector: &v1alpha1.ComponentInstanceSelector{
								Expression: `object[?"metadata"][?"labels"][?"ray.io/group"].orValue(null)`,
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
								ComponentName:      "head",
								GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"ray.io/cluster"].orValue(null)`},
							},
							{
								ComponentName:      "worker",
								GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"ray.io/cluster"].orValue(null)`},
							},
						},
					}},
				},
			},
		},
	}
}
