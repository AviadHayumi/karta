// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
)

// Draft owns one local edit transaction. Selections and reads refer to its
// starting snapshot; edits apply in caller order to detached raw data. A Draft
// and its Cursors are single-caller objects, not safe for concurrent use.
type Draft struct {
	*draftState
}

// Copies of the public handle must share invalidation and overlap bookkeeping.
type draftState struct {
	owner    *editableTree
	baseline *resource.ComponentFactory
	snapshot *WorkloadTree
	document *editDocument
	original any
	revision uint64
	parents  resource.ParentPolicy
	anchors  []string
	err      error
	closed   bool
}

// Cursor selects raw data, not a typed projection. Errors poison its draft.
type Cursor struct {
	draft *Draft
	node  *editNode
}

type EditOption func(*editOptions)
type editOptions struct{ parents resource.ParentPolicy }
type draftProvider interface {
	beginEdit(context.Context, editOptions) (*Draft, error)
}

var (
	ErrEditUnsupported = errors.New("editable does not support edit drafts")
	ErrStaleDraft      = errors.New("workload changed since the draft began")
	ErrDraftClosed     = errors.New("edit draft is closed")
)

// WithEditParents permits absent map parents when CreateMapParents is selected.
// Existing null/scalar parents and missing arrays are never silently converted.
func WithEditParents(policy resource.ParentPolicy) EditOption {
	return func(options *editOptions) { options.parents = policy }
}

// BeginEdit captures a local snapshot without extending the Editable interface.
// A commit never sends an API request or retries against a newer workload.
func BeginEdit(ctx context.Context, editor Editable, options ...EditOption) (*Draft, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	settings := editOptions{parents: resource.RequireParents}
	for _, option := range options {
		if option == nil {
			return nil, errors.New("nil edit option")
		}
		option(&settings)
	}
	if settings.parents != resource.RequireParents && settings.parents != resource.CreateMapParents {
		return nil, fmt.Errorf("unknown edit parent policy %q", settings.parents)
	}
	provider, ok := editor.(draftProvider)
	if !ok {
		return nil, ErrEditUnsupported
	}
	return provider.beginEdit(ctx, settings)
}

func (t *editableTree) beginEdit(ctx context.Context, options editOptions) (*Draft, error) {
	if t == nil {
		return nil, ErrEditUnsupported
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	baseline, err := t.factory.Fork()
	if err != nil {
		return nil, err
	}
	object, err := baseline.GetResource()
	if err != nil {
		return nil, err
	}
	document, err := newEditDocument(object)
	if err != nil {
		return nil, fmt.Errorf("begin draft: %w", err)
	}
	return &Draft{draftState: &draftState{owner: t, baseline: baseline, snapshot: cloneWorkloadTree(t.snapshot),
		document: document, original: document.value(), revision: t.revision, parents: options.parents}}, nil
}

// Snapshot returns a detached typed view of the starting workload, not staged edits.
func (d *Draft) Snapshot() *WorkloadTree {
	if d.check() != nil {
		return nil
	}
	return cloneWorkloadTree(d.snapshot)
}

// Target resolves against the starting workload. Acquire a target once and share
// its cursor: overlapping independently acquired targets are errors.
func (d *Draft) Target(ctx context.Context, target Target) (*Cursor, error) {
	if err := d.check(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, d.fail(err)
	}
	resolved, err := resolveTarget(ctx, d.baseline, target)
	if err != nil {
		return nil, d.fail(fmt.Errorf("select %s/%s/%s: %w", target.Component, target.Instance, target.Field, err))
	}
	for _, previous := range d.anchors {
		if pathsOverlap(previous, resolved.Path) {
			return nil, d.fail(fmt.Errorf("overlapping draft targets %q and %q; share one cursor", previous, resolved.Path))
		}
	}
	node, err := d.document.atPointer(resolved.Path)
	if err != nil {
		return nil, d.fail(err)
	}
	d.anchors = append(d.anchors, resolved.Path)
	return &Cursor{draft: d, node: node}, nil
}

// Commit publishes workload and extraction together. Any error closes the draft
// without publishing its changes. Cluster persistence remains caller-owned.
func (d *Draft) Commit(ctx context.Context) error {
	if err := d.check(); err != nil {
		return err
	}
	defer func() { d.closed = true }()
	if err := ctx.Err(); err != nil {
		return d.fail(err)
	}
	d.owner.mu.RLock()
	stale := d.owner.revision != d.revision
	d.owner.mu.RUnlock()
	if stale {
		return d.fail(ErrStaleDraft)
	}
	value := d.document.value()
	if reflect.DeepEqual(value, d.original) {
		d.owner.mu.Lock()
		defer d.owner.mu.Unlock()
		if d.owner.revision != d.revision {
			return d.fail(ErrStaleDraft)
		}
		return ctx.Err()
	}
	staged, err := d.baseline.Fork()
	if err != nil {
		return d.fail(err)
	}
	writer, err := staged.PathWriter()
	if err != nil {
		return d.fail(err)
	}
	// This is the retained raw document, not a serialized typed projection.
	// JSONPatch assignment preserves literal nulls in the computed result.
	root := ""
	if err := writer.WriteValues(ctx, v1alpha1.ComponentDefinition{}, &v1alpha1.ValueAccessor{PathWrite: &root}, []any{value},
		resource.MutationOptions{PatchType: resource.PatchTypeJSONPatch, Strategy: resource.Replace, Parents: resource.RequireParents}); err != nil {
		return d.fail(fmt.Errorf("apply draft: %w", err))
	}
	if _, err := staged.GetResource(); err != nil {
		return d.fail(fmt.Errorf("invalid mutated workload: %w", err))
	}
	snapshot, err := Build(ctx, staged)
	if err != nil {
		return d.fail(fmt.Errorf("extract mutated workload: %w", err))
	}
	d.owner.mu.Lock()
	defer d.owner.mu.Unlock()
	if d.owner.revision != d.revision {
		return d.fail(ErrStaleDraft)
	}
	if err := ctx.Err(); err != nil {
		return d.fail(err)
	}
	d.owner.factory, d.owner.snapshot = staged, snapshot
	d.owner.revision++
	return nil
}

// Abort is idempotent and invalidates every cursor without publishing anything.
func (d *Draft) Abort() {
	if d != nil && d.draftState != nil {
		d.closed = true
	}
}

func (d *Draft) check() error {
	if d == nil || d.draftState == nil || d.owner == nil {
		return ErrDraftClosed
	}
	if d.err != nil {
		return d.err
	}
	if d.closed {
		return ErrDraftClosed
	}
	return nil
}

func (d *Draft) fail(err error) error {
	if d != nil && d.draftState != nil && err != nil && d.err == nil {
		d.err, d.closed = err, true
	}
	return err
}

// At selects literal object keys: At("labels", "example.com/team") needs no escaping.
func (c *Cursor) At(keys ...string) *Cursor {
	if err := c.check(); err != nil {
		return c
	}
	node, err := c.node.member(keys...)
	if err != nil {
		_ = c.draft.fail(err)
		return &Cursor{draft: c.draft}
	}
	return &Cursor{draft: c.draft, node: node}
}

// Items returns baseline list handles. Their identity survives moves, but their
// position is not a stable external identifier.
func (c *Cursor) Items() ([]*Cursor, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	nodes, err := c.node.baselineItems()
	if err != nil {
		return nil, c.draft.fail(err)
	}
	items := make([]*Cursor, len(nodes))
	for i, node := range nodes {
		items[i] = &Cursor{draft: c.draft, node: node}
	}
	return items, nil
}

// Match requires one baseline item with the given scalar member. This is an
// explicit selector, not an inferred Kubernetes list identity or merge key.
func (c *Cursor) Match(key string, value any) (*Cursor, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	node, err := c.node.matchBaseline(key, value)
	if err != nil {
		return nil, c.draft.fail(err)
	}
	return &Cursor{draft: c.draft, node: node}, nil
}

// Read returns a detached baseline value and its existence separately.
func (c *Cursor) Read() (any, bool, error) {
	if err := c.check(); err != nil {
		return nil, false, err
	}
	value, exists, err := c.node.readBaseline()
	if err != nil {
		return nil, false, c.draft.fail(err)
	}
	return value, exists, nil
}

// ReadInto decodes only for inspection. It never enables typed writeback.
func (c *Cursor) ReadInto(out any) error {
	value, exists, err := c.Read()
	if err != nil {
		return err
	}
	if !exists {
		return c.draft.fail(errors.New("cannot decode an absent value"))
	}
	encoded, err := json.Marshal(value)
	if err == nil {
		err = json.Unmarshal(encoded, out)
	}
	if err != nil {
		return c.draft.fail(fmt.Errorf("decode draft value: %w", err))
	}
	return nil
}

// Path returns the current staged JSON Pointer. Keep the cursor, not this string,
// when later operations can move list items.
func (c *Cursor) Path() (string, error) {
	if err := c.check(); err != nil {
		return "", err
	}
	path, err := c.node.currentPath()
	if err != nil {
		return "", c.draft.fail(err)
	}
	return path, nil
}

// Set assigns a scalar, including literal null. Use Replace for objects/lists.
func (c *Cursor) Set(value any) error {
	if err := c.check(); err != nil {
		return err
	}
	return c.draft.fail(c.node.assign(value, false, c.draft.parents))
}

// Replace owns the complete selected subtree. Omitted descendants are removed.
func (c *Cursor) Replace(value any) error {
	if err := c.check(); err != nil {
		return err
	}
	return c.draft.fail(c.node.assign(value, true, c.draft.parents))
}

// Remove explicitly deletes the selected member or list item and its descendants.
func (c *Cursor) Remove() error {
	if err := c.check(); err != nil {
		return err
	}
	return c.draft.fail(c.node.remove())
}

// InsertBefore inserts copied JSON into this list. A nil before appends. The
// returned new handle supports direct operations, not baseline child selection.
func (c *Cursor) InsertBefore(before *Cursor, value any) (*Cursor, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	node, err := c.siblingNode(before)
	if err != nil {
		return nil, c.draft.fail(err)
	}
	inserted, err := c.node.insertBefore(node, value)
	if err != nil {
		return nil, c.draft.fail(err)
	}
	return &Cursor{draft: c.draft, node: inserted}, nil
}

// MoveBefore moves the entire raw list item within its list, preserving unknown
// properties. A nil before moves it to the end.
func (c *Cursor) MoveBefore(before *Cursor) error {
	if err := c.check(); err != nil {
		return err
	}
	node, err := c.siblingNode(before)
	if err != nil {
		return c.draft.fail(err)
	}
	return c.draft.fail(c.node.moveBefore(node))
}

func (c *Cursor) siblingNode(other *Cursor) (*editNode, error) {
	if other == nil {
		return nil, nil
	}
	if other.draft == nil || other.draft.draftState != c.draft.draftState {
		return nil, errors.New("cursor belongs to a different draft")
	}
	if err := other.check(); err != nil {
		return nil, err
	}
	return other.node, nil
}

func (c *Cursor) check() error {
	if c == nil || c.draft == nil {
		return ErrDraftClosed
	}
	if err := c.draft.check(); err != nil {
		return err
	}
	if c.node == nil {
		return c.draft.fail(errors.New("cursor has no selected node"))
	}
	return nil
}

func pathsOverlap(a, b string) bool {
	return a == b || a == "" || b == "" || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}
