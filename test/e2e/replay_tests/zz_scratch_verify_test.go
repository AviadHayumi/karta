// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package replay

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/references"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/test/e2e/recorder"
)

// TestScratchTrainerFrames walks every recorded trainer frame and reports MatchedStatuses,
// extracted image/resources/scale. Exploration only; deleted after review.
func TestScratchTrainerFrames(t *testing.T) {
	ctx := context.Background()
	karta := kartas.TrainJob()
	paths, _ := filepath.Glob("../recorded_data/trainer/*/*/*.yaml")
	if len(paths) == 0 {
		t.Fatal("no trainer recordings")
	}
	for _, path := range paths {
		r, err := recorder.OpenRecording(path)
		if err != nil {
			t.Fatal(err)
		}
		frame := 0
		for r.Next() {
			frame++
			state, cr := r.State(), r.Object()
			reader := &recordedReader{objects: r.References()}
			root, err := resource.NewComponentFactoryFromObject(karta, cr,
				resource.WithReferenceReader(reader)).GetRootComponent()
			if err != nil {
				t.Fatalf("%s frame %d: %v", path, frame, err)
			}
			status, err := root.GetStatus(ctx)
			if err != nil {
				t.Fatalf("%s frame %d GetStatus: %v", path, frame, err)
			}
			fmt.Printf("%s frame %d recorded=%s matched=%v\n", filepath.Base(path), frame, state, status.MatchedStatuses)
			if len(status.MatchedStatuses) != 1 {
				t.Errorf("%s frame %d: expected exactly one matched status, got %v", path, frame, status.MatchedStatuses)
			}
			instances, err := root.GetExtractedInstances(ctx)
			if err != nil {
				t.Fatalf("%s frame %d extract: %v", path, frame, err)
			}
			for id, summary := range instances {
				fmt.Printf("  instance=%q image=%q replicas=%v resources=%v\n",
					id, summary.FragmentedPodSpec.Image, deref(summary.Scale.Replicas), summary.FragmentedPodSpec.Resources)
			}
		}
	}
}

func deref(v *int32) any {
	if v == nil {
		return nil
	}
	return *v
}

func trainJobObj(spec map[string]any, status map[string]any) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": "trainer.kubeflow.org/v1alpha1",
		"kind":       "TrainJob",
		"metadata":   map[string]any{"name": "tj", "namespace": "ns"},
		"spec":       spec,
	}
	if status != nil {
		obj["status"] = status
	}
	return &unstructured.Unstructured{Object: obj}
}

// TestScratchSuspendedAndComplete probes whether a frame carrying both Suspended=True and
// Complete=True double-matches.
func TestScratchSuspendedAndComplete(t *testing.T) {
	ctx := context.Background()
	cr := trainJobObj(
		map[string]any{"runtimeRef": map[string]any{"name": "rt"}, "suspend": true},
		map[string]any{"conditions": []any{
			map[string]any{"type": "Complete", "status": "True"},
			map[string]any{"type": "Suspended", "status": "True"},
		}},
	)
	root, err := resource.NewComponentFactoryFromObject(kartas.TrainJob(), cr,
		resource.WithReferences(references.ResolvedReferences{})).GetRootComponent()
	if err != nil {
		t.Fatal(err)
	}
	status, err := root.GetStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("suspended+complete matched=%v\n", status.MatchedStatuses)
}

// TestScratchMultiJobRuntime probes the containers[0]/replicatedJobs[0] index against a runtime
// shaped like the upstream torchtune runtimes: initializer jobs first, node job last.
func TestScratchMultiJobRuntime(t *testing.T) {
	ctx := context.Background()
	runtime := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "trainer.kubeflow.org/v1alpha1",
		"kind":       "ClusterTrainingRuntime",
		"metadata":   map[string]any{"name": "torchtune"},
		"spec": map[string]any{
			"mlPolicy": map[string]any{"numNodes": int64(2)},
			"template": map[string]any{"spec": map[string]any{"replicatedJobs": []any{
				map[string]any{
					"name": "dataset-initializer",
					"template": map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
						"containers": []any{map[string]any{"name": "dataset-initializer", "image": "ghcr.io/kubeflow/trainer/dataset-initializer:v2"}},
					}}}},
				},
				map[string]any{
					"name": "node",
					"template": map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
						"containers": []any{map[string]any{"name": "node", "image": "ghcr.io/kubeflow/trainer/torchtune-trainer:v2"}},
					}}}},
				},
			}}},
		},
	}}
	cr := trainJobObj(map[string]any{"runtimeRef": map[string]any{"name": "torchtune"}}, nil)
	root, err := resource.NewComponentFactoryFromObject(kartas.TrainJob(), cr,
		resource.WithReferences(references.ResolvedReferences{"trainingRuntime": {Object: runtime}})).GetRootComponent()
	if err != nil {
		t.Fatal(err)
	}
	instances, err := root.GetExtractedInstances(ctx)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for id, summary := range instances {
		fmt.Printf("torchtune instance=%q image=%q replicas=%v\n", id, summary.FragmentedPodSpec.Image, deref(summary.Scale.Replicas))
	}
}

// TestScratchUnboundReference probes whether an override-carrying TrainJob still reads when the
// runtime is gone (orValue laziness) and what an override-free TrainJob does.
func TestScratchUnboundReference(t *testing.T) {
	ctx := context.Background()
	withOverrides := trainJobObj(map[string]any{
		"runtimeRef": map[string]any{"name": "gone"},
		"trainer":    map[string]any{"image": "custom:1", "numNodes": int64(3), "resourcesPerNode": map[string]any{"requests": map[string]any{"cpu": "1"}}},
	}, nil)
	root, err := resource.NewComponentFactoryFromObject(kartas.TrainJob(), withOverrides,
		resource.WithReferences(references.ResolvedReferences{"trainingRuntime": {}})).GetRootComponent()
	if err != nil {
		t.Fatal(err)
	}
	instances, err := root.GetExtractedInstances(ctx)
	fmt.Printf("unbound+overrides err=%v\n", err)
	for id, summary := range instances {
		fmt.Printf("  instance=%q image=%q replicas=%v\n", id, summary.FragmentedPodSpec.Image, deref(summary.Scale.Replicas))
	}

	noOverrides := trainJobObj(map[string]any{"runtimeRef": map[string]any{"name": "gone"}}, nil)
	root2, err := resource.NewComponentFactoryFromObject(kartas.TrainJob(), noOverrides,
		resource.WithReferences(references.ResolvedReferences{"trainingRuntime": {}})).GetRootComponent()
	if err != nil {
		t.Fatal(err)
	}
	_, err = root2.GetExtractedInstances(ctx)
	fmt.Printf("unbound+no-overrides err=%v\n", err)

	// Status must not need the reference at all.
	status, err := root2.GetStatus(ctx)
	fmt.Printf("unbound status err=%v matched=%v\n", err, statusOrNil(status))
}

func statusOrNil(s *resource.Status) any {
	if s == nil {
		return nil
	}
	return s.MatchedStatuses
}

// TestScratchScaleType checks the int64 -> int32 conversion and a large numNodes.
func TestScratchScaleType(t *testing.T) {
	ctx := context.Background()
	cr := trainJobObj(map[string]any{
		"runtimeRef": map[string]any{"name": "rt"},
		"trainer":    map[string]any{"numNodes": int64(2147483648)}, // MaxInt32+1
	}, nil)
	root, err := resource.NewComponentFactoryFromObject(kartas.TrainJob(), cr,
		resource.WithReferences(references.ResolvedReferences{})).GetRootComponent()
	if err != nil {
		t.Fatal(err)
	}
	scales, err := root.GetScale(ctx)
	fmt.Printf("overflow scale err=%v scales=%v\n", err, scales)
}
