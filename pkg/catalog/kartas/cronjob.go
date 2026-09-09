// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// CronJob returns the built-in Karta for the batch/v1 CronJob workload.
func CronJob() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "batch-cronjob-v1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "specJobTemplateSpecParallelism", Expression: `([dyn(object[?"spec"][?"jobTemplate"][?"spec"][?"parallelism"].orValue(null))].filter(v, v != null && v != false) + [1])[0]`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "cronjob",
					Kind: &v1alpha1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"},
					ScaleDefinition: &v1alpha1.ScaleDefinition{
						Replicas: &v1alpha1.ValueAccessor{Expression: `variables.specJobTemplateSpecParallelism`},
					},
					SpecDefinition: &v1alpha1.SpecDefinition{
						PodTemplateSpec: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"jobTemplate"][?"spec"][?"template"].orValue(null)`, Patch: `{"spec": {"jobTemplate": {"spec": {"template": value}}}}`, Replace: true},
					},
					SuspendDefinition: &v1alpha1.SuspendDefinition{
						SuspendActions: []v1alpha1.SuspendAction{{Patch: `{"spec": {"suspend": true}}`}},
						ResumeActions:  []v1alpha1.SuspendAction{{Patch: `{"spec": {"suspend": false}}`}},
					},
					StatusDefinition: &v1alpha1.StatusDefinition{
						StatusMappings: v1alpha1.StatusMappings{
							Initializing: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `object.?spec.?suspend.orValue(false) != true && object.?status.?lastScheduleTime.orValue(null) == null`,
								ExpectedResult: "true",
							}}},
							Running: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `object.?spec.?suspend.orValue(false) != true && object.?status.?lastScheduleTime.orValue(null) != null`,
								ExpectedResult: "true",
							}}},
							Suspended: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `object.?spec.?suspend.orValue(false) == true`,
								ExpectedResult: "true",
							}}},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "job",
						Kind:     &v1alpha1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
						OwnerRef: ptr.To("cronjob"),
					},
				},
			},
		},
	}
}
