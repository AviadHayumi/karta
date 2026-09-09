// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// BatchJob returns the built-in Karta for the batch/v1 Job workload.
func BatchJob() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "batch-job-v1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "specParallelism", Expression: `([dyn(object[?"spec"][?"parallelism"].orValue(null))].filter(v, v != null && v != false) + [1])[0]`},
				{Name: "statusActive", Expression: `([dyn(object.?status.?active.orValue(null))].filter(v, v != null && v != false) + [0])[0]`},
				{Name: "statusReady", Expression: `([dyn(object.?status.?ready.orValue(null))].filter(v, v != null && v != false) + [0])[0]`},
				{Name: "specParallelismZero", Expression: `([dyn(object.?spec.?parallelism.orValue(null))].filter(v, v != null && v != false) + [0])[0]`},
				{Name: "statusSucceeded", Expression: `([dyn(object.?status.?succeeded.orValue(null))].filter(v, v != null && v != false) + [0])[0]`},
				{Name: "statusFailed", Expression: `([dyn(object.?status.?failed.orValue(null))].filter(v, v != null && v != false) + [0])[0]`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "job",
					Kind: &v1alpha1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
					ScaleDefinition: &v1alpha1.ScaleDefinition{
						Replicas: &v1alpha1.ValueAccessor{Expression: `variables.specParallelism`},
					},
					SpecDefinition: &v1alpha1.SpecDefinition{
						PodTemplateSpec: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"template"].orValue(null)`, Patch: `{"spec": {"template": value}}`, Replace: true},
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
								Expression:     `variables.statusActive > 0 && variables.statusReady == 0`,
								ExpectedResult: "true",
							}}},
							Running: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `variables.statusActive > 0 && variables.statusReady > 0`,
								ExpectedResult: "true",
							}}},
							Completed: []v1alpha1.StatusMatcher{
								{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Complete", Status: ptr.To("True")}}},
								{ByConditions: []v1alpha1.ExpectedCondition{{Type: "SuccessCriteriaMet", Status: ptr.To("True")}}},
							},
							Failed: []v1alpha1.StatusMatcher{
								{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Failed", Status: ptr.To("True")}}},
								{ByConditions: []v1alpha1.ExpectedCondition{{Type: "FailureTarget", Status: ptr.To("True")}}},
							},
							Degraded: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `variables.specParallelismZero > 1 && variables.statusReady < variables.specParallelismZero && (variables.statusSucceeded > 0 || variables.statusFailed > 0)`,
								ExpectedResult: "true",
							}}},
							Suspended: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Suspended", Status: ptr.To("True")}}}},
						},
					},
					SuspendDefinition: &v1alpha1.SuspendDefinition{
						SuspendActions: []v1alpha1.SuspendAction{{Patch: `{"spec": {"suspend": true}}`}},
						ResumeActions:  []v1alpha1.SuspendAction{{Patch: `{"spec": {"suspend": false}}`}},
					},
				},
			},
			Instructions: v1alpha1.OptimizationInstructions{
				GangScheduling: &v1alpha1.GangSchedulingInstruction{
					PodGroups: []v1alpha1.PodGroupDefinition{{
						Name: "job",
						Members: []v1alpha1.PodGroupMemberDefinition{{
							ComponentName:      "job",
							GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"batch.kubernetes.io/job-name"].orValue(null)`},
						}},
					}},
				},
			},
		},
	}
}
