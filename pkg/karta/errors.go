// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package karta

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotSupported is the permanent sentinel for operations the workload's
// Karta definition cannot express. errors.Is(err, ErrNotSupported) is the one
// check consumers write; typed detail stays available via errors.As.
var ErrNotSupported = errors.New("karta: not supported by the workload's karta definition")

// ErrEmptyPatch reports a PodPatch that sets no fields, which is almost
// always a lost pointer on the caller side rather than an intended no-op.
var ErrEmptyPatch = errors.New("karta: pod patch sets no fields")

// UnsupportedFieldsError reports every PodPatch field the component's
// definition cannot route. It is returned before any mutation happens; the
// underlying object is unchanged.
type UnsupportedFieldsError struct {
	Component string
	Fields    []PodField
}

func (e *UnsupportedFieldsError) Error() string {
	names := make([]string, len(e.Fields))
	for i, field := range e.Fields {
		names[i] = string(field)
	}
	return fmt.Sprintf("karta: component %q cannot route pod patch fields [%s]: %s",
		e.Component, strings.Join(names, ", "), ErrNotSupported)
}

func (e *UnsupportedFieldsError) Is(target error) bool { return target == ErrNotSupported }

// IsUnsupportedFields reports whether err carries an *UnsupportedFieldsError.
func IsUnsupportedFields(err error) bool {
	var unsupported *UnsupportedFieldsError
	return errors.As(err, &unsupported)
}

// UnsupportedOperationError reports a verb the workload's definition does not
// support, such as suspend when no component declares a SuspendDefinition.
type UnsupportedOperationError struct {
	Op string
}

func (e *UnsupportedOperationError) Error() string {
	return fmt.Sprintf("karta: no component supports %s: %s", e.Op, ErrNotSupported)
}

func (e *UnsupportedOperationError) Is(target error) bool { return target == ErrNotSupported }

// IsUnsupportedOperation reports whether err carries an *UnsupportedOperationError.
func IsUnsupportedOperation(err error) bool {
	var unsupported *UnsupportedOperationError
	return errors.As(err, &unsupported)
}
