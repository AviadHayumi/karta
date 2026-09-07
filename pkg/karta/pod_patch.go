// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package karta

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// PodPatch is a typed partial pod update: nil or empty fields are not
// touched. An all-unset patch is rejected with ErrEmptyPatch, never treated
// as a silent no-op.
type PodPatch struct {
	SchedulerName     *string
	PriorityClassName *string

	// Labels and Annotations are merged by key into the existing maps.
	// A key cannot be deleted through a patch.
	Labels      map[string]string
	Annotations map[string]string

	// NodeAffinity, PodAffinity and ResourceClaims replace the existing
	// value wholesale.
	NodeAffinity   *corev1.NodeAffinity
	PodAffinity    *corev1.PodAffinity
	ResourceClaims []corev1.PodResourceClaim

	// Image and Resources target the pod's single container and fail when
	// the pod has more than one container. Multi-container pods use
	// Containers.
	Image     *string
	Resources *corev1.ResourceRequirements

	// Containers are merged by container name, touching only the fields set
	// on each entry.
	Containers []ContainerPatch
}

// AsPodMergePatch compiles the typed patch into the raw merge-patch view, so
// the typed and raw doors go through one router. Image and Resources become a
// single unnamed container entry - the sole-container bridge.
func (p PodPatch) AsPodMergePatch() Patch {
	metadata := map[string]any{}
	spec := map[string]any{}
	if p.SchedulerName != nil {
		spec["schedulerName"] = *p.SchedulerName
	}
	if p.PriorityClassName != nil {
		spec["priorityClassName"] = *p.PriorityClassName
	}
	if len(p.Labels) > 0 {
		metadata["labels"] = stringMapToAny(p.Labels)
	}
	if len(p.Annotations) > 0 {
		metadata["annotations"] = stringMapToAny(p.Annotations)
	}
	affinity := map[string]any{}
	if p.NodeAffinity != nil {
		affinity["nodeAffinity"] = mustRaw(p.NodeAffinity)
	}
	if p.PodAffinity != nil {
		affinity["podAffinity"] = mustRaw(p.PodAffinity)
	}
	if len(affinity) > 0 {
		spec["affinity"] = affinity
	}
	if len(p.ResourceClaims) > 0 {
		spec["resourceClaims"] = mustRaw(p.ResourceClaims)
	}
	var containers []any
	if p.Image != nil || p.Resources != nil {
		sole := map[string]any{}
		if p.Image != nil {
			sole["image"] = *p.Image
		}
		if p.Resources != nil {
			sole["resources"] = mustRaw(p.Resources)
		}
		containers = append(containers, sole)
	}
	for _, entry := range p.Containers {
		container := map[string]any{"name": entry.Name}
		if entry.Image != nil {
			container["image"] = *entry.Image
		}
		if entry.Resources != nil {
			container["resources"] = mustRaw(entry.Resources)
		}
		containers = append(containers, container)
	}
	if len(containers) > 0 {
		spec["containers"] = containers
	}
	patch := Patch{}
	if len(metadata) > 0 {
		patch["metadata"] = metadata
	}
	if len(spec) > 0 {
		patch["spec"] = spec
	}
	return patch
}

func stringMapToAny(entries map[string]string) map[string]any {
	raw := make(map[string]any, len(entries))
	for key, value := range entries {
		raw[key] = value
	}
	return raw
}

// mustRaw converts a typed corev1 value to raw JSON shape. The types are
// always marshalable, so a failure is a programming error.
func mustRaw(value any) any {
	raw, err := toRaw(value)
	if err != nil {
		panic(err)
	}
	return raw
}

// ContainerPatch is a partial update of one named container.
type ContainerPatch struct {
	Name      string
	Image     *string
	Resources *corev1.ResourceRequirements
}

// UpdateOptions is the resolved form of a set of UpdateOption values.
type UpdateOptions struct {
	// Instances limits the update to these instance ids; nil means all.
	Instances []string
}

// UpdateOption configures one UpdatePodTemplate call.
type UpdateOption func(*UpdateOptions)

// ResolveUpdateOptions folds opts into their resolved form. Exposed so fakes
// and decorators interpret options exactly like the real implementation.
func ResolveUpdateOptions(opts ...UpdateOption) UpdateOptions {
	options := UpdateOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// WithInstances limits the update to the given instance ids of a
// multi-instance component. The default is all instances. Unknown ids fail
// before any write.
func WithInstances(ids ...string) UpdateOption {
	return func(o *UpdateOptions) { o.Instances = append(o.Instances, ids...) }
}

// PodField names one routable pod field as its logical path in the pod
// template view - the same strings UnsupportedFieldsError carries for raw
// patches, so both doors report capability failures in one vocabulary.
type PodField string

const (
	PodFieldSchedulerName     PodField = "spec.schedulerName"
	PodFieldPriorityClassName PodField = "spec.priorityClassName"
	PodFieldLabels            PodField = "metadata.labels"
	PodFieldAnnotations       PodField = "metadata.annotations"
	PodFieldNodeAffinity      PodField = "spec.affinity.nodeAffinity"
	PodFieldPodAffinity       PodField = "spec.affinity.podAffinity"
	PodFieldResourceClaims    PodField = "spec.resourceClaims"
	PodFieldImage             PodField = "spec.containers.image"
	PodFieldResources         PodField = "spec.containers.resources"
	PodFieldContainers        PodField = "spec.containers"
)

// SetFields lists the fields the patch sets, in stable order. Exposed so
// fakes and tooling classify a patch exactly like the real implementation.
func (p PodPatch) SetFields() []PodField {
	var fields []PodField
	if p.SchedulerName != nil {
		fields = append(fields, PodFieldSchedulerName)
	}
	if p.PriorityClassName != nil {
		fields = append(fields, PodFieldPriorityClassName)
	}
	if len(p.Labels) > 0 {
		fields = append(fields, PodFieldLabels)
	}
	if len(p.Annotations) > 0 {
		fields = append(fields, PodFieldAnnotations)
	}
	if p.NodeAffinity != nil {
		fields = append(fields, PodFieldNodeAffinity)
	}
	if p.PodAffinity != nil {
		fields = append(fields, PodFieldPodAffinity)
	}
	if len(p.ResourceClaims) > 0 {
		fields = append(fields, PodFieldResourceClaims)
	}
	if p.Image != nil {
		fields = append(fields, PodFieldImage)
	}
	if p.Resources != nil {
		fields = append(fields, PodFieldResources)
	}
	if len(p.Containers) > 0 {
		fields = append(fields, PodFieldContainers)
	}
	return fields
}

type podShape int

const (
	shapeNone podShape = iota
	shapeTemplate
	shapePodSpec
	shapeSplit
	shapeFragmented
)

func specShape(def v1alpha1.ComponentDefinition) podShape {
	spec := def.SpecDefinition
	switch {
	case spec == nil:
		return shapeNone
	case spec.PodTemplateSpecPath != nil:
		return shapeTemplate
	case spec.PodSpecPath != nil && spec.MetadataPath != nil:
		return shapeSplit
	case spec.PodSpecPath != nil:
		return shapePodSpec
	case spec.FragmentedPodSpecDefinition != nil:
		return shapeFragmented
	default:
		return shapeNone
	}
}

// WritablePodFields reports which PodPatch fields the component definition can
// route. A field is writable only when a route exists AND every jq path on
// that route is a statically assignable pure path - computed projections,
// formulas and pipe expressions are read-only. The typed vocabulary never
// routes through containerPath. Pure function of the definition; the real implementation, the
// kartatest fake and consumers all share it as the one source of capability
// truth.
func WritablePodFields(def v1alpha1.ComponentDefinition) []PodField {
	spec := def.SpecDefinition
	switch specShape(def) {
	case shapeTemplate:
		if !isWritablePath(*spec.PodTemplateSpecPath) {
			return nil
		}
		return []PodField{
			PodFieldSchedulerName, PodFieldPriorityClassName,
			PodFieldLabels, PodFieldAnnotations,
			PodFieldNodeAffinity, PodFieldPodAffinity,
			PodFieldResourceClaims, PodFieldImage, PodFieldResources,
			PodFieldContainers,
		}
	case shapePodSpec, shapeSplit:
		if !isWritablePath(*spec.PodSpecPath) {
			return nil
		}
		fields := []PodField{
			PodFieldSchedulerName, PodFieldPriorityClassName,
			PodFieldNodeAffinity, PodFieldPodAffinity,
			PodFieldResourceClaims, PodFieldImage, PodFieldResources,
			PodFieldContainers,
		}
		if specShape(def) == shapeSplit && isWritablePath(*spec.MetadataPath) {
			fields = append(fields, PodFieldLabels, PodFieldAnnotations)
		}
		return fields
	case shapeFragmented:
		fragmented := spec.FragmentedPodSpecDefinition
		writable := func(path *string) bool {
			return path != nil && isWritablePath(*path)
		}
		var fields []PodField
		if writable(fragmented.SchedulerNamePath) {
			fields = append(fields, PodFieldSchedulerName)
		}
		if writable(fragmented.PriorityClassNamePath) {
			fields = append(fields, PodFieldPriorityClassName)
		}
		if writable(fragmented.LabelsPath) {
			fields = append(fields, PodFieldLabels)
		}
		if writable(fragmented.AnnotationsPath) {
			fields = append(fields, PodFieldAnnotations)
		}
		if writable(fragmented.NodeAffinityPath) {
			fields = append(fields, PodFieldNodeAffinity)
		}
		if writable(fragmented.PodAffinityPath) {
			fields = append(fields, PodFieldPodAffinity)
		}
		if writable(fragmented.ResourceClaimsPath) {
			fields = append(fields, PodFieldResourceClaims)
		}
		if writable(fragmented.ImagePath) {
			fields = append(fields, PodFieldImage)
		}
		if writable(fragmented.ResourcesPath) {
			fields = append(fields, PodFieldResources)
		}
		if writable(fragmented.ContainersPath) {
			fields = append(fields, PodFieldContainers)
		}
		return fields
	default:
		return nil
	}
}
