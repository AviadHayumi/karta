// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package replay

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// The jq catalog predates TrainJob and resource references. These recordings
// remain covered by status, mutation, tree extraction and Draft replay tests.
var postJQRecordings = map[string]struct{}{
	"trainer/v1.34.0/trainer-kubeflow-org-trainjob-v1alpha1/completed.yaml": {},
	"trainer/v1.34.0/trainer-kubeflow-org-trainjob-v1alpha1/failed.yaml":    {},
	"trainer/v1.34.0/trainer-kubeflow-org-trainjob-v1alpha1/lifecycle.yaml": {},
	"trainer/v1.34.0/trainer-kubeflow-org-trainjob-v1alpha1/resumed.yaml":   {},
	"trainer/v1.34.0/trainer-kubeflow-org-trainjob-v1alpha1/suspended.yaml": {},
}

func parityExpectedDocument(rel string, base any, input map[string]any) (any, error) {
	expected, ok := deepCanon(base).(map[string]any)
	if !ok {
		return base, nil
	}
	var retained [][]string
	switch {
	case strings.HasPrefix(rel, "knative/"):
		retained = [][]string{
			{"spec", "template", "spec", "containerConcurrency"},
			{"spec", "template", "spec", "timeoutSeconds"},
		}
	case strings.HasPrefix(rel, "kserve/"):
		retained = [][]string{
			{"spec", "predictor", "model", "storageUri"},
			{"spec", "predictor", "model", "modelFormat"},
		}
		// The scheduler write succeeds before the labels write. jq's broad
		// transformer metadata replacement then loses the scheduler; Merge keeps it.
		if err := unstructured.SetNestedField(expected, parityScheduler, "spec", "transformer", "schedulerName"); err != nil {
			return nil, err
		}
	}
	for _, path := range retained {
		value, found, err := unstructured.NestedFieldCopy(input, path...)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		if err := unstructured.SetNestedField(expected, value, path...); err != nil {
			return nil, err
		}
	}
	return expected, nil
}

func TestParityBaselineCoverage(t *testing.T) {
	recordings := replayPaths(t)
	seen := make(map[string]bool, len(recordings))
	for _, path := range recordings {
		rel := strings.TrimPrefix(path, "../recorded_data/")
		seen[rel] = true
		_, err := os.Stat(filepath.Join(parityBaselineRoot, rel))
		if _, addedAfterJQ := postJQRecordings[rel]; addedAfterJQ {
			if !os.IsNotExist(err) {
				t.Fatalf("post-jq recording %s unexpectedly has a baseline or cannot be inspected: %v", rel, err)
			}
		} else if err != nil {
			t.Errorf("recording %s has no historical baseline: %v", rel, err)
		}
	}
	for rel := range postJQRecordings {
		if !seen[rel] {
			t.Errorf("declared post-jq recording %s is missing", rel)
		}
	}
	baselines, err := filepath.Glob(filepath.Join(parityBaselineRoot, "*", "*", "*", "*.yaml"))
	if err != nil || len(baselines) != len(recordings)-len(postJQRecordings) {
		t.Fatalf("historical baseline coverage does not match recordings: count=%d, err=%v", len(baselines), err)
	}
	for _, path := range baselines {
		if rel := strings.TrimPrefix(path, parityBaselineRoot+"/"); !seen[rel] {
			t.Errorf("historical baseline %s lost its recording", rel)
		}
	}
}

func TestParityExpectedDocumentPinsOnlyDocumentedCorrections(t *testing.T) {
	for _, tc := range []struct {
		name string
		path []string
	}{
		{"knative", []string{"spec", "template", "spec", "timeoutSeconds"}},
		{"kserve", []string{"spec", "predictor", "model", "storageUri"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := map[string]any{"kind": "example", "unrelated": "baseline"}
			input := map[string]any{"unrelated": "not an accepted change"}
			if err := unstructured.SetNestedField(input, "retained", tc.path...); err != nil {
				t.Fatal(err)
			}
			before := deepCanon(base)
			value, err := parityExpectedDocument(tc.name+"/fixture.yaml", base, input)
			if err != nil {
				t.Fatal(err)
			}
			expected := value.(map[string]any)
			retained, _, err := unstructured.NestedString(expected, tc.path...)
			if err != nil || retained != "retained" || expected["unrelated"] != "baseline" {
				t.Fatalf("correction did not retain exactly the allowed input: %v, %v", expected, err)
			}
			if !reflect.DeepEqual(base, before) {
				t.Fatal("correction changed the historical baseline")
			}
			wrong := deepCanon(expected).(map[string]any)
			if err := unstructured.SetNestedField(wrong, "changed", tc.path...); err != nil {
				t.Fatal(err)
			}
			if reflect.DeepEqual(stripNulls(wrong), stripNulls(expected)) {
				t.Fatal("a corrupted retained value must fail the document comparison")
			}
			unrelated, err := parityExpectedDocument("unrelated/fixture.yaml", base, input)
			if err != nil || !reflect.DeepEqual(unrelated, base) {
				t.Fatal("correction changed an unrelated catalog")
			}
		})
	}
}
