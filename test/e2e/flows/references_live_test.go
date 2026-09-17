// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	authorizationv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog/kartas"
	"github.com/run-ai/karta/pkg/references"
	"github.com/run-ai/karta/pkg/resource"
)

// liveReader adapts the suite's cluster client to karta's reference reader and counts fetches,
// so the test can observe that resolution is lazy and happens exactly once per factory. It also
// implements the permission probe with a real SelfSubjectAccessReview, so every reference fetch
// in this test goes through the same allowed-or-explained gate a production consumer gets.
type liveReader struct {
	client client.Client
	gets   int
	checks []string
}

func (r *liveReader) Get(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
	r.gets++
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(gvk)
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, object)
	if apierrors.IsNotFound(err) {
		return nil, references.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return object, nil
}

func (r *liveReader) List(ctx context.Context, gvk schema.GroupVersionKind, query references.ListQuery) ([]unstructured.Unstructured, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)
	if err := r.client.List(ctx, list,
		client.InNamespace(query.Namespace),
		client.MatchingLabelsSelector{Selector: query.Selector}); err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (r *liveReader) CheckRead(ctx context.Context, gvk schema.GroupVersionKind, namespace, verb string) error {
	mapping, err := r.client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("map %s to a resource: %w", gvk, err)
	}
	review := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:     gvk.Group,
				Version:   gvk.Version,
				Resource:  mapping.Resource.Resource,
				Namespace: namespace,
				Verb:      verb,
			},
		},
	}
	if err := r.client.Create(ctx, review); err != nil {
		return fmt.Errorf("access review for %s: %w", verb, err)
	}
	r.checks = append(r.checks, verb+" "+gvk.Kind)
	if !review.Status.Allowed {
		return fmt.Errorf("%s on %s is not allowed: %s", verb, mapping.Resource.Resource, review.Status.Reason)
	}

	return nil
}

// The lifecycle of a workload whose definition reads through references, driven and observed by
// the karta library alone - no recorder: a live reader resolves the runtime, status comes from
// karta reads, and suspension is written through karta's suspend actions and applied back.
var _ = Describe("TrainJob references live (no recorder)", Ordered, Label("trainer"), func() {
	karta := kartas.TrainJob()
	const name = "trainer-live-refs"

	newFactory := func(ctx context.Context, reader *liveReader) (*resource.ComponentFactory, *unstructured.Unstructured) {
		trainJob := &unstructured.Unstructured{}
		trainJob.SetGroupVersionKind(schema.GroupVersionKind{Group: "trainer.kubeflow.org", Version: "v1alpha1", Kind: "TrainJob"})
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: name}, trainJob)).To(Succeed())

		return resource.NewComponentFactoryFromObject(karta, trainJob, resource.WithReferenceReader(reader)), trainJob
	}

	statusOf := func(ctx context.Context, reader *liveReader) []kartav1alpha1.ResourceStatus {
		factory, _ := newFactory(ctx, reader)
		root, err := factory.GetRootComponent()
		Expect(err).NotTo(HaveOccurred())
		status, err := root.GetStatus(ctx)
		Expect(err).NotTo(HaveOccurred())

		return status.MatchedStatuses
	}

	waitFor := func(ctx context.Context, want kartav1alpha1.ResourceStatus) {
		Eventually(func() []kartav1alpha1.ResourceStatus {
			return statusOf(ctx, &liveReader{client: k8sClient})
		}).WithContext(ctx).WithTimeout(2*time.Minute).WithPolling(2*time.Second).
			Should(ContainElement(want), "karta never read %s", want)
	}

	BeforeAll(func(ctx SpecContext) {
		trainJob := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "trainer.kubeflow.org/v1alpha1",
			"kind":       "TrainJob",
			"metadata":   map[string]any{"name": name, "namespace": testNamespace},
			"spec": map[string]any{
				"runtimeRef": map[string]any{"name": "karta-busybox"},
				"trainer":    map[string]any{"command": []any{"sh", "-c", "sleep 25"}},
			},
		}}
		Expect(k8sClient.Create(ctx, trainJob)).To(Succeed())
		DeferCleanup(func(ctx SpecContext) {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, trainJob))).To(Succeed())
		})
	})

	It("reads status without ever touching the reader", func(ctx SpecContext) {
		reader := &liveReader{client: k8sClient}
		Expect(statusOf(ctx, reader)).NotTo(BeEmpty())
		Expect(reader.gets).To(Equal(0), "status expressions never mention references")
	})

	It("reads the effective pod values through the referenced runtime, permission-checked", func(ctx SpecContext) {
		waitFor(ctx, kartav1alpha1.RunningStatus)

		reader := &liveReader{client: k8sClient}
		factory, _ := newFactory(ctx, reader)
		root, err := factory.GetRootComponent()
		Expect(err).NotTo(HaveOccurred())

		instances, err := root.GetExtractedInstances(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(instances).To(HaveLen(1))
		for _, summary := range instances {
			Expect(summary.FragmentedPodSpec.Image).To(Equal("busybox:1.36"),
				"the image lives only in the ClusterTrainingRuntime; karta read it through the reference")
			Expect(summary.Scale.Replicas).To(HaveValue(Equal(int32(1))))
		}
		Expect(reader.checks).To(ContainElement("get ClusterTrainingRuntime"), "the fetch was permission-checked first")
		Expect(reader.gets).To(Equal(1), "one lookup, resolved lazily and memoized")
	})

	It("suspends and resumes through karta's own actions", func(ctx SpecContext) {
		for _, step := range []struct {
			act  func(component *resource.Component) error
			want kartav1alpha1.ResourceStatus
		}{
			{act: func(c *resource.Component) error { return c.Suspend(ctx) }, want: kartav1alpha1.SuspendedStatus},
			{act: func(c *resource.Component) error { return c.Resume(ctx) }, want: kartav1alpha1.CompletedStatus},
		} {
			factory, _ := newFactory(ctx, &liveReader{client: k8sClient})
			root, err := factory.GetRootComponent()
			Expect(err).NotTo(HaveOccurred())
			Expect(step.act(root)).To(Succeed())

			updated, err := factory.GetResource()
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Update(ctx, updated.(*unstructured.Unstructured))).To(Succeed())

			waitFor(ctx, step.want)
		}
	})
})
