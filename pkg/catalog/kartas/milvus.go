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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"components": {"standalone": {"schedulerName": value}}}}`, Replace: true},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"podLabels"].orValue(null)`, Patch: `{"spec": {"components": {"standalone": {"podLabels": value}}}}`, Replace: true},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"podAnnotations"].orValue(null)`, Patch: `{"spec": {"components": {"standalone": {"podAnnotations": value}}}}`, Replace: true},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"resources"].orValue(null)`, Patch: `{"spec": {"components": {"standalone": {"resources": value}}}}`, Replace: true},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"priorityClassName"].orValue(null)`, Patch: `{"spec": {"components": {"standalone": {"priorityClassName": value}}}}`, Replace: true},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"standalone": {"affinity": {"nodeAffinity": value}}}}}`, Replace: true},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"standalone": {"affinity": {"podAffinity": value}}}}}`, Replace: true},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"components": {"proxy": {"schedulerName": value}}}}`, Replace: true},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"podLabels"].orValue(null)`, Patch: `{"spec": {"components": {"proxy": {"podLabels": value}}}}`, Replace: true},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"podAnnotations"].orValue(null)`, Patch: `{"spec": {"components": {"proxy": {"podAnnotations": value}}}}`, Replace: true},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"resources"].orValue(null)`, Patch: `{"spec": {"components": {"proxy": {"resources": value}}}}`, Replace: true},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"priorityClassName"].orValue(null)`, Patch: `{"spec": {"components": {"proxy": {"priorityClassName": value}}}}`, Replace: true},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"proxy": {"affinity": {"nodeAffinity": value}}}}}`, Replace: true},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"proxy": {"affinity": {"podAffinity": value}}}}}`, Replace: true},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"components": {"mixCoord": {"schedulerName": value}}}}`, Replace: true},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"podLabels"].orValue(null)`, Patch: `{"spec": {"components": {"mixCoord": {"podLabels": value}}}}`, Replace: true},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"podAnnotations"].orValue(null)`, Patch: `{"spec": {"components": {"mixCoord": {"podAnnotations": value}}}}`, Replace: true},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"resources"].orValue(null)`, Patch: `{"spec": {"components": {"mixCoord": {"resources": value}}}}`, Replace: true},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"priorityClassName"].orValue(null)`, Patch: `{"spec": {"components": {"mixCoord": {"priorityClassName": value}}}}`, Replace: true},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"mixCoord": {"affinity": {"nodeAffinity": value}}}}}`, Replace: true},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"mixCoord": {"affinity": {"podAffinity": value}}}}}`, Replace: true},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"components": {"dataNode": {"schedulerName": value}}}}`, Replace: true},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"podLabels"].orValue(null)`, Patch: `{"spec": {"components": {"dataNode": {"podLabels": value}}}}`, Replace: true},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"podAnnotations"].orValue(null)`, Patch: `{"spec": {"components": {"dataNode": {"podAnnotations": value}}}}`, Replace: true},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"resources"].orValue(null)`, Patch: `{"spec": {"components": {"dataNode": {"resources": value}}}}`, Replace: true},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"priorityClassName"].orValue(null)`, Patch: `{"spec": {"components": {"dataNode": {"priorityClassName": value}}}}`, Replace: true},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"dataNode": {"affinity": {"nodeAffinity": value}}}}}`, Replace: true},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"dataNode": {"affinity": {"podAffinity": value}}}}}`, Replace: true},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"components": {"queryNode": {"schedulerName": value}}}}`, Replace: true},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"podLabels"].orValue(null)`, Patch: `{"spec": {"components": {"queryNode": {"podLabels": value}}}}`, Replace: true},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"podAnnotations"].orValue(null)`, Patch: `{"spec": {"components": {"queryNode": {"podAnnotations": value}}}}`, Replace: true},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"resources"].orValue(null)`, Patch: `{"spec": {"components": {"queryNode": {"resources": value}}}}`, Replace: true},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"priorityClassName"].orValue(null)`, Patch: `{"spec": {"components": {"queryNode": {"priorityClassName": value}}}}`, Replace: true},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"queryNode": {"affinity": {"nodeAffinity": value}}}}}`, Replace: true},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"queryNode": {"affinity": {"podAffinity": value}}}}}`, Replace: true},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"components": {"streamingNode": {"schedulerName": value}}}}`, Replace: true},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"podLabels"].orValue(null)`, Patch: `{"spec": {"components": {"streamingNode": {"podLabels": value}}}}`, Replace: true},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"podAnnotations"].orValue(null)`, Patch: `{"spec": {"components": {"streamingNode": {"podAnnotations": value}}}}`, Replace: true},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"resources"].orValue(null)`, Patch: `{"spec": {"components": {"streamingNode": {"resources": value}}}}`, Replace: true},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"priorityClassName"].orValue(null)`, Patch: `{"spec": {"components": {"streamingNode": {"priorityClassName": value}}}}`, Replace: true},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"streamingNode": {"affinity": {"nodeAffinity": value}}}}}`, Replace: true},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"streamingNode": {"affinity": {"podAffinity": value}}}}}`, Replace: true},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"components": {"indexNode": {"schedulerName": value}}}}`, Replace: true},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"podLabels"].orValue(null)`, Patch: `{"spec": {"components": {"indexNode": {"podLabels": value}}}}`, Replace: true},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"podAnnotations"].orValue(null)`, Patch: `{"spec": {"components": {"indexNode": {"podAnnotations": value}}}}`, Replace: true},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"resources"].orValue(null)`, Patch: `{"spec": {"components": {"indexNode": {"resources": value}}}}`, Replace: true},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"priorityClassName"].orValue(null)`, Patch: `{"spec": {"components": {"indexNode": {"priorityClassName": value}}}}`, Replace: true},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"indexNode": {"affinity": {"nodeAffinity": value}}}}}`, Replace: true},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"indexNode": {"affinity": {"podAffinity": value}}}}}`, Replace: true},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"components": {"rootCoord": {"schedulerName": value}}}}`, Replace: true},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"podLabels"].orValue(null)`, Patch: `{"spec": {"components": {"rootCoord": {"podLabels": value}}}}`, Replace: true},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"podAnnotations"].orValue(null)`, Patch: `{"spec": {"components": {"rootCoord": {"podAnnotations": value}}}}`, Replace: true},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"resources"].orValue(null)`, Patch: `{"spec": {"components": {"rootCoord": {"resources": value}}}}`, Replace: true},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"priorityClassName"].orValue(null)`, Patch: `{"spec": {"components": {"rootCoord": {"priorityClassName": value}}}}`, Replace: true},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"rootCoord": {"affinity": {"nodeAffinity": value}}}}}`, Replace: true},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"rootCoord": {"affinity": {"podAffinity": value}}}}}`, Replace: true},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"components": {"dataCoord": {"schedulerName": value}}}}`, Replace: true},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"podLabels"].orValue(null)`, Patch: `{"spec": {"components": {"dataCoord": {"podLabels": value}}}}`, Replace: true},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"podAnnotations"].orValue(null)`, Patch: `{"spec": {"components": {"dataCoord": {"podAnnotations": value}}}}`, Replace: true},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"resources"].orValue(null)`, Patch: `{"spec": {"components": {"dataCoord": {"resources": value}}}}`, Replace: true},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"priorityClassName"].orValue(null)`, Patch: `{"spec": {"components": {"dataCoord": {"priorityClassName": value}}}}`, Replace: true},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"dataCoord": {"affinity": {"nodeAffinity": value}}}}}`, Replace: true},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"dataCoord": {"affinity": {"podAffinity": value}}}}}`, Replace: true},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"components": {"queryCoord": {"schedulerName": value}}}}`, Replace: true},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"podLabels"].orValue(null)`, Patch: `{"spec": {"components": {"queryCoord": {"podLabels": value}}}}`, Replace: true},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"podAnnotations"].orValue(null)`, Patch: `{"spec": {"components": {"queryCoord": {"podAnnotations": value}}}}`, Replace: true},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"resources"].orValue(null)`, Patch: `{"spec": {"components": {"queryCoord": {"resources": value}}}}`, Replace: true},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"priorityClassName"].orValue(null)`, Patch: `{"spec": {"components": {"queryCoord": {"priorityClassName": value}}}}`, Replace: true},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"queryCoord": {"affinity": {"nodeAffinity": value}}}}}`, Replace: true},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"queryCoord": {"affinity": {"podAffinity": value}}}}}`, Replace: true},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"components": {"indexCoord": {"schedulerName": value}}}}`, Replace: true},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"podLabels"].orValue(null)`, Patch: `{"spec": {"components": {"indexCoord": {"podLabels": value}}}}`, Replace: true},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"podAnnotations"].orValue(null)`, Patch: `{"spec": {"components": {"indexCoord": {"podAnnotations": value}}}}`, Replace: true},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"resources"].orValue(null)`, Patch: `{"spec": {"components": {"indexCoord": {"resources": value}}}}`, Replace: true},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"priorityClassName"].orValue(null)`, Patch: `{"spec": {"components": {"indexCoord": {"priorityClassName": value}}}}`, Replace: true},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"indexCoord": {"affinity": {"nodeAffinity": value}}}}}`, Replace: true},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"indexCoord": {"affinity": {"podAffinity": value}}}}}`, Replace: true},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"schedulerName"].orValue(null)`, Patch: `{"spec": {"components": {"cdc": {"schedulerName": value}}}}`, Replace: true},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"podLabels"].orValue(null)`, Patch: `{"spec": {"components": {"cdc": {"podLabels": value}}}}`, Replace: true},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"podAnnotations"].orValue(null)`, Patch: `{"spec": {"components": {"cdc": {"podAnnotations": value}}}}`, Replace: true},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"resources"].orValue(null)`, Patch: `{"spec": {"components": {"cdc": {"resources": value}}}}`, Replace: true},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"priorityClassName"].orValue(null)`, Patch: `{"spec": {"components": {"cdc": {"priorityClassName": value}}}}`, Replace: true},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"affinity"][?"nodeAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"cdc": {"affinity": {"nodeAffinity": value}}}}}`, Replace: true},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"affinity"][?"podAffinity"].orValue(null)`, Patch: `{"spec": {"components": {"cdc": {"affinity": {"podAffinity": value}}}}}`, Replace: true},
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
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"etcd"][?"inCluster"][?"values"][?"resources"].orValue(null)`, Patch: `{"spec": {"dependencies": {"etcd": {"inCluster": {"values": {"resources": value}}}}}}`, Replace: true},
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
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"storage"][?"inCluster"][?"values"][?"resources"].orValue(null)`, Patch: `{"spec": {"dependencies": {"storage": {"inCluster": {"values": {"resources": value}}}}}}`, Replace: true},
							},
						},
					},
					{
						Name:     "pulsar-zookeeper",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"zookeeper"][?"resources"].orValue(null)`, Patch: `{"spec": {"dependencies": {"pulsar": {"inCluster": {"values": {"zookeeper": {"resources": value}}}}}}}`, Replace: true},
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
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"bookkeeper"][?"resources"].orValue(null)`, Patch: `{"spec": {"dependencies": {"pulsar": {"inCluster": {"values": {"bookkeeper": {"resources": value}}}}}}}`, Replace: true},
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
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"broker"][?"resources"].orValue(null)`, Patch: `{"spec": {"dependencies": {"pulsar": {"inCluster": {"values": {"broker": {"resources": value}}}}}}}`, Replace: true},
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
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"proxy"][?"resources"].orValue(null)`, Patch: `{"spec": {"dependencies": {"pulsar": {"inCluster": {"values": {"proxy": {"resources": value}}}}}}}`, Replace: true},
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
