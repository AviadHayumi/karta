// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package replay

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/test/e2e/recorder"
)

const (
	goldenRoot      = "testdata/mutation_goldens"
	goldenScheduler = "karta-golden-scheduler"
	updateGoldens   = "UPDATE_GOLDENS"
)

var goldenNodeAffinity = &corev1.NodeAffinity{
	RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
		NodeSelectorTerms: []corev1.NodeSelectorTerm{{
			MatchExpressions: []corev1.NodeSelectorRequirement{{
				Key: "karta.golden/pool", Operator: corev1.NodeSelectorOpIn, Values: []string{"trains"},
			}},
		}},
	},
}

// Every recorded workload gets mutated through whatever its definition declares
// writable - pod templates, fragmented fields, suspend actions - and the resulting
// document is compared against a golden. The goldens pin the write semantics per
// workload: merge vs replace, per-instance addressing, and the parent-creating adds.
// Regenerate with UPDATE_GOLDENS=1 go test ./replay_tests/...
var _ = Describe("Karta mutates the recorded CRs", func() {
	recordings, _ := filepath.Glob(recordedGlob)
	if len(recordings) == 0 {
		return
	}

	for _, path := range recordings {
		path := path
		rel := strings.TrimPrefix(path, "../recorded_data/")
		It("mutates "+rel, func(ctx SpecContext) {
			r, err := recorder.OpenRecording(path)
			Expect(err).NotTo(HaveOccurred())

			name := filepath.Base(r.Recording().KartaFile)
			kartaYAML, err := os.ReadFile(filepath.Join(repoRoot, "docs", "catalog", name))
			Expect(err).NotTo(HaveOccurred())
			karta := &kartav1alpha1.Karta{}
			Expect(yaml.Unmarshal(kartaYAML, karta)).To(Succeed())

			// One deterministic input per recording: the "running" capture when the
			// flow has one, otherwise the last capture - the most populated CR.
			cr := pickInput(r)
			Expect(cr).NotTo(BeNil())

			options := resource.MutationOptions{
				PatchType: resource.PatchType(os.Getenv("KARTA_REPLAY_PATCH_TYPE")),
				Strategy:  resource.MergeStrategy(os.Getenv("KARTA_REPLAY_MERGE_STRATEGY")),
			}
			// This historical golden records the typed template replacement that
			// discarded Knative-only fields. Preserve that compatibility assertion;
			// the independent test below verifies today's default Merge preserves them.
			// Explicit policy overrides still compare their actual result to the
			// historical golden, so intentional differences remain visible.
			if karta.Name == "serving-knative-dev-service-v1" && options.PatchType == "" && options.Strategy == "" {
				options.Strategy = resource.Replace
			}
			factory := resource.NewComponentFactoryFromObject(karta, cr, resource.WithMutationOptions(options))
			root, err := factory.GetRootComponent()
			Expect(err).NotTo(HaveOccurred())
			children, err := factory.GetChildComponents()
			Expect(err).NotTo(HaveOccurred())

			var applied []string
			for _, comp := range append([]*resource.Component{root}, children...) {
				applied = append(applied, mutateComponent(ctx, comp)...)
			}
			if root.Definition().SuspendDefinition != nil {
				Expect(root.Suspend(ctx)).To(Succeed())
				applied = append(applied, root.Name()+":suspend")
			}
			Expect(applied).NotTo(BeEmpty(),
				"the %s definition declares nothing writable; every workload needs a mutation example", name)

			mutated, err := factory.GetResource()
			Expect(err).NotTo(HaveOccurred())
			got, err := yaml.Marshal(mutated.(interface{ UnstructuredContent() map[string]any }).UnstructuredContent())
			Expect(err).NotTo(HaveOccurred())

			goldenPath := filepath.Join(goldenRoot, strings.TrimSuffix(rel, ".yaml")+".golden.yaml")
			if os.Getenv(updateGoldens) != "" {
				Expect(os.MkdirAll(filepath.Dir(goldenPath), 0o755)).To(Succeed())
				Expect(os.WriteFile(goldenPath, got, 0o644)).To(Succeed())
				return
			}
			want, err := os.ReadFile(goldenPath)
			Expect(err).NotTo(HaveOccurred(),
				"missing golden for %s; run UPDATE_GOLDENS=1 go test ./replay_tests/...", rel)
			Expect(string(got)).To(Equal(string(want)),
				"mutated %s drifted from its golden (applied: %s)", rel, strings.Join(applied, ", "))
		})
	}
})

func TestRecordedKnativeDefaultMergeExact(t *testing.T) {
	ctx := context.Background()
	var recordings int
	for _, path := range replayPaths(t) {
		if !strings.Contains(path, "/knative/") {
			continue
		}
		recordings++
		t.Run(strings.TrimPrefix(path, "../recorded_data/"), func(t *testing.T) {
			r, karta := replayDefinition(t, path)
			original, reader := replayInput(t, r)
			var expected map[string]any
			if err := normalizedJSONValue(original, &expected); err != nil {
				t.Fatal(err)
			}
			before := jsonText(original)
			template := expected["spec"].(map[string]any)["template"].(map[string]any)
			spec := template["spec"].(map[string]any)
			metadata := template["metadata"].(map[string]any)
			if value, exists := metadata["creationTimestamp"]; !exists || value != nil || spec["containerConcurrency"] != float64(0) || spec["timeoutSeconds"] != float64(300) {
				t.Fatal("recording must retain the three fields omitted by the historical typed replacement")
			}
			spec["schedulerName"] = goldenScheduler
			factory := resource.NewComponentFactoryFromObject(karta, original, resource.WithReferenceReader(reader))
			revision, err := factory.GetComponent("revision")
			if err != nil {
				t.Fatal(err)
			}
			values, err := revision.GetPodTemplateSpec(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for id, value := range values {
				value.Spec.SchedulerName = goldenScheduler
				values[id] = value
			}
			if err := revision.UpdatePodTemplateSpec(ctx, values); err != nil {
				t.Fatal(err)
			}
			result, err := factory.GetResource()
			if err != nil {
				t.Fatal(err)
			}
			var actual map[string]any
			if err := normalizedJSONValue(result, &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatal("default Merge changed raw Knative fields beyond the requested scheduler")
			}
			if jsonText(original) != before {
				t.Fatal("default Merge changed caller-owned recording input")
			}
		})
	}
	if recordings == 0 {
		t.Fatal("Knative recording is required")
	}
}

// pickInput walks the recording and returns the CR of the "running" state when
// present, otherwise the last captured state.
func pickInput(r *recorder.Reader) resource.KubernetesObject {
	var last resource.KubernetesObject
	for r.Next() {
		last = r.Object()
		if r.State() == "running" {
			return last
		}
	}
	return last
}

// mutateComponent applies one deterministic write per writable field family the
// component declares, and reports what it applied.
func mutateComponent(ctx context.Context, comp *resource.Component) []string {
	var applied []string
	def := comp.Definition()
	if def.SpecDefinition == nil {
		return nil
	}

	if via := def.SpecDefinition.PodTemplateSpec; via != nil && (via.PathWrite != nil || via.PathWriteExpression != "") {
		templates, err := comp.GetPodTemplateSpec(ctx)
		Expect(err).NotTo(HaveOccurred(), "read pod templates of %s", comp.Name())
		for id, template := range templates {
			template.Spec.SchedulerName = goldenScheduler
			templates[id] = template
		}
		Expect(comp.UpdatePodTemplateSpec(ctx, templates)).To(Succeed(), "write pod templates of %s", comp.Name())
		applied = append(applied, comp.Name()+":podTemplateSpec")
	}

	if via := def.SpecDefinition.PodSpec; via != nil && (via.PathWrite != nil || via.PathWriteExpression != "") {
		podSpecs, err := comp.GetPodSpec(ctx)
		Expect(err).NotTo(HaveOccurred(), "read pod specs of %s", comp.Name())
		for id, podSpec := range podSpecs {
			podSpec.SchedulerName = goldenScheduler
			podSpecs[id] = podSpec
		}
		Expect(comp.UpdatePodSpec(ctx, podSpecs)).To(Succeed(), "write pod specs of %s", comp.Name())
		applied = append(applied, comp.Name()+":podSpec")
	}

	if fragmented := def.SpecDefinition.FragmentedPodSpecDefinition; fragmented != nil {
		writableScheduler := fragmented.SchedulerName != nil && (fragmented.SchedulerName.PathWrite != nil || fragmented.SchedulerName.PathWriteExpression != "")
		writableAffinity := fragmented.NodeAffinity != nil && (fragmented.NodeAffinity.PathWrite != nil || fragmented.NodeAffinity.PathWriteExpression != "")
		if writableScheduler || writableAffinity {
			current, err := comp.GetFragmentedPodSpec(ctx)
			Expect(err).NotTo(HaveOccurred(), "read fragmented pod spec of %s", comp.Name())
			update := make(map[string]resource.FragmentedPodSpec, len(current))
			for id := range current {
				fragment := resource.FragmentedPodSpec{}
				if writableScheduler {
					fragment.SchedulerName = goldenScheduler
				}
				if writableAffinity {
					fragment.NodeAffinity = goldenNodeAffinity
				}
				update[id] = fragment
			}
			Expect(comp.UpdateFragmentedPodSpec(ctx, update)).To(Succeed(), "write fragmented pod spec of %s", comp.Name())
			if writableScheduler {
				applied = append(applied, comp.Name()+":schedulerName")
			}
			if writableAffinity {
				applied = append(applied, comp.Name()+":nodeAffinity")
			}
		}
	}

	return applied
}
