// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cel_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	celpkg "github.com/run-ai/karta/pkg/cel"
	"github.com/run-ai/karta/pkg/expression"
)

var _ = Describe("References binding", func() {
	ctx := context.Background()
	workload := map[string]any{
		"metadata": map[string]any{"name": "fine-tune"},
		"spec":     map[string]any{"trainer": map[string]any{}},
	}
	runtimeImage := map[string]any{
		"trainingRuntime": map[string]any{
			"spec": map[string]any{"image": "pytorch/pytorch:2.3.0"},
		},
	}

	It("reads references.<name> like any other input", func() {
		calls := 0
		runner, err := celpkg.NewRunnerWithVariables(workload, nil,
			celpkg.WithReferenceProvider(func(context.Context) (map[string]any, error) {
				calls++

				return runtimeImage, nil
			}))
		Expect(err).NotTo(HaveOccurred())

		results, err := runner.Evaluate(ctx, `references.trainingRuntime.spec.image`)
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(Equal([]any{"pytorch/pytorch:2.3.0"}))
		Expect(calls).To(Equal(1))
	})

	It("coalesces an absent workload override into the referenced base", func() {
		runner, err := celpkg.NewRunnerWithVariables(workload, nil,
			celpkg.WithReferenceProvider(func(context.Context) (map[string]any, error) {
				return runtimeImage, nil
			}))
		Expect(err).NotTo(HaveOccurred())

		results, err := runner.Evaluate(ctx,
			`object.?spec.?trainer.?image.orValue(references.trainingRuntime.spec.image)`)
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(Equal([]any{"pytorch/pytorch:2.3.0"}))
	})

	It("never calls the provider for an expression that does not mention references", func() {
		called := false
		runner, err := celpkg.NewRunnerWithVariables(workload, nil,
			celpkg.WithReferenceProvider(func(context.Context) (map[string]any, error) {
				called = true

				return nil, errors.New("must not run")
			}))
		Expect(err).NotTo(HaveOccurred())

		results, err := runner.Evaluate(ctx, `object.metadata.name`)
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(Equal([]any{"fine-tune"}))
		Expect(called).To(BeFalse())
	})

	It("memoizes the provider across evaluations", func() {
		calls := 0
		runner, err := celpkg.NewRunnerWithVariables(workload, nil,
			celpkg.WithReferenceProvider(func(context.Context) (map[string]any, error) {
				calls++

				return runtimeImage, nil
			}))
		Expect(err).NotTo(HaveOccurred())

		for range 3 {
			_, err := runner.Evaluate(ctx, `references.trainingRuntime.spec.image`)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(calls).To(Equal(1))
	})

	It("returns ErrReferencesNotSupported when nothing was provided", func() {
		runner, err := celpkg.NewRunner(workload)
		Expect(err).NotTo(HaveOccurred())

		_, err = runner.Evaluate(ctx, `references.trainingRuntime.spec.image`)
		Expect(errors.Is(err, expression.ErrReferencesNotSupported)).To(BeTrue())
	})

	It("makes references visible to definition variables", func() {
		runner, err := celpkg.NewRunnerWithVariables(workload,
			[]celpkg.NamedExpression{{
				Name:       "baseImage",
				Expression: `references.trainingRuntime.spec.image`,
			}},
			celpkg.WithReferenceProvider(func(context.Context) (map[string]any, error) {
				return runtimeImage, nil
			}))
		Expect(err).NotTo(HaveOccurred())

		results, err := runner.Evaluate(ctx, `variables.baseImage`)
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(Equal([]any{"pytorch/pytorch:2.3.0"}))
	})

	It("fails on use when a lookup found nothing, and defaults through optional access", func() {
		runner, err := celpkg.NewRunnerWithVariables(workload, nil,
			celpkg.WithReferenceProvider(func(context.Context) (map[string]any, error) {
				return map[string]any{}, nil
			}))
		Expect(err).NotTo(HaveOccurred())

		_, err = runner.Evaluate(ctx, `references.trainingRuntime.spec.image`)
		Expect(err).To(HaveOccurred())

		results, err := runner.Evaluate(ctx, `references[?"trainingRuntime"].orValue({"spec": {"image": "fallback"}}).spec.image`)
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(Equal([]any{"fallback"}))
	})

	It("binds references inside patch construction", func() {
		runner, err := celpkg.NewRunnerWithVariables(workload, nil,
			celpkg.WithReferenceProvider(func(context.Context) (map[string]any, error) {
				return runtimeImage, nil
			}))
		Expect(err).NotTo(HaveOccurred())

		results, err := runner.EvaluateWithVariables(ctx,
			`{"spec": {"image": references.trainingRuntime.spec.image}}`, map[string]any{})
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(HaveLen(1))
		Expect(results[0]).To(Equal(map[string]any{"spec": map[string]any{"image": "pytorch/pytorch:2.3.0"}}))
	})
})
