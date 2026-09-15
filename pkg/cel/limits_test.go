// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cel_test

import (
	"context"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	celpkg "github.com/run-ai/karta/pkg/cel"
)

// CEL cannot loop, and every step is charged against a budget, so a runaway expression fails fast
// instead of stalling a reconcile.
var _ = Describe("CEL is bounded", func() {
	var evaluator celpkg.Evaluator

	BeforeEach(func() {
		var err error
		evaluator, err = celpkg.Shared()
		Expect(err).NotTo(HaveOccurred())
	})

	// A workload with enough elements that a nested comprehension over it would not finish.
	workload := func(items int) map[string]any {
		list := make([]any, items)
		for i := range list {
			list[i] = map[string]any{"n": i}
		}

		return map[string]any{"items": list}
	}

	DescribeTable("stops an expression whose cost explodes",
		func(expression string) {
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)

			start := time.Now()
			_, err := evaluator.Evaluate(context.Background(), expression, workload(100))
			elapsed := time.Since(start)

			runtime.ReadMemStats(&after)
			allocated := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)

			Expect(err).To(HaveOccurred(), "a matcher over 100^n elements must not be allowed to finish")
			Expect(elapsed).To(BeNumerically("<", 5*time.Second), "stopped quickly")
			Expect(allocated).To(BeNumerically("<", 512), "stopped before allocating heavily (MiB)")
		},
		Entry("three nested comprehensions",
			`object.items.all(a, object.items.all(b, object.items.all(c, c.n >= 0)))`),
		Entry("four nested comprehensions",
			`object.items.all(a, object.items.all(b, object.items.all(c, object.items.all(d, d.n >= 0))))`),
		Entry("a comprehension building a large list",
			`object.items.map(a, object.items.map(b, object.items.map(c, c.n)))`),
	)

	It("leaves an honest matcher far below the budget", func() {
		// The shape every catalog matcher has: a few field reads and a condition scan.
		expression := `object.?status.?conditions.orValue([]).exists(c, c.type == "Available" && c.status == "True") && object.?spec.?replicas.orValue(0) > 0`
		object := map[string]any{
			"spec": map[string]any{"replicas": 3},
			"status": map[string]any{"conditions": []any{
				map[string]any{"type": "Available", "status": "True"},
			}},
		}
		Expect(evaluator.Evaluate(context.Background(), expression, object)).Error().NotTo(HaveOccurred())
	})
})
