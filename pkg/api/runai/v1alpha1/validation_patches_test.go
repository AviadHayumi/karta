// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func kartaWithAccessor(accessor *ValueAccessor) *Karta {
	return &Karta{
		ObjectMeta: metav1.ObjectMeta{Name: "patch-karta"},
		Spec: KartaSpec{
			StructureDefinition: StructureDefinition{
				RootComponent: ComponentDefinition{
					Name:             "root",
					Kind:             &GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
					StatusDefinition: &StatusDefinition{StatusMappings: StatusMappings{}},
					SpecDefinition:   &SpecDefinition{PodTemplateSpec: accessor},
				},
			},
		},
	}
}

var _ = Describe("patch entry validation", func() {
	entry := func(conditions ...MatchCondition) PatchEntry {
		return PatchEntry{
			MatchConditions: conditions,
			PatchType:       PatchTypeMergePatch,
			Expression:      `{"spec": {"template": value}}`,
		}
	}

	It("accepts conditional entries with an unconditional entry last", func() {
		karta := kartaWithAccessor(&ValueAccessor{
			Expression: `object[?"spec"][?"template"].orValue(null)`,
			Patches: []PatchEntry{
				entry(MatchCondition{Name: "has-job-template", Expression: `object.?spec.?jobTemplate.hasValue()`}),
				entry(),
			},
		})
		Expect(NewKartaValidator(karta).Validate()).To(Succeed())
	})

	It("rejects an unconditional entry that is not last", func() {
		karta := kartaWithAccessor(&ValueAccessor{
			Patches: []PatchEntry{
				entry(),
				entry(MatchCondition{Name: "never-reached", Expression: `true`}),
			},
		})
		Expect(NewKartaValidator(karta).Validate()).To(MatchError(ContainSubstring("may only appear last")))
	})

	It("rejects an entry without a patch type", func() {
		karta := kartaWithAccessor(&ValueAccessor{
			Patches: []PatchEntry{{Expression: `{"spec": {"template": value}}`}},
		})
		Expect(NewKartaValidator(karta).Validate()).To(MatchError(ContainSubstring("patchType must be MergePatch or JSONPatch")))
	})

	It("rejects an entry without an expression", func() {
		karta := kartaWithAccessor(&ValueAccessor{
			Patches: []PatchEntry{{PatchType: PatchTypeJSONPatch}},
		})
		Expect(NewKartaValidator(karta).Validate()).To(MatchError(ContainSubstring("expression is required")))
	})

	It("rejects an unknown patch strategy", func() {
		karta := kartaWithAccessor(&ValueAccessor{
			Patches:       []PatchEntry{entry()},
			PatchStrategy: "Overwrite",
		})
		Expect(NewKartaValidator(karta).Validate()).To(MatchError(ContainSubstring("unknown patchStrategy")))
	})

	It("rejects unnamed and duplicate match conditions", func() {
		karta := kartaWithAccessor(&ValueAccessor{
			Patches: []PatchEntry{entry(
				MatchCondition{Expression: `true`},
				MatchCondition{Name: "twice", Expression: `true`},
				MatchCondition{Name: "twice", Expression: `true`},
				MatchCondition{Name: "empty"},
			)},
		})
		err := NewKartaValidator(karta).Validate()
		Expect(err).To(MatchError(ContainSubstring("name is required")))
		Expect(err).To(MatchError(ContainSubstring(`name "twice" is not unique`)))
		Expect(err).To(MatchError(ContainSubstring("expression is required")))
	})

	It("rejects patches on instanceIds", func() {
		karta := kartaWithAccessor(&ValueAccessor{Expression: `object`})
		karta.Spec.StructureDefinition.RootComponent.InstanceIds = &ValueAccessor{
			Expression: `object.spec.jobs.map(x, x.name)`,
			Patches:    []PatchEntry{entry()},
		}
		karta.Spec.StructureDefinition.RootComponent.PodSelector = &PodSelector{
			ComponentInstanceSelector: &ComponentInstanceSelector{},
		}
		Expect(NewKartaValidator(karta).Validate()).To(MatchError(ContainSubstring("instanceIds: is read-only")))
	})

	It("validates suspend and resume action entries", func() {
		karta := kartaWithAccessor(&ValueAccessor{Expression: `object`})
		karta.Spec.StructureDefinition.RootComponent.SuspendDefinition = &SuspendDefinition{
			SuspendActions: []PatchEntry{{PatchType: "Weird", Expression: `{"spec": {"suspend": true}}`}},
			ResumeActions:  []PatchEntry{{PatchType: PatchTypeMergePatch}},
		}
		err := NewKartaValidator(karta).Validate()
		Expect(err).To(MatchError(ContainSubstring(`suspendActions[0]: patchType must be`)))
		Expect(err).To(MatchError(ContainSubstring("resumeActions[0]: expression is required")))
	})
})
