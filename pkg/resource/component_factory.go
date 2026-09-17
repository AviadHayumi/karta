// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package resource

import (
	"context"
	"errors"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	celpkg "github.com/run-ai/karta/pkg/cel"
	"github.com/run-ai/karta/pkg/expression"
	"github.com/run-ai/karta/pkg/references"
)

type ComponentReader interface {
	ExtractPodTemplateSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodTemplateSpec, error)
	ExtractPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodSpec, error)
	ExtractPodMetadata(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]metav1.ObjectMeta, error)
	ExtractFragmentedPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]FragmentedPodSpec, error)
	ExtractScale(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]Scale, error)
	ExtractStatus(ctx context.Context, definition v1alpha1.ComponentDefinition) (*Status, error)
	ExtractInstanceIds(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]string, error)
	GetObject() (map[string]interface{}, error)
}

type ComponentWriter interface {
	UpdatePodTemplateSpec(ctx context.Context, definition v1alpha1.ComponentDefinition, podTemplateSpecs []corev1.PodTemplateSpec) error
	UpdatePodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition, podSpecs []corev1.PodSpec) error
	UpdatePodMetadata(ctx context.Context, definition v1alpha1.ComponentDefinition, podMetadata []metav1.ObjectMeta) error
	UpdateFragmentedPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition, fragmentedPodSpecs []FragmentedPodSpec) error
	ApplySuspendActions(ctx context.Context, definition v1alpha1.ComponentDefinition) error
	ApplyResumeActions(ctx context.Context, definition v1alpha1.ComponentDefinition) error
}

// PathWriter resolves definition-owned destinations and applies caller-owned values.
// A target belongs to the current object snapshot; resolve it again after a write.
type PathWriter interface {
	ResolveWriteTarget(ctx context.Context, via *v1alpha1.ValueAccessor, instance string, index int) (WriteTarget, error)
	WriteValues(ctx context.Context, definition v1alpha1.ComponentDefinition, via *v1alpha1.ValueAccessor, values []any, options MutationOptions) error
}

//go:generate go run go.uber.org/mock/mockgen -source=component_factory.go -destination=accessor_mock.go -package=resource ComponentAccessor
type ComponentAccessor interface {
	ComponentReader
	ComponentWriter
}

type ComponentFactory struct {
	karta    *v1alpha1.Karta
	accessor ComponentAccessor
	settings *objectFactorySettings

	componentDefinitionsByName map[string]v1alpha1.ComponentDefinition
	childNamesByParent         map[string][]string
}

// objectFactorySettings stays immutable across forks. The provider shares a
// successful reference snapshot so a staged write cannot fetch a different target.
type objectFactorySettings struct {
	mutationOptions   MutationOptions
	referenceProvider func(context.Context) (map[string]any, error)
}

// NewComponentFactory creates a new Karta-based component factory
func NewComponentFactory(karta *v1alpha1.Karta, accessor ComponentAccessor) *ComponentFactory {
	definitionsByName := make(map[string]v1alpha1.ComponentDefinition)
	childNamesByParent := make(map[string][]string)

	// Create single slice with all components (root + children)
	allDefinitions := make([]v1alpha1.ComponentDefinition, 0, len(karta.Spec.StructureDefinition.ChildComponents)+1)
	allDefinitions = append(allDefinitions, karta.Spec.StructureDefinition.ChildComponents...)
	allDefinitions = append(allDefinitions, karta.Spec.StructureDefinition.RootComponent)
	for _, componentDefinition := range allDefinitions {
		definitionsByName[componentDefinition.Name] = componentDefinition
		if componentDefinition.OwnerRef != nil {
			parent := *componentDefinition.OwnerRef
			childNamesByParent[parent] = append(childNamesByParent[parent], componentDefinition.Name)
		}
	}

	return &ComponentFactory{
		karta:                      karta,
		accessor:                   accessor,
		componentDefinitionsByName: definitionsByName,
		childNamesByParent:         childNamesByParent,
	}
}

// FactoryOption configures NewComponentFactoryFromObject.
type FactoryOption func(*factoryOptions)

type factoryOptions struct {
	resolved        references.ResolvedReferences
	hasResolved     bool
	reader          references.ResourceReader
	mutationOptions MutationOptions
}

// WithMutationOptions selects SDK policy for this factory's typed setters.
// Definitions only supply destinations; they cannot override caller policy.
func WithMutationOptions(options MutationOptions) FactoryOption {
	return func(o *factoryOptions) { o.mutationOptions = options }
}

// WithReferences passes pre-resolved reference values: the consumer fetched them from wherever
// its data lives and Karta only sees the finished values. A nil map means "resolved to nothing":
// every lookup stays unbound, every list is empty.
func WithReferences(resolved references.ResolvedReferences) FactoryOption {
	return func(o *factoryOptions) {
		if resolved == nil {
			resolved = references.ResolvedReferences{}
		}
		o.resolved = resolved
		o.hasResolved = true
	}
}

// WithReferenceReader hands the factory a reader to resolve references with. Resolution is lazy:
// the first expression that reads references.<name> resolves all of them with that call's
// context and memoizes the result, so a definition without references never touches the reader.
func WithReferenceReader(reader references.ResourceReader) FactoryOption {
	return func(o *factoryOptions) { o.reader = reader }
}

// NewComponentFactoryFromObject creates a new Karta-based component factory from a Kubernetes
// object. Expressions are CEL, evaluated against the object bound as `object`.
func NewComponentFactoryFromObject(karta *v1alpha1.Karta, object KubernetesObject, opts ...FactoryOption) *ComponentFactory {
	var options factoryOptions
	for _, opt := range opts {
		opt(&options)
	}
	if options.hasResolved && options.reader != nil {
		err := errors.New("both WithReferences and WithReferenceReader were provided; pass exactly one")

		return NewComponentFactory(karta, NewAccessor(errRunner{err}))
	}

	referenceKarta := karta.DeepCopy()
	var referenceObject any
	var referenceObjectError error
	if options.reader != nil {
		// Normalize first: callers may put Go ints in an unstructured object,
		// which Kubernetes DeepCopyObject rejects with a panic.
		referenceObject, referenceObjectError = jsonCopy(object)
	}
	var suppliedBindings map[string]any
	var suppliedError error
	if options.hasResolved {
		suppliedBindings, suppliedError = withDeclaredLists(options.resolved, referenceKarta).Bindings()
	}
	var referenceMu sync.Mutex
	var referenceBindings map[string]any
	provider := func(ctx context.Context) (map[string]any, error) {
		referenceMu.Lock()
		defer referenceMu.Unlock()
		if referenceBindings != nil {
			copy, err := jsonCopy(referenceBindings)
			if err != nil {
				return nil, err
			}
			return copy.(map[string]any), nil
		}
		var bindings map[string]any
		switch {
		case options.hasResolved:
			if suppliedError != nil {
				return nil, suppliedError
			}
			bindings = suppliedBindings
		case options.reader != nil:
			if referenceObjectError != nil {
				return nil, fmt.Errorf("snapshot reference workload: %w", referenceObjectError)
			}
			resolved, err := references.Resolve(ctx, options.reader, referenceKarta, referenceObject)
			if err != nil {
				return nil, err
			}
			bindings, err = resolved.Bindings()
			if err != nil {
				return nil, err
			}
		default:
			return nil, expression.ErrReferencesNotSupported
		}
		if bindings == nil {
			bindings = map[string]any{}
		}
		referenceBindings = bindings
		copy, err := jsonCopy(bindings)
		if err != nil {
			return nil, err
		}
		return copy.(map[string]any), nil
	}

	return newObjectFactory(karta, object, &objectFactorySettings{
		mutationOptions:   options.mutationOptions,
		referenceProvider: provider,
	})
}

func newObjectFactory(karta *v1alpha1.Karta, object KubernetesObject, settings *objectFactorySettings) *ComponentFactory {
	celRunner, err := celpkg.NewRunnerWithVariables(object, namedExpressions(karta),
		celpkg.WithReferenceProvider(settings.referenceProvider))
	if err != nil {
		// Nothing may silently evaluate against the wrong document, so every call reports
		// the construction error instead.
		return NewComponentFactory(karta, NewAccessor(errRunner{err}))
	}

	factory := NewComponentFactory(karta, NewAccessor(celRunner, settings.mutationOptions))
	factory.settings = settings
	return factory
}

// Fork creates a detached workload and definition for staging mutations. Reference
// values are a shared, lazy snapshot, not a fresh read of external resources.
// Factories constructed with a custom accessor cannot be rebuilt safely.
func (f *ComponentFactory) Fork() (*ComponentFactory, error) {
	if f.settings == nil {
		return nil, errors.New("fork is unsupported for custom-accessor factories; use NewComponentFactoryFromObject")
	}
	if f.karta == nil {
		return nil, errors.New("cannot fork a nil Karta definition")
	}
	object, err := f.GetResource()
	if err != nil {
		return nil, fmt.Errorf("fork current workload: %w", err)
	}
	copy, ok := object.DeepCopyObject().(KubernetesObject)
	if !ok {
		return nil, errors.New("fork current workload: copied object does not implement KubernetesObject")
	}
	return newObjectFactory(f.karta.DeepCopy(), copy, f.settings), nil
}

// PathWriter returns the optional path-based SDK supported by this accessor.
func (f *ComponentFactory) PathWriter() (PathWriter, error) {
	writer, ok := f.accessor.(PathWriter)
	if !ok {
		return nil, errors.New("the component accessor does not support path-based writes")
	}
	return writer, nil
}

// MutationOptions returns the SDK defaults configured for this factory.
// Zero-valued fields use the standard SDK defaults when a write is applied.
func (f *ComponentFactory) MutationOptions() MutationOptions {
	if f.settings == nil {
		return MutationOptions{}
	}
	return f.settings.mutationOptions
}

// namedExpressions converts the definition's variables into the engine's shape.
func namedExpressions(karta *v1alpha1.Karta) []expression.NamedExpression {
	if karta == nil {
		return nil
	}
	out := make([]expression.NamedExpression, 0, len(karta.Spec.Variables))
	for _, variable := range karta.Spec.Variables {
		out = append(out, expression.NamedExpression{Name: variable.Name, Expression: variable.Expression})
	}

	return out
}

// errRunner is the runner a definition gets when the engine cannot be built: every operation
// returns the construction error instead of evaluating against the wrong document.
type errRunner struct{ err error }

func (r errRunner) Evaluate(context.Context, string) ([]any, error) { return nil, r.err }
func (r errRunner) Assign(context.Context, string, any) error       { return r.err }
func (r errRunner) GetObject() (any, error)                         { return nil, r.err }
func (r errRunner) EvaluateWithVariables(context.Context, string, map[string]any) ([]any, error) {
	return nil, r.err
}
func (r errRunner) EvaluateWritePath(context.Context, string, map[string]any) ([]any, error) {
	return nil, r.err
}
func (r errRunner) ResolveVariables(context.Context, ...string) (map[string]any, error) {
	return nil, r.err
}

// GetComponent retrieves a component by name
func (f *ComponentFactory) GetComponent(name string) (*Component, error) {
	definition, exists := f.componentDefinitionsByName[name]
	if !exists {
		return nil, fmt.Errorf("component %s not found", name)
	}

	return &Component{
		name:       name,
		definition: definition,
		accessor:   f.accessor,
	}, nil
}

// GetKarta returns the Karta definition the factory was built from.
func (f *ComponentFactory) GetKarta() *v1alpha1.Karta {
	return f.karta
}

// GetRootComponent retrieves the root component
func (f *ComponentFactory) GetRootComponent() (*Component, error) {
	if f.karta == nil {
		return nil, fmt.Errorf("karta is nil")
	}

	return f.GetComponent(f.karta.Spec.StructureDefinition.RootComponent.Name)
}

// GetChildComponentsOf retrieves the direct child components of the named parent,
// resolved from the structure's OwnerRef relationships.
func (f *ComponentFactory) GetChildComponentsOf(parentName string) ([]*Component, error) {
	names := f.childNamesByParent[parentName]
	children := make([]*Component, 0, len(names))
	for _, name := range names {
		component, err := f.GetComponent(name)
		if err != nil {
			return nil, fmt.Errorf("failed to get child component %s: %w", name, err)
		}
		children = append(children, component)
	}
	return children, nil
}

// GetChildComponents retrieves all child components
func (f *ComponentFactory) GetChildComponents() ([]*Component, error) {
	if f.karta == nil {
		return nil, fmt.Errorf("karta is nil")
	}

	childComponents := make([]*Component, 0, len(f.karta.Spec.StructureDefinition.ChildComponents))
	for _, childDefinition := range f.karta.Spec.StructureDefinition.ChildComponents {
		component, err := f.GetComponent(childDefinition.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get child component %s: %w", childDefinition.Name, err)
		}
		childComponents = append(childComponents, component)
	}

	return childComponents, nil
}

func (f *ComponentFactory) GetResource() (KubernetesObject, error) {
	object, err := f.accessor.GetObject()
	if err != nil {
		return nil, fmt.Errorf("failed to get updated data: %w", err)
	}
	u := &unstructured.Unstructured{Object: object}
	if err := validateKubernetesObject(u); err != nil {
		return nil, fmt.Errorf("invalid Kubernetes object: %w", err)
	}
	return u, nil
}

func (f *ComponentFactory) IsContainSpecDefinition() (bool, error) {
	components, err := f.GetChildComponents()
	if err != nil {
		return false, fmt.Errorf("failed to get child components: %w", err)
	}
	rootComponent, err := f.GetRootComponent()
	if err != nil {
		return false, fmt.Errorf("failed to get root component: %w", err)
	}
	components = append(components, rootComponent)
	for _, component := range components {
		if isComponentHasSpecDefinition(component.Definition()) {
			return true, nil
		}
	}
	return false, nil
}

func isComponentHasSpecDefinition(componentDefinition v1alpha1.ComponentDefinition) bool {
	if componentDefinition.SpecDefinition == nil {
		return false
	}

	return componentDefinition.SpecDefinition.PodTemplateSpec != nil ||
		componentDefinition.SpecDefinition.PodSpec != nil ||
		componentDefinition.SpecDefinition.FragmentedPodSpecDefinition != nil
}

func validateKubernetesObject(u *unstructured.Unstructured) error {
	gvk := u.GroupVersionKind()
	if gvk.Group == "" && gvk.Version == "" { // Core groups might have empty Group, but need Version
		return fmt.Errorf("missing apiVersion")
	}
	if gvk.Kind == "" {
		return fmt.Errorf("missing kind")
	}
	if u.GetName() == "" && u.GetGenerateName() == "" {
		return fmt.Errorf("missing metadata.name or metadata.generateName")
	}
	return nil
}

// withDeclaredLists fills every declared list reference the consumer did not resolve with an
// empty list, so references.<name>.size() reads zero instead of failing. A missing lookup stays
// unbound by design: only an expression that reads it fails.
func withDeclaredLists(resolved references.ResolvedReferences, karta *v1alpha1.Karta) references.ResolvedReferences {
	filled := make(references.ResolvedReferences, len(resolved))
	for name, value := range resolved {
		filled[name] = value
	}
	for _, ref := range karta.Spec.StructureDefinition.References {
		if _, ok := filled[ref.Name]; !ok && ref.List != nil {
			filled[ref.Name] = references.NewListValue(nil)
		}
	}

	return filled
}
