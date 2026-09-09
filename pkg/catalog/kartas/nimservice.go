// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// NIMService returns the built-in Karta for the NIMService workload
// (apps.nvidia.com/v1alpha1).
func NIMService() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "apps-nvidia-com-nimservice-v1alpha1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "specReplicas", Expression: `([dyn(object[?"spec"][?"replicas"].orValue(null))].filter(v, v != null && v != false) + [1])[0]`},
				{Name: "statusState", Expression: `([dyn(object.?status.?state.orValue(null))].filter(v, v != null && v != false) + [""])[0]`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "nimservice",
					Kind: &v1alpha1.GroupVersionKind{Group: "apps.nvidia.com", Version: "v1alpha1", Kind: "NIMService"},
					SpecDefinition: &v1alpha1.SpecDefinition{
						FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
							SchedulerName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"schedulerName": value}}`, Replace: true},
							Labels:        &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"labels"].orValue(null)`, Patch: `{"spec": {"labels": value}}`, Replace: true},
							Annotations:   &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"annotations"].orValue(null)`, Patch: `{"spec": {"annotations": value}}`, Replace: true},
							Resources:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"resources"].orValue(null)`, Patch: `{"spec": {"resources": value}}`, Replace: true},
							PodAffinity:   &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"affinity": {"podAffinity": value}}}`, Replace: true},
							NodeAffinity:  &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"affinity": {"nodeAffinity": value}}}`, Replace: true},
						},
					},
					ScaleDefinition: &v1alpha1.ScaleDefinition{
						Replicas: &v1alpha1.ValueAccessor{Expression: `variables.specReplicas`},
					},
					StatusDefinition: &v1alpha1.StatusDefinition{
						PhaseDefinition: &v1alpha1.PhaseDefinition{
							Expression: `object[?"status"][?"state"].orValue(null)`,
						},
						ConditionsDefinition: &v1alpha1.ConditionsDefinition{
							Expression:       `object[?"status"][?"conditions"].orValue(null)`,
							TypeFieldName:    "type",
							StatusFieldName:  "status",
							MessageFieldName: ptr.To("message"),
						},
						StatusMappings: v1alpha1.StatusMappings{
							// In progress: any state that is not the terminal Ready or Failed. Covers
							// the empty just-created state and every intermediate the operator writes
							// (PVC-Created, NotReady, Pending, ...).
							Initializing: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `variables.statusState != "Ready" && variables.statusState != "Failed"`,
								ExpectedResult: "true",
							}}},
							Running: []v1alpha1.StatusMatcher{
								{ByPhase: "Ready"},
							},
							Failed: []v1alpha1.StatusMatcher{
								{ByPhase: "Failed"},
							},
						},
					},
				},
			},
			Instructions: v1alpha1.OptimizationInstructions{
				GangScheduling: &v1alpha1.GangSchedulingInstruction{
					PodGroups: []v1alpha1.PodGroupDefinition{{
						Name: "service",
						Members: []v1alpha1.PodGroupMemberDefinition{{
							ComponentName:      "nimservice",
							GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"app.kubernetes.io/name"].orValue(null)`},
						}},
					}},
				},
			},
		},
	}
}
