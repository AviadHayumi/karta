// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package blackbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/resource"
)

func recordedPathWriteObject(t *testing.T, path string) map[string]any {
	t.Helper()
	g := NewWithT(t)
	data, err := os.ReadFile(filepath.Join("..", "e2e", "recorded_data", path))
	g.Expect(err).NotTo(HaveOccurred())
	var recording struct {
		Events []struct {
			State  string         `json:"state"`
			Object map[string]any `json:"object"`
		} `json:"events"`
	}
	g.Expect(yaml.Unmarshal(data, &recording)).To(Succeed())
	for _, event := range recording.Events {
		if event.State == "Running" {
			return event.Object
		}
	}
	t.Fatal("recording has no Running event")
	return nil
}

func TestRecordedKnativeMergePreservesFieldsOutsidePodTemplateSpec(t *testing.T) {
	for _, patchType := range []resource.PatchType{"", resource.PatchTypeJSONPatch} {
		name := "default MergePatch Merge"
		if patchType != "" {
			name = "JSONPatch Merge"
		}
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()
			object := recordedPathWriteObject(t, "knative/v1.34.0/serving-knative-dev-service-v1/running.yaml")
			template := object["spec"].(map[string]any)["template"].(map[string]any)
			before := template["spec"].(map[string]any)
			g.Expect(before).To(HaveKeyWithValue("timeoutSeconds", float64(300)))
			g.Expect(before).To(HaveKeyWithValue("containerConcurrency", float64(0)))
			g.Expect(template["metadata"]).To(HaveKeyWithValue("creationTimestamp", BeNil()))

			factory, accessor := newCatalogPathFactory(t, kartas.KnativeServing(), object, resource.MutationOptions{PatchType: patchType})
			revision, err := factory.GetComponent("revision")
			g.Expect(err).NotTo(HaveOccurred())
			templates, err := revision.GetPodTemplateSpec(ctx)
			g.Expect(err).NotTo(HaveOccurred())
			updatedTemplate := templates[""]
			updatedTemplate.Spec.SchedulerName = "updated-scheduler"
			templates[""] = updatedTemplate
			g.Expect(revision.UpdatePodTemplateSpec(ctx, templates)).To(Succeed())

			updated, err := accessor.GetObject()
			g.Expect(err).NotTo(HaveOccurred())
			template = updated["spec"].(map[string]any)["template"].(map[string]any)
			after := template["spec"].(map[string]any)
			g.Expect(after).To(HaveKeyWithValue("schedulerName", "updated-scheduler"))
			g.Expect(after).To(HaveKeyWithValue("timeoutSeconds", float64(300)))
			g.Expect(after).To(HaveKeyWithValue("containerConcurrency", float64(0)))
			g.Expect(template["metadata"]).To(HaveKeyWithValue("creationTimestamp", BeNil()))
		})
	}
}

// An absent optional component still produces a zero-valued PodSpec. This test
// characterizes the resulting write and does not endorse creating that component.
func TestRecordedKServeMissingTransformerNullSemantics(t *testing.T) {
	for _, patchType := range []resource.PatchType{resource.PatchTypeMergePatch, resource.PatchTypeJSONPatch} {
		t.Run(string(patchType), func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()
			object := recordedPathWriteObject(t, "kserve/v1.34.0/serving-kserve-io-inferenceservice-v1beta1/running.yaml")
			g.Expect(object["spec"]).NotTo(HaveKey("transformer"))
			factory, accessor := newCatalogPathFactory(t, kartas.KServe(), object, resource.MutationOptions{PatchType: patchType})
			transformer, err := factory.GetComponent("transformer")
			g.Expect(err).NotTo(HaveOccurred())
			specs, err := transformer.GetPodSpec(ctx)
			g.Expect(err).NotTo(HaveOccurred())
			spec := specs[""]
			g.Expect(spec.Containers).To(BeNil())
			spec.SchedulerName = "updated-scheduler"
			specs[""] = spec
			g.Expect(transformer.UpdatePodSpec(ctx, specs)).To(Succeed())
			updated, err := accessor.GetObject()
			g.Expect(err).NotTo(HaveOccurred())
			written := updated["spec"].(map[string]any)["transformer"].(map[string]any)
			want := map[string]any{"schedulerName": "updated-scheduler"}
			if patchType == resource.PatchTypeJSONPatch {
				want["containers"] = nil
			}
			g.Expect(written).To(Equal(want))
		})
	}
}
