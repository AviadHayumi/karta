// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cel_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	celpkg "github.com/run-ai/karta/pkg/cel"
)

func TestCEL(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CEL Suite")
}

var _ = Describe("CEL evaluator", func() {
	var evaluator celpkg.Evaluator

	BeforeEach(func() {
		var err error
		evaluator, err = celpkg.Shared()
		Expect(err).NotTo(HaveOccurred())
	})

	evaluate := func(expression string, object map[string]any) (any, error) {
		out, err := evaluator.Evaluate(context.Background(), expression, object)
		if err != nil {
			return nil, err
		}

		return out.Value(), nil
	}

	DescribeTable("reads a workload's own fields",
		func(expression string, object map[string]any, want bool) {
			Expect(evaluate(expression, object)).To(Equal(want))
		},
		Entry("a present field",
			`object.status.phase == "Running"`,
			map[string]any{"status": map[string]any{"phase": "Running"}}, true),
		Entry("an absent field falls back to the default",
			`object.?status.?readyReplicas.orValue(0) > 0`,
			map[string]any{"status": map[string]any{}}, false),
		Entry("an absent parent falls back to the default",
			`object.?status.?readyReplicas.orValue(0) > 0`,
			map[string]any{}, false),
		Entry("a condition that is present",
			`object.?status.?conditions.orValue([]).exists(c, c.type == "Ready" && c.status == "True")`,
			map[string]any{"status": map[string]any{"conditions": []any{
				map[string]any{"type": "Ready", "status": "True"},
			}}}, true),
		// The absent condition is the case a declarative condition matcher cannot express: a
		// just-created workload has no Available condition at all, and must still be recognised.
		Entry("a condition that is absent",
			`!object.?status.?conditions.orValue([]).exists(c, c.type == "Available" && c.status == "True")`,
			map[string]any{"status": map[string]any{"conditions": []any{
				map[string]any{"type": "Progressing", "status": "True"},
			}}}, true),
		Entry("no conditions at all",
			`!object.?status.?conditions.orValue([]).exists(c, c.type == "Available" && c.status == "True")`,
			map[string]any{}, true),
	)

	It("rejects comparing a list to a number", func() {
		// A CronJob's status.active is a list of Job references, not a count. An untyped ordering
		// makes `[...] > 0` true; CEL refuses the comparison instead of inventing an answer.
		_, err := evaluate(`object.status.active > 0`, map[string]any{
			"status": map[string]any{"active": []any{map[string]any{"kind": "Job"}}},
		})
		Expect(err).To(HaveOccurred())
	})

	It("reports an expression that does not compile", func() {
		Expect(celpkg.Validate(`object.status.phase ==`)).To(HaveOccurred())
	})

	It("accepts an expression that compiles", func() {
		Expect(celpkg.Validate(`object.?status.?phase.orValue("") == "Running"`)).To(Succeed())
	})

	It("honours a cancelled context once a comprehension is long enough to notice", func() {
		// Cancellation is checked every checkFrequency steps, so a long comprehension observes it
		// while a short expression finishes first. That is the bound that matters: a matcher over a
		// large status cannot run past a cancelled reconcile.
		items := make([]any, 10000)
		for i := range items {
			items[i] = map[string]any{"ready": 1}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := evaluator.Evaluate(ctx, `object.status.items.all(i, i.ready > 0)`, map[string]any{
			"status": map[string]any{"items": items},
		})
		Expect(err).To(HaveOccurred())
	})

	It("returns a short expression before it ever checks for cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		// Documented behaviour, not a bug: CEL cannot loop, so a simple matcher is already bounded.
		Expect(evaluate(`object.?status.?phase.orValue("") == "Running"`, map[string]any{})).To(BeFalse())
		_ = ctx
	})
})
