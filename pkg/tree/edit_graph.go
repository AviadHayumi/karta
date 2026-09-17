// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/run-ai/karta/pkg/resource"
)

type editDocument struct {
	root *editNode
}

type editNode struct {
	document *editDocument
	parent   *editNode
	key      string
	listItem bool

	baseline         any
	baselineExists   bool
	baselineMembers  map[string]*editNode
	baselineElements []*editNode

	kind             editNodeKind
	exists           bool
	scalar           any
	members          map[string]*editNode
	elements         []*editNode
	invalid          bool
	navigationClosed bool
}

type editNodeKind uint8

const (
	editScalar editNodeKind = iota
	editMap
	editList
	editMaxPathDepth = 128
)

func newEditDocument(value any) (*editDocument, error) {
	value, err := copyEditJSON(value)
	if err != nil {
		return nil, fmt.Errorf("copy edit baseline: %w", err)
	}
	document := &editDocument{}
	document.root = &editNode{document: document}
	document.root.load(value, true)
	return document, nil
}

func (d *editDocument) atPointer(path string) (*editNode, error) {
	if d == nil || d.root == nil {
		return nil, fmt.Errorf("invalid edit document")
	}
	if !utf8.ValidString(path) {
		return nil, fmt.Errorf("edit path must be valid UTF-8")
	}
	if path == "" {
		return d.root, d.root.check()
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("edit path %q must be an RFC 6901 JSON Pointer", path)
	}
	parts := strings.Split(path[1:], "/")
	if len(parts) > editMaxPathDepth {
		return nil, fmt.Errorf("edit path exceeds %d segments", editMaxPathDepth)
	}
	current := d.root
	for _, part := range parts {
		for i := 0; i < len(part); i++ {
			if part[i] != '~' {
				continue
			}
			if i+1 >= len(part) || (part[i+1] != '0' && part[i+1] != '1') {
				return nil, fmt.Errorf("invalid JSON Pointer escape in %q", path)
			}
			i++
		}
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		if err := current.checkNavigation(); err != nil {
			return nil, err
		}
		if _, ok := current.baseline.([]any); current.baselineExists && ok {
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(current.baselineElements) || strconv.Itoa(index) != part {
				return nil, fmt.Errorf("%q is not an existing baseline array index", part)
			}
			current = current.baselineElements[index]
			if err := current.check(); err != nil {
				return nil, err
			}
			continue
		}
		var err error
		current, err = current.member(part)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

func (d *editDocument) value() any {
	if d == nil || d.root == nil {
		return nil
	}
	return d.root.currentValue()
}

func (n *editNode) member(keys ...string) (*editNode, error) {
	if err := n.check(); err != nil {
		return nil, err
	}
	depth := len(keys)
	for parent := n.parent; parent != nil; parent = parent.parent {
		depth++
	}
	if depth > editMaxPathDepth {
		return nil, fmt.Errorf("edit path exceeds %d segments", editMaxPathDepth)
	}
	current := n
	for _, key := range keys {
		// JSON encoding replaces invalid UTF-8. Reject it before selection so
		// an invalid key cannot alias an unrelated replacement-character key.
		if !utf8.ValidString(key) {
			return nil, fmt.Errorf("edit member key must be valid UTF-8")
		}
		if err := current.checkNavigation(); err != nil {
			return nil, err
		}
		if _, ok := current.baseline.(map[string]any); current.baselineExists && !ok {
			return nil, fmt.Errorf("cannot select map key %q from baseline %T", key, current.baseline)
		}
		if current.baselineMembers == nil {
			current.baselineMembers = make(map[string]*editNode)
		}
		child, ok := current.baselineMembers[key]
		if !ok {
			child = &editNode{document: n.document, parent: current, key: key}
			current.baselineMembers[key] = child
			if current.members == nil {
				current.members = make(map[string]*editNode)
			}
			current.members[key] = child
		}
		if err := child.check(); err != nil {
			return nil, err
		}
		current = child
	}
	return current, nil
}

func (n *editNode) baselineItems() ([]*editNode, error) {
	if err := n.checkNavigation(); err != nil {
		return nil, err
	}
	if _, ok := n.baseline.([]any); !n.baselineExists || !ok {
		return nil, fmt.Errorf("baseline value is not a list")
	}
	return slices.Clone(n.baselineElements), nil
}

func (n *editNode) matchBaseline(key string, value any) (*editNode, error) {
	if !utf8.ValidString(key) {
		return nil, fmt.Errorf("match key must be valid UTF-8")
	}
	// JSON normalization must not turn an invalid selector into another ID.
	selector := reflect.ValueOf(value)
	var pointers map[uintptr]bool
	for selector.IsValid() && (selector.Kind() == reflect.Pointer || selector.Kind() == reflect.Interface) {
		if selector.IsNil() {
			break
		}
		if selector.Kind() == reflect.Pointer {
			if pointers == nil {
				pointers = make(map[uintptr]bool)
			}
			address := selector.Pointer()
			if pointers[address] {
				return nil, fmt.Errorf("match value contains a pointer cycle")
			}
			pointers[address] = true
		}
		selector = selector.Elem()
	}
	if selector.IsValid() && selector.Kind() == reflect.String && !utf8.ValidString(selector.String()) {
		return nil, fmt.Errorf("match string value must be valid UTF-8")
	}
	items, err := n.baselineItems()
	if err != nil {
		return nil, err
	}
	value, err = copyEditJSON(value)
	if err != nil {
		return nil, fmt.Errorf("copy match value: %w", err)
	}
	if !isEditScalar(value) {
		return nil, fmt.Errorf("match value must be a scalar or null")
	}
	var match *editNode
	for _, item := range items {
		object, ok := item.baseline.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("matching a map key requires object list items")
		}
		candidate, exists := object[key]
		if !exists || !reflect.DeepEqual(candidate, value) {
			continue
		}
		if match != nil {
			return nil, fmt.Errorf("baseline match for key %q is ambiguous", key)
		}
		match = item
	}
	if match == nil {
		return nil, fmt.Errorf("no baseline item matches key %q", key)
	}
	if err := match.check(); err != nil {
		return nil, err
	}
	return match, nil
}

func (n *editNode) readBaseline() (any, bool, error) {
	if err := n.check(); err != nil {
		return nil, false, err
	}
	if !n.baselineExists {
		return nil, false, nil
	}
	value, err := copyEditJSON(n.baseline)
	return value, true, err
}

func (n *editNode) assign(value any, replace bool, parents resource.ParentPolicy) error {
	if err := n.check(); err != nil {
		return err
	}
	if parents == "" {
		parents = resource.RequireParents
	}
	if parents != resource.RequireParents && parents != resource.CreateMapParents {
		return fmt.Errorf("unknown edit parent policy %q", parents)
	}
	value, err := copyEditJSON(value)
	if err != nil {
		return fmt.Errorf("copy edit value: %w", err)
	}
	if !replace && (!isEditScalar(value) || (n.exists && n.kind != editScalar)) {
		return fmt.Errorf("Set accepts scalar or null values; use Replace to own a map or list")
	}
	if err := n.prepareParents(parents); err != nil {
		return err
	}
	n.invalidateDescendants()
	n.load(value, false)
	return nil
}

func (n *editNode) remove() error {
	if err := n.check(); err != nil {
		return err
	}
	if n.parent == nil {
		return fmt.Errorf("cannot remove the document root")
	}
	if !n.exists {
		return nil
	}
	n.invalidateDescendants()
	if n.listItem {
		index := slices.Index(n.parent.elements, n)
		if index < 0 {
			return fmt.Errorf("list item is no longer attached")
		}
		n.parent.elements = slices.Delete(n.parent.elements, index, index+1)
		n.invalid = true
	}
	n.exists = false
	n.navigationClosed = true
	n.members, n.elements, n.scalar = nil, nil, nil
	return nil
}

func (n *editNode) insertBefore(before *editNode, value any) (*editNode, error) {
	if err := n.check(); err != nil {
		return nil, err
	}
	if !n.exists || n.kind != editList {
		return nil, fmt.Errorf("insertion requires an existing list")
	}
	index, err := n.siblingIndex(before)
	if err != nil {
		return nil, err
	}
	value, err = copyEditJSON(value)
	if err != nil {
		return nil, fmt.Errorf("copy inserted value: %w", err)
	}
	item := &editNode{document: n.document, parent: n, listItem: true}
	item.load(value, false)
	n.elements = slices.Insert(n.elements, index, item)
	return item, nil
}

func (n *editNode) moveBefore(before *editNode) error {
	if err := n.check(); err != nil {
		return err
	}
	if !n.listItem || n.parent == nil || !n.parent.exists || n.parent.kind != editList {
		return fmt.Errorf("movement requires an existing list item")
	}
	index, err := n.parent.siblingIndex(before)
	if err != nil {
		return err
	}
	oldIndex := slices.Index(n.parent.elements, n)
	if oldIndex < 0 {
		return fmt.Errorf("list item is no longer attached")
	}
	if before == n {
		return nil
	}
	if oldIndex < index {
		index--
	}
	n.parent.elements = slices.Delete(n.parent.elements, oldIndex, oldIndex+1)
	n.parent.elements = slices.Insert(n.parent.elements, index, n)
	return nil
}

func (n *editNode) currentPath() (string, error) {
	if err := n.check(); err != nil {
		return "", err
	}
	var segments []string
	for current := n; current.parent != nil; current = current.parent {
		segment := current.key
		if current.listItem {
			index := slices.Index(current.parent.elements, current)
			if index < 0 {
				return "", fmt.Errorf("list item is no longer attached")
			}
			segment = strconv.Itoa(index)
		}
		segments = append(segments, strings.ReplaceAll(strings.ReplaceAll(segment, "~", "~0"), "/", "~1"))
	}
	slices.Reverse(segments)
	if len(segments) == 0 {
		return "", nil
	}
	return "/" + strings.Join(segments, "/"), nil
}

func (n *editNode) check() error {
	if n == nil || n.document == nil || n.invalid {
		return fmt.Errorf("invalid edit node handle")
	}
	return nil
}

func (n *editNode) checkNavigation() error {
	if err := n.check(); err != nil {
		return err
	}
	if n.navigationClosed {
		return fmt.Errorf("children of a replaced, removed, or inserted node require a fresh draft")
	}
	return nil
}

func (n *editNode) siblingIndex(before *editNode) (int, error) {
	if before == nil {
		return len(n.elements), nil
	}
	if err := before.check(); err != nil {
		return 0, err
	}
	if before.document != n.document || before.parent != n || !before.listItem {
		return 0, fmt.Errorf("sibling handle must belong to the same draft and list")
	}
	index := slices.Index(n.elements, before)
	if index < 0 {
		return 0, fmt.Errorf("sibling is no longer attached")
	}
	return index, nil
}

func (n *editNode) prepareParents(policy resource.ParentPolicy) error {
	var missing []*editNode
	for child := n; child.parent != nil; child = child.parent {
		parent := child.parent
		if err := parent.check(); err != nil {
			return err
		}
		if child.listItem {
			if !parent.exists || parent.kind != editList || slices.Index(parent.elements, child) < 0 {
				return fmt.Errorf("list item is no longer attached")
			}
			continue
		}
		if !parent.exists {
			if policy != resource.CreateMapParents {
				return fmt.Errorf("missing map parent; use CreateMapParents to create absent maps")
			}
			missing = append(missing, parent)
			continue
		}
		if parent.kind != editMap {
			return fmt.Errorf("parent of map key %q is not a map", child.key)
		}
	}
	// Check every ancestor before materializing any missing parent.
	for _, parent := range missing {
		parent.kind, parent.exists = editMap, true
		if parent.members == nil {
			parent.members = make(map[string]*editNode)
		}
	}
	return nil
}

func (n *editNode) load(value any, baseline bool) {
	n.exists = true
	n.kind, n.scalar = editScalar, nil
	n.members, n.elements = nil, nil
	n.navigationClosed = !baseline
	if baseline {
		n.baseline, n.baselineExists = value, true
	}
	switch value := value.(type) {
	case map[string]any:
		n.kind = editMap
		n.members = make(map[string]*editNode, len(value))
		if baseline {
			n.baselineMembers = make(map[string]*editNode, len(value))
		}
		for key, value := range value {
			child := &editNode{document: n.document, parent: n, key: key}
			child.load(value, baseline)
			n.members[key] = child
			if baseline {
				n.baselineMembers[key] = child
			}
		}
	case []any:
		n.kind = editList
		n.elements = make([]*editNode, len(value))
		for index, value := range value {
			child := &editNode{document: n.document, parent: n, listItem: true}
			child.load(value, baseline)
			n.elements[index] = child
		}
		if baseline {
			n.baselineElements = slices.Clone(n.elements)
		}
	default:
		n.scalar = value
	}
}

func (n *editNode) invalidateDescendants() {
	for _, children := range []map[string]*editNode{n.baselineMembers, n.members} {
		for _, child := range children {
			if !child.invalid {
				child.invalid = true
				child.invalidateDescendants()
			}
		}
	}
	for _, children := range [][]*editNode{n.baselineElements, n.elements} {
		for _, child := range children {
			if !child.invalid {
				child.invalid = true
				child.invalidateDescendants()
			}
		}
	}
}

func (n *editNode) currentValue() any {
	if !n.exists {
		return nil
	}
	switch n.kind {
	case editMap:
		value := make(map[string]any, len(n.members))
		for key, child := range n.members {
			if child.exists {
				value[key] = child.currentValue()
			}
		}
		return value
	case editList:
		value := make([]any, len(n.elements))
		for index, child := range n.elements {
			value[index] = child.currentValue()
		}
		return value
	default:
		return n.scalar
	}
}

func copyEditJSON(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var copied any
	if err := json.Unmarshal(data, &copied); err != nil {
		return nil, err
	}
	return copied, nil
}

func isEditScalar(value any) bool {
	switch value.(type) {
	case map[string]any, []any:
		return false
	default:
		return true
	}
}
