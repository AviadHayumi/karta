// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// TrainJob returns the built-in Karta for the Kubeflow Trainer v2 TrainJob workload.
//
// A TrainJob holds only overrides: the base pod template lives in the ClusterTrainingRuntime its
// runtimeRef names. The definition declares that runtime as a reference, so every effective value
// is the TrainJob's override coalesced onto the runtime's base - the first definition in the
// catalog whose reads need more than the workload object.
func TrainJob() *v1alpha1.Karta {
	// The runtime's single replicated job holds the pod template at
	// template.spec.replicatedJobs[0].template.spec.template.spec, container "node".
	const runtimeContainer = `references.trainingRuntime.spec.template.spec.replicatedJobs[0].template.spec.template.spec.containers[0]`

	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "trainer-kubeflow-org-trainjob-v1alpha1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "conditions", Expression: `object.?status.?conditions.orValue([])`},
				{Name: "isSuspended", Expression: `variables.conditions.exists(c, c.type == "Suspended" && c.status == "True") || object.?spec.?suspend.orValue(false) == true`},
				{Name: "isTerminal", Expression: `variables.conditions.exists(c, (c.type == "Complete" || c.type == "Failed") && c.status == "True")`},
				{Name: "jobsActive", Expression: `object.?status.?jobsStatus.orValue([]).exists(j, j.?active.orValue(0) > 0 || j.?ready.orValue(0) > 0)`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				References: []v1alpha1.ResourceReference{{
					Name: "trainingRuntime",
					GVK: v1alpha1.GroupVersionKind{
						Group: "trainer.kubeflow.org", Version: "v1alpha1", Kind: "ClusterTrainingRuntime",
					},
					Lookup: &v1alpha1.LookupReference{NameExpression: `object.spec.runtimeRef.name`},
				}},
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "trainjob",
					Kind: &v1alpha1.GroupVersionKind{Group: "trainer.kubeflow.org", Version: "v1alpha1", Kind: "TrainJob"},
					// The trainer override subtree is immutable on a live TrainJob, so every
					// effective value below is read-only: an expression without a patch.
					SpecDefinition: &v1alpha1.SpecDefinition{
						FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
							Image: &v1alpha1.ValueAccessor{
								Expression: `object.?spec.?trainer.?image.orValue(` + runtimeContainer + `[?"image"].orValue(null))`,
							},
							Resources: &v1alpha1.ValueAccessor{
								Expression: `object.?spec.?trainer.?resourcesPerNode.orValue(` + runtimeContainer + `[?"resources"].orValue(null))`,
							},
						},
					},
					ScaleDefinition: &v1alpha1.ScaleDefinition{
						Replicas: &v1alpha1.ValueAccessor{
							Expression: `object.?spec.?trainer.?numNodes.orValue(references.trainingRuntime.?spec.?mlPolicy.?numNodes.orValue(1))`,
						},
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
								Expression:     `!variables.isTerminal && !variables.isSuspended && !variables.jobsActive`,
								ExpectedResult: "true",
							}}},
							Running: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `!variables.isTerminal && !variables.isSuspended && variables.jobsActive`,
								ExpectedResult: "true",
							}}},
							Completed: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{
								{Type: "Complete", Status: ptr.To("True")},
							}}},
							Failed: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{
								{Type: "Failed", Status: ptr.To("True")},
							}}},
							Suspended: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								// A status-less TrainJob created suspended is suspended from the first
								// frame, before the controller publishes the condition.
								Expression:     `variables.isSuspended`,
								ExpectedResult: "true",
							}}},
						},
					},
					SuspendDefinition: &v1alpha1.SuspendDefinition{
						SuspendActions: []v1alpha1.SuspendAction{{Patch: `{"spec": {"suspend": true}}`}},
						ResumeActions:  []v1alpha1.SuspendAction{{Patch: `{"spec": {"suspend": false}}`}},
					},
				},
				AdditionalChildKinds: []v1alpha1.GroupVersionKind{
					{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
				},
			},
			Instructions: v1alpha1.OptimizationInstructions{
				GangScheduling: &v1alpha1.GangSchedulingInstruction{
					PodGroups: []v1alpha1.PodGroupDefinition{{
						Name: "trainjob",
						Members: []v1alpha1.PodGroupMemberDefinition{{
							ComponentName:      "trainjob",
							GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"jobset.sigs.k8s.io/jobset-name"].orValue(null)`},
						}},
					}},
				},
			},
		},
	}
}
