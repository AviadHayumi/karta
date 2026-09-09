// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cel_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	celpkg "github.com/run-ai/karta/pkg/cel"
)

func decode(document string) map[string]any {
	out := map[string]any{}
	Expect(json.Unmarshal([]byte(document), &out)).To(Succeed())

	return out
}

var _ = Describe("CEL runner", func() {
	ctx := context.Background()

	It("evaluates a field", func() {
		r, err := celpkg.NewRunner(decode(`{"spec":{"replicas":3}}`))
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Evaluate(ctx, `object.spec.replicas`)).To(Equal([]any{3.0}))
	})

	It("reads a missing field as null through optional chaining", func() {
		r, err := celpkg.NewRunner(decode(`{"spec":{}}`))
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Evaluate(ctx, `object[?"spec"][?"replicas"].orValue(null)`)).To(Equal([]any{nil}))
	})

	It("spreads a list result into the stream shape", func() {
		r, err := celpkg.NewRunner(decode(`{"spec":{"items":[{"n":1},{"n":2}]}}`))
		Expect(err).NotTo(HaveOccurred())
		results, err := r.Evaluate(ctx, `object.spec.items.map(x, x.n)`)
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(HaveLen(2))
	})

	It("binds value, instance and index for a patch expression", func() {
		r, err := celpkg.NewRunner(decode(`{"spec":{}}`))
		Expect(err).NotTo(HaveOccurred())
		evaluator, ok := r.(interface {
			EvaluateWithVariables(context.Context, string, map[string]any) ([]any, error)
		})
		Expect(ok).To(BeTrue())
		out, err := evaluator.EvaluateWithVariables(ctx,
			`{"spec": {"suspend": value, "id": instance, "at": index}}`,
			map[string]any{"value": true, "instance": "a", "index": 2})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(HaveLen(1))
		spec := out[0].(map[string]any)["spec"].(map[string]any)
		Expect(spec["suspend"]).To(Equal(true))
		Expect(spec["id"]).To(Equal("a"))
	})

	It("replaces the whole document on a root assign", func() {
		cel, err := celpkg.NewRunner(decode(`{"spec":{"replicas":1}}`))
		Expect(err).NotTo(HaveOccurred())

		Expect(cel.Assign(ctx, `.`, "replaced")).To(Succeed())
		Expect(cel.GetObject()).To(Equal("replaced"))
	})

	It("refuses to assign anywhere but the root", func() {
		r, err := celpkg.NewRunner(decode(`{"spec":{}}`))
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Assign(ctx, `.spec.replicas`, 2)).To(MatchError(ContainSubstring("writes are patches")))
	})

	It("keeps an object handed out earlier intact across a root replace", func() {
		r, err := celpkg.NewRunner(decode(`{"spec":{"replicas":3}}`))
		Expect(err).NotTo(HaveOccurred())
		before, err := r.GetObject()
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Assign(ctx, `.`, map[string]any{"spec": map[string]any{"replicas": 9}})).To(Succeed())
		Expect(before.(map[string]any)["spec"].(map[string]any)["replicas"]).To(Equal(3.0))
		after, err := r.GetObject()
		Expect(err).NotTo(HaveOccurred())
		Expect(after.(map[string]any)["spec"].(map[string]any)["replicas"]).To(Equal(9.0))
	})
})
