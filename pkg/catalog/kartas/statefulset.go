// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// StatefulSet returns the built-in Karta for the apps/v1 StatefulSet workload.
func StatefulSet() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "apps-statefulset-v1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "specReplicas", Expression: `([dyn(object[?"spec"][?"replicas"].orValue(null))].filter(v, v != null && v != false) + [1])[0]`},
				{Name: "statusObservedGeneration", Expression: `([dyn(object.?status.?observedGeneration.orValue(null))].filter(v, v != null && v != false) + [0])[0]`},
				{Name: "metadataGeneration", Expression: `([dyn(object.?metadata.?generation.orValue(null))].filter(v, v != null && v != false) + [0])[0]`},
				{Name: "statusReadyReplicas", Expression: `([dyn(object.?status.?readyReplicas.orValue(null))].filter(v, v != null && v != false) + [0])[0]`},
				{Name: "specReplicasOne", Expression: `([dyn(object.?spec.?replicas.orValue(null))].filter(v, v != null && v != false) + [1])[0]`},
				{Name: "statusUpdatedReplicas", Expression: `([dyn(object.?status.?updatedReplicas.orValue(null))].filter(v, v != null && v != false) + [0])[0]`},
				{Name: "statusCurrentRevision", Expression: `([dyn(object.?status.?currentRevision.orValue(null))].filter(v, v != null && v != false) + [""])[0]`},
				{Name: "statusUpdateRevision", Expression: `([dyn(object.?status.?updateRevision.orValue(null))].filter(v, v != null && v != false) + [""])[0]`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "statefulset",
					Kind: &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
					ScaleDefinition: &v1alpha1.ScaleDefinition{
						Replicas: &v1alpha1.ValueAccessor{Expression: `variables.specReplicas`},
					},
					SpecDefinition: &v1alpha1.SpecDefinition{
						PodTemplateSpec: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"template"].orValue(null)`, Patch: `{"spec": {"template": value}}`, Replace: true},
					},
					StatusDefinition: &v1alpha1.StatusDefinition{
						StatusMappings: v1alpha1.StatusMappings{
							Running: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `variables.statusObservedGeneration == variables.metadataGeneration && variables.statusReadyReplicas == variables.specReplicasOne && variables.statusUpdatedReplicas == variables.specReplicasOne && variables.statusCurrentRevision == variables.statusUpdateRevision`,
								ExpectedResult: "true",
							}}},
							Degraded: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `variables.statusReadyReplicas > 0 && variables.statusReadyReplicas < variables.specReplicasOne && variables.statusUpdatedReplicas == variables.specReplicasOne && variables.statusCurrentRevision == variables.statusUpdateRevision && variables.statusObservedGeneration == variables.metadataGeneration`,
								ExpectedResult: "true",
							}}},
							Initializing: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     `variables.specReplicasOne > 0 && (variables.statusObservedGeneration != variables.metadataGeneration || variables.statusReadyReplicas == 0 || variables.statusReadyReplicas > variables.specReplicasOne || variables.statusUpdatedReplicas != variables.specReplicasOne || variables.statusCurrentRevision != variables.statusUpdateRevision)`,
								ExpectedResult: "true",
							}}},
						},
					},
				},
			},
		},
	}
}
