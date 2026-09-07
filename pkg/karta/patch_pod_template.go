// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package karta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/internal/jq/execution"
	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
)

// Patch is a raw partial pod template in the {metadata, spec} view, karta
// merge-patch style: maps merge by key, scalars replace (including zero
// values), lists replace wholesale - except spec.containers, which merges by
// container name touching only the provided keys. A single unnamed container
// entry means the sole logical container. Null never deletes in v1. The patch
// is strict-validated against corev1.PodTemplateSpec, so a typo like
// spec.labels fails validation instead of aliasing a route.
type Patch map[string]any

// AsPodMergePatch implements PodTemplateUpdate.
func (p Patch) AsPodMergePatch() Patch { return p }

// ValidatePodTemplateUpdate checks an update against a component definition
// exactly like UpdatePodTemplate does before writing anything: patch
// structure, strict pod-template validation and route capability. It is the
// same code path production runs, exported so fakes and tooling cannot drift
// from it.
func ValidatePodTemplateUpdate(definition v1alpha1.ComponentDefinition, component string, update PodTemplateUpdate) error {
	if update == nil {
		return ErrEmptyPatch
	}
	_, err := compileWrites(definition, component, update.AsPodMergePatch())
	return err
}

// compileWrites turns a patch into physical leaf writes for the definition:
// validate, flatten, route, and report every unroutable path at once.
func compileWrites(definition v1alpha1.ComponentDefinition, component string, patch Patch) ([]leafWrite, error) {
	leaves, err := patchLeaves(patch)
	if err != nil {
		return nil, err
	}
	if len(leaves) == 0 {
		return nil, ErrEmptyPatch
	}
	writes, unsupported, err := routeRawLeaves(definition, leaves)
	if err != nil {
		return nil, err
	}
	if len(unsupported) > 0 {
		return nil, &UnsupportedFieldsError{Component: component, Fields: unsupported}
	}
	return writes, nil
}

func (w *workload) patchPodTemplate(ctx context.Context, component string, patch Patch, opts ...UpdateOption) error {
	options := ResolveUpdateOptions(opts...)

	target, err := w.factory.GetComponent(component)
	if err != nil {
		return fmt.Errorf("karta: %w", err)
	}
	writes, err := compileWrites(target.Definition(), component, patch)
	if err != nil {
		return err
	}

	targeted, err := resolveTargets(ctx, target, options, writes)
	if err != nil {
		return err
	}

	current, err := w.factory.GetResource()
	if err != nil {
		return fmt.Errorf("karta: get object: %w", err)
	}
	raw, ok := current.DeepCopyObject().(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("karta: unsupported object type %T", current)
	}
	// raw.Object is JSON-primitive: GetResource returns the accessor's
	// converted data and DeepCopyObject preserves the primitive types.
	runner := execution.NewPrimitiveRunner(raw.Object)
	for _, write := range writes {
		if err := applyLeaf(ctx, runner, write, targeted); err != nil {
			return err
		}
	}
	mutated, err := runner.GetObject()
	if err != nil {
		return fmt.Errorf("karta: get mutated object: %w", err)
	}
	mutatedMap, ok := mutated.(map[string]any)
	if !ok {
		return fmt.Errorf("karta: mutated object is %T, not an object", mutated)
	}
	w.factory = resource.NewComponentFactoryFromPrimitiveObject(w.karta, &unstructured.Unstructured{Object: mutatedMap})
	return nil
}

// rawLeaf is one leaf of the patch in the template view.
type rawLeaf struct {
	view  []string
	value any
	merge bool // map leaf merged by key instead of replaced
}

// patchLeaves validates the patch and flattens it to writable leaves:
// strict-decode against corev1.PodTemplateSpec, every null path reported,
// metadata restricted to labels and annotations in v1.
func patchLeaves(patch Patch) ([]rawLeaf, error) {
	if len(patch) == 0 {
		return nil, ErrEmptyPatch
	}
	canonical, err := toRaw(map[string]any(patch))
	if err != nil {
		return nil, err
	}
	root, ok := canonical.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("karta: patch is %T, not an object", canonical)
	}
	for key := range root {
		if key != "metadata" && key != "spec" {
			return nil, fmt.Errorf("karta: patch is the pod template view - top-level keys are metadata and spec, got %q", key)
		}
	}
	if nulls := nullPaths(root, nil); len(nulls) > 0 {
		return nil, fmt.Errorf("karta: null at %s: deleting values is not supported", strings.Join(nulls, ", "))
	}
	if meta, ok := root["metadata"].(map[string]any); ok {
		for key := range meta {
			if key != "labels" && key != "annotations" {
				return nil, fmt.Errorf("karta: metadata.%s: only labels and annotations are patchable in v1", key)
			}
		}
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("karta: encode patch: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var strict corev1.PodTemplateSpec
	if err := decoder.Decode(&strict); err != nil {
		return nil, fmt.Errorf("karta: patch is not a valid partial pod template: %w", err)
	}

	var leaves []rawLeaf
	var walk func(prefix []string, value any) error
	walk = func(prefix []string, value any) error {
		switch typed := value.(type) {
		case map[string]any:
			if len(typed) == 0 {
				return nil // empty maps contribute no operation
			}
			if isMergeMapView(prefix) {
				leaves = append(leaves, rawLeaf{view: prefix, value: typed, merge: true})
				return nil
			}
			for key, entry := range typed {
				if err := walk(append(slices.Clone(prefix), key), entry); err != nil {
					return err
				}
			}
			return nil
		default:
			leaves = append(leaves, rawLeaf{view: prefix, value: value})
			return nil
		}
	}
	if err := walk(nil, root); err != nil {
		return nil, err
	}
	return leaves, nil
}

func nullPaths(value any, prefix []string) []string {
	switch typed := value.(type) {
	case nil:
		return []string{strings.Join(prefix, ".")}
	case map[string]any:
		var paths []string
		for key, entry := range typed {
			paths = append(paths, nullPaths(entry, append(slices.Clone(prefix), key))...)
		}
		slices.Sort(paths)
		return paths
	case []any:
		var paths []string
		for i, entry := range typed {
			paths = append(paths, nullPaths(entry, append(slices.Clone(prefix), strconv.Itoa(i)))...)
		}
		return paths
	default:
		return nil
	}
}

func isMergeMapView(view []string) bool {
	return len(view) == 2 && view[0] == "metadata" &&
		(view[1] == "labels" || view[1] == "annotations")
}

// purePathBase parses an optional definition path as a pure assignment
// target. A nil or impure path is not a write route.
func purePathBase(path *string) ([]pathSegment, bool) {
	if path == nil {
		return nil, false
	}
	return parsePurePath(*path)
}

// routeRawLeaves maps template-view leaves onto physical jq leaf writes per
// shape. Unroutable logical paths are collected all at once, sorted.
func routeRawLeaves(definition v1alpha1.ComponentDefinition, leaves []rawLeaf) ([]leafWrite, []PodField, error) {
	shape := specShape(definition)
	spec := definition.SpecDefinition

	var writes []leafWrite
	var unsupported []string

	toWrite := func(base []pathSegment, relative []string, leaf rawLeaf) leafWrite {
		segments := make([]pathSegment, len(relative))
		for i, field := range relative {
			segments[i] = pathSegment{field: field}
		}
		write := leafWrite{path: appendPath(base, segments...)}
		switch {
		case leaf.merge:
			write.transform = mergeRawMap(leaf.value.(map[string]any))
		case len(leaf.view) == 2 && leaf.view[0] == "spec" && leaf.view[1] == "containers":
			write.transform = mergeRawContainerList(leaf.value)
		default:
			value := leaf.value
			write.transform = func(any) (any, error) { return value, nil }
		}
		return write
	}
	switch shape {
	case shapeTemplate:
		base, ok := purePathBase(spec.PodTemplateSpecPath)
		if !ok {
			return nil, nil, fmt.Errorf("karta: pod path is not a writable pure path: %w", ErrNotSupported)
		}
		for _, leaf := range leaves {
			writes = append(writes, toWrite(base, leaf.view, leaf))
		}
	case shapePodSpec, shapeSplit:
		base, ok := purePathBase(spec.PodSpecPath)
		if !ok {
			return nil, nil, fmt.Errorf("karta: pod path is not a writable pure path: %w", ErrNotSupported)
		}
		var metaBase []pathSegment
		if shape == shapeSplit {
			metaBase, ok = purePathBase(spec.MetadataPath)
			if !ok {
				return nil, nil, fmt.Errorf("karta: metadata path is not a writable pure path: %w", ErrNotSupported)
			}
		}
		for _, leaf := range leaves {
			switch leaf.view[0] {
			case "spec":
				writes = append(writes, toWrite(base, leaf.view[1:], leaf))
			case "metadata":
				if shape != shapeSplit {
					unsupported = append(unsupported, strings.Join(leaf.view, "."))
					continue
				}
				writes = append(writes, toWrite(metaBase, leaf.view[1:], leaf))
			}
		}
	case shapeFragmented:
		fragmented := spec.FragmentedPodSpecDefinition
		for _, leaf := range leaves {
			fragmentWrites, bad := routeFragmentedLeaf(fragmented, leaf)
			writes = append(writes, fragmentWrites...)
			unsupported = append(unsupported, bad...)
		}
	default:
		for _, leaf := range leaves {
			unsupported = append(unsupported, strings.Join(leaf.view, "."))
		}
	}

	if len(unsupported) > 0 {
		slices.Sort(unsupported)
		unsupported = slices.Compact(unsupported)
		fields := make([]PodField, len(unsupported))
		for i, path := range unsupported {
			fields[i] = PodField(path)
		}
		return nil, fields, nil
	}
	return writes, nil, nil
}

// routeFragmentedLeaf resolves one template-view leaf against the fragmented
// path table, longest prefix first. Container entries decompose: an unnamed
// sole entry bridges image/resources to their singular paths; named entries
// need containersPath. No fallback from an impure path, ever.
func routeFragmentedLeaf(fragmented *v1alpha1.FragmentedPodSpecDefinition, leaf rawLeaf) ([]leafWrite, []string) {
	view := strings.Join(leaf.view, ".")
	direct := func(path *string, transform func(any) (any, error)) ([]leafWrite, []string) {
		base, ok := purePathBase(path)
		if !ok {
			return nil, []string{view}
		}
		return []leafWrite{{path: renderPath(base), transform: transform}}, nil
	}
	replace := func(value any) func(any) (any, error) {
		return func(any) (any, error) { return value, nil }
	}

	switch {
	case view == "metadata.labels":
		return direct(fragmented.LabelsPath, mergeRawMap(leaf.value.(map[string]any)))
	case view == "metadata.annotations":
		return direct(fragmented.AnnotationsPath, mergeRawMap(leaf.value.(map[string]any)))
	case view == "spec.schedulerName":
		return direct(fragmented.SchedulerNamePath, replace(leaf.value))
	case view == "spec.priorityClassName":
		return direct(fragmented.PriorityClassNamePath, replace(leaf.value))
	case strings.HasPrefix(view, "spec.affinity.nodeAffinity"):
		return fragmentSuffix(fragmented.NodeAffinityPath, leaf, 3, "spec.affinity.nodeAffinity")
	case strings.HasPrefix(view, "spec.affinity.podAffinity"):
		return fragmentSuffix(fragmented.PodAffinityPath, leaf, 3, "spec.affinity.podAffinity")
	case view == "spec.resourceClaims":
		return direct(fragmented.ResourceClaimsPath, replace(leaf.value))
	case view == "spec.containers":
		return routeFragmentedContainers(fragmented, leaf)
	default:
		return nil, []string{view}
	}
}

// fragmentSuffix writes one leaf below a fragment root: the fragment's own
// path plus the leaf's remaining view fields. Each leaf is its own assignment,
// so sibling leaves under the same fragment merge instead of stomping each
// other. An unroutable leaf reports the fragment-view path - the same
// vocabulary the typed capability check uses.
func fragmentSuffix(path *string, leaf rawLeaf, depth int, label string) ([]leafWrite, []string) {
	base, ok := purePathBase(path)
	if !ok {
		return nil, []string{label}
	}
	suffix := make([]pathSegment, 0, len(leaf.view)-depth)
	for _, field := range leaf.view[depth:] {
		suffix = append(suffix, pathSegment{field: field})
	}
	value := leaf.value
	return []leafWrite{{path: appendPath(base, suffix...), transform: func(any) (any, error) { return value, nil }}}, nil
}

func routeFragmentedContainers(fragmented *v1alpha1.FragmentedPodSpecDefinition, leaf rawLeaf) ([]leafWrite, []string) {
	entries, ok := leaf.value.([]any)
	if !ok || len(entries) == 0 {
		return nil, []string{"spec.containers"}
	}
	// named entries: the whole list merges at containersPath
	first, _ := entries[0].(map[string]any)
	if name, _ := first["name"].(string); name != "" || len(entries) > 1 {
		base, ok := purePathBase(fragmented.ContainersPath)
		if !ok {
			return nil, []string{"spec.containers"}
		}
		return []leafWrite{{path: renderPath(base), transform: mergeRawContainerList(leaf.value)}}, nil
	}
	// the sole logical container: bridge each provided key to its own path
	var writes []leafWrite
	var unsupported []string
	for key, value := range first {
		switch key {
		case "image":
			base, ok := purePathBase(fragmented.ImagePath)
			if !ok {
				unsupported = append(unsupported, "spec.containers.image")
				continue
			}
			replace := value
			writes = append(writes, leafWrite{path: renderPath(base), transform: func(any) (any, error) { return replace, nil }})
		case "resources":
			base, ok := purePathBase(fragmented.ResourcesPath)
			if !ok {
				unsupported = append(unsupported, "spec.containers.resources")
				continue
			}
			replace := value
			writes = append(writes, leafWrite{path: renderPath(base), transform: func(any) (any, error) { return replace, nil }})
		default:
			base, ok := purePathBase(fragmented.ContainerPath)
			if !ok {
				unsupported = append(unsupported, "spec.containers."+key)
				continue
			}
			field, replace := key, value
			writes = append(writes, leafWrite{path: renderPath(base), transform: func(current any) (any, error) {
				container, ok := current.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("karta: containerPath value is %T, not an object", current)
				}
				copied := maps.Clone(container)
				copied[field] = replace
				return copied, nil
			}})
		}
	}
	return writes, unsupported
}

// mergeRawContainerList merges patch containers into the current raw list by
// container name. A single unnamed entry targets the sole container. Named
// entries must exist - UpdatePodTemplate does not add sidecars.
func mergeRawContainerList(value any) func(any) (any, error) {
	return func(current any) (any, error) {
		patchList, ok := value.([]any)
		if !ok {
			return nil, fmt.Errorf("karta: spec.containers must be a list, got %T", value)
		}
		list, ok := current.([]any)
		if !ok {
			return nil, fmt.Errorf("karta: containers value is %T, not a list", current)
		}
		containers := make([]any, len(list))
		names := map[string]int{}
		for i, entry := range list {
			raw, ok := entry.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("karta: container %d is %T, not an object", i, entry)
			}
			containers[i] = maps.Clone(raw)
			if name, _ := raw["name"].(string); name != "" {
				if _, dup := names[name]; dup {
					return nil, fmt.Errorf("karta: duplicate container name %q in the pod", name)
				}
				names[name] = i
			}
		}
		apply := func(index int, patchContainer map[string]any) {
			target := containers[index].(map[string]any)
			for key, item := range patchContainer {
				if key == "name" {
					continue
				}
				target[key] = item
			}
		}
		if len(patchList) == 1 {
			if entry, ok := patchList[0].(map[string]any); ok {
				if name, _ := entry["name"].(string); name == "" {
					if len(containers) != 1 {
						return nil, fmt.Errorf("karta: an unnamed container entry targets the sole container, pod has %d; name the container", len(containers))
					}
					apply(0, entry)
					return containers, nil
				}
			}
		}
		seen := map[string]bool{}
		for _, entry := range patchList {
			patchContainer, ok := entry.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("karta: container patch entries must be objects, got %T", entry)
			}
			name, _ := patchContainer["name"].(string)
			if name == "" {
				return nil, fmt.Errorf("karta: container patch entries must be named when patching multiple containers")
			}
			if seen[name] {
				return nil, fmt.Errorf("karta: container %q patched twice", name)
			}
			seen[name] = true
			index, ok := names[name]
			if !ok {
				return nil, fmt.Errorf("karta: container %q not found in pod", name)
			}
			apply(index, patchContainer)
		}
		return containers, nil
	}
}
