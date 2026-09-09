// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Flow is a workload journey recorded for testing: the workload manifest plus the ordered steps the workload
// must reach.
type Flow struct {
	rec      *Recorder
	name     string
	manifest string
	journey  []journeyStep
	captures []CapturedReference
}

// CapturedReference names a CR the flow captures alongside every recorded state, so a replay can
// resolve the workload's references against the recording alone. Namespaced objects are read from
// the recorder's namespace; cluster-scoped ones by name.
type CapturedReference struct {
	GVK  kartav1alpha1.GroupVersionKind
	Name string
}

func (c CapturedReference) key() string {
	return c.GVK.Group + "/" + c.GVK.Version + "/" + c.GVK.Kind + "/" + c.Name
}

// Step is one declared journey step, built with Reaches and refined with Optional, With, and Do.
type Step struct {
	step journeyStep
}

// journeyStep is one step of a journey. State borrows Karta's ResourceStatus as the shared vocabulary so the
// replay compares one-to-one; the value is judged from the workload's own fields, never from Karta.
// ReachedWhen lets the same state appear more than once (a scale flow is Running at 1, 3, then 1): the
// step is reached only once the predicate holds, performing its action.
type journeyStep struct {
	State       kartav1alpha1.ResourceStatus
	Action      *Action
	ReachedWhen StateCheck
	Optional    bool // a transient step the workload may miss; the order check tolerates its absence
}

type ActionType string

const (
	ActionSuspend ActionType = "Suspend"
	ActionResume  ActionType = "Resume"
	ActionScale   ActionType = "Scale"
)

// Action is a merge-patch applied to the workload to drive a transition.
type Action struct {
	Type  ActionType
	Patch []byte
}

// StateCheck recognises a state from the workload's own fields, never from Karta.
type StateCheck func(*unstructured.Unstructured) bool

type namedState struct {
	Name  kartav1alpha1.ResourceStatus
	Match StateCheck
}

// NewFlow starts a flow seeded from a manifest (path relative to test/e2e); declare its journey with Through.
func NewFlow(r *Recorder, name, manifest string) *Flow {
	return &Flow{rec: r, name: name, manifest: manifest}
}

// Capturing declares referenced CRs to snapshot with every recorded state. A reference that
// does not exist is simply absent from the recording - the shape a lookup miss replays as.
func (f *Flow) Capturing(refs ...CapturedReference) *Flow {
	f.captures = append(f.captures, refs...)
	return f
}

// Through declares the ordered steps of the journey.
func (f *Flow) Through(steps ...Step) *Flow {
	for _, s := range steps {
		f.journey = append(f.journey, s.step)
	}
	return f
}

// Reaches declares a step the workload must pass through.
func Reaches(state kartav1alpha1.ResourceStatus) Step {
	return Step{step: journeyStep{State: state}}
}

// Optional marks a step the workload may skip; the order check tolerates its absence.
func (s Step) Optional() Step { s.step.Optional = true; return s }

// With gates the step on a predicate over the workload's own fields: the step is reached once the state
// matches and the predicate holds.
func (s Step) With(gate StateCheck) Step { s.step.ReachedWhen = gate; return s }

// Do attaches an action performed once the step is reached.
func (s Step) Do(action *Action) Step { s.step.Action = action; return s }

func (f *Flow) terminalState() kartav1alpha1.ResourceStatus { return f.journey[len(f.journey)-1].State }

func (f *Flow) client() client.Client { return f.rec.config.Cluster.Client }
func (f *Flow) log() io.Writer        { return f.rec.config.Log }

// classify returns the furthest-along state the workload matches, judged from its own fields.
func classify(cr *unstructured.Unstructured, states []namedState) kartav1alpha1.ResourceStatus {
	// States are declared least- to most-advanced, so the walk runs from the end.
	for i := len(states) - 1; i >= 0; i-- {
		if states[i].Match(cr) {
			return states[i].Name
		}
	}
	return ""
}
