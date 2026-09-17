// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package v1alpha1

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var _ = Describe("custom named fields", func() {
	DescribeTable("accepts read and write accessors", func(accessor ValueAccessor) {
		karta := kartaWithAccessor(nil)
		karta.Spec.StructureDefinition.RootComponent.Fields = map[string]ValueAccessor{"schedulerName": accessor}
		karta.Spec.StructureDefinition.ChildComponents = []ComponentDefinition{{
			Name: "child", OwnerRef: ptr.To("root"),
			Fields: map[string]ValueAccessor{"worker queue/name": accessor},
		}}
		Expect(NewKartaValidator(karta).Validate()).To(Succeed())
	},
		Entry("read-only expression", ValueAccessor{Expression: "object.spec.schedulerName"}),
		Entry("read and static write", ValueAccessor{Expression: "object.spec.schedulerName", PathWrite: ptr.To("/spec/schedulerName")}),
		Entry("read and dynamic write", ValueAccessor{Expression: "object.spec.jobs[index].schedulerName", PathWriteExpression: `"/spec/jobs/" + string(index) + "/schedulerName"`}),
		Entry("write-only static destination", ValueAccessor{PathWrite: ptr.To("/spec/schedulerName")}),
		Entry("write-only root destination", ValueAccessor{PathWrite: ptr.To("")}),
		Entry("write-only dynamic destination", ValueAccessor{PathWriteExpression: `"/spec/schedulerName"`}),
	)

	DescribeTable("validates accessors on root and child components", func(accessor ValueAccessor, message string) {
		karta := kartaWithAccessor(nil)
		karta.Spec.StructureDefinition.RootComponent.Fields = map[string]ValueAccessor{"queue": accessor}
		karta.Spec.StructureDefinition.ChildComponents = []ComponentDefinition{{
			Name: "child", OwnerRef: ptr.To("root"), Fields: map[string]ValueAccessor{"queue": accessor},
		}}
		err := NewKartaValidator(karta).Validate()
		for _, name := range []string{"root", "child"} {
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("component '%s' fields[\"queue\"]: %s", name, message))))
		}
	},
		Entry("empty accessor", ValueAccessor{}, "expression, pathWrite, or pathWriteExpression is required"),
		Entry("relative destination", ValueAccessor{PathWrite: ptr.To("spec/queue")}, "pathWrite"),
		Entry("invalid pointer escape", ValueAccessor{PathWrite: ptr.To("/spec/~2queue")}, "pathWrite"),
		Entry("both write destinations", ValueAccessor{PathWrite: ptr.To("/spec/queue"), PathWriteExpression: `"/spec/queue"`}, "pathWrite and pathWriteExpression are mutually exclusive"),
	)

	It("rejects empty and reserved names on every component", func() {
		fields := map[string]ValueAccessor{"": {Expression: "object"}}
		reserved := []string{
			"podTemplateSpec", "podSpec", "metadata",
			"fragmented.schedulerName", "fragmented.labels", "fragmented.annotations",
			"fragmented.resources", "fragmented.resourceClaims", "fragmented.podAffinity",
			"fragmented.nodeAffinity", "fragmented.containers", "fragmented.container",
			"fragmented.priorityClassName", "fragmented.image",
			"scale.replicas", "scale.minReplicas", "scale.maxReplicas", "suspend",
		}
		for _, name := range reserved {
			fields[name] = ValueAccessor{Expression: "object"}
		}
		karta := kartaWithAccessor(nil)
		karta.Spec.StructureDefinition.RootComponent.Fields = fields
		karta.Spec.StructureDefinition.ChildComponents = []ComponentDefinition{{
			Name: "child", OwnerRef: ptr.To("root"), Fields: fields,
		}}
		err := NewKartaValidator(karta).Validate()
		for _, component := range []string{"root", "child"} {
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("component '%s' fields: field name is empty", component))))
			for _, name := range reserved {
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("component '%s' fields[%q]: field name is reserved", component, name))))
			}
		}
	})

	It("does not impose a custom field count limit", func() {
		karta := kartaWithAccessor(nil)
		fields := make(map[string]ValueAccessor, 257)
		karta.Spec.StructureDefinition.RootComponent.Fields = fields
		for i := range 257 {
			fields[fmt.Sprintf("custom%d", i)] = ValueAccessor{Expression: "object"}
		}
		Expect(NewKartaValidator(karta).Validate()).To(Succeed())
	})

	It("round-trips custom field names and root write destinations", func() {
		component := ComponentDefinition{
			Name: "root",
			Fields: map[string]ValueAccessor{
				"schedulerName":      {Expression: "object.spec.schedulerName"},
				"custom field/a~b.c": {PathWrite: ptr.To("")},
			},
		}
		data, err := json.Marshal(component)
		Expect(err).NotTo(HaveOccurred())
		var decoded ComponentDefinition
		Expect(json.Unmarshal(data, &decoded)).To(Succeed())
		Expect(decoded).To(Equal(component))
	})

	It("deep-copies field maps and their write path pointers", func() {
		karta := kartaWithAccessor(nil)
		karta.Spec.StructureDefinition.RootComponent.Fields = map[string]ValueAccessor{
			"queue": {Expression: "object.spec.queue", PathWrite: ptr.To("/spec/queue")},
		}
		karta.Spec.StructureDefinition.ChildComponents = []ComponentDefinition{{
			Name: "child", OwnerRef: ptr.To("root"),
			Fields: map[string]ValueAccessor{"priority": {PathWrite: ptr.To("/spec/priority")}},
		}}
		copied := karta.DeepCopy()
		*copied.Spec.StructureDefinition.RootComponent.Fields["queue"].PathWrite = "/spec/otherQueue"
		copied.Spec.StructureDefinition.RootComponent.Fields["extra"] = ValueAccessor{Expression: "object"}
		*copied.Spec.StructureDefinition.ChildComponents[0].Fields["priority"].PathWrite = "/spec/otherPriority"
		delete(copied.Spec.StructureDefinition.ChildComponents[0].Fields, "priority")
		Expect(karta.Spec.StructureDefinition.RootComponent.Fields).To(Equal(map[string]ValueAccessor{
			"queue": {Expression: "object.spec.queue", PathWrite: ptr.To("/spec/queue")},
		}))
		Expect(karta.Spec.StructureDefinition.ChildComponents[0].Fields).To(Equal(map[string]ValueAccessor{
			"priority": {PathWrite: ptr.To("/spec/priority")},
		}))
	})
})
