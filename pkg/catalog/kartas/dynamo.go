// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Dynamo returns the built-in Karta for the DynamoGraphDeployment workload
// (nvidia.com/v1alpha1).
func Dynamo() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "nvidia-com-dynamographdeployment-v1alpha1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "services", Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0]`},
				{Name: "statusState", Expression: `([dyn(object.?status.?state.orValue(null))].filter(v, v != null && v != false) + [""])[0]`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "dynamographdeployment",
					Kind: &v1alpha1.GroupVersionKind{Group: "nvidia.com", Version: "v1alpha1", Kind: "DynamoGraphDeployment"},
					StatusDefinition: &v1alpha1.StatusDefinition{
						PhaseDefinition: &v1alpha1.PhaseDefinition{Expression: `object[?"status"][?"state"].orValue(null)`},
						StatusMappings: v1alpha1.StatusMappings{
							Initializing: []v1alpha1.StatusMatcher{
								{ByPhase: "initializing"},
								{ByPhase: "pending"},
								// Just created: the operator has not written status.state yet.
								{ByExpression: &v1alpha1.ExpressionMatcher{
									Expression:     `variables.statusState == ""`,
									ExpectedResult: "true",
								}},
							},
							Running: []v1alpha1.StatusMatcher{{ByPhase: "successful"}},
							Failed:  []v1alpha1.StatusMatcher{{ByPhase: "failed"}},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "service",
						OwnerRef: ptr.To("dynamographdeployment"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort().map(k, object.spec.services[k][?"extraPodSpec"][?"schedulerName"].orValue(null))`, Patch: `{"spec": {"services": {instance: {"extraPodSpec": {"schedulerName": value}}}}}`, Replace: true},
								Labels:            &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort().map(k, object.spec.services[k][?"labels"].orValue(null))`, Patch: `{"spec": {"services": {instance: {"labels": value}}}}`, Replace: true},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort().map(k, object.spec.services[k][?"annotations"].orValue(null))`, Patch: `{"spec": {"services": {instance: {"annotations": value}}}}`, Replace: true},
								Resources:         &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort().map(k, object.spec.services[k][?"resources"].orValue(null))`, Patch: `{"spec": {"services": {instance: {"resources": value}}}}`, Replace: true},
								ResourceClaims:    &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort().map(k, object.spec.services[k][?"extraPodSpec"][?"resourceClaims"].orValue(null))`, Patch: `{"spec": {"services": {instance: {"extraPodSpec": {"resourceClaims": value}}}}}`, Replace: true},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort().map(k, object.spec.services[k][?"extraPodSpec"][?"affinity"][?"podAffinity"].orValue(null))`, Patch: `{"spec": {"services": {instance: {"extraPodSpec": {"affinity": {"podAffinity": value}}}}}}`, Replace: true},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort().map(k, object.spec.services[k][?"extraPodSpec"][?"affinity"][?"nodeAffinity"].orValue(null))`, Patch: `{"spec": {"services": {instance: {"extraPodSpec": {"affinity": {"nodeAffinity": value}}}}}}`, Replace: true},
								Container:         &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort().map(k, object.spec.services[k][?"extraPodSpec"][?"mainContainer"].orValue(null))`, Patch: `{"spec": {"services": {instance: {"extraPodSpec": {"mainContainer": value}}}}}`, Replace: true},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort().map(k, object.spec.services[k][?"extraPodSpec"][?"priorityClassName"].orValue(null))`, Patch: `{"spec": {"services": {instance: {"extraPodSpec": {"priorityClassName": value}}}}}`, Replace: true},
								Image:             &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort().map(k, object.spec.services[k][?"extraPodSpec"][?"mainContainer"][?"image"].orValue(null))`, Patch: `{"spec": {"services": {instance: {"extraPodSpec": {"mainContainer": {"image": value}}}}}}`, Replace: true},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas:    &v1alpha1.ValueAccessor{Expression: `variables.services.map(k, string(k)).sort().map(k, ([dyn(variables.services[k].?replicas.orValue(null))].filter(v, v != null && v != false) + [1.0])[0] * ([dyn(variables.services[k].?multinode.?nodeCount.orValue(null))].filter(v, v != null && v != false) + [1.0])[0])`},
							MinReplicas: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort().map(k, object.spec.services[k][?"autoscaling"][?"minReplicas"].orValue(null))`},
							MaxReplicas: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort().map(k, object.spec.services[k][?"autoscaling"][?"maxReplicas"].orValue(null))`},
						},
						InstanceIds: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort()`},
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
					{Group: "nvidia.com", Version: "v1alpha1", Kind: "DynamoComponentDeployment"},
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
						Name: "service",
						Members: []v1alpha1.PodGroupMemberDefinition{{
							ComponentName:      "service",
							GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"nvidia.com/dynamo-component"].orValue(null)`},
						}},
					}},
				},
			},
		},
	}
}
