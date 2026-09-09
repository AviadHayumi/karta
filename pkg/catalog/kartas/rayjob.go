// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Rayjob returns the built-in Karta for the RayJob workload
// (ray.io/v1).
func Rayjob() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "ray-io-rayjob-v1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "workerGroups", Expression: `([dyn(object[?"spec"][?"rayClusterSpec"][?"workerGroupSpecs"].orValue(null))].filter(v, type(v) == list) + [[]])[0]`},
				{Name: "workerReplicas", Expression: `variables.workerGroups.map(x, dyn(x[?"replicas"].orValue(null))).filter(v, v != null && v != false)`},
				{Name: "statusJobStatus", Expression: `([dyn(object.?status.?jobStatus.orValue(null))].filter(v, v != null && v != false) + [""])[0]`},
				{Name: "statusJobDeploymentStatus", Expression: `([dyn(object.?status.?jobDeploymentStatus.orValue(null))].filter(v, v != null && v != false) + [""])[0]`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "rayjob",
					Kind: &v1alpha1.GroupVersionKind{Group: "ray.io", Version: "v1", Kind: "RayJob"},
					SuspendDefinition: &v1alpha1.SuspendDefinition{
						SuspendActions: []v1alpha1.SuspendAction{{Patch: `{"spec": {"suspend": true}}`}},
						ResumeActions:  []v1alpha1.SuspendAction{{Patch: `{"spec": {"suspend": false}}`}},
					},
					StatusDefinition: &v1alpha1.StatusDefinition{
						PhaseDefinition: &v1alpha1.PhaseDefinition{
							Expression: `object[?"status"][?"jobStatus"].orValue(null)`,
						},
						ConditionsDefinition: &v1alpha1.ConditionsDefinition{
							Expression:       `object[?"status"][?"conditions"].orValue(null)`,
							TypeFieldName:    "type",
							StatusFieldName:  "status",
							MessageFieldName: ptr.To("message"),
						},
						StatusMappings: v1alpha1.StatusMappings{
							// PENDING once the job is queued, plus the provisioning window before
							// that: jobStatus empty while the RayJob brings up its cluster and it is
							// not suspended (jobDeploymentStatus Initializing/Running, or empty).
							Initializing: []v1alpha1.StatusMatcher{
								{ByPhase: "PENDING"},
								{ByExpression: &v1alpha1.ExpressionMatcher{
									Expression:     `variables.statusJobStatus == "" && variables.statusJobDeploymentStatus != "Suspended" && variables.statusJobDeploymentStatus != "Suspending"`,
									ExpectedResult: "true",
								}},
							},
							Running:   []v1alpha1.StatusMatcher{{ByPhase: "RUNNING"}},
							Completed: []v1alpha1.StatusMatcher{{ByPhase: "SUCCEEDED"}},
							Failed:    []v1alpha1.StatusMatcher{{ByPhase: "FAILED"}},
							// Suspended or on the way there: the operator reports Suspending while it
							// tears the cluster down before settling on Suspended.
							Suspended: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `variables.statusJobDeploymentStatus == "Suspended" || variables.statusJobDeploymentStatus == "Suspending"`,
								ExpectedResult: "true",
							}}},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "head",
						Kind:     &v1alpha1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
						OwnerRef: ptr.To("rayjob"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodTemplateSpec: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"rayClusterSpec"][?"headGroupSpec"][?"template"].orValue(null)`, Patch: `{"spec": {"rayClusterSpec": {"headGroupSpec": {"template": value}}}}`, Replace: true},
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
						OwnerRef: ptr.To("rayjob"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodTemplateSpec: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"rayClusterSpec"][?"workerGroupSpecs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"template"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/rayClusterSpec/workerGroupSpecs/" + string(index) + "/template", "value": value}]`},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas:    &v1alpha1.ValueAccessor{Expression: `(variables.workerReplicas.size() > 0 ? variables.workerReplicas : [dyn(1)])`},
							MinReplicas: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"rayClusterSpec"][?"workerGroupSpecs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"minReplicas"].orValue(null))`},
							MaxReplicas: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"rayClusterSpec"][?"workerGroupSpecs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"maxReplicas"].orValue(null))`},
						},
						InstanceIds: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"rayClusterSpec"][?"workerGroupSpecs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"groupName"].orValue(null))`},
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
						Name: "job",
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
