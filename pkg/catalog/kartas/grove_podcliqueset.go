// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// GrovePodCliqueSet returns the built-in Karta for the Grove PodCliqueSet
// workload (grove.io/v1alpha1).
func GrovePodCliqueSet() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "grove-io-podcliqueset-v1alpha1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "cliques", Expression: `([dyn(object[?"spec"][?"template"][?"cliques"].orValue(null))].filter(v, type(v) == list) + [[]])[0]`},
				{Name: "scalingGroups", Expression: `([dyn(object[?"spec"][?"template"][?"podCliqueScalingGroups"].orValue(null))].filter(v, type(v) == list) + [[]])[0]`},
				{Name: "cliqueReplicas", Expression: `variables.cliques.map(c, dyn(c.?spec.?replicas.orValue(null))).filter(v, v != null && v != false)`},
				{Name: "scalingGroupReplicas", Expression: `variables.scalingGroups.map(g, dyn(g.?replicas.orValue(null))).filter(v, v != null && v != false)`},
				{Name: "statusAvailableReplicas", Expression: `([dyn(object.?status.?availableReplicas.orValue(null))].filter(v, v != null && v != false) + [0])[0]`},
				{Name: "specReplicas", Expression: `([dyn(object.?spec.?replicas.orValue(null))].filter(v, v != null && v != false) + [0])[0]`},
				{Name: "specReplicasFloat", Expression: `([dyn(object.?spec.?replicas.orValue(null))].filter(v, v != null && v != false) + [1.0])[0]`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "podcliqueset",
					Kind: &v1alpha1.GroupVersionKind{Group: "grove.io", Version: "v1alpha1", Kind: "PodCliqueSet"},
					// Grove PCS has no aggregate phase field. The reliable signal for
					// running/initializing is replica counts (availableReplicas vs
					// spec.replicas). TopologyLevelsUnavailable is the only PCS-level
					// failure condition (TAS-specific); broader failures surface on
					// child PodClique/Pod resources.
					StatusDefinition: &v1alpha1.StatusDefinition{
						ConditionsDefinition: &v1alpha1.ConditionsDefinition{
							Expression:       `object[?"status"][?"conditions"].orValue(null)`,
							TypeFieldName:    "type",
							StatusFieldName:  "status",
							ReasonFieldName:  ptr.To("reason"),
							MessageFieldName: ptr.To("message"),
						},
						StatusMappings: v1alpha1.StatusMappings{
							Failed: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "TopologyLevelsUnavailable", Status: ptr.To("True")}}}},
							Running: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								// Matches when all desired replicas are available (including the
								// vacuous replicas=0 case, same as k8s Deployment Available=True).
								Expression:     `variables.statusAvailableReplicas >= variables.specReplicas`,
								ExpectedResult: "true",
							}}},
							Initializing: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `variables.specReplicas > 0 && variables.statusAvailableReplicas < variables.specReplicas`,
								ExpectedResult: "true",
							}}},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					// Standalone PodCliques (entries under .spec.template.cliques that are NOT
					// referenced by any .spec.template.podCliqueScalingGroups[].cliqueNames).
					// PodCliques inside a scaling group are owned by the PodCliqueScalingGroup,
					// so they are reached through the `scalinggroup` child below.
					{
						Name:     "clique",
						Kind:     &v1alpha1.GroupVersionKind{Group: "grove.io", Version: "v1alpha1", Kind: "PodClique"},
						OwnerRef: ptr.To("podcliqueset"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"cliques"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"spec"][?"podSpec"][?"schedulerName"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/template/cliques/" + string(index) + "/spec/podSpec/schedulerName", "value": value}]`},
								Labels:            &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"cliques"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"labels"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/template/cliques/" + string(index) + "/labels", "value": value}]`},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"cliques"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"annotations"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/template/cliques/" + string(index) + "/annotations", "value": value}]`},
								Containers:        &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"cliques"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"spec"][?"podSpec"][?"containers"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/template/cliques/" + string(index) + "/spec/podSpec/containers", "value": value}]`},
								ResourceClaims:    &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"cliques"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"spec"][?"podSpec"][?"resourceClaims"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/template/cliques/" + string(index) + "/spec/podSpec/resourceClaims", "value": value}]`},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"cliques"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"spec"][?"podSpec"][?"priorityClassName"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/template/cliques/" + string(index) + "/spec/podSpec/priorityClassName", "value": value}]`},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"cliques"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"spec"][?"podSpec"][?"affinity"][?"podAffinity"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/template/cliques/" + string(index) + "/spec/podSpec/affinity/podAffinity", "value": value}]`},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"cliques"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"spec"][?"podSpec"][?"affinity"][?"nodeAffinity"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/template/cliques/" + string(index) + "/spec/podSpec/affinity/nodeAffinity", "value": value}]`},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							// PCS multiplies replicas: total pods = PCS.replicas * clique.replicas
							Replicas:    &v1alpha1.ValueAccessor{Expression: `(variables.cliqueReplicas.size() > 0 ? variables.cliqueReplicas : [dyn(1.0)]).map(v, variables.specReplicasFloat * v)`},
							MinReplicas: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"cliques"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"spec"][?"autoScalingConfig"][?"minReplicas"].orValue(null))`},
							MaxReplicas: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"cliques"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"spec"][?"autoScalingConfig"][?"maxReplicas"].orValue(null))`},
						},
						InstanceIds: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"cliques"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"name"].orValue(null))`},
						PodSelector: &v1alpha1.PodSelector{
							ComponentInstanceSelector: &v1alpha1.ComponentInstanceSelector{
								Expression: `object[?"metadata"][?"labels"][?"grove.io/podclique"].orValue(null)`,
							},
							ReplicaSelector: &v1alpha1.ReplicaSelector{
								Expression: `object[?"metadata"][?"labels"][?"grove.io/podcliqueset-replica-index"].orValue(null)`,
							},
						},
					},
					// PodCliqueScalingGroups (each entry under .spec.template.podCliqueScalingGroups
					// becomes a PodCliqueScalingGroup CR per PCS replica).
					{
						Name:     "scalinggroup",
						Kind:     &v1alpha1.GroupVersionKind{Group: "grove.io", Version: "v1alpha1", Kind: "PodCliqueScalingGroup"},
						OwnerRef: ptr.To("podcliqueset"),
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas:    &v1alpha1.ValueAccessor{Expression: `(variables.scalingGroupReplicas.size() > 0 ? variables.scalingGroupReplicas : [dyn(1.0)]).map(v, variables.specReplicasFloat * v)`},
							MinReplicas: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"podCliqueScalingGroups"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"scaleConfig"][?"minReplicas"].orValue(null))`},
							MaxReplicas: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"podCliqueScalingGroups"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"scaleConfig"][?"maxReplicas"].orValue(null))`},
						},
						InstanceIds: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"template"][?"podCliqueScalingGroups"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"name"].orValue(null))`},
						PodSelector: &v1alpha1.PodSelector{
							ComponentInstanceSelector: &v1alpha1.ComponentInstanceSelector{
								Expression: `object[?"metadata"][?"labels"][?"grove.io/podcliquescalinggroup"].orValue(null)`,
							},
							ReplicaSelector: &v1alpha1.ReplicaSelector{
								Expression: `object[?"metadata"][?"labels"][?"grove.io/podcliquescalinggroup-replica-index"].orValue(null)`,
							},
						},
					},
				},
				AdditionalChildKinds: []v1alpha1.GroupVersionKind{
					// PodCliques owned indirectly via PodCliqueScalingGroups also live in the tree.
					{Group: "grove.io", Version: "v1alpha1", Kind: "PodClique"},
					// PodCliqueScalingGroup must also appear here (even though it is a childComponent
					// above) so the external-workload-integrator's top-owner walker recognises it as
					// part of this workload and continues past it to reach the PCS root. Without this,
					// scaling-group pods get a top-owner of their own PodClique CR and form a separate workload.
					{Group: "grove.io", Version: "v1alpha1", Kind: "PodCliqueScalingGroup"},
					// Gang-scheduling primitive emitted by Grove for each PCS replica.
					{Group: "scheduler.grove.io", Version: "v1alpha1", Kind: "PodGang"},
				},
			},
			// Each PodCliqueSet replica is a gang: every pod (from standalone cliques and
			// scaling groups) sharing the same PCS name + replica index must be co-scheduled.
			Instructions: v1alpha1.OptimizationInstructions{
				GangScheduling: &v1alpha1.GangSchedulingInstruction{
					PodGroups: []v1alpha1.PodGroupDefinition{{
						Name: "podcliqueset-replica",
						Members: []v1alpha1.PodGroupMemberDefinition{
							{
								ComponentName:      "clique",
								GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"app.kubernetes.io/part-of"].orValue(null)`, `object[?"metadata"][?"labels"][?"grove.io/podcliqueset-replica-index"].orValue(null)`},
							},
							{
								ComponentName:      "scalinggroup",
								GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"app.kubernetes.io/part-of"].orValue(null)`, `object[?"metadata"][?"labels"][?"grove.io/podcliqueset-replica-index"].orValue(null)`},
							},
						},
					}},
				},
			},
		},
	}
}
