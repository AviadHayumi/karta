// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cel_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	celpkg "github.com/run-ai/karta/pkg/cel"
)

// Definition-level variables compose expressions the way ValidatingAdmissionPolicy variables do:
// declared once, referenced everywhere, later variables see earlier ones - in both engines.
var _ = Describe("named variables", func() {
	ctx := context.Background()
	document := `{"spec":{"predictor":{"minReplicas":1,"model":{"storageUri":"s3://m"}}}}`

	It("compose in CEL, later ones seeing earlier ones", func() {
		r, err := celpkg.NewRunnerWithVariables(decode(document), []celpkg.NamedExpression{
			{Name: "keys", Expression: `object.spec.predictor.map(k, k).sort()`},
			{Name: "first", Expression: `variables.keys[0]`},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Evaluate(ctx, `variables.first`)).To(Equal([]any{"minReplicas"}))
	})

	It("are visible to a patch expression alongside value", func() {
		r, err := celpkg.NewRunnerWithVariables(decode(document), []celpkg.NamedExpression{
			{Name: "key", Expression: `"model"`},
		})
		Expect(err).NotTo(HaveOccurred())
		evaluator := r.(interface {
			EvaluateWithVariables(context.Context, string, map[string]any) ([]any, error)
		})
		out, err := evaluator.EvaluateWithVariables(ctx,
			`{"spec": {"predictor": {variables.key: value}}}`, map[string]any{"value": "X"})
		Expect(err).NotTo(HaveOccurred())
		predictor := out[0].(map[string]any)["spec"].(map[string]any)["predictor"].(map[string]any)
		Expect(predictor["model"]).To(Equal("X"))
	})

	It("are lazy: a failing variable cannot poison an expression that never references it", func() {
		r, err := celpkg.NewRunnerWithVariables(decode(document), []celpkg.NamedExpression{
			{Name: "broken", Expression: `object.spec.predictor.map(k, k) + 1`},
			{Name: "key", Expression: `"model"`},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Evaluate(ctx, `object.spec.predictor[variables.key].storageUri`)).To(Equal([]any{"s3://m"}))
		_, err = r.Evaluate(ctx, `variables.broken`)
		Expect(err).To(HaveOccurred())
	})

})
