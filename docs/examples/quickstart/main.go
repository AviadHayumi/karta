// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package main demonstrates how to use Karta to uniformly read and mutate
// distributed training workloads without writing per-CRD integration code.
//
// The same tree API reads and edits JobSet and LeaderWorkerSet without
// branching on workload kind. All edits are local; no cluster is required.
//
// Usage (from the docs/examples/quickstart directory):
//
//	go run . [flags]
//
// Flags:
//
//	--scheduler name to inject (default: kai-scheduler)
//	--print-mutated  Print the full mutated CRD YAML after injection
//
// Examples:
//
//	go run .
//	go run . --scheduler volcano
//	go run . --scheduler kai-scheduler --print-mutated
package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

// Sample workload objects embedded at compile time.
// In a real controller these come from the Kubernetes API (Reconcile request).

//go:embed jobset.yaml
var jobsetWorkloadYAML []byte

//go:embed lws.yaml
var lwsWorkloadYAML []byte

// workloadExample pairs a sample workload with its Karta definition path.
// Karta definitions live in docs/catalog/ and are read at runtime so they
// always reflect the latest version in the repository.
type workloadExample struct {
	name         string
	kartaPath    string
	workloadYAML []byte
}

// opts carries the parsed CLI flags shared across all workload samples.
type opts struct {
	scheduler    string
	printMutated bool
}

func formatQuantity(q *apiresource.Quantity) string {
	if q == nil || q.IsZero() {
		return "<none>"
	}
	return q.String()
}

// eachComponent visits every (ComponentNode, InstanceNode) pair depth-first,
// including nested children (e.g. LWS leader/worker inside each group instance).
func eachComponent(nodes []tree.ComponentNode, fn func(tree.ComponentNode, tree.InstanceNode)) {
	for _, comp := range nodes {
		for _, inst := range comp.Instances {
			fn(comp, inst)
			eachComponent(inst.Children, fn)
		}
	}
}

func main() {
	o := opts{}
	flag.StringVar(&o.scheduler, "scheduler", "kai-scheduler",
		"scheduler name to inject into all pod-bearing components")
	flag.BoolVar(&o.printMutated, "print-mutated", false,
		"print the full mutated CRD YAML after injection")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: go run . [flags]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  go run .\n")
		fmt.Fprintf(os.Stderr, "  go run . --scheduler volcano\n")
		fmt.Fprintf(os.Stderr, "  go run . --scheduler kai-scheduler --print-mutated\n")
	}
	flag.Parse()

	ctx := context.Background()

	examples := []workloadExample{
		{
			name:         "JobSet",
			kartaPath:    "../../catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml",
			workloadYAML: jobsetWorkloadYAML,
		},
		{
			name:         "LeaderWorkerSet",
			kartaPath:    "../../catalog/leaderworkerset-x-k8s-io-leaderworkerset-v1.yaml",
			workloadYAML: lwsWorkloadYAML,
		},
	}

	for _, ex := range examples {
		fmt.Println("==========================================")
		fmt.Printf("  %s  (scheduler: %s)\n", ex.name, o.scheduler)
		fmt.Println("==========================================")
		fmt.Println()
		if err := run(ctx, ex, o); err != nil {
			log.Fatalf("%s: %v", ex.name, err)
		}
		fmt.Println()
	}
}

// run executes the Karta operations for a single workload type.
// There is no switch on CRD kind anywhere in this function. The Karta
// definition absorbs all structural differences between workload types.
func run(ctx context.Context, ex workloadExample, o opts) error {
	// Load the Karta definition from docs/catalog/.
	kartaYAML, err := os.ReadFile(ex.kartaPath)
	if err != nil {
		return fmt.Errorf("read Karta definition %s: %w", ex.kartaPath, err)
	}
	karta := &v1alpha1.Karta{}
	if err := yaml.Unmarshal(kartaYAML, karta); err != nil {
		return fmt.Errorf("parse Karta: %w", err)
	}

	// Parse the workload into an unstructured object.
	var rawObj map[string]any
	if err := yaml.Unmarshal(ex.workloadYAML, &rawObj); err != nil {
		return fmt.Errorf("parse workload: %w", err)
	}
	obj := &unstructured.Unstructured{Object: rawObj}

	// Open one local editor for inspection, path resolution, and mutations.
	editor, err := tree.Open(ctx, karta, obj)
	if err != nil {
		return fmt.Errorf("open workload tree: %w", err)
	}
	wt := editor.Snapshot()
	// Start at Root so workloads that store their template on the root are included.
	nodes := []tree.ComponentNode{*wt.Root}

	// Step 1: read unified status.
	fmt.Println("=== Workload status ===")
	status := "<none>"
	if wt.Status != nil {
		status = strings.Join(wt.Status.Phases, ", ")
	}
	fmt.Printf("  Karta workload status: %s\n\n", status)

	// Step 2: read scale through the catalog's CEL expressions.
	fmt.Println("=== Component replica counts ===")
	eachComponent(nodes, func(comp tree.ComponentNode, inst tree.InstanceNode) {
		label := comp.Name
		if inst.InstanceKey != nil {
			label = fmt.Sprintf("%s[%s]", comp.Name, *inst.InstanceKey)
		}
		if !comp.HasPodDefinition {
			label += " (virtual)"
		}
		if inst.Scale != nil && inst.Scale.Replicas != nil {
			fmt.Printf("  %-28s replicas=%d\n", label, *inst.Scale.Replicas)
		}
	})
	fmt.Println()

	// Step 3: inspect the extracted container resources.
	// PodTemplateSpec is extracted from the workload spec via Karta, so container
	// resources are directly accessible without knowing CRD-specific paths.
	fmt.Println("=== Resource requests per component ===")
	eachComponent(nodes, func(comp tree.ComponentNode, inst tree.InstanceNode) {
		if !comp.HasPodDefinition || inst.ExtractedInstance == nil || inst.ExtractedInstance.PodTemplateSpec == nil {
			return
		}
		compLabel := comp.Name
		if inst.InstanceKey != nil {
			compLabel = fmt.Sprintf("%s[%s]", comp.Name, *inst.InstanceKey)
		}
		for _, c := range inst.ExtractedInstance.PodTemplateSpec.Spec.Containers {
			req := c.Resources.Requests
			lim := c.Resources.Limits
			cpu := req.Cpu()
			mem := req.Memory()
			// Extended resources (e.g. GPUs) are often set only in limits;
			// the API server normalises requests=limits, but offline YAML won't.
			gpu := req[corev1.ResourceName("nvidia.com/gpu")]
			if gpu.IsZero() {
				gpu = lim[corev1.ResourceName("nvidia.com/gpu")]
			}
			fmt.Printf("  %-28s container=%-12s cpu=%-8s memory=%-10s gpu=%s\n",
				compLabel, c.Name,
				formatQuantity(cpu),
				formatQuantity(mem),
				formatQuantity(&gpu),
			)
		}
	})
	fmt.Println()

	// Step 4: select instances from the tree, then edit two explicit fields.
	// These examples have writable podTemplateSpec fields. Other catalog shapes
	// can expose podSpec, metadata, or fragmented fields instead.
	fmt.Printf("=== Injecting scheduler %q + label ===\n", o.scheduler)
	var targets []tree.Target
	seen := make(map[tree.Target]bool)
	eachComponent(nodes, func(comp tree.ComponentNode, inst tree.InstanceNode) {
		if !comp.HasPodDefinition || inst.ExtractedInstance == nil || inst.ExtractedInstance.PodTemplateSpec == nil {
			return
		}
		id := "" // A single-instance component uses the empty ID.
		if inst.InstanceKey != nil {
			id = *inst.InstanceKey
		}
		target := tree.Target{Component: comp.Name, Instance: id, Field: tree.PodTemplateSpec}
		// A logical component can appear under several parent instances.
		if seen[target] {
			return
		}
		seen[target] = true
		targets = append(targets, target)
	})
	if len(targets) == 0 {
		return fmt.Errorf("no pod-template instances were extracted from %s", ex.name)
	}
	for _, target := range targets {
		location, err := editor.ResolveWriteTarget(ctx, target)
		if err != nil {
			return fmt.Errorf("resolve %s[%s]: %w", target.Component, target.Instance, err)
		}
		fmt.Printf("  %s[%s] -> %s\n", target.Component, target.Instance, location.Path)
	}
	// All destinations resolve against one starting snapshot. A failed write
	// leaves the entire batch unchanged. No Kubernetes request is sent.
	if err := editTemplates(ctx, editor, targets, o.scheduler); err != nil {
		return fmt.Errorf("edit templates: %w", err)
	}
	fmt.Println()

	// Step 5: mutation refreshed the tree. Read back and check both changes.
	fmt.Println("=== Verification ===")
	refreshed := editor.Snapshot()
	var verificationErr error
	pending := make(map[tree.Target]bool, len(targets))
	for _, target := range targets {
		pending[target] = true
	}
	eachComponent([]tree.ComponentNode{*refreshed.Root}, func(comp tree.ComponentNode, inst tree.InstanceNode) {
		if !comp.HasPodDefinition || inst.ExtractedInstance == nil || inst.ExtractedInstance.PodTemplateSpec == nil {
			return
		}
		pts := inst.ExtractedInstance.PodTemplateSpec
		label := comp.Name
		id := ""
		if inst.InstanceKey != nil {
			id = *inst.InstanceKey
			label = fmt.Sprintf("%s[%s]", comp.Name, *inst.InstanceKey)
		}
		delete(pending, tree.Target{Component: comp.Name, Instance: id, Field: tree.PodTemplateSpec})
		if pts.Spec.SchedulerName != o.scheduler || pts.Labels["app.kubernetes.io/managed-by"] != "karta" {
			verificationErr = fmt.Errorf("read-back mismatch for %s", label)
		}
		fmt.Printf("  %-28s schedulerName=%-20q managed-by=%q\n",
			label, pts.Spec.SchedulerName, pts.Labels["app.kubernetes.io/managed-by"])
	})
	if verificationErr != nil {
		return verificationErr
	}
	if len(pending) != 0 {
		return fmt.Errorf("%d mutated instances were missing from the refreshed tree", len(pending))
	}

	// Step 6: retrieve the full local result.
	// GetResource returns the modified unstructured object ready for
	//   k8sClient.Update(ctx, updated)
	updated, err := editor.GetResource()
	if err != nil {
		return fmt.Errorf("get updated resource: %w", err)
	}
	fmt.Printf("\n  In a real controller: k8sClient.Update(ctx, updated)\n")

	// Optionally print the full mutated workload YAML.
	if o.printMutated {
		mutatedYAML, err := yaml.Marshal(updated.(*unstructured.Unstructured).Object)
		if err != nil {
			return fmt.Errorf("marshal mutated %s: %w", ex.name, err)
		}
		fmt.Printf("\n=== Mutated %s YAML ===\n%s", ex.name, string(mutatedYAML))
	}

	return nil
}

func editTemplates(ctx context.Context, editor tree.Editable, targets []tree.Target, scheduler string) error {
	// Metadata and labels may be absent. Existing null or scalar parents fail.
	draft, err := tree.BeginEdit(ctx, editor, tree.WithEditParents(resource.CreateMapParents))
	if err != nil {
		return err
	}
	defer draft.Abort()
	for _, target := range targets {
		template, err := draft.Target(ctx, target)
		if err != nil {
			return err
		}
		if err := template.At("spec", "schedulerName").Set(scheduler); err != nil {
			return err
		}
		if err := template.At("metadata", "labels", "app.kubernetes.io/managed-by").Set("karta"); err != nil {
			return err
		}
	}
	return draft.Commit(ctx)
}
