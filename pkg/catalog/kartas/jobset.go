// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Jobset returns the built-in Karta for the JobSet workload
// (jobset.x-k8s.io/v1alpha2).
func Jobset() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "jobset-x-k8s-io-jobset-v1alpha2"},
		Spec: v1alpha1.KartaSpec{
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "jobset",
					Kind: &v1alpha1.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
					SuspendDefinition: &v1alpha1.SuspendDefinition{
						SuspendActions: []v1alpha1.SuspendAction{{Patch: `{"spec": {"suspend": true}}`}},
						ResumeActions:  []v1alpha1.SuspendAction{{Patch: `{"spec": {"suspend": false}}`}},
					},
					StatusDefinition: &v1alpha1.StatusDefinition{
						ConditionsDefinition: &v1alpha1.ConditionsDefinition{
							Expression:       `object[?"status"][?"conditions"].orValue(null)`,
							TypeFieldName:    "type",
							StatusFieldName:  "status",
							MessageFieldName: ptr.To("message"),
							ReasonFieldName:  ptr.To("reason"),
						},
						StatusMappings: v1alpha1.StatusMappings{
							Initializing: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								// In progress with no working pods: status exists, no replicatedJob has
								// active or ready pods, and no terminal or suspended condition is set.
								// Covers a just-created JobSet (all counts zero) and the window after a
								// job succeeds but before the JobSet-level Completed condition is set.
								Expression:     `size(([dyn(object.?status.?replicatedJobsStatus.orValue(null))].filter(v, type(v) == list) + [[]])[0]) > 0 && ([dyn(object.?status.?replicatedJobsStatus.orValue(null))].filter(v, type(v) == list) + [[]])[0].all(r, r.?active.orValue(0) == 0 && r.?ready.orValue(0) == 0) && !([dyn(object.?status.?conditions.orValue(null))].filter(v, type(v) == list) + [[]])[0].exists(c, (c.?type.orValue(null) == "Completed" || c.?type.orValue(null) == "Failed" || c.?type.orValue(null) == "Suspended") && c.?status.orValue(null) == "True")`,
								ExpectedResult: "true",
							}}},
							Running: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								// Working: at least one replicatedJob has active or ready pods and none
								// have failed. Reading either count (not both) keeps the state stable
								// while the controller briefly flaps ready to 0 mid-run.
								Expression:     `([dyn(object.?status.?replicatedJobsStatus.orValue(null))].filter(v, type(v) == list) + [[]])[0].exists(r, r.?active.orValue(0) > 0 || r.?ready.orValue(0) > 0) && ([dyn(object.?status.?replicatedJobsStatus.orValue(null))].filter(v, type(v) == list) + [[]])[0].all(r, r.?failed.orValue(0) == 0)`,
								ExpectedResult: "true",
							}}},
							Completed: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Completed", Status: ptr.To("True")}}}},
							Failed:    []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Failed", Status: ptr.To("True")}}}},
							Suspended: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Suspended", Status: ptr.To("True")}}}},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					// ReplicatedJob - represents the actual Job resources created by JobSet.
					{
						Name:     "replicatedjob",
						Kind:     &v1alpha1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
						OwnerRef: ptr.To("jobset"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodTemplateSpec: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"replicatedJobs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"template"][?"spec"][?"template"].orValue(null))`, Patch: `[{"op": "add", "path": "/spec/replicatedJobs/" + string(index) + "/template/spec/template", "value": value}]`},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							// Total pods per replicatedJob = replicas (Job instances) * parallelism (pods per Job).
							Replicas: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"replicatedJobs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(j, j.replicas * j.template.spec.parallelism)`},
						},
						InstanceIds: &v1alpha1.ValueAccessor{Expression: `([dyn(object[?"spec"][?"replicatedJobs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"name"].orValue(null))`},
						PodSelector: &v1alpha1.PodSelector{
							ComponentInstanceSelector: &v1alpha1.ComponentInstanceSelector{
								Expression: `object[?"metadata"][?"labels"][?"jobset.sigs.k8s.io/replicatedjob-name"].orValue(null)`,
							},
						},
					},
				},
			},
			Instructions: v1alpha1.OptimizationInstructions{
				GangScheduling: &v1alpha1.GangSchedulingInstruction{
					PodGroups: []v1alpha1.PodGroupDefinition{{
						Name: "job",
						Members: []v1alpha1.PodGroupMemberDefinition{{
							ComponentName:      "replicatedjob",
							GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"jobset.sigs.k8s.io/replicatedjob-name"].orValue(null)`},
						}},
					}},
				},
			},
		},
	}
}
