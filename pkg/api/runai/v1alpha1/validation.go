// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package v1alpha1

import (
	"errors"
	"fmt"
)

var kindsWithoutGroup = map[string]bool{
	"Pod": true,
}

type KartaValidator struct {
	karta         *Karta
	rootComponent ComponentDefinition
	allComponents map[string]ComponentDefinition
}

func NewKartaValidator(karta *Karta) *KartaValidator {
	return &KartaValidator{karta: karta}
}

func (v *KartaValidator) Validate() error {
	if v.karta == nil {
		return errors.New("karta is nil")
	}

	var errs []error

	if initErrs := v.initialize(); initErrs != nil {
		errs = append(errs, initErrs...)
		return errors.Join(errs...)
	}

	if specErrs := v.validateStructureDefinition(); specErrs != nil {
		errs = append(errs, specErrs...)
	}

	if instructionErrs := v.validateInstructions(); instructionErrs != nil {
		errs = append(errs, instructionErrs...)
	}

	return errors.Join(errs...)
}

func (v *KartaValidator) initialize() []error {
	var errs []error

	v.rootComponent = v.karta.Spec.StructureDefinition.RootComponent

	v.allComponents = make(map[string]ComponentDefinition)
	allComponents := make([]ComponentDefinition, 0, len(v.karta.Spec.StructureDefinition.ChildComponents)+1)
	allComponents = append(allComponents, v.karta.Spec.StructureDefinition.ChildComponents...)
	allComponents = append(allComponents, v.karta.Spec.StructureDefinition.RootComponent)
	for _, component := range allComponents {
		if _, ok := v.allComponents[component.Name]; ok {
			errs = append(errs, fmt.Errorf("component name %s is not unique", component.Name))
		}
		v.allComponents[component.Name] = component
	}

	return errs
}

func (v *KartaValidator) validateStructureDefinition() []error {
	var errs []error

	// Root component validation
	errs = append(errs, v.validateRootComponent()...)

	// Child components validation
	for _, component := range v.karta.Spec.StructureDefinition.ChildComponents {
		// Must have non-empty owner ref
		if component.OwnerRef == nil || *component.OwnerRef == "" {
			errs = append(errs, fmt.Errorf("child component '%s' has no owner ref", component.Name))
		} else {
			// Owner ref must point to an existing component
			if _, ok := v.allComponents[*component.OwnerRef]; !ok {
				errs = append(errs, fmt.Errorf("child component '%s' has owner ref to non-existing component '%s'", component.Name, *component.OwnerRef))
			}
		}

		// Component validation
		errs = append(errs, v.validateComponent(component)...)
	}

	// Stop here if there are any errors - futher validations are relying on the structure definition to be valid.
	if len(errs) > 0 {
		return errs
	}

	// No ownership cycles
	if ownershipCycleErr := v.validateNoOwnershipCycles(); ownershipCycleErr != nil {
		errs = append(errs, ownershipCycleErr)
	}

	return errs
}

func (v *KartaValidator) validateRootComponent() []error {
	var errs []error

	// Has full gvk
	if v.rootComponent.Kind == nil ||
		v.rootComponent.Kind.Version == "" || v.rootComponent.Kind.Kind == "" ||
		(v.rootComponent.Kind.Group == "" && !kindsWithoutGroup[v.rootComponent.Kind.Kind]) {
		errs = append(errs, fmt.Errorf("root component must have full kind (group, version, kind)"))
	}

	// No owner ref
	if v.rootComponent.OwnerRef != nil {
		errs = append(errs, fmt.Errorf("root component cannot have owner ref"))
	}

	// Has status definition
	if v.rootComponent.StatusDefinition == nil {
		errs = append(errs, fmt.Errorf("root component must have status definition"))
	}

	// Component validation
	errs = append(errs, v.validateComponent(v.rootComponent)...)

	return errs
}

func (v *KartaValidator) validateComponent(component ComponentDefinition) []error {
	var errs []error

	// Non-empty name
	if component.Name == "" {
		errs = append(errs, fmt.Errorf("component name is empty"))
	}

	// Mutually exclusive pod spec definitions
	if component.SpecDefinition != nil {
		counter := 0

		if component.SpecDefinition.PodTemplateSpec != nil {
			counter++
		}
		if component.SpecDefinition.PodSpec != nil {
			counter++
		}
		if component.SpecDefinition.FragmentedPodSpecDefinition != nil {
			counter++
		}

		if counter > 1 {
			errs = append(errs, fmt.Errorf("component '%s' has multiple pod spec definitions", component.Name))
		}
	}

	// Component's PodSelector has instance selector if has the component has instance id path defined or the opposite
	if err := validateMultiInstanceComponent(component); err != nil {
		errs = append(errs, err)
	}

	errs = append(errs, validateComponentPatches(component)...)

	return errs
}

// validateComponentPatches checks every patch entry the component declares: a
// declared patch type and an expression on each entry, named conditions, and in a
// value accessor an unconditional entry only as the last one, since entry selection
// is first match wins.
func validateComponentPatches(component ComponentDefinition) []error {
	var errs []error

	accessors := map[string]*ValueAccessor{
		"instanceIds": component.InstanceIds,
	}
	if spec := component.SpecDefinition; spec != nil {
		accessors["specDefinition.podTemplateSpec"] = spec.PodTemplateSpec
		accessors["specDefinition.podSpec"] = spec.PodSpec
		accessors["specDefinition.metadata"] = spec.Metadata
		if fragmented := spec.FragmentedPodSpecDefinition; fragmented != nil {
			accessors["fragmentedPodSpecDefinition.schedulerName"] = fragmented.SchedulerName
			accessors["fragmentedPodSpecDefinition.labels"] = fragmented.Labels
			accessors["fragmentedPodSpecDefinition.annotations"] = fragmented.Annotations
			accessors["fragmentedPodSpecDefinition.resources"] = fragmented.Resources
			accessors["fragmentedPodSpecDefinition.resourceClaims"] = fragmented.ResourceClaims
			accessors["fragmentedPodSpecDefinition.podAffinity"] = fragmented.PodAffinity
			accessors["fragmentedPodSpecDefinition.nodeAffinity"] = fragmented.NodeAffinity
			accessors["fragmentedPodSpecDefinition.containers"] = fragmented.Containers
			accessors["fragmentedPodSpecDefinition.container"] = fragmented.Container
			accessors["fragmentedPodSpecDefinition.priorityClassName"] = fragmented.PriorityClassName
			accessors["fragmentedPodSpecDefinition.image"] = fragmented.Image
		}
	}
	if scale := component.ScaleDefinition; scale != nil {
		accessors["scaleDefinition.replicas"] = scale.Replicas
		accessors["scaleDefinition.minReplicas"] = scale.MinReplicas
		accessors["scaleDefinition.maxReplicas"] = scale.MaxReplicas
	}
	for field, accessor := range accessors {
		if accessor == nil {
			continue
		}
		if accessor.PatchStrategy != "" && accessor.PatchStrategy != PatchStrategyMerge && accessor.PatchStrategy != PatchStrategyReplace {
			errs = append(errs, fmt.Errorf("component '%s' %s: unknown patchStrategy %q", component.Name, field, accessor.PatchStrategy))
		}
		for i, entry := range accessor.Patches {
			errs = append(errs, validatePatchEntry(component.Name, fmt.Sprintf("%s.patches[%d]", field, i), entry)...)
			if len(entry.MatchConditions) == 0 && i != len(accessor.Patches)-1 {
				errs = append(errs, fmt.Errorf("component '%s' %s.patches[%d]: an entry without matchConditions always matches and may only appear last", component.Name, field, i))
			}
		}
	}
	if component.InstanceIds != nil && len(component.InstanceIds.Patches) > 0 {
		errs = append(errs, fmt.Errorf("component '%s' instanceIds: is read-only and cannot declare patches", component.Name))
	}
	if suspend := component.SuspendDefinition; suspend != nil {
		for i, entry := range suspend.SuspendActions {
			errs = append(errs, validatePatchEntry(component.Name, fmt.Sprintf("suspendActions[%d]", i), entry)...)
		}
		for i, entry := range suspend.ResumeActions {
			errs = append(errs, validatePatchEntry(component.Name, fmt.Sprintf("resumeActions[%d]", i), entry)...)
		}
	}

	return errs
}

func validatePatchEntry(componentName, field string, entry PatchEntry) []error {
	var errs []error

	if entry.PatchType != PatchTypeMergePatch && entry.PatchType != PatchTypeJSONPatch {
		errs = append(errs, fmt.Errorf("component '%s' %s: patchType must be %s or %s, got %q", componentName, field, PatchTypeMergePatch, PatchTypeJSONPatch, entry.PatchType))
	}
	if entry.Expression == "" {
		errs = append(errs, fmt.Errorf("component '%s' %s: expression is required", componentName, field))
	}
	seen := map[string]bool{}
	for j, condition := range entry.MatchConditions {
		if condition.Name == "" {
			errs = append(errs, fmt.Errorf("component '%s' %s.matchConditions[%d]: name is required", componentName, field, j))
		} else if seen[condition.Name] {
			errs = append(errs, fmt.Errorf("component '%s' %s.matchConditions[%d]: name %q is not unique", componentName, field, j, condition.Name))
		}
		seen[condition.Name] = true
		if condition.Expression == "" {
			errs = append(errs, fmt.Errorf("component '%s' %s.matchConditions[%d]: expression is required", componentName, field, j))
		}
	}

	return errs
}

func validateMultiInstanceComponent(component ComponentDefinition) error {
	hasInstanceIds := component.InstanceIds != nil && component.InstanceIds.Expression != ""
	if hasInstanceIds &&
		(component.PodSelector == nil || component.PodSelector.ComponentInstanceSelector == nil) {
		return fmt.Errorf("component '%s' has instance ids but no pod component instance selector", component.Name)
	}

	if (component.PodSelector != nil && component.PodSelector.ComponentInstanceSelector != nil) &&
		!hasInstanceIds {
		return fmt.Errorf("component '%s' has pod component instance selector but no instance ids", component.Name)
	}

	return nil
}

// validateNoOwnershipCycles detects circular dependencies by following owner ref chains
func (v *KartaValidator) validateNoOwnershipCycles() error {
	validated := make(map[string]bool)

	// For each child component, follow its parent chain to ensure it reaches root
	for _, child := range v.karta.Spec.StructureDefinition.ChildComponents {
		if err := v.checkPathToRoot(child, &validated); err != nil {
			return err
		}
	}
	return nil
}

// checkPathToRoot follows owner ref chain from a component to ensure it reaches root without cycles
// assumes that owner refs were already validated to be existing components
func (v *KartaValidator) checkPathToRoot(component ComponentDefinition, validatedComponents *map[string]bool) error {
	visited := make(map[string]bool)
	current := component.Name
	alreadyValidated := false

	for current != "" && !alreadyValidated {
		if visited[current] {
			return fmt.Errorf("ownership cycle detected involving component %s", current)
		}
		visited[current] = true

		currentComponent := v.allComponents[current]
		// If we reached root, we're done
		if currentComponent.OwnerRef == nil {
			return nil
		}

		// Move to parent
		current = *currentComponent.OwnerRef
		_, alreadyValidated = (*validatedComponents)[current]
	}

	(*validatedComponents)[component.Name] = true
	return nil
}

func (v *KartaValidator) validateInstructions() []error {
	return v.validateGangScheduling()
}

func (v *KartaValidator) validateGangScheduling() []error {
	if v.karta.Spec.Instructions.GangScheduling == nil {
		return nil
	}

	// All member components are defined
	var errs []error
	for _, group := range v.karta.Spec.Instructions.GangScheduling.PodGroups {
		for _, member := range group.Members {
			if _, ok := v.allComponents[member.ComponentName]; !ok {
				errs = append(errs, fmt.Errorf("pod-group member component '%s' is not defined (should be a root or child component)", member.ComponentName))
			}
		}
	}
	return errs
}
