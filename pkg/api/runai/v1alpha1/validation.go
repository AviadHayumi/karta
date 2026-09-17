// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package v1alpha1

import (
	"errors"
	"fmt"
	"regexp"
)

var kindsWithoutGroup = map[string]bool{
	"Pod": true,
}

var writePath = regexp.MustCompile(`^(/([^~/]|~[01])*)*$`)

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

	if referenceErrs := v.validateReferences(); referenceErrs != nil {
		errs = append(errs, referenceErrs...)
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

	errs = append(errs, validateComponentWritePaths(component)...)

	return errs
}

func validateComponentWritePaths(component ComponentDefinition) []error {
	var errs []error

	for name, accessor := range component.Fields {
		switch name {
		case "":
			errs = append(errs, fmt.Errorf("component '%s' fields: field name is empty", component.Name))
		case "podTemplateSpec", "podSpec", "metadata",
			"fragmented.schedulerName", "fragmented.labels", "fragmented.annotations",
			"fragmented.resources", "fragmented.resourceClaims", "fragmented.podAffinity",
			"fragmented.nodeAffinity", "fragmented.containers", "fragmented.container",
			"fragmented.priorityClassName", "fragmented.image",
			"scale.replicas", "scale.minReplicas", "scale.maxReplicas", "suspend":
			errs = append(errs, fmt.Errorf("component '%s' fields[%q]: field name is reserved for a built-in SDK field", component.Name, name))
		}
		if accessor.Expression == "" && accessor.PathWrite == nil && accessor.PathWriteExpression == "" {
			errs = append(errs, fmt.Errorf("component '%s' fields[%q]: expression, pathWrite, or pathWriteExpression is required", component.Name, name))
		}
		if err := validateWriteTarget(accessor.PathWrite, accessor.PathWriteExpression); err != nil {
			errs = append(errs, fmt.Errorf("component '%s' fields[%q]: %w", component.Name, name, err))
		}
	}

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
		if err := validateWriteTarget(accessor.PathWrite, accessor.PathWriteExpression); err != nil {
			errs = append(errs, fmt.Errorf("component '%s' %s: %w", component.Name, field, err))
		}
	}
	if ids := component.InstanceIds; ids != nil && (ids.PathWrite != nil || ids.PathWriteExpression != "") {
		errs = append(errs, fmt.Errorf("component '%s' instanceIds: is read-only and cannot declare pathWrite or pathWriteExpression", component.Name))
	}
	if suspend := component.SuspendDefinition; suspend != nil {
		if suspend.PathWrite == nil && suspend.PathWriteExpression == "" {
			errs = append(errs, fmt.Errorf("component '%s' suspendDefinition: exactly one of pathWrite or pathWriteExpression is required", component.Name))
		}
		if err := validateWriteTarget(suspend.PathWrite, suspend.PathWriteExpression); err != nil {
			errs = append(errs, fmt.Errorf("component '%s' suspendDefinition: %w", component.Name, err))
		}
	}

	return errs
}

func validateWriteTarget(pathWrite *string, pathWriteExpression string) error {
	if pathWrite != nil && pathWriteExpression != "" {
		return errors.New("pathWrite and pathWriteExpression are mutually exclusive")
	}
	if pathWrite != nil && !writePath.MatchString(*pathWrite) {
		return fmt.Errorf("pathWrite %q must be an RFC 6901 JSON Pointer: empty or slash-prefixed, with only ~0 and ~1 escapes", *pathWrite)
	}
	return nil
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

// referenceName constrains a reference name to a CEL identifier, so references.<name> always
// parses.
var referenceName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func (v *KartaValidator) validateReferences() []error {
	var errs []error

	names := make(map[string]bool, len(v.karta.Spec.StructureDefinition.References))
	for _, ref := range v.karta.Spec.StructureDefinition.References {
		if ref.Name == "" {
			errs = append(errs, fmt.Errorf("reference name is empty"))
		} else if !referenceName.MatchString(ref.Name) {
			errs = append(errs, fmt.Errorf("reference name %q is not a valid identifier (want %s)", ref.Name, referenceName))
		}
		if names[ref.Name] {
			errs = append(errs, fmt.Errorf("reference name %q is not unique", ref.Name))
		}
		names[ref.Name] = true

		if ref.GVK.Version == "" || ref.GVK.Kind == "" {
			errs = append(errs, fmt.Errorf("reference %q must have version and kind", ref.Name))
		}

		if (ref.Lookup == nil) == (ref.List == nil) {
			errs = append(errs, fmt.Errorf("reference %q must set exactly one of lookup or list", ref.Name))
			continue
		}

		switch {
		case ref.Lookup != nil:
			if ref.Lookup.NameExpression == "" {
				errs = append(errs, fmt.Errorf("reference %q lookup has an empty nameExpression", ref.Name))
			}
		case ref.List != nil:
			if len(ref.List.MatchLabels) == 0 && len(ref.List.MatchExpressions) == 0 {
				errs = append(errs, fmt.Errorf("reference %q list must set matchLabels or matchExpressions", ref.Name))
			}
			for key, value := range ref.List.MatchLabels {
				if (value.Value == nil) == (value.Expression == nil) {
					errs = append(errs, fmt.Errorf("reference %q matchLabels[%s] must set exactly one of value or expression", ref.Name, key))
				}
			}
			for _, req := range ref.List.MatchExpressions {
				switch req.Operator {
				case LabelSelectorOpIn, LabelSelectorOpNotIn:
					if len(req.Values) == 0 {
						errs = append(errs, fmt.Errorf("reference %q matchExpressions[%s] requires values for operator %s", ref.Name, req.Key, req.Operator))
					}
				case LabelSelectorOpExists, LabelSelectorOpDoesNotExist:
					if len(req.Values) != 0 {
						errs = append(errs, fmt.Errorf("reference %q matchExpressions[%s] must not set values for operator %s", ref.Name, req.Key, req.Operator))
					}
				default:
					errs = append(errs, fmt.Errorf("reference %q matchExpressions[%s] has unknown operator %q", ref.Name, req.Key, req.Operator))
				}
				for _, value := range req.Values {
					if (value.Value == nil) == (value.Expression == nil) {
						errs = append(errs, fmt.Errorf("reference %q matchExpressions[%s] values must set exactly one of value or expression", ref.Name, req.Key))
					}
				}
			}
		}
	}

	return errs
}
