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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"schedulerName"].orValue(null)`, PathWrite: ptr.To("/spec/components/standalone/schedulerName")},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"podLabels"].orValue(null)`, PathWrite: ptr.To("/spec/components/standalone/podLabels")},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"podAnnotations"].orValue(null)`, PathWrite: ptr.To("/spec/components/standalone/podAnnotations")},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/components/standalone/resources")},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"priorityClassName"].orValue(null)`, PathWrite: ptr.To("/spec/components/standalone/priorityClassName")},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"affinity"][?"nodeAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/standalone/affinity/nodeAffinity")},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"standalone"][?"affinity"][?"podAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/standalone/affinity/podAffinity")},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"schedulerName"].orValue(null)`, PathWrite: ptr.To("/spec/components/proxy/schedulerName")},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"podLabels"].orValue(null)`, PathWrite: ptr.To("/spec/components/proxy/podLabels")},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"podAnnotations"].orValue(null)`, PathWrite: ptr.To("/spec/components/proxy/podAnnotations")},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/components/proxy/resources")},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"priorityClassName"].orValue(null)`, PathWrite: ptr.To("/spec/components/proxy/priorityClassName")},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"affinity"][?"nodeAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/proxy/affinity/nodeAffinity")},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"proxy"][?"affinity"][?"podAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/proxy/affinity/podAffinity")},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"schedulerName"].orValue(null)`, PathWrite: ptr.To("/spec/components/mixCoord/schedulerName")},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"podLabels"].orValue(null)`, PathWrite: ptr.To("/spec/components/mixCoord/podLabels")},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"podAnnotations"].orValue(null)`, PathWrite: ptr.To("/spec/components/mixCoord/podAnnotations")},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/components/mixCoord/resources")},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"priorityClassName"].orValue(null)`, PathWrite: ptr.To("/spec/components/mixCoord/priorityClassName")},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/mixCoord/affinity/nodeAffinity")},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"mixCoord"][?"affinity"][?"podAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/mixCoord/affinity/podAffinity")},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"schedulerName"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataNode/schedulerName")},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"podLabels"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataNode/podLabels")},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"podAnnotations"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataNode/podAnnotations")},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataNode/resources")},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"priorityClassName"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataNode/priorityClassName")},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"affinity"][?"nodeAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataNode/affinity/nodeAffinity")},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataNode"][?"affinity"][?"podAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataNode/affinity/podAffinity")},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"schedulerName"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryNode/schedulerName")},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"podLabels"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryNode/podLabels")},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"podAnnotations"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryNode/podAnnotations")},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryNode/resources")},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"priorityClassName"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryNode/priorityClassName")},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"affinity"][?"nodeAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryNode/affinity/nodeAffinity")},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryNode"][?"affinity"][?"podAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryNode/affinity/podAffinity")},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"schedulerName"].orValue(null)`, PathWrite: ptr.To("/spec/components/streamingNode/schedulerName")},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"podLabels"].orValue(null)`, PathWrite: ptr.To("/spec/components/streamingNode/podLabels")},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"podAnnotations"].orValue(null)`, PathWrite: ptr.To("/spec/components/streamingNode/podAnnotations")},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/components/streamingNode/resources")},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"priorityClassName"].orValue(null)`, PathWrite: ptr.To("/spec/components/streamingNode/priorityClassName")},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"affinity"][?"nodeAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/streamingNode/affinity/nodeAffinity")},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"streamingNode"][?"affinity"][?"podAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/streamingNode/affinity/podAffinity")},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"schedulerName"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexNode/schedulerName")},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"podLabels"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexNode/podLabels")},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"podAnnotations"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexNode/podAnnotations")},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexNode/resources")},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"priorityClassName"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexNode/priorityClassName")},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"affinity"][?"nodeAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexNode/affinity/nodeAffinity")},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexNode"][?"affinity"][?"podAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexNode/affinity/podAffinity")},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"schedulerName"].orValue(null)`, PathWrite: ptr.To("/spec/components/rootCoord/schedulerName")},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"podLabels"].orValue(null)`, PathWrite: ptr.To("/spec/components/rootCoord/podLabels")},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"podAnnotations"].orValue(null)`, PathWrite: ptr.To("/spec/components/rootCoord/podAnnotations")},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/components/rootCoord/resources")},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"priorityClassName"].orValue(null)`, PathWrite: ptr.To("/spec/components/rootCoord/priorityClassName")},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/rootCoord/affinity/nodeAffinity")},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"rootCoord"][?"affinity"][?"podAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/rootCoord/affinity/podAffinity")},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"schedulerName"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataCoord/schedulerName")},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"podLabels"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataCoord/podLabels")},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"podAnnotations"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataCoord/podAnnotations")},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataCoord/resources")},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"priorityClassName"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataCoord/priorityClassName")},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataCoord/affinity/nodeAffinity")},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"dataCoord"][?"affinity"][?"podAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/dataCoord/affinity/podAffinity")},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"schedulerName"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryCoord/schedulerName")},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"podLabels"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryCoord/podLabels")},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"podAnnotations"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryCoord/podAnnotations")},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryCoord/resources")},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"priorityClassName"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryCoord/priorityClassName")},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryCoord/affinity/nodeAffinity")},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"queryCoord"][?"affinity"][?"podAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/queryCoord/affinity/podAffinity")},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"schedulerName"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexCoord/schedulerName")},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"podLabels"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexCoord/podLabels")},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"podAnnotations"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexCoord/podAnnotations")},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexCoord/resources")},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"priorityClassName"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexCoord/priorityClassName")},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"affinity"][?"nodeAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexCoord/affinity/nodeAffinity")},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"indexCoord"][?"affinity"][?"podAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/indexCoord/affinity/podAffinity")},
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
								SchedulerName:     &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"schedulerName"].orValue(null)`, PathWrite: ptr.To("/spec/components/cdc/schedulerName")},
								Labels:            &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"podLabels"].orValue(null)`, PathWrite: ptr.To("/spec/components/cdc/podLabels")},
								Annotations:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"podAnnotations"].orValue(null)`, PathWrite: ptr.To("/spec/components/cdc/podAnnotations")},
								Resources:         &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/components/cdc/resources")},
								PriorityClassName: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"priorityClassName"].orValue(null)`, PathWrite: ptr.To("/spec/components/cdc/priorityClassName")},
								NodeAffinity:      &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"affinity"][?"nodeAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/cdc/affinity/nodeAffinity")},
								PodAffinity:       &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"components"][?"cdc"][?"affinity"][?"podAffinity"].orValue(null)`, PathWrite: ptr.To("/spec/components/cdc/affinity/podAffinity")},
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
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"etcd"][?"inCluster"][?"values"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/dependencies/etcd/inCluster/values/resources")},
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
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"storage"][?"inCluster"][?"values"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/dependencies/storage/inCluster/values/resources")},
							},
						},
					},
					{
						Name:     "pulsar-zookeeper",
						Kind:     &v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
						OwnerRef: ptr.To("milvus"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"zookeeper"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/dependencies/pulsar/inCluster/values/zookeeper/resources")},
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
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"bookkeeper"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/dependencies/pulsar/inCluster/values/bookkeeper/resources")},
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
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"broker"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/dependencies/pulsar/inCluster/values/broker/resources")},
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
								Resources: &v1alpha1.ValueAccessor{Expression: `object[?"spec"][?"dependencies"][?"pulsar"][?"inCluster"][?"values"][?"proxy"][?"resources"].orValue(null)`, PathWrite: ptr.To("/spec/dependencies/pulsar/inCluster/values/proxy/resources")},
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
