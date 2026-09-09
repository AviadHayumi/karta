// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Offline specs for the pure recorder helpers; no cluster.

var (
	initializing = kartav1alpha1.InitializingStatus
	running      = kartav1alpha1.RunningStatus
	completed    = kartav1alpha1.CompletedStatus
	failed       = kartav1alpha1.FailedStatus
)

var _ = Describe("builder guards", func() {
	It("panics when New has an empty Config.OutputDir", func() {
		Expect(func() { New(Config{}) }).To(Panic())
	})

	It("panics when AddState has an empty name", func() {
		Expect(func() { (&Recorder{}).AddState("", nil) }).To(Panic())
	})

	It("panics when AddState has a nil predicate", func() {
		Expect(func() { (&Recorder{}).AddState("Running", nil) }).To(Panic())
	})

	It("panics when SetTimeout is not positive", func() {
		Expect(func() { (&Recorder{}).SetTimeout(0) }).To(Panic())
		Expect(func() { (&Recorder{}).SetTimeout(-time.Second) }).To(Panic())
	})
})

var _ = Describe("Save", func() {
	It("rejects an incomplete fixture", func() {
		r := New(Config{OutputDir: "out"})
		for _, fx := range []Fixture{
			{Version: "v", KartaName: "n", KartaFile: "f"},
			{Operator: "op", KartaName: "n", KartaFile: "f"},
			{Operator: "op", Version: "v", KartaFile: "f"},
			{Operator: "op", Version: "v", KartaName: "n"},
		} {
			path, err := r.Save(fx, &Recording{Flow: "flow"})
			Expect(err).To(HaveOccurred())
			Expect(path).To(BeEmpty())
		}
	})

	It("ignores a nil recording", func() {
		fx := Fixture{Operator: "op", Version: "v", KartaName: "n", KartaFile: "f"}
		path, err := New(Config{OutputDir: "out"}).Save(fx, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(path).To(BeEmpty())
	})
})

var _ = Describe("classify", func() {
	It("keeps the furthest-along state the workload matches", func() {
		states := []namedState{
			{Name: running, Match: intAtLeast(1, "status", "active")},
			{Name: completed, Match: condTrue("Complete")},
		}
		Expect(classify(objWithStatus(map[string]any{"active": int64(1)}), states)).To(Equal(running))
		both := objWithStatus(map[string]any{
			"active":     int64(1),
			"conditions": []any{map[string]any{"type": "Complete", "status": "True"}},
		})
		Expect(classify(both, states)).To(Equal(completed))
		Expect(classify(objWithStatus(map[string]any{}), states)).To(BeEmpty())
	})
})

var _ = Describe("the recorded walk", func() {
	// A Running -> byte-identical Initializing dip survives dedup, and the strict order check flags it
	// unless the journey declares the dip.
	It("keeps a backwards dip and the order check flags it unless declared", func() {
		states := []namedState{
			{Name: initializing, Match: intAtLeast(1, "status", "active")},
			{Name: running, Match: intAtLeast(1, "status", "ready")},
			{Name: completed, Match: condTrue("Complete")},
		}
		initCR := func() *unstructured.Unstructured {
			return objWithStatus(map[string]any{"active": int64(1), "ready": int64(0)})
		}
		seq := []*unstructured.Unstructured{
			initCR(),
			objWithStatus(map[string]any{"active": int64(1), "ready": int64(1)}),
			initCR(),
			objWithStatus(map[string]any{"conditions": []any{map[string]any{"type": "Complete", "status": "True"}}}),
		}

		o := &observation{}
		for _, cr := range seq {
			o.keep(context.Background(), cr, classify(cr, states), true)
		}

		Expect(o.states()).To(Equal([]kartav1alpha1.ResourceStatus{initializing, running, initializing, completed}))
		Expect(validateObservedOrder(steps(initializing, running, completed), o.states(), completed)).
			To(HaveOccurred(), "strict journey should reject the undeclared Running -> Initializing dip")
		Expect(validateObservedOrder(steps(initializing, running, initializing, completed), o.states(), completed)).
			To(Succeed(), "declaring the Initializing revisit should accept the dip")
	})

	// A frame whose controller has not observed the spec yet is recorded for fidelity but stays out of the
	// order-checked walk.
	It("keeps a stale frame out of the judged walk", func() {
		o := &observation{}
		o.keep(context.Background(), objWithStatus(map[string]any{"active": int64(1)}), initializing, true)
		o.keep(context.Background(), objWithStatus(map[string]any{"active": int64(2)}), initializing, false)

		Expect(o.snapshots).To(HaveLen(2))
		Expect(o.snapshots[1].staleObservedGeneration).To(BeTrue())
		Expect(o.states()).To(HaveLen(1))
	})
})

var _ = DescribeTable("validateObservedOrder",
	func(journey []journeyStep, observed []kartav1alpha1.ResourceStatus, ok bool) {
		err := validateObservedOrder(journey, observed, terminal(journey))
		if ok {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
	Entry("exact walk", steps(initializing, running, completed), visits(initializing, running, completed), true),
	Entry("skip a required step fails", steps(initializing, running, completed), visits(initializing, completed), false),
	Entry("skip an optional step is ok",
		[]journeyStep{{State: initializing}, {State: running, Optional: true}, {State: completed}},
		visits(initializing, completed), true),
	Entry("undeclared state fails", steps(initializing, running), visits(initializing, failed), false),
	Entry("repeat dip missed is ok", steps(initializing, running, initializing, completed),
		visits(initializing, running, completed), true),
	Entry("optional dip missed is ok",
		[]journeyStep{{State: initializing}, {State: running}, {State: initializing, Optional: true}, {State: completed}},
		visits(initializing, running, completed), true),
	Entry("optional dip caught is ok",
		[]journeyStep{{State: initializing}, {State: running}, {State: initializing, Optional: true}, {State: completed}},
		visits(initializing, running, initializing, completed), true),
	Entry("undeclared dip fails", steps(initializing, running, completed),
		visits(initializing, running, initializing, completed), false),
	Entry("wrong terminal fails", steps(initializing, running, completed), visits(initializing, running), false),
)

var _ = Describe("hasObservedCurrentGeneration", func() {
	withGen := func(gen int64, observedGen *int64) *unstructured.Unstructured {
		status := map[string]any{}
		if observedGen != nil {
			status["observedGeneration"] = *observedGen
		}
		return &unstructured.Unstructured{Object: map[string]any{
			"metadata": map[string]any{"generation": gen},
			"status":   status,
		}}
	}

	It("compares observedGeneration to generation", func() {
		two := int64(2)
		Expect(hasObservedCurrentGeneration(withGen(2, &two))).To(BeTrue(), "observedGeneration == generation is observed")
		Expect(hasObservedCurrentGeneration(withGen(3, &two))).To(BeFalse(), "observedGeneration < generation is not observed")
		Expect(hasObservedCurrentGeneration(withGen(2, nil))).To(BeTrue(), "missing observedGeneration counts as observed")
	})
})

func objWithStatus(status map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{"status": status}}
}

// intAtLeast and condTrue are minimal StateCheck helpers for these unit tests; the flows package holds the
// full predicate vocabulary.
func intAtLeast(n int64, path ...string) StateCheck {
	return func(u *unstructured.Unstructured) bool {
		v, _, _ := unstructured.NestedInt64(u.Object, path...)
		return v >= n
	}
}

func condTrue(condType string) StateCheck {
	return func(u *unstructured.Unstructured) bool {
		conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
		for _, c := range conds {
			if m, ok := c.(map[string]any); ok && m["type"] == condType && m["status"] == "True" {
				return true
			}
		}
		return false
	}
}

func terminal(journey []journeyStep) kartav1alpha1.ResourceStatus {
	return journey[len(journey)-1].State
}

func steps(states ...kartav1alpha1.ResourceStatus) []journeyStep {
	j := make([]journeyStep, len(states))
	for i, s := range states {
		j[i] = journeyStep{State: s}
	}
	return j
}

func visits(states ...kartav1alpha1.ResourceStatus) []kartav1alpha1.ResourceStatus {
	return states
}

var _ = Describe("reference capture failures", func() {
	It("stops the run and keeps the capture error", func() {
		o := &observation{}
		Expect(o.keep(context.Background(), objWithStatus(map[string]any{"active": int64(1)}), initializing, true)).To(Succeed())

		// A fake client with no registered kinds fails the scope lookup; record must stop
		// immediately so the capture error is not overwritten by the timeout message.
		o.flow = &Flow{
			rec:      &Recorder{config: Config{Cluster: Cluster{Client: fake.NewClientBuilder().Build()}}},
			captures: []CapturedReference{{GVK: kartav1alpha1.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Missing"}, Name: "missing"}},
		}
		stop := o.record(context.Background(), objWithStatus(map[string]any{"active": int64(2)}))
		Expect(stop).To(BeTrue())
		Expect(o.failure).To(ContainSubstring("capture reference"))
	})
})

var _ = Describe("reference-aware dedup", func() {
	It("keeps a frame when only the captured reference changed", func() {
		o := &observation{}
		cr := objWithStatus(map[string]any{"active": int64(1)})

		Expect(o.keep(context.Background(), cr, initializing, true)).To(Succeed())
		Expect(o.keep(context.Background(), cr, initializing, true)).To(Succeed(), "identical frame dedups")
		Expect(o.snapshots).To(HaveLen(1))
	})
})
