// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/utils/ptr"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/expression"
)

// DefinitionNotFoundError represents an error when a requested definition is not found
type DefinitionNotFoundError string

func (e DefinitionNotFoundError) Error() string {
	return string(e)
}

// Accessor implements extraction and updating of resource data through the engine contract.
type Accessor struct {
	runner expression.Runner
}

func NewAccessor(runner expression.Runner) *Accessor {
	return &Accessor{runner: runner}
}

// GetObject returns the object as a map[string]interface{}
func (a *Accessor) GetObject() (map[string]interface{}, error) {
	object, err := a.runner.GetObject()
	if err != nil {
		return nil, err
	}
	objectMap, ok := object.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("object is not a map[string]interface{}")
	}
	return objectMap, nil
}

func (a *Accessor) ExtractPodTemplateSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodTemplateSpec, error) {
	if definition.SpecDefinition == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have spec definition", definition.Name))
	}

	if definition.SpecDefinition.PodTemplateSpec == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have pod template spec definition", definition.Name))
	}

	var podTemplateSpec []corev1.PodTemplateSpec
	err := extractVia(ctx, definition.SpecDefinition.PodTemplateSpec, instancedComponent(definition), a.runner, &podTemplateSpec)

	return podTemplateSpec, err
}

func (a *Accessor) ExtractPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodSpec, error) {
	if definition.SpecDefinition == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have spec definition", definition.Name))
	}

	if definition.SpecDefinition.PodSpec == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have pod spec definition", definition.Name))
	}

	var podSpec []corev1.PodSpec
	err := extractVia(ctx, definition.SpecDefinition.PodSpec, instancedComponent(definition), a.runner, &podSpec)

	return podSpec, err
}

func (a *Accessor) ExtractPodMetadata(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]metav1.ObjectMeta, error) {
	if definition.SpecDefinition == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have spec definition", definition.Name))
	}

	if definition.SpecDefinition.Metadata == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have pod metadata definition", definition.Name))
	}

	var podMetadata []metav1.ObjectMeta
	err := extractVia(ctx, definition.SpecDefinition.Metadata, instancedComponent(definition), a.runner, &podMetadata)

	return podMetadata, err
}

func (a *Accessor) ExtractScale(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]Scale, error) {
	if definition.ScaleDefinition == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have scale definition", definition.Name))
	}

	var (
		replicas    []*int32
		minReplicas []*int32
		maxReplicas []*int32
	)

	scaleCount := 0

	if err := extractVia(ctx, definition.ScaleDefinition.Replicas, instancedComponent(definition), a.runner, &replicas); err != nil {
		return nil, err
	}
	scaleCount = max(scaleCount, len(replicas))

	if err := extractVia(ctx, definition.ScaleDefinition.MinReplicas, instancedComponent(definition), a.runner, &minReplicas); err != nil {
		return nil, err
	}
	scaleCount = max(scaleCount, len(minReplicas))

	if err := extractVia(ctx, definition.ScaleDefinition.MaxReplicas, instancedComponent(definition), a.runner, &maxReplicas); err != nil {
		return nil, err
	}
	scaleCount = max(scaleCount, len(maxReplicas))

	scales := make([]Scale, scaleCount)
	for i := 0; i < scaleCount; i++ {
		scales[i] = Scale{
			Replicas:    safeGetByIndex(replicas, i),
			MaxReplicas: safeGetByIndex(maxReplicas, i),
			MinReplicas: safeGetByIndex(minReplicas, i),
		}
	}

	return scales, nil
}

func (a *Accessor) ExtractFragmentedPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]FragmentedPodSpec, error) {
	if definition.SpecDefinition == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have spec definition", definition.Name))
	}

	if definition.SpecDefinition.FragmentedPodSpecDefinition == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have fragmented pod spec definition", definition.Name))
	}

	fragmentedDefinition := definition.SpecDefinition.FragmentedPodSpecDefinition

	var (
		schedulerNameResults     []string
		labelsResults            []map[string]string
		annotationsResults       []map[string]string
		resourcesResults         []*corev1.ResourceRequirements
		resourceClaimsResults    [][]corev1.PodResourceClaim
		podAffinityResults       []*corev1.PodAffinity
		nodeAffinityResults      []*corev1.NodeAffinity
		containersResults        [][]corev1.Container
		containerResults         []*corev1.Container
		priorityClassNameResults []string
		imageResults             []string
	)

	specCount := 0

	if err := extractVia(ctx, fragmentedDefinition.SchedulerName, instancedComponent(definition), a.runner, &schedulerNameResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(schedulerNameResults))

	if err := extractVia(ctx, fragmentedDefinition.Labels, instancedComponent(definition), a.runner, &labelsResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(labelsResults))

	if err := extractVia(ctx, fragmentedDefinition.Annotations, instancedComponent(definition), a.runner, &annotationsResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(annotationsResults))

	if err := extractVia(ctx, fragmentedDefinition.Resources, instancedComponent(definition), a.runner, &resourcesResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(resourcesResults))

	if err := extractVia(ctx, fragmentedDefinition.ResourceClaims, instancedComponent(definition), a.runner, &resourceClaimsResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(resourceClaimsResults))

	if err := extractVia(ctx, fragmentedDefinition.PodAffinity, instancedComponent(definition), a.runner, &podAffinityResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(podAffinityResults))

	if err := extractVia(ctx, fragmentedDefinition.NodeAffinity, instancedComponent(definition), a.runner, &nodeAffinityResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(nodeAffinityResults))

	if err := extractVia(ctx, fragmentedDefinition.Containers, instancedComponent(definition), a.runner, &containersResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(containersResults))

	if err := extractVia(ctx, fragmentedDefinition.Container, instancedComponent(definition), a.runner, &containerResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(containerResults))

	if err := extractVia(ctx, fragmentedDefinition.PriorityClassName, instancedComponent(definition), a.runner, &priorityClassNameResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(priorityClassNameResults))

	if err := extractVia(ctx, fragmentedDefinition.Image, instancedComponent(definition), a.runner, &imageResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(imageResults))

	fragmentedSpecs := make([]FragmentedPodSpec, specCount)
	for i := 0; i < specCount; i++ {
		fragmentedSpecs[i] = FragmentedPodSpec{
			SchedulerName:     safeGetByIndex(schedulerNameResults, i),
			Labels:            safeGetByIndex(labelsResults, i),
			Annotations:       safeGetByIndex(annotationsResults, i),
			Resources:         safeGetByIndex(resourcesResults, i),
			ResourceClaims:    safeGetByIndex(resourceClaimsResults, i),
			PodAffinity:       safeGetByIndex(podAffinityResults, i),
			NodeAffinity:      safeGetByIndex(nodeAffinityResults, i),
			Containers:        safeGetByIndex(containersResults, i),
			Container:         safeGetByIndex(containerResults, i),
			PriorityClassName: safeGetByIndex(priorityClassNameResults, i),
			Image:             safeGetByIndex(imageResults, i),
		}
	}

	return fragmentedSpecs, nil
}

// ExtractStatus evaluates the status of the component based on the status definition.
func (a *Accessor) ExtractStatus(ctx context.Context, definition v1alpha1.ComponentDefinition) (*Status, error) {
	if definition.StatusDefinition == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have status definition", definition.Name))
	}

	statusDef := definition.StatusDefinition

	var phase *string
	if statusDef.PhaseDefinition != nil {
		phaseField := &v1alpha1.ValueAccessor{Expression: statusDef.PhaseDefinition.Expression}
		var phases []string
		if err := extractVia(ctx, phaseField, false, a.runner, &phases); err != nil {
			return nil, fmt.Errorf("failed to extract phase: %w", err)
		}
		if len(phases) > 0 {
			phase = &phases[0]
		}
	}

	conditions, err := a.extractConditions(ctx, statusDef.ConditionsDefinition)
	if err != nil {
		return nil, err
	}

	matchedStatuses, err := matchStatus(ctx, a.runner, phase, conditions, statusDef.StatusMappings)
	if err != nil {
		return nil, fmt.Errorf("failed to match status: %w", err)
	}

	status := Status{
		Phase:           phase,
		Conditions:      conditions,
		MatchedStatuses: matchedStatuses,
	}

	return &status, nil
}

func (a *Accessor) extractConditions(ctx context.Context, condDef *v1alpha1.ConditionsDefinition) ([]Condition, error) {
	if condDef == nil {
		return []Condition{}, nil
	}

	conditionsField := &v1alpha1.ValueAccessor{Expression: condDef.Expression}
	var extractedRawConditions [][]map[string]any
	if err := extractVia(ctx, conditionsField, false, a.runner, &extractedRawConditions); err != nil {
		return nil, fmt.Errorf("failed to extract conditions: %w", err)
	}

	// As Condition path is a path to a list of conditions, extract returns a slice of slice in size one/zero,
	// so we need to flatten it
	rawConditions := lo.Flatten(extractedRawConditions)

	conditions := make([]Condition, 0, len(rawConditions))
	for _, condMap := range rawConditions {
		cond := Condition{}

		if typeVal, ok := condMap[condDef.TypeFieldName]; ok {
			if typeStr, ok := typeVal.(string); ok {
				cond.Type = typeStr
			}
		}

		if statusVal, ok := condMap[condDef.StatusFieldName]; ok {
			if statusStr, ok := statusVal.(string); ok {
				cond.Status = &statusStr
			}
		}

		if condDef.MessageFieldName != nil {
			if msgVal, ok := condMap[*condDef.MessageFieldName]; ok {
				if msgStr, ok := msgVal.(string); ok {
					cond.Message = msgStr
				}
			}
		}

		if condDef.ReasonFieldName != nil {
			if reasonVal, ok := condMap[*condDef.ReasonFieldName]; ok {
				if reasonStr, ok := reasonVal.(string); ok {
					cond.Reason = &reasonStr
				}
			}
		}

		conditions = append(conditions, cond)
	}
	return conditions, nil
}

// ApplySuspendActions applies the component's SuspendActions in sequence against the manifest.
// Each action's patch is applied to the document.
// Returns DefinitionNotFoundError if the component has no SuspendDefinition.
func (a *Accessor) ApplySuspendActions(ctx context.Context, definition v1alpha1.ComponentDefinition) error {
	if definition.SuspendDefinition == nil {
		return DefinitionNotFoundError(fmt.Sprintf("component %s does not have suspendDefinition", definition.Name))
	}
	if err := a.applyActions(ctx, definition.SuspendDefinition.SuspendActions); err != nil {
		return fmt.Errorf("suspendActions: %w", err)
	}
	return nil
}

// ApplyResumeActions applies the component's ResumeActions in sequence against the manifest.
// Each action's patch is applied to the document.
// Returns DefinitionNotFoundError if the component has no SuspendDefinition.
func (a *Accessor) ApplyResumeActions(ctx context.Context, definition v1alpha1.ComponentDefinition) error {
	if definition.SuspendDefinition == nil {
		return DefinitionNotFoundError(fmt.Sprintf("component %s does not have suspendDefinition", definition.Name))
	}
	if err := a.applyActions(ctx, definition.SuspendDefinition.ResumeActions); err != nil {
		return fmt.Errorf("resumeActions: %w", err)
	}
	return nil
}

// applyActions runs an ordered action list: each patch evaluates its expression and the
// constructed object merges into the workload, applyConfiguration style.
func (a *Accessor) applyActions(ctx context.Context, actions []v1alpha1.SuspendAction) error {
	for i, action := range actions {
		results, err := a.runner.Evaluate(ctx, action.Patch)
		if err != nil {
			return fmt.Errorf("[%d]: evaluate patch: %w", i, err)
		}
		if len(results) != 1 {
			return fmt.Errorf("[%d]: a patch must construct exactly one object, got %d results", i, len(results))
		}
		live, err := a.runner.GetObject()
		if err != nil {
			return fmt.Errorf("[%d]: %w", i, err)
		}
		if err := a.runner.Assign(ctx, ".", mergePatch(live, results[0])); err != nil {
			return fmt.Errorf("[%d]: apply patch: %w", i, err)
		}
	}

	return nil
}

func (a *Accessor) ExtractInstanceIds(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]string, error) {
	if definition.InstanceIds == nil || definition.InstanceIds.Expression == "" {
		return nil, DefinitionNotFoundError("no instance ids defined")
	}

	// The expression returns the whole id list as one value.
	var instanceIds []string
	results, err := a.runner.EvaluateWithVariables(ctx, definition.InstanceIds.Expression, nil)
	if err != nil {
		return nil, err
	}
	if len(results) == 1 {
		if list, ok := results[0].([]any); ok {
			for _, id := range list {
				instanceIds = append(instanceIds, fmt.Sprintf("%v", id))
			}
		}
	}

	// Validate all instance ids are not empty
	if lo.Contains(instanceIds, "") {
		return nil, fmt.Errorf("instance ids contained empty string values [%s]", strings.Join(instanceIds, ","))
	}

	return instanceIds, nil
}

func (a *Accessor) UpdatePodTemplateSpec(ctx context.Context, definition v1alpha1.ComponentDefinition, podTemplateSpecs []corev1.PodTemplateSpec) error {
	if definition.SpecDefinition == nil {
		return DefinitionNotFoundError(fmt.Sprintf("component %s does not have spec definition", definition.Name))
	}

	if definition.SpecDefinition.PodTemplateSpec == nil {
		return DefinitionNotFoundError(fmt.Sprintf("component %s does not have pod template spec definition", definition.Name))
	}

	return a.assignVia(ctx, definition, definition.SpecDefinition.PodTemplateSpec, lo.Map(podTemplateSpecs, func(podTemplateSpec corev1.PodTemplateSpec, _ int) any { return podTemplateSpec }))
}

func (a *Accessor) UpdatePodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition, podSpecs []corev1.PodSpec) error {
	if definition.SpecDefinition == nil {
		return DefinitionNotFoundError(fmt.Sprintf("component %s does not have spec definition", definition.Name))
	}

	if definition.SpecDefinition.PodSpec == nil {
		return DefinitionNotFoundError(fmt.Sprintf("component %s does not have pod spec definition", definition.Name))
	}

	return a.assignVia(ctx, definition, definition.SpecDefinition.PodSpec, lo.Map(podSpecs, func(podSpec corev1.PodSpec, _ int) any { return podSpec }))
}

func (a *Accessor) UpdatePodMetadata(ctx context.Context, definition v1alpha1.ComponentDefinition, podMetadata []metav1.ObjectMeta) error {
	if definition.SpecDefinition == nil {
		return DefinitionNotFoundError(fmt.Sprintf("component %s does not have spec definition", definition.Name))
	}

	if definition.SpecDefinition.Metadata == nil {
		return DefinitionNotFoundError(fmt.Sprintf("component %s does not have pod metadata definition", definition.Name))
	}
	return a.assignVia(ctx, definition, definition.SpecDefinition.Metadata, lo.Map(podMetadata, func(podMetadata metav1.ObjectMeta, _ int) any { return podMetadata }))
}

func (a *Accessor) UpdateFragmentedPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition, fragmentedPodSpecs []FragmentedPodSpec) (retErr error) {
	if definition.SpecDefinition == nil {
		return DefinitionNotFoundError(fmt.Sprintf("component %s does not have spec definition", definition.Name))
	}

	if definition.SpecDefinition.FragmentedPodSpecDefinition == nil {
		return DefinitionNotFoundError(fmt.Sprintf("component %s does not have fragmented pod spec definition", definition.Name))
	}

	fragmentedDef := definition.SpecDefinition.FragmentedPodSpecDefinition

	// The fields are written in sequence into the live document, so an error midway would leave
	// the earlier writes applied - and the two engines fail at different points on the same bad
	// input. The update restores the pre-write document on any error, making it all-or-nothing.
	live, err := a.runner.GetObject()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(live)
	if err != nil {
		return err
	}
	var snapshot any
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			if restoreErr := a.runner.Assign(context.WithoutCancel(ctx), ".", snapshot); restoreErr != nil {
				retErr = fmt.Errorf("%w (restoring the pre-write document also failed: %v)", retErr, restoreErr)
			}
		}
	}()

	// String fields
	if err := a.updateStringField(ctx, definition, fragmentedDef.SchedulerName, fragmentedPodSpecs, func(s FragmentedPodSpec) string { return s.SchedulerName }); err != nil {
		return fmt.Errorf("failed to update scheduler name: %w", err)
	}
	if err := a.updateStringField(ctx, definition, fragmentedDef.PriorityClassName, fragmentedPodSpecs, func(s FragmentedPodSpec) string { return s.PriorityClassName }); err != nil {
		return fmt.Errorf("failed to update priority class name: %w", err)
	}
	if err := a.updateStringField(ctx, definition, fragmentedDef.Image, fragmentedPodSpecs, func(s FragmentedPodSpec) string { return s.Image }); err != nil {
		return fmt.Errorf("failed to update image: %w", err)
	}

	// Map fields
	if err := updateMapField(a, ctx, definition, fragmentedDef.Labels, fragmentedPodSpecs, func(s FragmentedPodSpec) map[string]string { return s.Labels }); err != nil {
		return fmt.Errorf("failed to update labels: %w", err)
	}
	if err := updateMapField(a, ctx, definition, fragmentedDef.Annotations, fragmentedPodSpecs, func(s FragmentedPodSpec) map[string]string { return s.Annotations }); err != nil {
		return fmt.Errorf("failed to update annotations: %w", err)
	}

	// Pointer fields
	if err := updateStructPointerField(a, ctx, definition, fragmentedDef.Resources, fragmentedPodSpecs, func(s FragmentedPodSpec) *corev1.ResourceRequirements { return s.Resources }); err != nil {
		return fmt.Errorf("failed to update resources: %w", err)
	}
	if err := updateStructPointerField(a, ctx, definition, fragmentedDef.PodAffinity, fragmentedPodSpecs, func(s FragmentedPodSpec) *corev1.PodAffinity { return s.PodAffinity }); err != nil {
		return fmt.Errorf("failed to update pod affinity: %w", err)
	}
	if err := updateStructPointerField(a, ctx, definition, fragmentedDef.NodeAffinity, fragmentedPodSpecs, func(s FragmentedPodSpec) *corev1.NodeAffinity { return s.NodeAffinity }); err != nil {
		return fmt.Errorf("failed to update node affinity: %w", err)
	}
	if err := updateStructPointerField(a, ctx, definition, fragmentedDef.Container, fragmentedPodSpecs, func(s FragmentedPodSpec) *corev1.Container { return s.Container }); err != nil {
		return fmt.Errorf("failed to update container: %w", err)
	}

	// Slice fields
	if err := updateSliceField(a, ctx, definition, fragmentedDef.ResourceClaims, fragmentedPodSpecs, func(s FragmentedPodSpec) []corev1.PodResourceClaim { return s.ResourceClaims }); err != nil {
		return fmt.Errorf("failed to update resource claims: %w", err)
	}
	if err := updateSliceField(a, ctx, definition, fragmentedDef.Containers, fragmentedPodSpecs, func(s FragmentedPodSpec) []corev1.Container { return s.Containers }); err != nil {
		return fmt.Errorf("failed to update containers: %w", err)
	}

	return nil
}

// extractVia reads one field through the pair's expression. An instanced component's expression
// returns one value per instance in a list, which is spread into the stream shape the converters
// expect; a single-instance field keeps its one result, so a list-valued field such as containers
// survives intact.
func extractVia[T any](ctx context.Context, via *v1alpha1.ValueAccessor, instanced bool, runner expression.Runner, out *[]T) error {
	if via == nil || via.Expression == "" {
		return nil
	}
	results, err := runner.EvaluateWithVariables(ctx, via.Expression, nil)
	if err != nil {
		return err
	}
	if instanced && len(results) == 1 {
		if list, ok := results[0].([]any); ok {
			results = list
		}
	}
	converted, err := safeConvertSlice[T](results)
	if err != nil {
		return err
	}
	*out = converted

	return nil
}

// instancedComponent reports whether the component holds one pod definition per instance, in
// which case a pair expression returns a per-instance list.
func instancedComponent(definition v1alpha1.ComponentDefinition) bool {
	return definition.InstanceIds != nil && definition.InstanceIds.Expression != ""
}

// applyPatches writes values through the pair's patch: one evaluation per value with `value` and
// `instance` bound, each result merged into the workload and applied at the root.
func (a *Accessor) applyPatches(ctx context.Context, def v1alpha1.ComponentDefinition, via *v1alpha1.ValueAccessor, values []any) error {
	ids, err := a.ExtractInstanceIds(ctx, def)
	if err != nil {
		ids = nil
	}
	// Every patch is constructed against the pre-write document before any of them is applied -
	// a stable order, every path resolved before the first write. A Replace deletes before it
	// sets, and an addressing expression (a variable or the patch itself) must keep naming the
	// location the delete just emptied; a later instance must not address through an earlier
	// instance's write either.
	var frozen map[string]any
	if resolved, err := a.runner.ResolveVariables(ctx, via.Patch); err == nil {
		frozen = resolved
	}
	var patches []any
	for i, value := range values {
		var instance any
		if i < len(ids) {
			instance = ids[i]
		}
		// Replace = delete first : the same patch with value bound to null removes the field ,
		// so the second application sets the value clean instead of merging into what was there.
		binds := []any{value}
		if via.Replace {
			binds = []any{nil, value}
		}
		for _, bound := range binds {
			vars := map[string]any{"value": bound, "instance": instance, "index": i}
			if frozen != nil {
				vars["variables"] = frozen
			}
			results, err := a.runner.EvaluateWithVariables(ctx, via.Patch, vars)
			if err != nil {
				return fmt.Errorf("evaluate patch: %w", err)
			}
			if len(results) != 1 {
				return fmt.Errorf("a patch must construct exactly one object, got %d results", len(results))
			}
			patches = append(patches, results[0])
		}
	}
	// The patches land all-or-nothing: every path is resolved before the
	// first assignment: a failure applying a later instance restores the pre-write document.
	prewrite, err := a.runner.GetObject()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(prewrite)
	if err != nil {
		return err
	}
	var snapshot any
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return err
	}
	for _, patch := range patches {
		live, err := a.runner.GetObject()
		if err != nil {
			return err
		}
		merged, err := applyConstructedPatch(live, patch)
		if err == nil {
			err = a.runner.Assign(ctx, ".", merged)
		}
		if err != nil {
			if restoreErr := a.runner.Assign(context.WithoutCancel(ctx), ".", snapshot); restoreErr != nil {
				return fmt.Errorf("apply patch: %w (restoring the pre-write document also failed: %v)", err, restoreErr)
			}

			return fmt.Errorf("apply patch: %w", err)
		}
	}

	return nil
}

// assignVia writes one field through the pair's patch.
func (a *Accessor) assignVia(ctx context.Context, def v1alpha1.ComponentDefinition, via *v1alpha1.ValueAccessor, values []any) error {
	if via == nil || via.Patch == "" {
		return fmt.Errorf("the field has no patch and cannot be written")
	}

	return a.applyPatches(ctx, def, via, values)
}

func (a *Accessor) updateField(ctx context.Context, def v1alpha1.ComponentDefinition, via *v1alpha1.ValueAccessor, values []any, isEmpty func(any) bool) error {
	if via != nil && via.Patch != "" {
		// Skip assignment if all values are empty/nil to avoid writing null
		// into the JSON.
		allEmpty := true
		for _, v := range values {
			if !isEmpty(v) {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			return nil
		}
		return a.assignVia(ctx, def, via, values)
	}
	for _, v := range values {
		if !isEmpty(v) {
			return fmt.Errorf("the field has no patch and values are not empty")
		}
	}
	return nil
}

func (a *Accessor) updateStringField(ctx context.Context, def v1alpha1.ComponentDefinition, via *v1alpha1.ValueAccessor, specs []FragmentedPodSpec, getter func(FragmentedPodSpec) string) error {
	values := lo.Map(specs, func(s FragmentedPodSpec, _ int) any { return getter(s) })
	return a.updateField(ctx, def, via, values, func(v any) bool { return v.(string) == "" })
}

func updateMapField[K comparable, V any](a *Accessor, ctx context.Context, def v1alpha1.ComponentDefinition, via *v1alpha1.ValueAccessor, specs []FragmentedPodSpec, getter func(FragmentedPodSpec) map[K]V) error {
	values := lo.Map(specs, func(s FragmentedPodSpec, _ int) any { return getter(s) })
	return a.updateField(ctx, def, via, values, func(v any) bool { return len(v.(map[K]V)) == 0 })
}

func updateStructPointerField[T any](a *Accessor, ctx context.Context, def v1alpha1.ComponentDefinition, via *v1alpha1.ValueAccessor, specs []FragmentedPodSpec, getter func(FragmentedPodSpec) *T) error {
	values := lo.Map(specs, func(s FragmentedPodSpec, _ int) any { return getter(s) })
	return a.updateField(ctx, def, via, values, func(v any) bool { return v.(*T) == nil })
}

func updateSliceField[T any](a *Accessor, ctx context.Context, def v1alpha1.ComponentDefinition, via *v1alpha1.ValueAccessor, specs []FragmentedPodSpec, getter func(FragmentedPodSpec) []T) error {
	values := lo.Map(specs, func(s FragmentedPodSpec, _ int) any { return getter(s) })
	return a.updateField(ctx, def, via, values, func(v any) bool { return len(v.([]T)) == 0 })
}

func safeGetByIndex[T any](s []T, i int) T {
	var zero T
	if i < 0 || i >= len(s) {
		return zero
	}

	return s[i]
}

func safeConvertSlice[T any](slice []any) ([]T, error) {
	if slice == nil {
		return nil, nil
	}

	convertedResults := make([]T, len(slice))
	for i, object := range slice {
		// First try direct type assertion (for simple types)
		if converted, ok := object.(T); ok {
			convertedResults[i] = converted
			continue
		}

		// For complex types (structs), use JSON marshaling/unmarshaling
		var converted T
		if err := convertViaJSON(object, &converted); err != nil {
			return nil, fmt.Errorf("failed to convert object at index %d to type %T: %w", i, converted, err)
		}
		convertedResults[i] = converted
	}

	return convertedResults, nil
}

// convertViaJSON converts between types using JSON marshaling
func convertViaJSON(src any, dst any) error {
	jsonBytes, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("failed to marshal source: %w", err)
	}

	if err := json.Unmarshal(jsonBytes, dst); err != nil {
		return fmt.Errorf("failed to unmarshal to destination: %w", err)
	}

	return nil
}

func matchStatus(ctx context.Context, runner expression.Runner, phase *string, conditions []Condition, mappings v1alpha1.StatusMappings) ([]v1alpha1.ResourceStatus, error) {
	conditionsMap := make(map[string]Condition, len(conditions))
	for _, cond := range conditions {
		conditionsMap[cond.Type] = cond
	}

	matchedStatuses := make([]v1alpha1.ResourceStatus, 0)
	for _, entry := range mappings.Entries() {
		matched, err := evaluateMatchers(ctx, runner, phase, conditionsMap, entry.Matchers)
		if err != nil {
			return nil, err
		}
		if matched {
			matchedStatuses = append(matchedStatuses, entry.Status)
		}
	}

	if len(matchedStatuses) == 0 {
		return []v1alpha1.ResourceStatus{v1alpha1.UndefinedStatus}, nil
	}

	return matchedStatuses, nil
}

func evaluateMatchers(ctx context.Context, runner expression.Runner, phase *string, conditionsMap map[string]Condition, matchers []v1alpha1.StatusMatcher) (bool, error) {
	for _, matcher := range matchers {
		matched, err := match(ctx, runner, phase, conditionsMap, matcher)
		if err != nil {
			return false, fmt.Errorf("failed to evaluate matcher: %w", err)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func match(ctx context.Context, runner expression.Runner, phase *string, conditionsMap map[string]Condition, matcher v1alpha1.StatusMatcher) (bool, error) {
	if matcher.ByPhase != "" {
		if phase == nil || *phase != matcher.ByPhase {
			return false, nil
		}
	}

	for _, expectedCond := range matcher.ByConditions {
		actualCond, found := checkCondition(conditionsMap, expectedCond)
		if !found {
			return false, nil
		}

		expectedValue := ptr.Deref(expectedCond.Status, "")
		if expectedValue != "" && expectedValue != ptr.Deref(actualCond.Status, "") {
			return false, nil
		}

		expectedValue = ptr.Deref(expectedCond.Reason, "")
		if expectedValue != "" && expectedValue != ptr.Deref(actualCond.Reason, "") {
			return false, nil
		}
	}

	if matcher.ByExpression != nil {
		matched, err := matchByExpression(ctx, runner, matcher)
		if err != nil {
			return false, err
		}

		return matched, nil
	}

	return true, nil
}

// matchByExpression evaluates a status matcher through the definition's own runner, so the
// variables stay bound. The result is compared to ExpectedResult in string form. The empty
// variable set binds value, instance and index to null; a matcher does not use them.
func matchByExpression(ctx context.Context, runner expression.Runner, matcher v1alpha1.StatusMatcher) (bool, error) {
	results, err := runner.EvaluateWithVariables(ctx, matcher.ByExpression.Expression, map[string]any{})
	if err != nil {
		return false, fmt.Errorf("failed to evaluate ByExpression: %w", err)
	}
	if len(results) == 0 {
		return false, nil
	}

	return fmt.Sprintf("%v", results[0]) == matcher.ByExpression.ExpectedResult, nil
}

func checkCondition(conditionsMap map[string]Condition, expectedCond v1alpha1.ExpectedCondition) (Condition, bool) {
	actualCond, found := conditionsMap[expectedCond.Type]
	if !found {
		return Condition{}, false
	}
	return actualCond, true
}
