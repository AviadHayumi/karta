// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/e2e/recorder"
)

// TrainJob states are judged from its own fields: only Suspended, Complete and Failed exist as
// conditions; a job with pods active and no terminal condition is running, and one with neither
// is initializing.
func trainjobRunning() recorder.StateCheck {
	return func(cr *unstructured.Unstructured) bool {
		jobs, _, _ := unstructured.NestedSlice(cr.Object, "status", "jobsStatus")
		for _, entry := range jobs {
			job, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			active, _, _ := unstructured.NestedInt64(job, "active")
			ready, _, _ := unstructured.NestedInt64(job, "ready")
			if active > 0 || ready > 0 {
				return true
			}
		}

		return false
	}
}

var _ = Describe("TrainJob (trainer)", Ordered, Label("trainer"), func() {
	var rec *recorder.Recorder
	var fx recorder.Fixture

	// The TrainJob karta reads the base pod template through its trainingRuntime reference, so
	// every flow captures the runtime alongside the workload: the replay resolves the reference
	// from the recording alone.
	runtime := recorder.CapturedReference{
		GVK:  kartav1alpha1.GroupVersionKind{Group: "trainer.kubeflow.org", Version: "v1alpha1", Kind: "ClusterTrainingRuntime"},
		Name: "karta-busybox",
	}

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/trainer-kubeflow-org-trainjob-v1alpha1.yaml", "trainer-kubeflow-org-trainjob-v1alpha1")
		fx = recorder.Fixture{
			Operator:  "trainer",
			Version:   operatorVersion("trainer"),
			KartaName: "trainer-kubeflow-org-trainjob-v1alpha1",
			KartaFile: "docs/catalog/trainer-kubeflow-org-trainjob-v1alpha1.yaml",
		}
		rec = recorder.New(cfg).
			AddState(kartav1alpha1.InitializingStatus, AllOf(CondNotTrue("Complete"), CondNotTrue("Failed"), CondNotTrue("Suspended"))).
			AddState(kartav1alpha1.RunningStatus, trainjobRunning()).
			AddState(kartav1alpha1.SuspendedStatus, CondTrue("Suspended")).
			AddState(kartav1alpha1.CompletedStatus, CondTrue("Complete"))
	})

	It("completed", func(ctx SpecContext) {
		out, err := recorder.NewFlow(rec, "completed", "testdata/trainer/completed.yaml").
			Capturing(runtime).
			Through(
				recorder.Reaches(kartav1alpha1.InitializingStatus).Optional(),
				recorder.Reaches(kartav1alpha1.RunningStatus).Optional(),
				recorder.Reaches(kartav1alpha1.CompletedStatus),
			).Run(ctx)
		Expect(rec.Save(fx, out)).Error().NotTo(HaveOccurred())
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		out, err := recorder.NewFlow(rec, "suspended", "testdata/trainer/suspended.yaml").
			Capturing(runtime).
			Through(recorder.Reaches(kartav1alpha1.SuspendedStatus)).Run(ctx)
		Expect(rec.Save(fx, out)).Error().NotTo(HaveOccurred())
		Expect(err).To(Succeed())
	})

	It("resumed", func(ctx SpecContext) {
		out, err := recorder.NewFlow(rec, "resumed", "testdata/trainer/suspended.yaml").
			Capturing(runtime).
			Through(
				recorder.Reaches(kartav1alpha1.SuspendedStatus).Do(Resume()),
				recorder.Reaches(kartav1alpha1.InitializingStatus).Optional(),
				recorder.Reaches(kartav1alpha1.RunningStatus).Optional(),
				recorder.Reaches(kartav1alpha1.CompletedStatus),
			).Run(ctx)
		Expect(rec.Save(fx, out)).Error().NotTo(HaveOccurred())
		Expect(err).To(Succeed())
	})
})
