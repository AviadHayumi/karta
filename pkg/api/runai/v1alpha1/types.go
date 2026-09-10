// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Karta
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName={krt}
// +kubebuilder:printcolumn:name="Framework",type="string",JSONPath=".spec.structureDefinition.rootComponent.kind.kind",description="Target framework kind"
// +kubebuilder:printcolumn:name="Root Component",type="string",JSONPath=".spec.structureDefinition.rootComponent.name",description="Root component name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Karta struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KartaSpec   `json:"spec,omitempty"`
	Status KartaStatus `json:"status,omitempty"`
}

type KartaSpec struct {
	// StructureDefinition defines the compute hierarchy and component relationships
	// +kubebuilder:validation:Required
	StructureDefinition StructureDefinition `json:"structureDefinition"`

	// Instructions contains optimization-specific instructions for the workload
	// +kubebuilder:validation:Optional
	Instructions OptimizationInstructions `json:"optimizationInstructions"`

	// Variables are named expressions available to every expression in the definition as
	// variables.<name> - the composition mechanism a ValidatingAdmissionPolicy has, for breaking
	// a complex expression into named parts and evaluating a shared part once per read or write.
	// Variables are evaluated in order, and a later variable may reference an earlier one.
	// +optional
	// +listType=atomic
	Variables []Variable `json:"variables,omitempty"`
}

// StructureDefinition defines the hierarchical structure of components in the workload.
type StructureDefinition struct {
	// RootComponent defines the top-level component of the workload hierarchy
	// +kubebuilder:validation:Required
	RootComponent ComponentDefinition `json:"rootComponent"`

	// ChildComponents defines the child components in the hierarchy
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	ChildComponents []ComponentDefinition `json:"childComponents,omitempty"`

	// AdditionalChildKinds lists Kubernetes kinds that are created/managed by this workload
	// but are not explicitly modeled as components (e.g., Deployments, Services).
	// Required for RBAC purposes, etc.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=kind
	AdditionalChildKinds []GroupVersionKind `json:"additionalChildKinds,omitempty"`

	// References declares cluster resources whose values are exposed to the
	// workload's expressions as references.<name>. Karta fetches nothing itself:
	// the consumer resolves each reference and passes the values in.
	// +optional
	// +listType=map
	// +listMapKey=name
	References []ResourceReference `json:"references,omitempty"`
}

// ResourceReference declares another resource (or set of resources) whose values are
// exposed to every component's expressions as references.<name>.
// Exactly one of Lookup or List is set:
//
//	Lookup -> references.<name> is a single object (absent when not found)
//	List   -> references.<name> is a list (possibly empty)
//
// A namespaced reference always resolves in the workload's own namespace; there is
// deliberately no namespace field.
// +kubebuilder:validation:XValidation:rule="has(self.lookup) != has(self.list)",message="exactly one of lookup or list must be set"
type ResourceReference struct {
	// Name is the variable name the reference is exposed under, as references.<name>.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// GVK is the group/version/kind of the referenced resource(s).
	// +kubebuilder:validation:Required
	GVK GroupVersionKind `json:"gvk"`

	// Lookup fetches a single object by name.
	// +optional
	Lookup *LookupReference `json:"lookup,omitempty"`

	// List fetches a set of objects by a structured label selector.
	// +optional
	List *ListReference `json:"list,omitempty"`
}

// LookupReference fetches a single resource by name.
type LookupReference struct {
	// NameExpression is a CEL expression against the root object resolving the
	// referenced resource's name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	NameExpression string `json:"nameExpression"`
}

// ListReference fetches a set of resources by a structured label selector. After value
// resolution it maps onto a Kubernetes label selector. At least one of MatchLabels or
// MatchExpressions must be set.
type ListReference struct {
	// MatchLabels selects resources whose labels equal each resolved value.
	// +optional
	MatchLabels map[string]LabelValue `json:"matchLabels,omitempty"`

	// MatchExpressions selects resources by label selector requirements.
	// +optional
	// +listType=atomic
	MatchExpressions []LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// LabelSelectorRequirement mirrors the Kubernetes selector requirement, except its
// values may be sourced from the root object.
type LabelSelectorRequirement struct {
	// Key is the label key the requirement applies to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`

	// Operator is the requirement's relationship to its values.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=In;NotIn;Exists;DoesNotExist
	Operator LabelSelectorOperator `json:"operator"`

	// Values are required for In/NotIn and must be empty for Exists/DoesNotExist.
	// +optional
	// +listType=atomic
	Values []LabelValue `json:"values,omitempty"`
}

// LabelSelectorOperator is the set of operators a selector requirement can use.
type LabelSelectorOperator string

const (
	LabelSelectorOpIn           LabelSelectorOperator = "In"
	LabelSelectorOpNotIn        LabelSelectorOperator = "NotIn"
	LabelSelectorOpExists       LabelSelectorOperator = "Exists"
	LabelSelectorOpDoesNotExist LabelSelectorOperator = "DoesNotExist"
)

// LabelValue is a single label value: either a literal (Value) or a CEL expression
// evaluated against the root object (Expression). Exactly one is set.
// +kubebuilder:validation:XValidation:rule="has(self.value) != has(self.expression)",message="exactly one of value or expression must be set"
type LabelValue struct {
	// Value is a literal label value.
	// +optional
	Value *string `json:"value,omitempty"`

	// Expression is a CEL expression against the root object resolving the value.
	// +optional
	Expression *string `json:"expression,omitempty"`
}

// OptimizationInstructions contains various optimization strategies that can be applied to the workload.
type OptimizationInstructions struct {
	// +kubebuilder:validation:Optional
	GangScheduling *GangSchedulingInstruction `json:"gangScheduling,omitempty"`
}

type KartaStatus struct {
	// +optional
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// KartaList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type KartaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Karta `json:"items"`
}
