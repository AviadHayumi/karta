// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/run-ai/karta/pkg/resource"
)

func TestEditGraphDetachedBaselineAndInputs(t *testing.T) {
	input := map[string]any{"item": map[string]any{"image": "old", "extra": []any{map[string]any{"zero": 0}}}}
	doc := editGraphDocument(t, input)
	input["item"].(map[string]any)["image"] = "caller mutation"
	item := editGraphPointer(t, doc, "/item")
	baseline, exists, err := item.readBaseline()
	editGraphOK(t, err)
	if !exists {
		t.Fatal("existing baseline reported absent")
	}
	baseline.(map[string]any)["image"] = "read mutation"
	exported := doc.value().(map[string]any)
	exported["item"].(map[string]any)["extra"].([]any)[0].(map[string]any)["zero"] = 99
	editGraphEqual(t, doc.value(), map[string]any{"item": map[string]any{"image": "old", "extra": []any{map[string]any{"zero": 0}}}})

	replacement := map[string]any{"image": "new", "nested": []any{map[string]any{"keep": true}}}
	editGraphOK(t, item.assign(replacement, true, ""))
	replacement["nested"].([]any)[0].(map[string]any)["keep"] = false
	editGraphEqual(t, doc.value(), map[string]any{"item": map[string]any{"image": "new", "nested": []any{map[string]any{"keep": true}}}})
	baseline, exists, err = item.readBaseline()
	editGraphOK(t, err)
	if !exists || baseline.(map[string]any)["image"] != "old" {
		t.Fatalf("read after replacement changed baseline: %#v, exists %v", baseline, exists)
	}

	type typedReplacement struct {
		Name   string   `json:"name"`
		Values []string `json:"values"`
	}
	typed := typedReplacement{Name: "typed", Values: []string{"accepted"}}
	editGraphOK(t, item.assign(typed, true, ""))
	typed.Values[0] = "changed"
	editGraphEqual(t, doc.value(), map[string]any{"item": map[string]any{"name": "typed", "values": []any{"accepted"}}})
}

func TestEditGraphExplicitValueSemantics(t *testing.T) {
	doc := editGraphDocument(t, map[string]any{
		"null": nil, "zero": 0, "false": false, "empty": "", "object": map[string]any{}, "list": []any{}, "remove": "old",
	})
	before := doc.value()
	for _, path := range []string{"/object", "/list"} {
		node := editGraphPointer(t, doc, path)
		for _, value := range []any{nil, "scalar", map[string]any{}, []any{}} {
			if err := node.assign(value, false, ""); err == nil {
				t.Fatalf("Set accepted replacement of %s with %#v", path, value)
			}
		}
	}
	editGraphEqual(t, doc.value(), before)
	missing := editGraphPointer(t, doc, "/missing")
	value, exists, err := missing.readBaseline()
	editGraphOK(t, err)
	if exists || value != nil {
		t.Fatalf("missing baseline = %#v, %v", value, exists)
	}
	editGraphOK(t, missing.remove())
	editGraphOK(t, missing.assign(nil, false, ""))
	editGraphOK(t, editGraphPointer(t, doc, "/zero").assign(0, false, ""))
	editGraphOK(t, editGraphPointer(t, doc, "/false").assign(false, false, ""))
	editGraphOK(t, editGraphPointer(t, doc, "/remove").remove())
	editGraphEqual(t, doc.value(), map[string]any{
		"null": nil, "zero": 0, "false": false, "empty": "", "object": map[string]any{}, "list": []any{}, "missing": nil,
	})
	for _, path := range []string{"/null", "/missing"} {
		node := editGraphPointer(t, doc, path)
		if err := node.assign(map[string]any{}, false, ""); err == nil {
			t.Fatal("Set accepted a map")
		}
		if err := node.assign([]any{}, false, ""); err == nil {
			t.Fatal("Set accepted a list")
		}
	}
}

func TestEditGraphStrictParents(t *testing.T) {
	for _, parent := range []any{nil, true, "scalar", 7, []any{map[string]any{}}} {
		t.Run(fmt.Sprintf("%T", parent), func(t *testing.T) {
			doc := editGraphDocument(t, map[string]any{"parent": parent})
			before := doc.value()
			if _, err := editGraphPointer(t, doc, "/parent").member("child"); err == nil {
				t.Fatal("map navigation accepted a wrong-shaped existing parent")
			}
			if _, err := doc.atPointer("/parent/child"); err == nil {
				t.Fatal("pointer navigation accepted a wrong-shaped existing parent")
			}
			editGraphEqual(t, doc.value(), before)
		})
	}
	doc := editGraphDocument(t, map[string]any{})
	leaf := editGraphMember(t, doc.root, "absent", "inner", "leaf")
	for _, policy := range []resource.ParentPolicy{"", resource.RequireParents, "invalid"} {
		if err := leaf.assign("value", false, policy); err == nil {
			t.Fatalf("missing parent accepted with policy %q", policy)
		}
		editGraphEqual(t, doc.value(), map[string]any{})
	}
	editGraphOK(t, leaf.remove())
	editGraphEqual(t, doc.value(), map[string]any{})
	editGraphOK(t, leaf.assign("value", false, resource.CreateMapParents))
	editGraphEqual(t, doc.value(), map[string]any{"absent": map[string]any{"inner": map[string]any{"leaf": "value"}}})
	if _, exists, err := leaf.readBaseline(); err != nil || exists {
		t.Fatalf("created leaf acquired a baseline: exists %v, err %v", exists, err)
	}
	other := editGraphMember(t, doc.root, "absent", "inner", "other")
	editGraphOK(t, other.assign("second", false, resource.RequireParents))

	doc = editGraphDocument(t, map[string]any{"existing": map[string]any{"0": map[string]any{}}})
	editGraphOK(t, editGraphPointer(t, doc, "/existing/0/key").assign("literal", false, ""))
	editGraphOK(t, editGraphMember(t, doc.root, "absent", "0", "-").assign("literal", false, resource.CreateMapParents))
	editGraphEqual(t, doc.value(), map[string]any{
		"existing": map[string]any{"0": map[string]any{"key": "literal"}},
		"absent":   map[string]any{"0": map[string]any{"-": "literal"}},
	})
	if _, err := doc.atPointer("/existing/0/key/child"); err == nil {
		t.Fatal("navigation through assigned scalar succeeded")
	}
}

func TestEditGraphPointersAndLiteralKeys(t *testing.T) {
	doc := editGraphDocument(t, map[string]any{
		"":      map[string]any{"a/b": map[string]any{"~key": "old", "~1": "escaped once"}},
		"items": []any{map[string]any{"0": "numeric map key"}},
	})
	root := editGraphPointer(t, doc, "")
	if root != doc.root {
		t.Fatal("empty pointer did not select root")
	}
	path, err := root.currentPath()
	editGraphOK(t, err)
	if path != "" {
		t.Fatalf("root path = %q", path)
	}
	node := editGraphPointer(t, doc, "//a~1b/~0key")
	if node != editGraphMember(t, root, "", "a/b", "~key") {
		t.Fatal("literal keys and escaped pointer resolved different identities")
	}
	editGraphOK(t, node.assign("new", false, ""))
	path, err = node.currentPath()
	editGraphOK(t, err)
	if path != "//a~1b/~0key" {
		t.Fatalf("escaped path = %q", path)
	}
	value, _, err := editGraphPointer(t, doc, "//a~1b/~01").readBaseline()
	editGraphOK(t, err)
	if value != "escaped once" {
		t.Fatalf("pointer was decoded repeatedly: %#v", value)
	}
	editGraphPointer(t, doc, "/items/0/0")
	for _, path := range []string{"relative", "/bad~", "/bad~2", "/items/-", "/items/-1", "/items/00", "/items/1", "/items/+0", strings.Repeat("/x", editMaxPathDepth+1)} {
		if _, err := doc.atPointer(path); err == nil {
			t.Fatalf("invalid pointer accepted: %q", path)
		}
	}
	if _, err := root.member(make([]string, editMaxPathDepth+1)...); err == nil {
		t.Fatal("unbounded member path accepted")
	}
	if err := root.remove(); err == nil {
		t.Fatal("root removal succeeded")
	}
	editGraphOK(t, root.assign(map[string]any{"fresh": true}, true, ""))
	editGraphEqual(t, doc.value(), map[string]any{"fresh": true})
}

func TestEditGraphListIdentityAndBaselineSelection(t *testing.T) {
	doc := editGraphDocument(t, map[string]any{"items": []any{
		map[string]any{"name": "a", "image": "old-a", "unknown": map[string]any{"nested": []any{nil, 0, false}}},
		map[string]any{"name": "b", "image": "old-b", "extension": true},
		map[string]any{"name": "c", "image": "old-c"},
	}})
	list := editGraphPointer(t, doc, "/items")
	items, err := list.baselineItems()
	editGraphOK(t, err)
	a, b, c := items[0], items[1], items[2]
	image := editGraphMember(t, a, "image")
	editGraphOK(t, editGraphMember(t, a, "name").assign("renamed", false, ""))
	selected, err := list.matchBaseline("name", "a")
	editGraphOK(t, err)
	if selected != a {
		t.Fatal("Match followed the staged name")
	}
	if _, err := list.matchBaseline("name", "renamed"); err == nil {
		t.Fatal("Match searched staged data")
	}
	editGraphOK(t, a.moveBefore(nil))
	if editGraphPointer(t, doc, "/items/0") != a {
		t.Fatal("baseline pointer followed current numeric position")
	}
	path, err := image.currentPath()
	editGraphOK(t, err)
	if path != "/items/2/image" {
		t.Fatalf("moved child path = %q", path)
	}
	editGraphOK(t, image.assign("new-a", false, ""))
	insertedInput := map[string]any{"name": "new", "opaque": []any{map[string]any{"flag": true}}}
	inserted, err := list.insertBefore(c, insertedInput)
	editGraphOK(t, err)
	insertedInput["opaque"].([]any)[0].(map[string]any)["flag"] = false
	editGraphOK(t, inserted.moveBefore(b))
	editGraphOK(t, b.remove())
	editGraphOK(t, a.moveBefore(c))
	editGraphOK(t, a.moveBefore(a))
	editGraphEqual(t, doc.value(), map[string]any{"items": []any{
		map[string]any{"name": "new", "opaque": []any{map[string]any{"flag": true}}},
		map[string]any{"name": "renamed", "image": "new-a", "unknown": map[string]any{"nested": []any{nil, 0, false}}},
		map[string]any{"name": "c", "image": "old-c"},
	}})
	items[0] = nil
	baselineItems, err := list.baselineItems()
	editGraphOK(t, err)
	if len(baselineItems) != 3 || baselineItems[0] != a || baselineItems[1] != b || baselineItems[2] != c {
		t.Fatal("baseline item order or returned slice was mutated")
	}
	baseline, _, err := a.readBaseline()
	editGraphOK(t, err)
	if baseline.(map[string]any)["name"] != "a" || baseline.(map[string]any)["image"] != "old-a" {
		t.Fatalf("baseline followed edits: %#v", baseline)
	}
	if _, err := inserted.member("opaque"); err == nil {
		t.Fatal("navigated inserted child without a fresh draft")
	}
	editGraphOK(t, inserted.assign(map[string]any{"replaced": true}, true, ""))
	editGraphOK(t, inserted.remove())
	if err := inserted.assign(nil, true, ""); err == nil {
		t.Fatal("removed insertion was revived")
	}
}

func TestEditGraphInvalidationIsPermanent(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(fmt.Sprintf("remove=%v", remove), func(t *testing.T) {
			doc := editGraphDocument(t, map[string]any{"parent": map[string]any{"list": []any{map[string]any{"leaf": "old"}}}})
			parent := editGraphPointer(t, doc, "/parent")
			list := editGraphMember(t, parent, "list")
			item := editGraphPointer(t, doc, "/parent/list/0")
			leaf := editGraphMember(t, item, "leaf")
			absent := editGraphMember(t, parent, "absent", "deep")
			if remove {
				editGraphOK(t, parent.remove())
			} else {
				editGraphOK(t, parent.assign(map[string]any{}, true, ""))
			}
			editGraphOK(t, parent.assign(map[string]any{"list": []any{map[string]any{"leaf": "recreated"}}, "absent": map[string]any{"deep": true}}, true, ""))
			before := doc.value()
			for _, invalid := range []*editNode{list, item, leaf, absent} {
				if err := invalid.assign("bad", true, resource.CreateMapParents); err == nil {
					t.Fatal("invalidated handle assigned after recreation")
				}
				if err := invalid.remove(); err == nil {
					t.Fatal("invalidated handle removed after recreation")
				}
				if _, _, err := invalid.readBaseline(); err == nil {
					t.Fatal("invalidated handle read baseline")
				}
				if _, err := invalid.currentPath(); err == nil {
					t.Fatal("invalidated handle returned current path")
				}
			}
			if _, err := parent.member("list"); err == nil {
				t.Fatal("replacement permitted new child navigation")
			}
			if _, err := doc.atPointer("/parent/list/0/leaf"); err == nil {
				t.Fatal("pointer revived an invalidated descendant")
			}
			editGraphEqual(t, doc.value(), before)
			editGraphOK(t, parent.assign(nil, true, ""))
		})
	}
	doc := editGraphDocument(t, map[string]any{"list": []any{map[string]any{"leaf": "old"}}})
	item := editGraphPointer(t, doc, "/list/0")
	leaf := editGraphMember(t, item, "leaf")
	editGraphOK(t, item.remove())
	for _, node := range []*editNode{item, leaf} {
		if err := node.remove(); err == nil {
			t.Fatal("removed list identity remained valid")
		}
	}
	if _, err := doc.atPointer("/list/0"); err == nil {
		t.Fatal("baseline pointer selected a destroyed identity")
	}
}

func TestEditGraphMatchAndWrongListTypes(t *testing.T) {
	doc := editGraphDocument(t, map[string]any{"list": []any{
		map[string]any{"id": 1, "nullable": nil},
		map[string]any{"id": 2, "duplicate": true},
		map[string]any{"id": 3, "duplicate": true},
	}})
	list := editGraphPointer(t, doc, "/list")
	for _, value := range []any{int(1), int32(1), int64(1), float64(1)} {
		match, err := list.matchBaseline("id", value)
		editGraphOK(t, err)
		if match != editGraphPointer(t, doc, "/list/0") {
			t.Fatalf("numeric match %T resolved wrong node", value)
		}
	}
	if _, err := list.matchBaseline("nullable", nil); err != nil {
		t.Fatalf("missing keys incorrectly matched null: %v", err)
	}
	for _, candidate := range []struct {
		key   string
		value any
	}{
		{"id", 4}, {"duplicate", true}, {"absent", nil}, {"id", map[string]any{}}, {"id", []any{}}, {"id", math.Inf(1)},
	} {
		if _, err := list.matchBaseline(candidate.key, candidate.value); err == nil {
			t.Fatalf("invalid match accepted: %#v", candidate)
		}
	}
	for _, value := range []any{nil, "scalar", map[string]any{}, 0} {
		doc := editGraphDocument(t, value)
		if _, err := doc.root.baselineItems(); err == nil {
			t.Fatalf("Items accepted %T", value)
		}
		if _, err := doc.root.insertBefore(nil, "new"); err == nil {
			t.Fatalf("Insert accepted %T", value)
		}
		if err := doc.root.moveBefore(nil); err == nil {
			t.Fatalf("Move accepted %T", value)
		}
	}
	for _, value := range []any{[]any{1}, []any{nil}, []any{[]any{}}, []any{map[string]any{"id": 1}, "scalar"}} {
		doc := editGraphDocument(t, value)
		if _, err := doc.root.matchBaseline("id", 1); err == nil {
			t.Fatalf("Match accepted non-object list %#v", value)
		}
	}
}

func TestEditGraphForeignSiblingsAndRejectedInputs(t *testing.T) {
	doc := editGraphDocument(t, map[string]any{"left": []any{"a", "b"}, "right": []any{"c"}})
	other := editGraphDocument(t, []any{"foreign"})
	left := editGraphPointer(t, doc, "/left")
	a := editGraphPointer(t, doc, "/left/0")
	b := editGraphPointer(t, doc, "/left/1")
	editGraphOK(t, b.remove())
	before := doc.value()
	for _, sibling := range []*editNode{editGraphPointer(t, other, "/0"), editGraphPointer(t, doc, "/right/0"), doc.root, b, {}} {
		if err := a.moveBefore(sibling); err == nil {
			t.Fatal("Move accepted a foreign, unrelated, or invalid sibling")
		}
		if _, err := left.insertBefore(sibling, "new"); err == nil {
			t.Fatal("Insert accepted a foreign, unrelated, or invalid sibling")
		}
		editGraphEqual(t, doc.value(), before)
	}
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	for _, input := range []any{math.NaN(), math.Inf(1), make(chan int), func() {}, cyclic} {
		if _, err := newEditDocument(input); err == nil {
			t.Fatalf("baseline accepted invalid JSON %T", input)
		}
		if err := a.assign(input, true, ""); err == nil {
			t.Fatalf("Replace accepted invalid JSON %T", input)
		}
		if _, err := left.insertBefore(nil, input); err == nil {
			t.Fatalf("Insert accepted invalid JSON %T", input)
		}
		editGraphEqual(t, doc.value(), before)
	}
	for _, invalid := range []*editNode{nil, {}} {
		if err := invalid.assign(nil, true, ""); err == nil {
			t.Fatal("zero node assignment succeeded")
		}
		if _, err := invalid.member("x"); err == nil {
			t.Fatal("zero node navigation succeeded")
		}
		if _, err := invalid.currentPath(); err == nil {
			t.Fatal("zero node path succeeded")
		}
	}
}

func TestEditGraphMixedOperationsReferenceModel(t *testing.T) {
	type item struct {
		handle *editNode
		leaf   *editNode
		value  map[string]any
	}
	for seed := uint64(1); seed <= 30; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(seed, seed+1))
			baseline := make([]any, 8)
			for index := range baseline {
				baseline[index] = map[string]any{"id": index, "value": 0, "unknown": map[string]any{"a/b": []any{nil, false, index}}}
			}
			doc := editGraphDocument(t, baseline)
			var model []*item
			for index := range baseline {
				handle := editGraphPointer(t, doc, "/"+fmt.Sprint(index))
				model = append(model, &item{handle: handle, leaf: editGraphMember(t, handle, "value"), value: baseline[index].(map[string]any)})
			}
			for step := 0; step < 100; step++ {
				operation := rng.IntN(4)
				if len(model) == 0 {
					operation = 0
				}
				switch operation {
				case 0:
					index := rng.IntN(len(model) + 1)
					var before *editNode
					if index < len(model) {
						before = model[index].handle
					}
					value := map[string]any{"inserted": step, "unknown": []any{true, nil}}
					handle, err := doc.root.insertBefore(before, value)
					editGraphOK(t, err)
					model = slices.Insert(model, index, &item{handle: handle, value: value})
				case 1:
					index := rng.IntN(len(model))
					editGraphOK(t, model[index].handle.remove())
					model = slices.Delete(model, index, index+1)
				case 2:
					index, target := rng.IntN(len(model)), rng.IntN(len(model)+1)
					var before *editNode
					if target < len(model) {
						before = model[target].handle
					}
					editGraphOK(t, model[index].handle.moveBefore(before))
					if index != target {
						moved := model[index]
						model = slices.Delete(model, index, index+1)
						if index < target {
							target--
						}
						model = slices.Insert(model, target, moved)
					}
				case 3:
					selected := model[rng.IntN(len(model))]
					if selected.leaf != nil {
						editGraphOK(t, selected.leaf.assign(step, false, ""))
						selected.value["value"] = step
					} else {
						selected.value = map[string]any{"replacement": step}
						editGraphOK(t, selected.handle.assign(selected.value, true, ""))
					}
				}
				expected := make([]any, len(model))
				for index, item := range model {
					expected[index] = item.value
					path, err := item.handle.currentPath()
					editGraphOK(t, err)
					if path != "/"+fmt.Sprint(index) {
						t.Fatalf("step %d: path %q differs from position %d", step, path, index)
					}
				}
				editGraphEqual(t, doc.value(), expected)
			}
		})
	}
}

func FuzzEditGraphUnknownPreservation(f *testing.F) {
	f.Add([]byte(`{"nested":[null,false,0,{},[]]}`), "escaped/key~")
	f.Add([]byte(`[]`), "")
	f.Add([]byte(`null`), "0")
	f.Fuzz(func(t *testing.T, encoded []byte, key string) {
		if len(encoded) > 8192 || len(key) > 256 || !utf8.ValidString(key) {
			t.Skip()
		}
		var unknown any
		if err := json.Unmarshal(encoded, &unknown); err != nil {
			t.Skip()
		}
		original := map[string]any{"image": "old", "extensions": map[string]any{key: unknown}}
		peer := map[string]any{"image": "peer"}
		doc := editGraphDocument(t, []any{original, peer})
		item := editGraphPointer(t, doc, "/0")
		image := editGraphMember(t, item, "image")
		extension := editGraphMember(t, item, "extensions", key)
		editGraphOK(t, item.moveBefore(nil))
		editGraphOK(t, image.assign("new", false, ""))
		editGraphEqual(t, doc.value(), []any{peer, map[string]any{"image": "new", "extensions": map[string]any{key: unknown}}})
		pointer, err := extension.currentPath()
		editGraphOK(t, err)
		wantPointer := "/1/extensions/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
		if pointer != wantPointer {
			t.Fatalf("escaped extension path = %q, want %q", pointer, wantPointer)
		}
		editGraphOK(t, item.assign(original, true, ""))
		if err := extension.assign(nil, true, ""); err == nil {
			t.Fatal("same-path replacement revived an old extension handle")
		}
		editGraphEqual(t, doc.value(), []any{peer, original})
	})
}

func editGraphDocument(t *testing.T, value any) *editDocument {
	t.Helper()
	doc, err := newEditDocument(value)
	editGraphOK(t, err)
	return doc
}

func editGraphPointer(t *testing.T, doc *editDocument, path string) *editNode {
	t.Helper()
	node, err := doc.atPointer(path)
	editGraphOK(t, err)
	return node
}

func editGraphMember(t *testing.T, node *editNode, keys ...string) *editNode {
	t.Helper()
	member, err := node.member(keys...)
	editGraphOK(t, err)
	return member
}

func editGraphOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func editGraphEqual(t *testing.T, got, want any) {
	t.Helper()
	want, err := copyEditJSON(want)
	editGraphOK(t, err)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("raw value mismatch\n got: %#v\nwant: %#v", got, want)
	}
}
