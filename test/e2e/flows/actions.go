// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	"github.com/run-ai/karta/test/e2e/recorder"
)

// Flow actions: each builds a recorder.Action, the merge-patch a flow fires to drive a transition the
// operator will not make itself.

// Suspend sets spec.suspend so a running workload pauses.
func Suspend() *recorder.Action {
	return &recorder.Action{Type: recorder.ActionSuspend, Patch: []byte(`{"spec":{"suspend":true}}`)}
}
