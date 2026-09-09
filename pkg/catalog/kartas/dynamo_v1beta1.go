// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// DynamoV1beta1 returns the built-in Karta for the DynamoGraphDeployment
// workload (nvidia.com/v1beta1).
func DynamoV1beta1() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "nvidia-com-dynamographdeployment-v1beta1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "components", Expression: `([dyn(object[?"spec"][?"components"].orValue(null))].filter(v, type(v) == list) + [[]])[0]`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "dynamographdeployment",
					Kind: &v1alpha1.GroupVersionKind{Group: "nvidia.com", Version: "v1beta1", Kind: "DynamoGraphDeployment"},
					StatusDefinition: &v1alpha1.StatusDefinition{
						PhaseDefinition: &v1alpha1.PhaseDefinition{Expression: `object[?"status"][?"state"].orValue(null)`},
						StatusMappings: v1alpha1.StatusMappings{
							Initializing: []v1alpha1.StatusMatcher{
								{ByPhase: "initializing"},
								{ByPhase: "pending"},
							},
							Running: []v1alpha1.StatusMatcher{{ByPhase: "successful"}},
							Failed:  []v1alpha1.StatusMatcher{{ByPhase: "failed"}},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "component",
						OwnerRef: ptr.To("dynamographdeployment"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"components"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"podTemplate"][?"spec"][?"schedulerName"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/components/" + string(index) + "/podTemplate/spec/schedulerName", "value": value}]`},
								Labels:            &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"components"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"podTemplate"][?"metadata"][?"labels"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/components/" + string(index) + "/podTemplate/metadata/labels", "value": value}]`},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"components"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"podTemplate"][?"metadata"][?"annotations"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/components/" + string(index) + "/podTemplate/metadata/annotations", "value": value}]`},
								ResourceClaims:    &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"components"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"podTemplate"][?"spec"][?"resourceClaims"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/components/" + string(index) + "/podTemplate/spec/resourceClaims", "value": value}]`},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"components"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"podTemplate"][?"spec"][?"affinity"][?"podAffinity"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/components/" + string(index) + "/podTemplate/spec/affinity/podAffinity", "value": value}]`},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"components"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"podTemplate"][?"spec"][?"affinity"][?"nodeAffinity"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/components/" + string(index) + "/podTemplate/spec/affinity/nodeAffinity", "value": value}]`},
								Container:         &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"components"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, (([dyn(x[?"podTemplate"][?"spec"][?"containers"].orValue(null))].filter(v, type(v) == list) + [[]])[0].filter(c, c[?"name"].orValue("") == "main") + [null])[0])`},
								Image:             &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"components"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, ((([dyn(x[?"podTemplate"][?"spec"][?"containers"].orValue(null))].filter(v, type(v) == list) + [[]])[0].filter(c, c[?"name"].orValue("") == "main") + [{}])[0])[?"image"].orValue(null))`},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"components"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"podTemplate"][?"spec"][?"priorityClassName"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/components/" + string(index) + "/podTemplate/spec/priorityClassName", "value": value}]`},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `variables.components.map(x, ([dyn(x.?replicas.orValue(null))].filter(v, v != null && v != false) + [1.0])[0] * ([dyn(x.?multinode.?nodeCount.orValue(null))].filter(v, v != null && v != false) + [1.0])[0])`},
						},
						InstanceIds: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"components"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"name"].orValue(null))`},
						PodSelector: &v1alpha1.PodSelector{
							ComponentInstanceSelector: &v1alpha1.ComponentInstanceSelector{
								Expression: `object[?"metadata"][?"labels"][?"nvidia.com/dynamo-component"].orValue(null)`,
							},
							ReplicaSelector: &v1alpha1.ReplicaSelector{
								Expression: `object[?"metadata"][?"labels"][?"grove.io/podcliquescalinggroup-replica-index"].orValue(object[?"metadata"][?"labels"][?"leaderworkerset.sigs.k8s.io/group-index"].orValue(null))`,
							},
						},
					},
				},
				AdditionalChildKinds: []v1alpha1.GroupVersionKind{
					{Group: "nvidia.com", Version: "v1beta1", Kind: "DynamoComponentDeployment"},
					{Group: "leaderworkerset.x-k8s.io", Version: "v1", Kind: "LeaderWorkerSet"},
					{Group: "scheduler.grove.io", Version: "v1alpha1", Kind: "PodGang"},
					{Group: "grove.io", Version: "v1alpha1", Kind: "PodClique"},
					{Group: "grove.io", Version: "v1alpha1", Kind: "PodCliqueSet"},
					{Group: "grove.io", Version: "v1alpha1", Kind: "PodCliqueScalingGroup"},
				},
			},
			Instructions: v1alpha1.OptimizationInstructions{
				GangScheduling: &v1alpha1.GangSchedulingInstruction{
					PodGroups: []v1alpha1.PodGroupDefinition{{
						Name: "component",
						Members: []v1alpha1.PodGroupMemberDefinition{{
							ComponentName:      "component",
							GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"nvidia.com/dynamo-component"].orValue(null).lowerAscii()`},
						}},
					}},
				},
			},
		},
	}
}
