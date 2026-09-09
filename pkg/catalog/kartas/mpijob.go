// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Mpijob returns the built-in Karta for the MPIJob workload
// (kubeflow.org/v2beta1).
func Mpijob() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "kubeflow-org-mpijob-v2beta1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "specMpiReplicaSpecsLauncherReplicas", Expression: `([dyn(object[?"spec"][?"mpiReplicaSpecs"][?"Launcher"][?"replicas"].orValue(null))].filter(v, v != null && v != false) + [1])[0]`},
				{Name: "specMpiReplicaSpecsWorkerReplicas", Expression: `([dyn(object[?"spec"][?"mpiReplicaSpecs"][?"Worker"][?"replicas"].orValue(null))].filter(v, v != null && v != false) + [1])[0]`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "mpijob",
					Kind: &v1alpha1.GroupVersionKind{Group: "kubeflow.org", Version: "v2beta1", Kind: "MPIJob"},
					SuspendDefinition: &v1alpha1.SuspendDefinition{
						SuspendActions: []v1alpha1.SuspendAction{{Patch: `{"spec": {"runPolicy": {"suspend": true}}}`}},
						ResumeActions:  []v1alpha1.SuspendAction{{Patch: `{"spec": {"runPolicy": {"suspend": false}}}`}},
					},
					StatusDefinition: &v1alpha1.StatusDefinition{
						ConditionsDefinition: &v1alpha1.ConditionsDefinition{
							Expression:       `object[?"status"][?"conditions"].orValue(null)`,
							TypeFieldName:    "type",
							StatusFieldName:  "status",
							ReasonFieldName:  ptr.To("reason"),
							MessageFieldName: ptr.To("message"),
						},
						StatusMappings: v1alpha1.StatusMappings{
							Initializing: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Created", Status: ptr.To("True")}}}},
							Running:      []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Running", Status: ptr.To("True")}}}},
							Completed:    []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Succeeded", Status: ptr.To("True")}}}},
							Failed:       []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Failed", Status: ptr.To("True")}}}},
							Suspended:    []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "Suspended", Status: ptr.To("True")}}}},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "launcher",
						Kind:     &v1alpha1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
						OwnerRef: ptr.To("mpijob"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodTemplateSpec: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"mpiReplicaSpecs"][?"Launcher"][?"template"].orValue(null)`, Patch: `{"spec": {"mpiReplicaSpecs": {"Launcher": {"template": value}}}}`, Replace: true},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `variables.specMpiReplicaSpecsLauncherReplicas`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"training.kubeflow.org/job-role"].orValue(null)`,
								Value:      ptr.To("launcher"),
							},
						},
					},
					{
						Name:     "worker",
						Kind:     &v1alpha1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
						OwnerRef: ptr.To("mpijob"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodTemplateSpec: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"mpiReplicaSpecs"][?"Worker"][?"template"].orValue(null)`, Patch: `{"spec": {"mpiReplicaSpecs": {"Worker": {"template": value}}}}`, Replace: true},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `variables.specMpiReplicaSpecsWorkerReplicas`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"training.kubeflow.org/job-role"].orValue(null)`,
								Value:      ptr.To("worker"),
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
								ComponentName:      "launcher",
								GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"training.kubeflow.org/job-name"].orValue(null)`},
							},
							{
								ComponentName:      "worker",
								GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"training.kubeflow.org/job-name"].orValue(null)`},
							},
						},
					}},
				},
			},
		},
	}
}
