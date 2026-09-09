// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Deployment returns the built-in Karta for the apps/v1 Deployment workload.
func Deployment() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "apps-deployment-v1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "specReplicas", Expression: `([dyn(object[?"spec"][?"replicas"].orValue(null))].filter(v, v != null && v != false) + [1])[0]`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "deployment",
					Kind: &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
					ScaleDefinition: &v1alpha1.ScaleDefinition{
						Replicas: &v1alpha1.ValueAccessor{Expression: `variables.specReplicas`},
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
							// Progressing while not yet Available: Available False, or absent (a just-created
							// Deployment is Progressing/NewReplicaSetCreated before Available is written).
							Initializing: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `([dyn(object.?status.?conditions.orValue(null))].filter(v, type(v) == list) + [[]])[0].exists(c, c.?type.orValue(null) == "Progressing" && c.?status.orValue(null) == "True") && !([dyn(object.?status.?conditions.orValue(null))].filter(v, type(v) == list) + [[]])[0].exists(c, c.?type.orValue(null) == "Available" && c.?status.orValue(null) == "True")`,
								ExpectedResult: "true",
							}}},
							Running: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{
								{Type: "Progressing", Status: ptr.To("True"), Reason: ptr.To("NewReplicaSetAvailable")},
							}}},
							Failed: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{
								{Type: "Progressing", Status: ptr.To("False"), Reason: ptr.To("ProgressDeadlineExceeded")},
							}}},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "replicaset",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "ReplicaSet"},
						OwnerRef: ptr.To("deployment"),
					},
				},
			},
		},
	}
}
