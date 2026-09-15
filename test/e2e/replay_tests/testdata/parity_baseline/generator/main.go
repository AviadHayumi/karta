// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Baseline generator for the cel-vs-jq parity suite. Runs at the last jq
// commit and dumps, per recorded object, the result of every read and every
// write the component API offers. The snapshot logic below is a verbatim copy
// of snapshotStep and its helpers from parity_test.go on the cel branch.
//
// Usage: go run ./paritydump <recordings-root> <output-root>
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
)

const (
	parityScheduler  = "karta-parity-scheduler"
	parityLabelKey   = "karta-parity"
	parityLabelValue = "true"
	parityImage      = "ghcr.io/example/karta-parity:v1"
	readErrorMarker  = "<read-error>"
)

var parityNodeAffinity = &corev1.NodeAffinity{
	RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
		NodeSelectorTerms: []corev1.NodeSelectorTerm{{
			MatchExpressions: []corev1.NodeSelectorRequirement{{
				Key: "karta-parity/pool", Operator: corev1.NodeSelectorOpIn, Values: []string{"parity"},
			}},
		}},
	},
}

type recordedEvent struct {
	Kind   string         `json:"kind"`
	State  string         `json:"state"`
	Object map[string]any `json:"object"`
}

type recordingFile struct {
	Events    []recordedEvent `json:"events"`
	KartaFile string          `json:"kartaFile"`
}

func main() {
	if len(os.Args) != 3 {
		die(fmt.Errorf("usage: %s <recordings-root> <output-root>", os.Args[0]))
	}
	recordingsRoot, outRoot := os.Args[1], os.Args[2]
	paths, err := filepath.Glob(filepath.Join(recordingsRoot, "*", "*", "*", "*.yaml"))
	die(err)
	if len(paths) == 0 {
		die(fmt.Errorf("no recordings under %s", recordingsRoot))
	}
	sort.Strings(paths)

	for _, path := range paths {
		rel, err := filepath.Rel(recordingsRoot, path)
		die(err)
		raw, err := os.ReadFile(path)
		die(err)
		var rec recordingFile
		die(yaml.Unmarshal(raw, &rec))

		kartaYAML, err := os.ReadFile(filepath.Join("docs", "catalog", filepath.Base(rec.KartaFile)))
		die(err)
		karta := &kartav1alpha1.Karta{}
		die(yaml.Unmarshal(kartaYAML, karta))

		var steps []any
		for _, ev := range rec.Events {
			if ev.Kind != "STATE" {
				continue
			}
			step := snapshotStep(karta, ev.Object)
			step["state"] = ev.State
			steps = append(steps, step)
		}

		out, err := yaml.Marshal(deepCanon(map[string]any{"steps": steps}))
		die(err)
		target := filepath.Join(outRoot, rel)
		die(os.MkdirAll(filepath.Dir(target), 0o755))
		die(os.WriteFile(target, out, 0o644))
		fmt.Println("wrote", target)
	}
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// snapshotStep drives every read and every write the component API offers
// against one recorded object and returns a serializable picture of what the
// engine did with it. The same function body runs on the pre-CEL commit to
// produce the parity baseline, so any edit here requires regenerating the fixtures.
func snapshotStep(karta *kartav1alpha1.Karta, raw map[string]any) map[string]any {
	ctx := context.Background()
	obj := (&unstructured.Unstructured{Object: raw}).DeepCopy()
	factory := resource.NewComponentFactoryFromObject(karta, obj)
	step := map[string]any{}

	root, err := factory.GetRootComponent()
	if err != nil {
		step["factoryError"] = true
		return step
	}
	children, err := factory.GetChildComponents()
	if err != nil {
		step["factoryError"] = true
		return step
	}
	components := append([]*resource.Component{root}, children...)

	reads := make([]any, 0, len(components))
	for _, comp := range components {
		reads = append(reads, readComponent(ctx, comp))
	}
	step["reads"] = reads

	mutations := make([]any, 0, len(components))
	hasSuspend := false
	for _, comp := range components {
		mutations = append(mutations, mutateForParity(ctx, comp))
		hasSuspend = hasSuspend || comp.HasSuspendDefinition()
	}
	step["mutations"] = mutations
	step["docAfterWrites"] = docSnapshot(factory)

	if hasSuspend {
		suspends := map[string]any{}
		for _, comp := range components {
			if comp.HasSuspendDefinition() {
				suspends[comp.Name()+":suspend"] = outcome(comp.Suspend(ctx))
			}
		}
		step["docAfterSuspend"] = docSnapshot(factory)
		for _, comp := range components {
			if comp.HasSuspendDefinition() {
				suspends[comp.Name()+":resume"] = outcome(comp.Resume(ctx))
			}
		}
		step["docAfterResume"] = docSnapshot(factory)
		step["suspends"] = suspends
	}
	return step
}

func readComponent(ctx context.Context, comp *resource.Component) map[string]any {
	read := map[string]any{
		"name":                    comp.Name(),
		"hasPodDefinition":        comp.HasPodDefinition(),
		"hasSuspendDefinition":    comp.HasSuspendDefinition(),
		"hasInstanceIdDefinition": comp.HasInstanceIdDefinition(),
	}
	if kind := comp.Kind(); kind != nil {
		read["kind"] = kind
	}
	record := func(key string, value any, err error) {
		if err != nil {
			read[key] = readErrorMarker
			return
		}
		read[key] = value
	}
	templates, err := comp.GetPodTemplateSpec(ctx)
	record("podTemplateSpec", templates, err)
	podSpecs, err := comp.GetPodSpec(ctx)
	record("podSpec", podSpecs, err)
	metadata, err := comp.GetPodMetadata(ctx)
	record("podMetadata", metadata, err)
	fragmented, err := comp.GetFragmentedPodSpec(ctx)
	record("fragmentedPodSpec", fragmented, err)
	scale, err := comp.GetScale(ctx)
	record("scale", scale, err)
	status, err := comp.GetStatus(ctx)
	record("status", status, err)
	instances, err := comp.GetExtractedInstances(ctx)
	record("extractedInstances", instances, err)
	ids, err := comp.GetInstanceIds(ctx)
	sort.Strings(ids)
	record("instanceIds", ids, err)
	return read
}

func mutateForParity(ctx context.Context, comp *resource.Component) map[string]any {
	result := map[string]any{"name": comp.Name()}

	if templates, err := comp.GetPodTemplateSpec(ctx); err != nil {
		result["podTemplateSpec"] = "read-error"
	} else if len(templates) == 0 {
		result["podTemplateSpec"] = "empty"
	} else {
		for id, template := range templates {
			template.Spec.SchedulerName = parityScheduler
			templates[id] = template
		}
		result["podTemplateSpec"] = outcome(comp.UpdatePodTemplateSpec(ctx, templates))
	}

	if podSpecs, err := comp.GetPodSpec(ctx); err != nil {
		result["podSpec"] = "read-error"
	} else if len(podSpecs) == 0 {
		result["podSpec"] = "empty"
	} else {
		for id, podSpec := range podSpecs {
			podSpec.SchedulerName = parityScheduler
			podSpecs[id] = podSpec
		}
		result["podSpec"] = outcome(comp.UpdatePodSpec(ctx, podSpecs))
	}

	if metadata, err := comp.GetPodMetadata(ctx); err != nil {
		result["podMetadata"] = "read-error"
	} else if len(metadata) == 0 {
		result["podMetadata"] = "empty"
	} else {
		for id, meta := range metadata {
			if meta.Labels == nil {
				meta.Labels = map[string]string{}
			}
			meta.Labels[parityLabelKey] = parityLabelValue
			metadata[id] = meta
		}
		result["podMetadata"] = outcome(comp.UpdatePodMetadata(ctx, metadata))
	}

	if fragmented, err := comp.GetFragmentedPodSpec(ctx); err != nil {
		result["fragmentedPodSpec"] = "read-error"
	} else if len(fragmented) == 0 {
		result["fragmentedPodSpec"] = "empty"
	} else {
		result["fragmentedPodSpec"] = fragmentWrites(ctx, comp, fragmented)
	}
	return result
}

// fragmentWrites drives every fragment field through its own update call, so
// every declared write path is exercised with a deterministic marker and a
// read-only field fails on its own without masking the writable ones. Both
// engines refuse a write to a field that has no write definition, so the
// per-field outcomes are comparable. Container shaped fields start from the
// values just read, so components that declare them get realistic payloads.
func fragmentWrites(ctx context.Context, comp *resource.Component, current map[string]resource.FragmentedPodSpec) map[string]any {
	fields := map[string]any{}
	apply := func(name string, fragment resource.FragmentedPodSpec) {
		update := make(map[string]resource.FragmentedPodSpec, len(current))
		for id := range current {
			update[id] = fragment
		}
		fields[name] = outcome(comp.UpdateFragmentedPodSpec(ctx, update))
	}
	apply("schedulerName", resource.FragmentedPodSpec{SchedulerName: parityScheduler})
	apply("priorityClassName", resource.FragmentedPodSpec{PriorityClassName: "karta-parity-priority"})
	apply("image", resource.FragmentedPodSpec{Image: parityImage})
	apply("labels", resource.FragmentedPodSpec{Labels: map[string]string{parityLabelKey: parityLabelValue}})
	apply("annotations", resource.FragmentedPodSpec{Annotations: map[string]string{parityLabelKey: parityLabelValue}})
	apply("nodeAffinity", resource.FragmentedPodSpec{NodeAffinity: parityNodeAffinity})
	apply("podAffinity", resource.FragmentedPodSpec{PodAffinity: &corev1.PodAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
			TopologyKey:   "kubernetes.io/hostname",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{parityLabelKey: parityLabelValue}},
		}},
	}})
	apply("resources", resource.FragmentedPodSpec{Resources: &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    apiresource.MustParse("150m"),
			corev1.ResourceMemory: apiresource.MustParse("96Mi"),
		},
	}})
	apply("resourceClaims", resource.FragmentedPodSpec{ResourceClaims: []corev1.PodResourceClaim{{Name: "karta-parity-claim"}}})

	containers := make(map[string]resource.FragmentedPodSpec, len(current))
	singles := make(map[string]resource.FragmentedPodSpec, len(current))
	haveContainers, haveSingle := false, false
	for id, fragment := range current {
		list := resource.FragmentedPodSpec{}
		for _, container := range fragment.Containers {
			container.Image = parityImage
			list.Containers = append(list.Containers, container)
		}
		haveContainers = haveContainers || len(list.Containers) > 0
		containers[id] = list
		single := resource.FragmentedPodSpec{}
		if fragment.Container != nil {
			container := *fragment.Container
			container.Image = parityImage
			single.Container = &container
		}
		haveSingle = haveSingle || single.Container != nil
		singles[id] = single
	}
	if haveContainers {
		fields["containers"] = outcome(comp.UpdateFragmentedPodSpec(ctx, containers))
	} else {
		fields["containers"] = "empty"
	}
	if haveSingle {
		fields["container"] = outcome(comp.UpdateFragmentedPodSpec(ctx, singles))
	} else {
		fields["container"] = "empty"
	}
	return fields
}

func outcome(err error) string {
	if err != nil {
		return "write-error"
	}
	return "applied"
}

func docSnapshot(factory *resource.ComponentFactory) any {
	obj, err := factory.GetResource()
	if err != nil {
		return readErrorMarker
	}
	content, ok := obj.(interface{ UnstructuredContent() map[string]any })
	if !ok {
		return readErrorMarker
	}
	return deepCanon(content.UnstructuredContent())
}

// deepCanon detaches a value from any live document and normalizes it to plain
// JSON types, so snapshots taken before and after later writes stay distinct
// and both engines serialize numbers and keys identically.
func deepCanon(value any) any {
	data, err := yaml.Marshal(value)
	if err != nil {
		return "<marshal-error>"
	}
	var out any
	if err := yaml.Unmarshal(data, &out); err != nil {
		return "<marshal-error>"
	}
	return out
}
