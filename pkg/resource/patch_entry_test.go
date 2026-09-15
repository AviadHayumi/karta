// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package resource

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/expression"
)

func patchTestObject() map[string]any {
	return map[string]any{
		"spec": map[string]any{
			"flavor": "a",
			"template": map[string]any{
				"metadata": map[string]any{"labels": map[string]any{"keep": "no"}},
			},
		},
	}
}

func patchTestDefinition(strategy v1alpha1.PatchStrategy, entries ...v1alpha1.PatchEntry) v1alpha1.ComponentDefinition {
	return v1alpha1.ComponentDefinition{
		Name: "job",
		SpecDefinition: &v1alpha1.SpecDefinition{
			PodTemplateSpec: &v1alpha1.ValueAccessor{
				Expression:    `object[?"spec"][?"template"].orValue(null)`,
				Patches:       entries,
				PatchStrategy: strategy,
			},
		},
	}
}

func suspendTestDefinition(suspend ...v1alpha1.PatchEntry) v1alpha1.ComponentDefinition {
	return v1alpha1.ComponentDefinition{
		Name: "job",
		SuspendDefinition: &v1alpha1.SuspendDefinition{
			SuspendActions: suspend,
			ResumeActions:  []v1alpha1.PatchEntry{{PatchType: v1alpha1.PatchTypeMergePatch, Expression: `{"spec": {"suspend": false}}`}},
		},
	}
}

func spec(runner expression.Runner) map[string]any {
	object, err := runner.GetObject()
	Expect(err).NotTo(HaveOccurred())

	return object.(map[string]any)["spec"].(map[string]any)
}

var _ = Describe("patch entries", func() {
	var (
		ctx      context.Context
		runner   expression.Runner
		accessor *Accessor
		template corev1.PodTemplateSpec
	)

	BeforeEach(func() {
		ctx = context.Background()
		runner = mustCelRunner(patchTestObject())
		accessor = NewAccessor(runner)
		template = corev1.PodTemplateSpec{Spec: corev1.PodSpec{NodeSelector: map[string]string{"pool": "trains"}}}
	})

	Describe("entry selection", func() {
		It("applies the first entry whose conditions hold and skips the rest", func() {
			definition := patchTestDefinition("",
				v1alpha1.PatchEntry{
					MatchConditions: []v1alpha1.MatchCondition{{Name: "flavor-a", Expression: `object.spec.flavor == "a"`}},
					PatchType:       v1alpha1.PatchTypeMergePatch,
					Expression:      `{"spec": {"winner": "conditional", "template": value}}`,
				},
				v1alpha1.PatchEntry{
					PatchType:  v1alpha1.PatchTypeMergePatch,
					Expression: `{"spec": {"winner": "fallback", "template": value}}`,
				},
			)
			Expect(accessor.UpdatePodTemplateSpec(ctx, definition, []corev1.PodTemplateSpec{template})).To(Succeed())
			Expect(spec(runner)["winner"]).To(Equal("conditional"))
		})

		It("falls through to the unconditional last entry", func() {
			definition := patchTestDefinition("",
				v1alpha1.PatchEntry{
					MatchConditions: []v1alpha1.MatchCondition{{Name: "flavor-z", Expression: `object.spec.flavor == "z"`}},
					PatchType:       v1alpha1.PatchTypeMergePatch,
					Expression:      `{"spec": {"winner": "conditional", "template": value}}`,
				},
				v1alpha1.PatchEntry{
					PatchType:  v1alpha1.PatchTypeMergePatch,
					Expression: `{"spec": {"winner": "fallback", "template": value}}`,
				},
			)
			Expect(accessor.UpdatePodTemplateSpec(ctx, definition, []corev1.PodTemplateSpec{template})).To(Succeed())
			Expect(spec(runner)["winner"]).To(Equal("fallback"))
		})

		It("fails loudly when no entry matches", func() {
			definition := patchTestDefinition("",
				v1alpha1.PatchEntry{
					MatchConditions: []v1alpha1.MatchCondition{{Name: "flavor-z", Expression: `object.spec.flavor == "z"`}},
					PatchType:       v1alpha1.PatchTypeMergePatch,
					Expression:      `{"spec": {"template": value}}`,
				},
			)
			err := accessor.UpdatePodTemplateSpec(ctx, definition, []corev1.PodTemplateSpec{template})
			Expect(err).To(MatchError(ContainSubstring("no patch entry matched")))
		})

		It("fails the write when a condition errors instead of treating it as false", func() {
			definition := patchTestDefinition("",
				v1alpha1.PatchEntry{
					MatchConditions: []v1alpha1.MatchCondition{{Name: "broken", Expression: `object.spec.missing == "a"`}},
					PatchType:       v1alpha1.PatchTypeMergePatch,
					Expression:      `{"spec": {"template": value}}`,
				},
				v1alpha1.PatchEntry{
					PatchType:  v1alpha1.PatchTypeMergePatch,
					Expression: `{"spec": {"template": value}}`,
				},
			)
			err := accessor.UpdatePodTemplateSpec(ctx, definition, []corev1.PodTemplateSpec{template})
			Expect(err).To(MatchError(ContainSubstring(`match condition "broken"`)))
		})

		It("fails the write when a condition returns a non-boolean", func() {
			definition := patchTestDefinition("",
				v1alpha1.PatchEntry{
					MatchConditions: []v1alpha1.MatchCondition{{Name: "stringy", Expression: `"yes"`}},
					PatchType:       v1alpha1.PatchTypeMergePatch,
					Expression:      `{"spec": {"template": value}}`,
				},
			)
			err := accessor.UpdatePodTemplateSpec(ctx, definition, []corev1.PodTemplateSpec{template})
			Expect(err).To(MatchError(ContainSubstring("must return a boolean")))
		})
	})

	Describe("declared patch types", func() {
		It("rejects a MergePatch expression that builds a list", func() {
			definition := patchTestDefinition("", v1alpha1.PatchEntry{
				PatchType:  v1alpha1.PatchTypeMergePatch,
				Expression: `[{"op": "add", "path": "/spec/template", "value": value}]`,
			})
			err := accessor.UpdatePodTemplateSpec(ctx, definition, []corev1.PodTemplateSpec{template})
			Expect(err).To(MatchError(ContainSubstring("a MergePatch expression must build a map")))
		})

		It("rejects a JSONPatch expression that builds a map", func() {
			definition := patchTestDefinition("", v1alpha1.PatchEntry{
				PatchType:  v1alpha1.PatchTypeJSONPatch,
				Expression: `{"spec": {"template": value}}`,
			})
			err := accessor.UpdatePodTemplateSpec(ctx, definition, []corev1.PodTemplateSpec{template})
			Expect(err).To(MatchError(ContainSubstring("a JSONPatch expression must build a list")))
		})

		It("treats an empty map as no change", func() {
			definition := patchTestDefinition("", v1alpha1.PatchEntry{
				PatchType:  v1alpha1.PatchTypeMergePatch,
				Expression: `{}`,
			})
			Expect(accessor.UpdatePodTemplateSpec(ctx, definition, []corev1.PodTemplateSpec{template})).To(Succeed())
			Expect(spec(runner)).To(Equal(patchTestObject()["spec"]))
		})

		It("applies a JSONPatch entry as RFC 6902 operations", func() {
			definition := patchTestDefinition("", v1alpha1.PatchEntry{
				PatchType:  v1alpha1.PatchTypeJSONPatch,
				Expression: `[{"op": "add", "path": "/spec/template", "value": value}]`,
			})
			Expect(accessor.UpdatePodTemplateSpec(ctx, definition, []corev1.PodTemplateSpec{template})).To(Succeed())
			templateDoc := spec(runner)["template"].(map[string]any)
			Expect(templateDoc["spec"].(map[string]any)["nodeSelector"]).To(HaveKeyWithValue("pool", "trains"))
			Expect(templateDoc["metadata"]).NotTo(HaveKey("labels"))
		})
	})

	Describe("patch strategy", func() {
		It("Replace clears the field before setting it", func() {
			definition := patchTestDefinition(v1alpha1.PatchStrategyReplace, v1alpha1.PatchEntry{
				PatchType:  v1alpha1.PatchTypeMergePatch,
				Expression: `{"spec": {"template": value}}`,
			})
			Expect(accessor.UpdatePodTemplateSpec(ctx, definition, []corev1.PodTemplateSpec{template})).To(Succeed())
			templateDoc := spec(runner)["template"].(map[string]any)
			Expect(templateDoc["metadata"]).NotTo(HaveKeyWithValue("labels", HaveKeyWithValue("keep", "no")))
		})

		It("Merge keeps what the patch does not name", func() {
			definition := patchTestDefinition(v1alpha1.PatchStrategyMerge, v1alpha1.PatchEntry{
				PatchType:  v1alpha1.PatchTypeMergePatch,
				Expression: `{"spec": {"template": value}}`,
			})
			Expect(accessor.UpdatePodTemplateSpec(ctx, definition, []corev1.PodTemplateSpec{template})).To(Succeed())
			templateDoc := spec(runner)["template"].(map[string]any)
			Expect(templateDoc["metadata"].(map[string]any)["labels"]).To(HaveKeyWithValue("keep", "no"))
		})

		It("Replace with a JSONPatch entry gets a single pass", func() {
			definition := patchTestDefinition(v1alpha1.PatchStrategyReplace, v1alpha1.PatchEntry{
				PatchType:  v1alpha1.PatchTypeJSONPatch,
				Expression: `[{"op": "add", "path": "/spec/template", "value": value}]`,
			})
			Expect(accessor.UpdatePodTemplateSpec(ctx, definition, []corev1.PodTemplateSpec{template})).To(Succeed())
			templateDoc := spec(runner)["template"].(map[string]any)
			Expect(templateDoc["spec"].(map[string]any)["nodeSelector"]).To(HaveKeyWithValue("pool", "trains"))
		})
	})

	Describe("suspend and resume actions", func() {
		It("applies every matching action in sequence and skips the rest", func() {
			definition := suspendTestDefinition(
				v1alpha1.PatchEntry{
					MatchConditions: []v1alpha1.MatchCondition{{Name: "flavor-a", Expression: `object.spec.flavor == "a"`}},
					PatchType:       v1alpha1.PatchTypeMergePatch,
					Expression:      `{"spec": {"suspend": true}}`,
				},
				v1alpha1.PatchEntry{
					MatchConditions: []v1alpha1.MatchCondition{{Name: "flavor-z", Expression: `object.spec.flavor == "z"`}},
					PatchType:       v1alpha1.PatchTypeMergePatch,
					Expression:      `{"spec": {"skipped": true}}`,
				},
				v1alpha1.PatchEntry{
					PatchType:  v1alpha1.PatchTypeMergePatch,
					Expression: `{"metadata": {"labels": {"state": "suspended"}}}`,
				},
			)
			Expect(accessor.ApplySuspendActions(ctx, definition)).To(Succeed())
			Expect(spec(runner)["suspend"]).To(BeTrue())
			Expect(spec(runner)).NotTo(HaveKey("skipped"))
			object, err := runner.GetObject()
			Expect(err).NotTo(HaveOccurred())
			labels := object.(map[string]any)["metadata"].(map[string]any)["labels"]
			Expect(labels).To(HaveKeyWithValue("state", "suspended"))
		})

		It("fails loudly when no action matches", func() {
			definition := suspendTestDefinition(v1alpha1.PatchEntry{
				MatchConditions: []v1alpha1.MatchCondition{{Name: "flavor-z", Expression: `object.spec.flavor == "z"`}},
				PatchType:       v1alpha1.PatchTypeMergePatch,
				Expression:      `{"spec": {"suspend": true}}`,
			})
			err := accessor.ApplySuspendActions(ctx, definition)
			Expect(err).To(MatchError(ContainSubstring("no action matched")))
		})

		It("applies a JSONPatch action", func() {
			definition := suspendTestDefinition(v1alpha1.PatchEntry{
				PatchType:  v1alpha1.PatchTypeJSONPatch,
				Expression: `[{"op": "add", "path": "/spec/suspend", "value": true}]`,
			})
			Expect(accessor.ApplySuspendActions(ctx, definition)).To(Succeed())
			Expect(spec(runner)["suspend"]).To(BeTrue())
		})

		It("fails the operation when an action condition errors", func() {
			definition := suspendTestDefinition(v1alpha1.PatchEntry{
				MatchConditions: []v1alpha1.MatchCondition{{Name: "broken", Expression: `object.spec.missing == true`}},
				PatchType:       v1alpha1.PatchTypeMergePatch,
				Expression:      `{"spec": {"suspend": true}}`,
			})
			err := accessor.ApplySuspendActions(ctx, definition)
			Expect(err).To(MatchError(ContainSubstring(`match condition "broken"`)))
		})
	})
})
