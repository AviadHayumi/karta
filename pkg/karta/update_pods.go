// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package karta

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/jq/execution"
	"github.com/run-ai/karta/pkg/resource"
)

func (w *workload) UpdatePods(ctx context.Context, component string, patch PodPatch, opts ...UpdateOption) error {
	options := ResolveUpdateOptions(opts...)

	if len(patch.SetFields()) == 0 {
		return ErrEmptyPatch
	}
	if err := rejectEmptyScalars(patch); err != nil {
		return err
	}

	target, err := w.factory.GetComponent(component)
	if err != nil {
		return fmt.Errorf("karta: %w", err)
	}
	definition := target.Definition()
	if fields := unsupportedFields(patch, definition); len(fields) > 0 {
		return &UnsupportedFieldsError{Component: component, Fields: fields}
	}

	targeted, err := resolveTargets(ctx, target, definition, options)
	if err != nil {
		return err
	}

	if specShape(definition) == shapeFragmented {
		return w.withScratch(func(factory *resource.ComponentFactory) error {
			scratchComponent, err := factory.GetComponent(component)
			if err != nil {
				return fmt.Errorf("karta: %w", err)
			}
			return updateFragmented(ctx, scratchComponent, patch, targeted.set)
		})
	}

	// The leaf-write path mutates a raw copy through karta's own runner and
	// adopts a factory built from the result only on full success.
	current, err := w.factory.GetResource()
	if err != nil {
		return fmt.Errorf("karta: get object: %w", err)
	}
	raw, ok := current.DeepCopyObject().(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("karta: unsupported object type %T", current)
	}
	runner := execution.NewDefaultRunner(raw.Object)
	if err := updateLeaves(ctx, runner, definition, patch, targeted); err != nil {
		return err
	}
	mutated, err := runner.GetObject()
	if err != nil {
		return fmt.Errorf("karta: get mutated object: %w", err)
	}
	mutatedMap, ok := mutated.(map[string]any)
	if !ok {
		return fmt.Errorf("karta: mutated object is %T, not an object", mutated)
	}
	w.factory = resource.NewComponentFactoryFromObject(w.karta, &unstructured.Unstructured{Object: mutatedMap})
	return nil
}

// targets carries the WithInstances resolution: set is nil for "all".
type targets struct {
	set   map[string]bool
	order []string // instance ids in evaluation order, when restricted
}

// resolveTargets validates WithInstances: known ids, no duplicates, and the v1
// one-to-one iteration rule between instanceIdPath and every written path.
func resolveTargets(ctx context.Context, component *resource.Component, definition v1alpha1.ComponentDefinition, options UpdateOptions) (targets, error) {
	if len(options.Instances) == 0 {
		return targets{}, nil
	}
	ids, err := component.GetInstanceIds(ctx)
	if err != nil {
		return targets{}, fmt.Errorf("karta: get instance ids: %w", err)
	}
	known := make(map[string]bool, len(ids))
	for _, id := range ids {
		if known[id] {
			return targets{}, fmt.Errorf("karta: duplicate instance id %q on component %s", id, component.Name())
		}
		known[id] = true
	}
	set := make(map[string]bool, len(options.Instances))
	for _, id := range options.Instances {
		if !known[id] {
			return targets{}, fmt.Errorf("karta: unknown instance id %q for component %s", id, component.Name())
		}
		if set[id] {
			return targets{}, fmt.Errorf("karta: instance id %q requested twice", id)
		}
		set[id] = true
	}
	if definition.InstanceIdPath == nil {
		return targets{}, fmt.Errorf("karta: WithInstances requires an instanceIdPath on component %s: %w", component.Name(), ErrNotSupported)
	}
	idSegments, ok := parsePurePath(*definition.InstanceIdPath)
	if !ok || iterCount(idSegments) != 1 {
		return targets{}, fmt.Errorf("karta: WithInstances requires a pure single-iteration instanceIdPath on component %s: %w", component.Name(), ErrNotSupported)
	}
	base, ok := shapeBase(definition)
	if !ok || !sharesIterationBase(idSegments, base) {
		return targets{}, fmt.Errorf("karta: WithInstances requires the pod path and instanceIdPath to share one iteration base on component %s: %w", component.Name(), ErrNotSupported)
	}
	return targets{set: set, order: ids}, nil
}

// shapeBase returns the parsed base path of the component's pod location for
// the template / podSpec / split shapes.
func shapeBase(definition v1alpha1.ComponentDefinition) ([]pathSegment, bool) {
	spec := definition.SpecDefinition
	switch specShape(definition) {
	case shapeTemplate:
		return parsePurePath(*spec.PodTemplateSpecPath)
	case shapePodSpec, shapeSplit:
		return parsePurePath(*spec.PodSpecPath)
	default:
		return nil, false
	}
}

// leafWrite is one raw jq assignment: read the current values at the leaf,
// transform each (or keep, when the instance is untargeted), write back.
type leafWrite struct {
	path      string
	transform func(current any) (any, error)
}

// updateLeaves routes the patch as individual raw leaf assignments under the
// definition's base path. Only leaves named by the patch are ever written, so
// unknown sibling fields survive untouched - including definitions where the
// pod spec and metadata paths point at the same object.
func updateLeaves(ctx context.Context, runner execution.Runner, definition v1alpha1.ComponentDefinition, patch PodPatch, targeted targets) error {
	shape := specShape(definition)

	base, ok := shapeBase(definition)
	if !ok {
		return fmt.Errorf("karta: pod path is not a writable pure path: %w", ErrNotSupported)
	}
	// specPrefix locates PodSpec fields under the base.
	var specPrefix []pathSegment
	if shape == shapeTemplate {
		specPrefix = fieldSegments("spec")
	}

	var writes []leafWrite
	scalar := func(relative []pathSegment, value any) {
		writes = append(writes, leafWrite{
			path:      appendPath(base, relative...),
			transform: func(any) (any, error) { return value, nil },
		})
	}
	if patch.SchedulerName != nil {
		scalar(append(specPrefix, pathSegment{field: "schedulerName"}), *patch.SchedulerName)
	}
	if patch.PriorityClassName != nil {
		scalar(append(specPrefix, pathSegment{field: "priorityClassName"}), *patch.PriorityClassName)
	}
	if patch.NodeAffinity != nil {
		value, err := toRaw(patch.NodeAffinity)
		if err != nil {
			return err
		}
		scalar(append(specPrefix, fieldSegments("affinity", "nodeAffinity")...), value)
	}
	if patch.PodAffinity != nil {
		value, err := toRaw(patch.PodAffinity)
		if err != nil {
			return err
		}
		scalar(append(specPrefix, fieldSegments("affinity", "podAffinity")...), value)
	}
	if len(patch.ResourceClaims) > 0 {
		value, err := toRaw(patch.ResourceClaims)
		if err != nil {
			return err
		}
		scalar(append(specPrefix, pathSegment{field: "resourceClaims"}), value)
	}
	if len(patch.Labels) > 0 || len(patch.Annotations) > 0 {
		metaBase, metaPrefix, err := metadataLocation(definition, base)
		if err != nil {
			return err
		}
		if len(patch.Labels) > 0 {
			writes = append(writes, leafWrite{
				path:      appendPath(metaBase, append(metaPrefix, pathSegment{field: "labels"})...),
				transform: mergeRawMap(patch.Labels),
			})
		}
		if len(patch.Annotations) > 0 {
			writes = append(writes, leafWrite{
				path:      appendPath(metaBase, append(metaPrefix, pathSegment{field: "annotations"})...),
				transform: mergeRawMap(patch.Annotations),
			})
		}
	}
	if patch.Image != nil || patch.Resources != nil || len(patch.Containers) > 0 {
		writes = append(writes, leafWrite{
			path:      appendPath(base, append(specPrefix, pathSegment{field: "containers"})...),
			transform: mergeRawContainers(patch),
		})
	}

	for _, write := range writes {
		if err := applyLeaf(ctx, runner, write, targeted); err != nil {
			return err
		}
	}
	return nil
}

// metadataLocation resolves where pod metadata lives: inside the template, or
// at the split shape's metadataPath (which may equal the pod spec path).
func metadataLocation(definition v1alpha1.ComponentDefinition, base []pathSegment) ([]pathSegment, []pathSegment, error) {
	switch specShape(definition) {
	case shapeTemplate:
		return base, fieldSegments("metadata"), nil
	case shapeSplit:
		metaBase, ok := parsePurePath(*definition.SpecDefinition.MetadataPath)
		if !ok {
			return nil, nil, fmt.Errorf("karta: metadata path is not a writable pure path: %w", ErrNotSupported)
		}
		return metaBase, nil, nil
	default:
		return nil, nil, &UnsupportedFieldsError{Component: definition.Name, Fields: []PodField{PodFieldLabels, PodFieldAnnotations}}
	}
}

// applyLeaf reads the leaf's current values, transforms the targeted ones and
// assigns them back in the same evaluation order.
func applyLeaf(ctx context.Context, runner execution.Runner, write leafWrite, targeted targets) error {
	current, err := runner.Evaluate(ctx, write.path)
	if err != nil {
		return fmt.Errorf("karta: read %s: %w", write.path, err)
	}
	if len(current) == 0 {
		return fmt.Errorf("karta: path %s matched nothing on the object", write.path)
	}
	if targeted.set != nil && len(current) != len(targeted.order) {
		return fmt.Errorf("karta: %s matched %d locations but the component has %d instances: %w",
			write.path, len(current), len(targeted.order), ErrNotSupported)
	}
	values := make([]any, len(current))
	for i, value := range current {
		if targeted.set != nil && !targeted.set[targeted.order[i]] {
			values[i] = value
			continue
		}
		transformed, err := write.transform(value)
		if err != nil {
			return err
		}
		values[i] = transformed
	}
	if err := runner.AssignZip(ctx, write.path, values); err != nil {
		return fmt.Errorf("karta: write %s: %w", write.path, err)
	}
	return nil
}

// mergeRawMap merges string entries into the current raw map, creating it when
// absent. Untouched keys survive verbatim.
func mergeRawMap(entries map[string]string) func(any) (any, error) {
	return func(current any) (any, error) {
		merged := map[string]any{}
		if existing, ok := current.(map[string]any); ok {
			for key, value := range existing {
				merged[key] = value
			}
		} else if current != nil {
			return nil, fmt.Errorf("karta: existing value is %T, not an object", current)
		}
		for key, value := range entries {
			merged[key] = value
		}
		return merged, nil
	}
}

// mergeRawContainers applies Image/Resources/Containers onto the raw container
// list, touching only the keys the patch sets on each targeted entry.
func mergeRawContainers(patch PodPatch) func(any) (any, error) {
	return func(current any) (any, error) {
		list, ok := current.([]any)
		if !ok {
			return nil, fmt.Errorf("karta: containers value is %T, not a list", current)
		}
		containers := make([]any, len(list))
		names := make(map[string]int, len(list))
		for i, entry := range list {
			raw, ok := entry.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("karta: container %d is %T, not an object", i, entry)
			}
			copied := make(map[string]any, len(raw))
			for key, value := range raw {
				copied[key] = value
			}
			containers[i] = copied
			if name, _ := raw["name"].(string); name != "" {
				names[name] = i
			}
		}
		if patch.Image != nil || patch.Resources != nil {
			if len(containers) != 1 {
				return nil, fmt.Errorf("karta: image/resources target a single container, pod has %d; use Containers", len(containers))
			}
			target := containers[0].(map[string]any)
			if patch.Image != nil {
				target["image"] = *patch.Image
			}
			if patch.Resources != nil {
				value, err := toRaw(patch.Resources)
				if err != nil {
					return nil, err
				}
				target["resources"] = value
			}
		}
		for _, entry := range patch.Containers {
			index, ok := names[entry.Name]
			if !ok {
				return nil, fmt.Errorf("karta: container %q not found in pod", entry.Name)
			}
			target := containers[index].(map[string]any)
			if entry.Image != nil {
				target["image"] = *entry.Image
			}
			if entry.Resources != nil {
				value, err := toRaw(entry.Resources)
				if err != nil {
					return nil, err
				}
				target["resources"] = value
			}
		}
		return containers, nil
	}
}

// toRaw converts a typed intent value into raw JSON shape. The value is
// consumer-provided intent, so the round-trip loses nothing of the object.
func toRaw(value any) (any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("karta: encode patch value: %w", err)
	}
	var raw any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return nil, fmt.Errorf("karta: decode patch value: %w", err)
	}
	return raw, nil
}

// rejectEmptyScalars enforces the v1 rule: explicit empty scalar values are
// rejected uniformly instead of behaving differently per shape.
func rejectEmptyScalars(patch PodPatch) error {
	if patch.SchedulerName != nil && *patch.SchedulerName == "" {
		return fmt.Errorf("karta: empty schedulerName: clearing values is not supported")
	}
	if patch.PriorityClassName != nil && *patch.PriorityClassName == "" {
		return fmt.Errorf("karta: empty priorityClassName: clearing values is not supported")
	}
	if patch.Image != nil && *patch.Image == "" {
		return fmt.Errorf("karta: empty image: clearing values is not supported")
	}
	for _, entry := range patch.Containers {
		if entry.Name == "" {
			return fmt.Errorf("karta: container patch without a name")
		}
		if entry.Image != nil && *entry.Image == "" {
			return fmt.Errorf("karta: empty image for container %q: clearing values is not supported", entry.Name)
		}
	}
	return nil
}

func updateFragmented(ctx context.Context, component *resource.Component, patch PodPatch, targeted map[string]bool) error {
	existing, err := component.GetFragmentedPodSpec(ctx)
	if err != nil {
		return fmt.Errorf("karta: get fragmented pod specs: %w", err)
	}
	updates := make(map[string]resource.FragmentedPodSpec, len(existing))
	for id, current := range existing {
		write, err := fragmentedWrite(current, patch, targeted == nil || targeted[id])
		if err != nil {
			return err
		}
		updates[id] = write
	}
	if err := component.UpdateFragmentedPodSpec(ctx, updates); err != nil {
		return fmt.Errorf("karta: update fragmented pod specs: %w", err)
	}
	return nil
}

// fragmentedWrite builds the FragmentedPodSpec written for one instance. It
// carries ONLY the fields the patch sets, so the accessor's all-empty skip
// leaves every untouched path unassigned. containerPath is read-only in v1,
// so Image/Resources/Containers route only through their own paths - the
// capability check guarantees those paths exist and are pure.
func fragmentedWrite(current resource.FragmentedPodSpec, patch PodPatch, targeted bool) (resource.FragmentedPodSpec, error) {
	var write resource.FragmentedPodSpec

	if patch.SchedulerName != nil {
		write.SchedulerName = current.SchedulerName
		if targeted {
			write.SchedulerName = *patch.SchedulerName
		}
	}
	if patch.PriorityClassName != nil {
		write.PriorityClassName = current.PriorityClassName
		if targeted {
			write.PriorityClassName = *patch.PriorityClassName
		}
	}
	if len(patch.Labels) > 0 {
		write.Labels = mergedStringMap(current.Labels, patch.Labels, targeted)
	}
	if len(patch.Annotations) > 0 {
		write.Annotations = mergedStringMap(current.Annotations, patch.Annotations, targeted)
	}
	if patch.NodeAffinity != nil {
		write.NodeAffinity = current.NodeAffinity
		if targeted {
			write.NodeAffinity = patch.NodeAffinity.DeepCopy()
		}
	}
	if patch.PodAffinity != nil {
		write.PodAffinity = current.PodAffinity
		if targeted {
			write.PodAffinity = patch.PodAffinity.DeepCopy()
		}
	}
	if len(patch.ResourceClaims) > 0 {
		write.ResourceClaims = current.ResourceClaims
		if targeted {
			write.ResourceClaims = append(write.ResourceClaims[:0:0], patch.ResourceClaims...)
		}
	}
	if patch.Image != nil {
		write.Image = current.Image
		if targeted {
			write.Image = *patch.Image
		}
	}
	if patch.Resources != nil {
		write.Resources = current.Resources
		if targeted {
			write.Resources = patch.Resources.DeepCopy()
		}
	}
	if len(patch.Containers) > 0 {
		containers := append(current.Containers[:0:0], current.Containers...)
		if targeted {
			if err := mergeContainersByName(containers, patch.Containers); err != nil {
				return write, err
			}
		}
		write.Containers = containers
	}
	return write, nil
}

func mergedStringMap(current, entries map[string]string, targeted bool) map[string]string {
	merged := make(map[string]string, len(current)+len(entries))
	for key, value := range current {
		merged[key] = value
	}
	if targeted {
		for key, value := range entries {
			merged[key] = value
		}
	}
	return merged
}
