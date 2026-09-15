// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Milvus returns the built-in Karta for the Milvus workload
// (milvus.io/v1beta1).
func Milvus() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "milvus-io-milvus-v1beta1"},
		Spec: v1alpha1.KartaSpec{
			Variables: []v1alpha1.Variable{
				{Name: "statusStatus", Expression: `([dyn(object.?status.?status.orValue(null))].filter(v, v != null && v != false) + [""])[0]`},
			},
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "milvus",
					Kind: &v1alpha1.GroupVersionKind{Group: "milvus.io", Version: "v1beta1", Kind: "Milvus"},
					StatusDefinition: &v1alpha1.StatusDefinition{
						PhaseDefinition: &v1alpha1.PhaseDefinition{
							// .status.status is an enum string: Healthy, Pending, Unhealthy
							Expression: `object[?"status"][?"status"].orValue(null)`,
						},
						ConditionsDefinition: &v1alpha1.ConditionsDefinition{
							Expression:      `object[?"status"][?"conditions"].orValue(null)`,
							TypeFieldName:   "type",
							StatusFieldName: "status",
						},
						StatusMappings: v1alpha1.StatusMappings{
							Running: []v1alpha1.StatusMatcher{
								{ByPhase: "Healthy"},
							},
							Initializing: []v1alpha1.StatusMatcher{
								{ByPhase: "Pending"},
								// Just created: the operator has not written status.status yet.
								{ByExpression: &v1alpha1.ExpressionMatcher{
									Expression:     `variables.statusStatus == ""`,
									ExpectedResult: "true",
								}},
							},
							Degraded: []v1alpha1.StatusMatcher{
								{ByPhase: "Unhealthy"},
							},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "standalone",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"schedulerName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"standalone": {"schedulerName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"podLabels"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"standalone": {"podLabels": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"podAnnotations"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"standalone": {"podAnnotations": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"standalone": {"resources": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"priorityClassName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"standalone": {"priorityClassName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"standalone": {"affinity": {"nodeAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"affinity"][?"podAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"standalone": {"affinity": {"podAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"replicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"app.kubernetes.io/component"].orValue(null)`,
								Value:      ptr.To("standalone"),
							},
						},
					},
					{
						Name:     "proxy",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"schedulerName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"proxy": {"schedulerName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"podLabels"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"proxy": {"podLabels": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"podAnnotations"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"proxy": {"podAnnotations": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"proxy": {"resources": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"priorityClassName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"proxy": {"priorityClassName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"proxy": {"affinity": {"nodeAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"affinity"][?"podAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"proxy": {"affinity": {"podAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"replicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"app.kubernetes.io/component"].orValue(null)`,
								Value:      ptr.To("proxy"),
							},
						},
					},
					{
						Name:     "mixcoord",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"schedulerName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"mixCoord": {"schedulerName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"podLabels"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"mixCoord": {"podLabels": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"podAnnotations"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"mixCoord": {"podAnnotations": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"mixCoord": {"resources": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"priorityClassName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"mixCoord": {"priorityClassName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"mixCoord": {"affinity": {"nodeAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"affinity"][?"podAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"mixCoord": {"affinity": {"podAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"replicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"app.kubernetes.io/component"].orValue(null)`,
								Value:      ptr.To("mixcoord"),
							},
						},
					},
					{
						Name:     "datanode",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"schedulerName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataNode": {"schedulerName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"podLabels"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataNode": {"podLabels": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"podAnnotations"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataNode": {"podAnnotations": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataNode": {"resources": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"priorityClassName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataNode": {"priorityClassName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataNode": {"affinity": {"nodeAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"affinity"][?"podAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataNode": {"affinity": {"podAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"replicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"app.kubernetes.io/component"].orValue(null)`,
								Value:      ptr.To("datanode"),
							},
						},
					},
					{
						Name:     "querynode",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"schedulerName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryNode": {"schedulerName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"podLabels"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryNode": {"podLabels": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"podAnnotations"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryNode": {"podAnnotations": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryNode": {"resources": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"priorityClassName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryNode": {"priorityClassName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryNode": {"affinity": {"nodeAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"affinity"][?"podAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryNode": {"affinity": {"podAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"replicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"app.kubernetes.io/component"].orValue(null)`,
								Value:      ptr.To("querynode"),
							},
						},
					},
					{
						Name:     "streamingnode",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"schedulerName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"streamingNode": {"schedulerName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"podLabels"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"streamingNode": {"podLabels": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"podAnnotations"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"streamingNode": {"podAnnotations": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"streamingNode": {"resources": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"priorityClassName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"streamingNode": {"priorityClassName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"streamingNode": {"affinity": {"nodeAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"affinity"][?"podAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"streamingNode": {"affinity": {"podAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"replicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"app.kubernetes.io/component"].orValue(null)`,
								Value:      ptr.To("streamingnode"),
							},
						},
					},
					{
						Name:     "indexnode",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"schedulerName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexNode": {"schedulerName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"podLabels"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexNode": {"podLabels": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"podAnnotations"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexNode": {"podAnnotations": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexNode": {"resources": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"priorityClassName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexNode": {"priorityClassName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexNode": {"affinity": {"nodeAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"affinity"][?"podAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexNode": {"affinity": {"podAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"replicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"app.kubernetes.io/component"].orValue(null)`,
								Value:      ptr.To("indexnode"),
							},
						},
					},
					{
						Name:     "rootcoord",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"schedulerName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"rootCoord": {"schedulerName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"podLabels"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"rootCoord": {"podLabels": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"podAnnotations"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"rootCoord": {"podAnnotations": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"rootCoord": {"resources": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"priorityClassName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"rootCoord": {"priorityClassName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"rootCoord": {"affinity": {"nodeAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"affinity"][?"podAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"rootCoord": {"affinity": {"podAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"replicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"app.kubernetes.io/component"].orValue(null)`,
								Value:      ptr.To("rootcoord"),
							},
						},
					},
					{
						Name:     "datacoord",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"schedulerName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataCoord": {"schedulerName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"podLabels"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataCoord": {"podLabels": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"podAnnotations"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataCoord": {"podAnnotations": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataCoord": {"resources": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"priorityClassName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataCoord": {"priorityClassName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataCoord": {"affinity": {"nodeAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"affinity"][?"podAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"dataCoord": {"affinity": {"podAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"replicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"app.kubernetes.io/component"].orValue(null)`,
								Value:      ptr.To("datacoord"),
							},
						},
					},
					{
						Name:     "querycoord",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"schedulerName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryCoord": {"schedulerName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"podLabels"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryCoord": {"podLabels": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"podAnnotations"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryCoord": {"podAnnotations": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryCoord": {"resources": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"priorityClassName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryCoord": {"priorityClassName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryCoord": {"affinity": {"nodeAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"affinity"][?"podAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"queryCoord": {"affinity": {"podAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"replicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"app.kubernetes.io/component"].orValue(null)`,
								Value:      ptr.To("querycoord"),
							},
						},
					},
					{
						Name:     "indexcoord",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"schedulerName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexCoord": {"schedulerName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"podLabels"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexCoord": {"podLabels": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"podAnnotations"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexCoord": {"podAnnotations": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexCoord": {"resources": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"priorityClassName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexCoord": {"priorityClassName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexCoord": {"affinity": {"nodeAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"affinity"][?"podAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"indexCoord": {"affinity": {"podAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"replicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"app.kubernetes.io/component"].orValue(null)`,
								Value:      ptr.To("indexcoord"),
							},
						},
					},
					{
						Name:     "cdc",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"schedulerName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"cdc": {"schedulerName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"podLabels"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"cdc": {"podLabels": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"podAnnotations"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"cdc": {"podAnnotations": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"cdc": {"resources": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"priorityClassName"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"cdc": {"priorityClassName": value}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"cdc": {"affinity": {"nodeAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"affinity"][?"podAffinity"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"components": {"cdc": {"affinity": {"podAffinity": value}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"replicas"].orValue(null)`},
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								Expression: `object[?"metadata"][?"labels"][?"app.kubernetes.io/component"].orValue(null)`,
								Value:      ptr.To("cdc"),
							},
						},
					},
					{
						Name:     "etcd",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"etcd"][?"inCluster"][?"values"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"dependencies": {"etcd": {"inCluster": {"values": {"resources": value}}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"etcd"][?"inCluster"][?"values"][?"replicaCount"].orValue(null)`},
						},
					},
					{
						Name:     "minio",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"storage"][?"inCluster"][?"values"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"dependencies": {"storage": {"inCluster": {"values": {"resources": value}}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
					},
					{
						Name:     "pulsar-zookeeper",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"zookeeper"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"dependencies": {"pulsar": {"inCluster": {"values": {"zookeeper": {"resources": value}}}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"zookeeper"][?"replicaCount"].orValue(null)`},
						},
					},
					{
						Name:     "pulsar-bookkeeper",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"bookkeeper"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"dependencies": {"pulsar": {"inCluster": {"values": {"bookkeeper": {"resources": value}}}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"bookkeeper"][?"replicaCount"].orValue(null)`},
						},
					},
					{
						Name:     "pulsar-broker",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"broker"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"dependencies": {"pulsar": {"inCluster": {"values": {"broker": {"resources": value}}}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"broker"][?"replicaCount"].orValue(null)`},
						},
					},
					{
						Name:     "pulsar-proxy",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"proxy"][?"resources"].orValue(null)`, Patches: []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"dependencies": {"pulsar": {"inCluster": {"values": {"proxy": {"resources": value}}}}}}}`}}, PatchStrategy: v1alpha1.PatchStrategyReplace},
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							Replicas: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"proxy"][?"replicaCount"].orValue(null)`},
						},
					},
				},
			},
			Instructions: v1alpha1.OptimizationInstructions{
				GangScheduling: &v1alpha1.GangSchedulingInstruction{
					PodGroups: []v1alpha1.PodGroupDefinition{{
						Name: "cluster",
						Members: []v1alpha1.PodGroupMemberDefinition{
							{
								ComponentName:      "querynode",
								GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"app.kubernetes.io/instance"].orValue(null)`},
							},
							{
								ComponentName:      "datanode",
								GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"app.kubernetes.io/instance"].orValue(null)`},
							},
							{
								ComponentName:      "streamingnode",
								GroupByExpressions: []string{`object[?"metadata"][?"labels"][?"app.kubernetes.io/instance"].orValue(null)`},
							},
						},
					}},
				},
			},
		},
	}
}
