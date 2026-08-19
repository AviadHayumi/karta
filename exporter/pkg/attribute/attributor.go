// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package attribute

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"

	"github.com/run-ai/karta/pkg/instructions"
	"github.com/run-ai/karta/pkg/resource"

	"github.com/run-ai/karta/exporter/pkg/collector"
	"github.com/run-ai/karta/exporter/pkg/registry"
)

// Result is the pod-to-component attribution outcome. Reason is empty on a
// clean attribution; a failed step degrades its field to the sentinel and
// records why, so workload-level aggregation keeps working (the pod is never
// dropped).
type Result struct {
	Component string
	Instance  string
	Replica   string
	Reason    string
}

// Attribute resolves which component, component instance, and replica of a
// workload the given pod belongs to. All jq runs against the pod only: the
// workload side is the instanceIDs map (component name to declared instance
// ids), which the caller computes once per workload event, not per pod.
func Attribute(ctx context.Context, pod *corev1.Pod, entry *registry.Entry, instanceIDs map[string][]string) Result {
	querier := resource.NewPodQuerier(pod)

	componentName, err := instructions.InferPodComponent(ctx, querier, entry.Summary)
	if err != nil {
		return Result{
			Component: collector.SentinelUnknown,
			Instance:  collector.SentinelUnknown,
			Reason:    collector.ReasonJQError,
		}
	}

	result := Result{Component: componentName}
	definition := entry.Definitions[componentName]
	if definition == nil {
		result.Instance = collector.SentinelUnknown
		result.Reason = collector.ReasonUnknownInstance
		return result
	}

	ids := instanceIDs[componentName]
	if len(ids) > 0 && ids[0] != "" && definition.PodSelector != nil {
		instance, err := querier.GetMatchingInstanceId(ctx, definition.PodSelector.ComponentInstanceSelector, ids)
		switch {
		case err == nil:
			result.Instance = instance
		case errors.As(err, new(resource.InstanceNotFoundError)):
			result.Instance = collector.SentinelUnknown
			result.Reason = collector.ReasonUnknownInstance
		default:
			result.Instance = collector.SentinelUnknown
			result.Reason = collector.ReasonJQError
		}
	}

	replica, err := extractReplica(ctx, querier, entry, componentName)
	if err != nil && result.Reason == "" {
		result.Reason = collector.ReasonJQError
	}
	result.Replica = replica

	return result
}

// InstanceIDs lists the declared instance ids per pod-producing component,
// straight from the workload object. This is the once-per-workload-event
// half of attribution.
func InstanceIDs(ctx context.Context, entry *registry.Entry, workload resource.KubernetesObject) (map[string][]string, error) {
	factory := resource.NewComponentFactoryFromObject(entry.Karta, workload)

	components, err := factory.GetChildComponents()
	if err != nil {
		return nil, err
	}
	root, err := factory.GetRootComponent()
	if err != nil {
		return nil, err
	}
	components = append(components, root)

	instanceIDs := make(map[string][]string, len(components))
	for _, component := range components {
		if !component.HasPodDefinition() {
			continue
		}
		ids, err := component.GetInstanceIds(ctx)
		if err != nil {
			return nil, err
		}
		instanceIDs[component.Name()] = ids
	}
	return instanceIDs, nil
}

// extractReplica finds the nearest ReplicaSelector on the component or its
// ancestors (descendants inherit the replica context from the ancestor that
// defines it) and evaluates it against the pod.
func extractReplica(ctx context.Context, querier *resource.PodQuerier, entry *registry.Entry, componentName string) (string, error) {
	current := componentName
	for current != "" {
		definition := entry.Definitions[current]
		if definition == nil {
			return "", nil
		}

		if selector := definition.PodSelector; selector != nil && selector.ReplicaSelector != nil {
			replica, found, err := querier.ExtractReplicaKey(ctx, selector.ReplicaSelector)
			if err != nil {
				return "", err
			}
			if found {
				return replica, nil
			}
			return "", nil
		}

		if definition.OwnerRef == nil {
			return "", nil
		}
		current = *definition.OwnerRef
	}
	return "", nil
}
