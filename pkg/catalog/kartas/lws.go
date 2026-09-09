// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// LWS returns the built-in Karta for the LeaderWorkerSet workload
// (leaderworkerset.x-k8s.io/v1).
func LWS() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "leaderworkerset-x-k8s-io-leaderworkerset-v1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "statusReplicas", Expression: `([dyn(object.?status.?replicas.orValue(null))].filter(v, v != null && v != false) + [0])[0]`},
				{Name: "specReplicas", Expression: `([dyn(object[?"spec"][?"replicas"].orValue(null))].filter(v, v != null && v != false) + [1])[0]`},
				{Name: "specReplicasFloat", Expression: `([dyn(object.?spec.?replicas.orValue(null))].filter(v, v != null && v != false) + [1.0])[0]`},
				{Name: "specLeaderWorkerTemplateSize", Expression: `([dyn(object.?spec.?leaderWorkerTemplate.?size.orValue(null))].filter(v, v != null && v != false) + [1.0])[0]`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "leaderworkerset",
					Kind: &v1alpha1.GroupVersionKind{Group: "leaderworkerset.x-k8s.io", Version: "v1", Kind: "LeaderWorkerSet"},
					StatusDefinition: &v1alpha1.StatusDefinition{
						ConditionsDefinition: &v1alpha1.ConditionsDefinition{
							Expression:       `object[?"status"][?"conditions"].orValue(null)`,
							TypeFieldName:    "type",
							StatusFieldName:  "status",
							MessageFieldName: ptr.To("message"),
						},
						StatusMappings: v1alpha1.StatusMappings{
							// Progressing while not yet available: Available False, or absent (a
							// starting LeaderWorkerSet is Progressing before it writes Available).
							Initializing: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `([dyn(object.?status.?conditions.orValue(null))].filter(v, type(v) == list) + [[]])[0].exists(c, c.?type.orValue(null) == "Progressing" && c.?status.orValue(null) == "True") && !([dyn(object.?status.?conditions.orValue(null))].filter(v, type(v) == list) + [[]])[0].exists(c, c.?type.orValue(null) == "Available" && c.?status.orValue(null) == "True")`,
								ExpectedResult: "true",
							}}},
							// Available is the authoritative "all groups ready" signal and stays
							// True while scaling down sheds an extra pod, so key on it alone rather
							// than on the replica counts (which lag) or UpdateInProgress (which is
							// absent mid-scale). This ConditionsDefinition does not extract reason,
							// so match on status only. The replica-settled expression is a fallback
							// for when the condition is not populated.
							Running: []v1alpha1.StatusMatcher{
								{ByConditions: []v1alpha1.ExpectedCondition{
									{Type: "Available", Status: ptr.To("True")},
								}},
								{ByExpression: &v1alpha1.ExpressionMatcher{
									Expression:     `variables.statusReplicas > 0 && object.?status.?readyReplicas.orValue(-1) == object.?status.?replicas.orValue(-2) && object.?status.?updatedReplicas.orValue(-1) == object.?status.?replicas.orValue(-2)`,
									ExpectedResult: "true",
								}},
							},
							Failed: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{
								{Type: "Available", Status: ptr.To("False")},
								{Type: "Progressing", Status: ptr.To("False")},
								{Type: "UpdateInProgress", Status: ptr.To("False")},
							}}},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "group",
						OwnerRef: ptr.To("leaderworkerset"),
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"leaderWorkerTemplate"][?"size"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ReplicaSelector: &v1alpha1.ReplicaSelector{
								Expression: `object[?"metadata"][?"labels"][?"leaderworkerset.sigs.k8s.io/group-index"].orValue(null)`,
							},
						},
					},
					{
						Name:     "leader",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
						OwnerRef: ptr.To("group"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodTemplateSpec: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"leaderWorkerTemplate"][?"leaderTemplate"].orValue(null)`, Patch: `{"spec": {"leaderWorkerTemplate": {"leaderTemplate": value}}}`, Replace: true},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `variables.specReplicas`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"leaderworkerset.sigs.k8s.io/worker-index"].orValue(null)`,
								Value:      ptr.To("0"),
							},
						},
					},
					{
						Name:     "worker",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
						OwnerRef: ptr.To("group"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodTemplateSpec: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"leaderWorkerTemplate"][?"workerTemplate"].orValue(null)`, Patch: `{"spec": {"leaderWorkerTemplate": {"workerTemplate": value}}}`, Replace: true},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `variables.specReplicasFloat * (variables.specLeaderWorkerTemplateSize - 1.0)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"annotations"][?"leaderworkerset.sigs.k8s.io/leader-name"].orValue(null)`,
							},
						},
					},
				},
			},
			Instructions: v1alpha1.OptimizationInstructions{
				GangScheduling: &v1alpha1.GangSchedulingInstruction{
					PodGroups: []v1alpha1.PodGroupDefinition{{
						Name: "group",
						Members: []v1alpha1.PodGroupMemberDefinition{{
							ComponentName:      "group",
							GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"leaderworkerset.sigs.k8s.io/name"].orValue(null)`, `([dyn(object[?"metadata"][?"labels"][?"leaderworkerset.sigs.k8s.io/group-index"].orValue(null))].filter(v, v != null && v != false) + ["0"])[0]`},
						}},
					}},
				},
			},
		},
	}
}
