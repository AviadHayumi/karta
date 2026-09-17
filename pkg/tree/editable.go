// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
)

// Editable owns a local workload snapshot. No method writes to Kubernetes.
// Mutate resolves all destinations against the starting snapshot, then publishes
// the new workload and extracted tree together only if every write succeeds.
type Editable interface {
	resource.Suspendable
	Snapshot() *WorkloadTree
	GetResource() (resource.KubernetesObject, error)
	ResolveWriteTarget(context.Context, Target) (resource.WriteTarget, error)
	Mutate(context.Context, ...Write) error
}

// Field identifies a built-in accessor or a name declared in ComponentDefinition.Fields.
type Field string

const (
	PodTemplateSpec   Field = "podTemplateSpec"
	PodSpec           Field = "podSpec"
	Metadata          Field = "metadata"
	SchedulerName     Field = "fragmented.schedulerName"
	LabelsField       Field = "fragmented.labels"
	Annotations       Field = "fragmented.annotations"
	Resources         Field = "fragmented.resources"
	ResourceClaims    Field = "fragmented.resourceClaims"
	PodAffinity       Field = "fragmented.podAffinity"
	NodeAffinity      Field = "fragmented.nodeAffinity"
	Containers        Field = "fragmented.containers"
	Container         Field = "fragmented.container"
	PriorityClassName Field = "fragmented.priorityClassName"
	Image             Field = "fragmented.image"
	Replicas          Field = "scale.replicas"
	MinReplicas       Field = "scale.minReplicas"
	MaxReplicas       Field = "scale.maxReplicas"
	SuspendField      Field = "suspend"
)

var ErrNotSuspendable = errors.New("root component does not support suspension")

// Target uses the catalog component name and stable instance ID, never a tree index.
// Instance is empty for a single-instance component.
type Target struct {
	Component string
	Instance  string
	Field     Field
}

// Write supplies the value and optional SDK policy. A partial map with Merge
// preserves unmentioned map members. Arrays are replaced, not merged by name.
type Write struct {
	Component string
	Instance  string
	Field     Field
	Value     any
	Options   resource.MutationOptions
}

type editableTree struct {
	mu       sync.RWMutex
	factory  *resource.ComponentFactory
	snapshot *WorkloadTree
	revision uint64
}

// Open copies the inputs and extracts the initial tree, including its root.
func Open(ctx context.Context, karta *v1alpha1.Karta, workload resource.KubernetesObject, options ...resource.FactoryOption) (Editable, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if karta == nil || workload == nil {
		return nil, errors.New("karta and workload are required")
	}
	// Normalize Go numbers before using Kubernetes' unstructured deep copier,
	// which panics on otherwise JSON-serializable int and int32 values.
	encoded, err := json.Marshal(workload)
	if err != nil {
		return nil, fmt.Errorf("copy workload: %w", err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		return nil, fmt.Errorf("copy workload: %w", err)
	}
	if object == nil {
		return nil, errors.New("workload must be a non-null object")
	}
	factory := resource.NewComponentFactoryFromObject(karta.DeepCopy(), &unstructured.Unstructured{Object: object}, options...)
	if _, err := factory.GetResource(); err != nil {
		return nil, err
	}
	snapshot, err := Build(ctx, factory)
	if err != nil {
		return nil, err
	}
	return &editableTree{factory: factory, snapshot: snapshot}, nil
}

// Snapshot returns a detached view. Mutating it does not edit the workload.
func (t *editableTree) Snapshot() *WorkloadTree {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return cloneWorkloadTree(t.snapshot)
}

func cloneWorkloadTree(snapshot *WorkloadTree) *WorkloadTree {
	out := &WorkloadTree{Children: cloneComponentNodes(snapshot.Children)}
	if snapshot.Root != nil {
		out.Root = &cloneComponentNodes([]ComponentNode{*snapshot.Root})[0]
	}
	if snapshot.Status != nil {
		out.Status = &WorkloadStatus{Phases: append([]string{}, snapshot.Status.Phases...)}
	}
	return out
}

func (t *editableTree) GetResource() (resource.KubernetesObject, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	object, err := t.factory.GetResource()
	if err != nil {
		return nil, err
	}
	copy, ok := object.DeepCopyObject().(resource.KubernetesObject)
	if !ok {
		return nil, errors.New("copied workload is not a KubernetesObject")
	}
	return copy, nil
}

// ResolveWriteTarget returns a detached value and a path for the current snapshot.
// Resolve again after a mutation: selectors and array positions can change.
func (t *editableTree) ResolveWriteTarget(ctx context.Context, target Target) (resource.WriteTarget, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return resource.WriteTarget{}, err
	}
	return resolveTarget(ctx, t.factory, target)
}

func (t *editableTree) Mutate(ctx context.Context, writes ...Write) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.mutate(ctx, writes)
}

func (t *editableTree) mutate(ctx context.Context, writes []Write) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(writes) == 0 {
		return nil
	}
	staged, err := t.factory.Fork()
	if err != nil {
		return err
	}
	writer, err := staged.PathWriter()
	if err != nil {
		return err
	}
	targets := make([]resource.WriteTarget, len(writes))
	for i, write := range writes {
		targets[i], err = resolveTarget(ctx, staged, Target{Component: write.Component, Instance: write.Instance, Field: write.Field})
		if err != nil {
			return fmt.Errorf("write %d: %w", i, err)
		}
		for j := 0; j < i; j++ {
			// Valid JSON Pointers have one canonical spelling per segment, so a
			// slash boundary distinguishes ancestors from similarly named keys.
			a, b := targets[j].Path, targets[i].Path
			if pathsOverlap(a, b) {
				return fmt.Errorf("overlapping write targets %q and %q; use separate Mutate calls", a, b)
			}
		}
	}
	for i, write := range writes {
		options := staged.MutationOptions()
		if write.Options.PatchType != "" {
			options.PatchType = write.Options.PatchType
		}
		if write.Options.Strategy != "" {
			options.Strategy = write.Options.Strategy
		}
		if write.Options.Parents != "" {
			options.Parents = write.Options.Parents
		}
		via := &v1alpha1.ValueAccessor{PathWrite: &targets[i].Path}
		if err := writer.WriteValues(ctx, v1alpha1.ComponentDefinition{}, via, []any{write.Value}, options); err != nil {
			return fmt.Errorf("write %d at %q: %w", i, targets[i].Path, err)
		}
	}
	// Replacing a root-metadata accessor must not silently drop Kubernetes identity.
	if _, err := staged.GetResource(); err != nil {
		return fmt.Errorf("invalid mutated workload: %w", err)
	}
	snapshot, err := Build(ctx, staged)
	if err != nil {
		return fmt.Errorf("extract mutated workload: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	t.factory, t.snapshot = staged, snapshot
	t.revision++
	return nil
}

func (t *editableTree) IsSuspendable() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.factory.GetKarta().Spec.StructureDefinition.RootComponent.SuspendDefinition != nil
}

func (t *editableTree) Suspend(ctx context.Context) error { return t.setSuspended(ctx, true) }
func (t *editableTree) Resume(ctx context.Context) error  { return t.setSuspended(ctx, false) }

func (t *editableTree) setSuspended(ctx context.Context, value bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	component, err := t.factory.GetRootComponent()
	if err != nil {
		return err
	}
	if !component.IsSuspendable() {
		return ErrNotSuspendable
	}
	ids, err := component.GetInstanceIds(ctx)
	if err != nil {
		return err
	}
	writes := make([]Write, len(ids))
	for i, id := range ids {
		writes[i] = Write{Component: component.Name(), Instance: id, Field: SuspendField, Value: value}
	}
	return t.mutate(ctx, writes)
}

func resolveTarget(ctx context.Context, factory *resource.ComponentFactory, target Target) (resource.WriteTarget, error) {
	component, err := factory.GetComponent(target.Component)
	if err != nil {
		return resource.WriteTarget{}, err
	}
	ids, err := component.GetInstanceIds(ctx)
	if err != nil {
		return resource.WriteTarget{}, err
	}
	index := -1
	for i, id := range ids {
		if id == target.Instance {
			index = i
			break
		}
	}
	if index < 0 {
		return resource.WriteTarget{}, fmt.Errorf("component %s has no instance %q", target.Component, target.Instance)
	}
	via := fieldAccessor(component.Definition(), target.Field)
	if via == nil {
		return resource.WriteTarget{}, fmt.Errorf("component %s has no field %q", target.Component, target.Field)
	}
	writer, err := factory.PathWriter()
	if err != nil {
		return resource.WriteTarget{}, err
	}
	return writer.ResolveWriteTarget(ctx, via, target.Instance, index)
}

func fieldAccessor(def v1alpha1.ComponentDefinition, field Field) *v1alpha1.ValueAccessor {
	if accessor, ok := def.Fields[string(field)]; ok {
		return &accessor
	}
	if field == SuspendField && def.SuspendDefinition != nil {
		return &v1alpha1.ValueAccessor{PathWrite: def.SuspendDefinition.PathWrite, PathWriteExpression: def.SuspendDefinition.PathWriteExpression}
	}
	if scale := def.ScaleDefinition; scale != nil {
		switch field {
		case Replicas:
			return scale.Replicas
		case MinReplicas:
			return scale.MinReplicas
		case MaxReplicas:
			return scale.MaxReplicas
		}
	}
	spec := def.SpecDefinition
	if spec == nil {
		return nil
	}
	switch field {
	case PodTemplateSpec:
		return spec.PodTemplateSpec
	case PodSpec:
		return spec.PodSpec
	case Metadata:
		return spec.Metadata
	}
	f := spec.FragmentedPodSpecDefinition
	if f == nil {
		return nil
	}
	switch field {
	case SchedulerName:
		return f.SchedulerName
	case LabelsField:
		return f.Labels
	case Annotations:
		return f.Annotations
	case Resources:
		return f.Resources
	case ResourceClaims:
		return f.ResourceClaims
	case PodAffinity:
		return f.PodAffinity
	case NodeAffinity:
		return f.NodeAffinity
	case Containers:
		return f.Containers
	case Container:
		return f.Container
	case PriorityClassName:
		return f.PriorityClassName
	case Image:
		return f.Image
	default:
		return nil
	}
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneScale(scale *resource.Scale) *resource.Scale {
	if scale == nil {
		return nil
	}
	return &resource.Scale{Replicas: clonePointer(scale.Replicas), MinReplicas: clonePointer(scale.MinReplicas), MaxReplicas: clonePointer(scale.MaxReplicas)}
}

func cloneExtracted(value *resource.ExtractedInstance) *resource.ExtractedInstance {
	if value == nil {
		return nil
	}
	copy := *value
	copy.PodTemplateSpec = value.PodTemplateSpec.DeepCopy()
	copy.PodSpec = value.PodSpec.DeepCopy()
	copy.Metadata = value.Metadata.DeepCopy()
	copy.Scale = cloneScale(value.Scale)
	if value.Fields != nil {
		copy.Fields = runtime.DeepCopyJSONValue(value.Fields).(map[string]any)
	}
	if f := value.FragmentedPodSpec; f != nil {
		fragment := *f
		fragment.Labels, fragment.Annotations = maps.Clone(f.Labels), maps.Clone(f.Annotations)
		fragment.Resources = f.Resources.DeepCopy()
		fragment.PodAffinity, fragment.NodeAffinity = f.PodAffinity.DeepCopy(), f.NodeAffinity.DeepCopy()
		fragment.Container = f.Container.DeepCopy()
		if f.Containers != nil {
			fragment.Containers = make([]corev1.Container, len(f.Containers))
			for i := range f.Containers {
				f.Containers[i].DeepCopyInto(&fragment.Containers[i])
			}
		}
		if f.ResourceClaims != nil {
			fragment.ResourceClaims = make([]corev1.PodResourceClaim, len(f.ResourceClaims))
			for i := range f.ResourceClaims {
				f.ResourceClaims[i].DeepCopyInto(&fragment.ResourceClaims[i])
			}
		}
		copy.FragmentedPodSpec = &fragment
	}
	return &copy
}
