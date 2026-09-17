// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package replay

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/test/e2e/recorder"
)

const (
	parityBaselineRoot = "testdata/parity_baseline"
	parityScheduler    = "karta-parity-scheduler"
	parityLabelKey     = "karta-parity"
	parityLabelValue   = "true"
	parityImage        = "ghcr.io/example/karta-parity:v1"
	readErrorMarker    = "<read-error>"
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

// The baseline under testdata/parity_baseline was produced by the jq engine at the
// last commit before the CEL adoption, by running snapshotStep - the exact same
// logic as below - over every recorded object. This suite replays the identical
// reads and writes through the CEL engine and requires byte-for-byte agreement,
// apart from the explicit corrections documented in
// testdata/parity_baseline/README.md, together with the regeneration steps.
var _ = Describe("CEL matches the previous engine baseline", func() {
	recordings, _ := filepath.Glob(recordedGlob)
	if len(recordings) == 0 {
		return
	}

	for _, path := range recordings {
		path := path
		rel := strings.TrimPrefix(path, "../recorded_data/")
		if _, addedAfterJQ := postJQRecordings[rel]; addedAfterJQ {
			continue
		}
		It("matches "+rel, func(_ SpecContext) {
			baselineBytes, err := os.ReadFile(filepath.Join(parityBaselineRoot, rel))
			Expect(err).NotTo(HaveOccurred(),
				"missing parity baseline for %s; see testdata/parity_baseline/README.md", rel)
			var baseline map[string]any
			Expect(yaml.Unmarshal(baselineBytes, &baseline)).To(Succeed())
			baselineSteps, _ := baseline["steps"].([]any)

			r, err := recorder.OpenRecording(path)
			Expect(err).NotTo(HaveOccurred())
			name := filepath.Base(r.Recording().KartaFile)
			kartaYAML, err := os.ReadFile(filepath.Join(repoRoot, "docs", "catalog", name))
			Expect(err).NotTo(HaveOccurred())
			karta := &kartav1alpha1.Karta{}
			Expect(yaml.Unmarshal(kartaYAML, karta)).To(Succeed())

			var steps []map[string]any
			var inputs []map[string]any
			for r.Next() {
				input := r.Object().Object
				inputs = append(inputs, input)
				step := snapshotStep(karta, input)
				step["state"] = r.State()
				steps = append(steps, step)
			}
			Expect(steps).To(HaveLen(len(baselineSteps)),
				"%s: cel walked a different number of recorded states than the previous engine did", rel)

			for i, step := range steps {
				baseStep, _ := baselineSteps[i].(map[string]any)
				state, _ := step["state"].(string)
				for _, section := range []string{"state", "factoryError", "mutations", "suspends"} {
					Expect(canonYAML(step[section])).To(Equal(canonYAML(baseStep[section])),
						"%s step %d (%s): %s diverges from the previous engine", rel, i, state, section)
				}
				compareReads(rel, i, state, step["reads"], baseStep["reads"], inputs[i])
				for _, section := range []string{"docAfterWrites", "docAfterSuspend", "docAfterResume"} {
					compareDoc(rel, i, section, step[section], baseStep[section], inputs[i])
				}
			}
		})
	}
})

// compareReads matches every component read against the baseline. Absent Grove
// scaling groups now return empty results instead of jq read errors. Matched
// statuses: where the engines
// disagree, the cel answer is required to be exactly the state label the
// recorder itself captured, which covers both the states the jq catalog left
// Undefined and the retuned jobset mappings. Anything else fails.
func compareReads(rel string, step int, state string, got, base any, input map[string]any) {
	gotList, _ := got.([]any)
	baseList, _ := base.([]any)
	Expect(gotList).To(HaveLen(len(baseList)),
		"%s step %d (%s): cel sees a different number of components than the previous engine did", rel, step, state)

	for i := range gotList {
		gotRead, _ := gotList[i].(map[string]any)
		baseRead, _ := baseList[i].(map[string]any)
		keys := map[string]struct{}{}
		for key := range gotRead {
			keys[key] = struct{}{}
		}
		for key := range baseRead {
			keys[key] = struct{}{}
		}
		names := make([]string, 0, len(keys))
		for key := range keys {
			names = append(names, key)
		}
		sort.Strings(names)
		for _, key := range names {
			gotVal, baseVal := gotRead[key], baseRead[key]
			switch {
			case key == "status":
				compareStatus(rel, step, state, gotRead["name"], gotVal, baseVal)
			case strings.HasPrefix(rel, "grove/") && gotRead["name"] == "scalinggroup" &&
				baseVal == readErrorMarker && (key == "instanceIds" || key == "scale" || key == "extractedInstances"):
				groups, _, err := unstructured.NestedSlice(input, "spec", "template", "podCliqueScalingGroups")
				Expect(err).NotTo(HaveOccurred())
				Expect(groups).To(BeEmpty(), "only an absent or empty scaling-group list permits empty reads")
				want := "{}\n"
				if key == "instanceIds" {
					want = "null\n"
				}
				Expect(canonYAML(gotVal)).To(Equal(want),
					"%s step %d (%s): Grove %s must return an empty result, not an error", rel, step, state, key)
			default:
				Expect(canonYAML(gotVal)).To(Equal(canonYAML(baseVal)),
					"%s step %d (%s) component %v: read %s diverges from the previous engine", rel, step, state, gotRead["name"], key)
			}
		}
	}
}

func compareStatus(rel string, step int, state string, name, got, base any) {
	if canonYAML(got) == canonYAML(base) {
		return
	}
	gotMap, gotOK := deepCanon(got).(map[string]any)
	baseMap, baseOK := deepCanon(base).(map[string]any)
	Expect(gotOK && baseOK).To(BeTrue(),
		"%s step %d (%s) component %v: status diverges from the previous engine (jq=%v cel=%v)", rel, step, state, name, base, got)
	for _, field := range []string{"phase", "conditions"} {
		Expect(canonYAML(gotMap[field])).To(Equal(canonYAML(baseMap[field])),
			"%s step %d (%s) component %v: status %s diverges from the previous engine", rel, step, state, name, field)
	}
	gotStatuses, _ := gotMap["matchedStatuses"].([]any)
	baseStatuses, _ := baseMap["matchedStatuses"].([]any)
	if canonYAML(gotStatuses) == canonYAML(baseStatuses) {
		return
	}
	celStatus := ""
	if len(gotStatuses) == 1 {
		celStatus, _ = gotStatuses[0].(string)
	}
	Expect(strings.EqualFold(celStatus, state)).To(BeTrue(),
		"%s step %d (%s) component %v: matchedStatuses diverge (jq=%v cel=%v) and cel is not exactly the recorded state",
		rel, step, state, name, baseStatuses, gotStatuses)
}

// compareDoc matches a document snapshot against the baseline. The pod definition's whole-document template
// write: the jq engine replaced the entire object with the template, dropping
// apiVersion and kind, so its own GetResource failed validation; against that
// baseline error cel is required to produce a valid object that still carries
// the parity scheduler write, and a cel error fails. Null elision: the jq
// engine stores a literal null where a merge patch removes the key, so both
// documents retain the historical suite's null-elision comparison. The expected
// document also restores only the fields named in parityExpectedDocument, using
// the original recording rather than the actual result as the source of truth.
func compareDoc(rel string, step int, section string, got, base any, input map[string]any) {
	if baseStr, ok := base.(string); ok && baseStr == readErrorMarker {
		doc, isMap := deepCanon(got).(map[string]any)
		Expect(isMap && doc["kind"] != nil && doc["apiVersion"] != nil).To(BeTrue(),
			"%s step %d: %s: jq failed to produce a document and cel must produce a valid one, got %v", rel, step, section, got)
		Expect(canonYAML(doc)).To(ContainSubstring(parityScheduler),
			"%s step %d: %s: the cel document lost the parity writes", rel, step, section)
		return
	}
	expected, err := parityExpectedDocument(rel, base, input)
	Expect(err).NotTo(HaveOccurred())
	if canonYAML(got) == canonYAML(expected) {
		return
	}
	Expect(canonYAML(stripNulls(deepCanon(got)))).To(Equal(canonYAML(stripNulls(expected))),
		"%s step %d: %s diverges beyond the documented corrections", rel, step, section)
}

func stripNulls(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			if item == nil {
				continue
			}
			out[key] = stripNulls(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, stripNulls(item))
		}
		return out
	default:
		return value
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

func canonYAML(value any) string {
	data, err := yaml.Marshal(deepCanon(value))
	if err != nil {
		return "<marshal-error>"
	}
	return string(data)
}
