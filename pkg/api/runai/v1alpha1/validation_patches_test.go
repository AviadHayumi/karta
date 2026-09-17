// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package v1alpha1

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func kartaWithAccessor(accessor *ValueAccessor) *Karta {
	return &Karta{
		ObjectMeta: metav1.ObjectMeta{Name: "path-karta"},
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

var _ = Describe("write path validation", func() {
	DescribeTable("accepts RFC 6901 pointers", func(path string) {
		karta := kartaWithAccessor(&ValueAccessor{Expression: "object", PathWrite: &path})
		Expect(NewKartaValidator(karta).Validate()).To(Succeed())
	},
		Entry("document root", ""),
		Entry("an empty key", "/"),
		Entry("a nested field", "/spec/template"),
		Entry("an array entry", "/spec/templates/0"),
		Entry("an escaped slash", "/metadata/annotations/example.com~1key"),
		Entry("an escaped tilde", "/metadata/annotations/~0key"),
		Entry("an escaped tilde followed by a digit", "/metadata/annotations/~01"),
		Entry("consecutive empty keys", "//"),
	)

	DescribeTable("rejects malformed pointers", func(path string) {
		karta := kartaWithAccessor(&ValueAccessor{PathWrite: &path})
		Expect(NewKartaValidator(karta).Validate()).To(MatchError(ContainSubstring("must be an RFC 6901 JSON Pointer")))
	},
		Entry("relative field", "spec/template"),
		Entry("URI fragment", "#/spec/template"),
		Entry("bare tilde", "/spec/~"),
		Entry("invalid escape", "/spec/~2"),
		Entry("double tilde", "/spec/~~0"),
	)

	It("accepts a dynamic target without evaluating it as a patch", func() {
		karta := kartaWithAccessor(&ValueAccessor{
			Expression:          `object.spec.templates[index]`,
			PathWriteExpression: `"/spec/templates/" + string(index)`,
		})
		Expect(NewKartaValidator(karta).Validate()).To(Succeed())
	})

	It("accepts a read-only accessor", func() {
		Expect(NewKartaValidator(kartaWithAccessor(&ValueAccessor{Expression: "object"})).Validate()).To(Succeed())
	})

	DescribeTable("rejects both target forms", func(path string) {
		karta := kartaWithAccessor(&ValueAccessor{PathWrite: &path, PathWriteExpression: `"/spec/template"`})
		Expect(NewKartaValidator(karta).Validate()).To(MatchError(ContainSubstring("pathWrite and pathWriteExpression are mutually exclusive")))
	},
		Entry("nested field", "/spec/template"),
		Entry("document root", ""),
	)

	DescribeTable("rejects write targets on instanceIds", func(accessor *ValueAccessor) {
		karta := kartaWithAccessor(&ValueAccessor{Expression: "object"})
		accessor = accessor.DeepCopy()
		accessor.Expression = `object.spec.jobs.map(x, x.name)`
		karta.Spec.StructureDefinition.RootComponent.InstanceIds = accessor
		karta.Spec.StructureDefinition.RootComponent.PodSelector = &PodSelector{
			ComponentInstanceSelector: &ComponentInstanceSelector{},
		}
		Expect(NewKartaValidator(karta).Validate()).To(MatchError(ContainSubstring("instanceIds: is read-only")))
	},
		Entry("static target", &ValueAccessor{PathWrite: ptr.To("/spec/jobs")}),
		Entry("root target", &ValueAccessor{PathWrite: ptr.To("")}),
		Entry("dynamic target", &ValueAccessor{PathWriteExpression: `"/spec/jobs"`}),
	)

	DescribeTable("validates suspension targets", func(suspend *SuspendDefinition, wantErr string) {
		karta := kartaWithAccessor(&ValueAccessor{Expression: "object"})
		karta.Spec.StructureDefinition.RootComponent.SuspendDefinition = suspend
		err := NewKartaValidator(karta).Validate()
		if wantErr == "" {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(MatchError(ContainSubstring("suspendDefinition: " + wantErr)))
		}
	},
		Entry("static target", &SuspendDefinition{PathWrite: ptr.To("/spec/suspend")}, ""),
		Entry("dynamic target", &SuspendDefinition{PathWriteExpression: `"/spec/suspend"`}, ""),
		Entry("root target", &SuspendDefinition{PathWrite: ptr.To("")}, ""),
		Entry("missing target", &SuspendDefinition{}, "exactly one of pathWrite or pathWriteExpression is required"),
		Entry("both targets", &SuspendDefinition{PathWrite: ptr.To("/spec/suspend"), PathWriteExpression: `"/spec/suspend"`}, "pathWrite and pathWriteExpression are mutually exclusive"),
		Entry("invalid pointer", &SuspendDefinition{PathWrite: ptr.To("/spec/~2suspend")}, "pathWrite"),
	)

	It("checks targets across fragmented spec fields and scaling fields", func() {
		invalid := &ValueAccessor{PathWrite: ptr.To("relative")}
		karta := kartaWithAccessor(nil)
		root := &karta.Spec.StructureDefinition.RootComponent
		root.SpecDefinition = &SpecDefinition{
			Metadata: invalid,
			FragmentedPodSpecDefinition: &FragmentedPodSpecDefinition{
				SchedulerName: invalid, Labels: invalid, Annotations: invalid,
				Resources: invalid, ResourceClaims: invalid, PodAffinity: invalid,
				NodeAffinity: invalid, Containers: invalid, Container: invalid,
				PriorityClassName: invalid, Image: invalid,
			},
		}
		root.ScaleDefinition = &ScaleDefinition{Replicas: invalid, MinReplicas: invalid, MaxReplicas: invalid}
		err := NewKartaValidator(karta).Validate()
		for _, field := range []string{
			"specDefinition.metadata", "fragmentedPodSpecDefinition.schedulerName",
			"fragmentedPodSpecDefinition.labels", "fragmentedPodSpecDefinition.annotations",
			"fragmentedPodSpecDefinition.resources", "fragmentedPodSpecDefinition.resourceClaims",
			"fragmentedPodSpecDefinition.podAffinity", "fragmentedPodSpecDefinition.nodeAffinity",
			"fragmentedPodSpecDefinition.containers", "fragmentedPodSpecDefinition.container",
			"fragmentedPodSpecDefinition.priorityClassName", "fragmentedPodSpecDefinition.image",
			"scaleDefinition.replicas", "scaleDefinition.minReplicas", "scaleDefinition.maxReplicas",
		} {
			Expect(err).To(MatchError(ContainSubstring(field + ": pathWrite")))
		}
	})

	It("checks write targets on child components", func() {
		karta := kartaWithAccessor(nil)
		karta.Spec.StructureDefinition.ChildComponents = []ComponentDefinition{{
			Name: "child", OwnerRef: ptr.To("root"),
			SpecDefinition: &SpecDefinition{PodSpec: &ValueAccessor{PathWrite: ptr.To("spec")}},
		}}
		Expect(NewKartaValidator(karta).Validate()).To(MatchError(ContainSubstring("component 'child' specDefinition.podSpec: pathWrite")))
	})

	It("preserves the distinction between a root target and a read-only accessor in JSON", func() {
		for _, target := range []*string{nil, ptr.To("")} {
			data, err := json.Marshal(ValueAccessor{Expression: "object", PathWrite: target})
			Expect(err).NotTo(HaveOccurred())
			var decoded ValueAccessor
			Expect(json.Unmarshal(data, &decoded)).To(Succeed())
			Expect(decoded.PathWrite).To(Equal(target))
			if target == nil {
				Expect(string(data)).NotTo(ContainSubstring("pathWrite"))
			} else {
				Expect(string(data)).To(ContainSubstring(`"pathWrite":""`))
			}
		}
	})
})
