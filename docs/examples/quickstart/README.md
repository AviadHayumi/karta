<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta tree quickstart

This example edits JobSet and LeaderWorkerSet through the same tree API. It runs offline. Karta describes where fields live; the Go caller selects the fields to change.

## Run it

From `docs/examples/quickstart`:

```bash
go run .
go run . --scheduler batch-scheduler
go run . --scheduler batch-scheduler --print-mutated
```

The program prints status, replica counts, resource requests, resolved write paths, and a read-back check. It sends no Kubernetes requests.

## Open, inspect, mutate, read back

`tree.Open` creates a local editor. `Snapshot` returns extracted data, including the root component and its children.

```go
editor, err := tree.Open(ctx, karta, workload)
if err != nil {
    return err
}
snapshot := editor.Snapshot()
// Walk snapshot.Root and each instance's Children.
```

A write selects a component and a stable instance ID. For the sample JobSet, `replicatedjob[workers]` means the replicated job whose name is `workers`, regardless of its array position.

```go
target := tree.Target{
    Component: "replicatedjob",
    Instance:  "workers",
    Field:     tree.PodTemplateSpec,
}
location, err := editor.ResolveWriteTarget(ctx, target)
if err != nil {
    return err
}
fmt.Println(location.Path)
// /spec/replicatedJobs/1/template/spec/template
```

The path above comes from this workload. Do not cache its array index across mutations. Begin a draft and select the catalog target to edit individual fields:

```go
draft, err := tree.BeginEdit(ctx, editor,
    tree.WithEditParents(resource.CreateMapParents))
if err != nil {
    return err
}
defer draft.Abort()
template, err := draft.Target(ctx, target)
if err != nil {
    return err
}
if err := template.At("spec", "schedulerName").Set("batch-scheduler"); err != nil {
    return err
}
if err := template.At("metadata", "labels", "app.kubernetes.io/managed-by").Set("karta"); err != nil {
    return err
}
if err := draft.Commit(ctx); err != nil {
    return err
}
```

`At` takes literal map keys, so the label key needs no escaping. `CreateMapParents` permits absent metadata and label maps; an existing null or scalar parent still fails. Without that option, every parent must already exist.

`main.go` selects the writable `PodTemplateSpec` accessor for these two catalog definitions. Other definitions can expose `PodSpec` or fragmented fields and need the corresponding explicit accessor choice. The program edits all selected templates in one draft, then publishes the raw workload and refreshed extraction together. Containers, sidecars, and unknown fields stay intact.

Selections and reads use the starting snapshot. An ignored edit error prevents commit. If another mutation changes the editor first, commit returns `ErrStaleDraft`; start a fresh draft from the new snapshot. `Abort` discards uncommitted edits.

The existing partial-map `Mutate` form remains supported:

```go
err = editor.Mutate(ctx, tree.Write{
    Component: "replicatedjob",
    Instance:  "workers",
    Field:     tree.PodTemplateSpec,
    Value: map[string]any{
        "spec": map[string]any{"schedulerName": "batch-scheduler"},
        "metadata": map[string]any{
            "labels": map[string]any{"app.kubernetes.io/managed-by": "karta"},
        },
    },
    Options: resource.MutationOptions{
        PatchType: resource.PatchTypeMergePatch,
        Strategy:  resource.Merge,
    },
})
if err != nil {
    return err
}
```

Only the supplied map members change. Existing containers, resource requests, and other labels stay. Arrays are replaced as whole values when supplied; Merge does not merge containers by name.

The executable tests check both forms against the complete raw JobSet and LeaderWorkerSet, including unknown nested container data and sidecars.

```go
fresh := editor.Snapshot() // Already reflects successful mutations.
updated, err := editor.GetResource()
if err != nil {
    return err
}
// A controller can now call k8sClient.Update(ctx, updated).
```

Changing `fresh` alone does not edit the workload. Use a draft or an explicit `Mutate` call to apply a change. These are local operations; a controller still handles Kubernetes resourceVersion conflicts when persisting the result.

<details>
<summary>Example paths printed by the program</summary>

```text
replicatedjob[leader]  -> /spec/replicatedJobs/0/template/spec/template
replicatedjob[workers] -> /spec/replicatedJobs/1/template/spec/template
leader[]              -> /spec/leaderWorkerTemplate/leaderTemplate
worker[]              -> /spec/leaderWorkerTemplate/workerTemplate
```

An empty instance ID means a single-instance component. After the call, the program checks that each extracted template has the requested scheduler and the `app.kubernetes.io/managed-by: karta` label.

</details>

## What is stored in Karta?

These are fragments of the generated catalog CRs, not complete manifests. The complete definitions are in `docs/catalog`.

JobSet stores templates in an array. Karta discovers instance names and evaluates the write path with the selected instance's current source index:

```yaml
name: replicatedjob
instanceIds:
  expression: '([dyn(object[?"spec"][?"replicatedJobs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"name"].orValue(null))'
specDefinition:
  podTemplateSpec:
    expression: '([dyn(object[?"spec"][?"replicatedJobs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"template"][?"spec"][?"template"].orValue(null))'
    pathWriteExpression: '"/spec/replicatedJobs/" + string(index) + "/template/spec/template"'
```

LeaderWorkerSet has a fixed worker-template location:

```yaml
name: worker
specDefinition:
  podTemplateSpec:
    expression: 'object[?"spec"][?"leaderWorkerTemplate"][?"workerTemplate"].orValue(null)'
    pathWrite: /spec/leaderWorkerTemplate/workerTemplate
```

`expression` reads the value. `pathWrite` is a fixed JSON Pointer. `pathWriteExpression` is CEL that returns a JSON Pointer. Neither field contains a patch or a merge strategy.

<details>
<summary>Suspension and paths that depend on the workload</summary>

The tree interface also exposes the root component's suspension capability:

```go
if editor.IsSuspendable() {
    if err := editor.Suspend(ctx); err != nil {
        return err
    }
    if err := editor.Resume(ctx); err != nil {
        return err
    }
}
```

These calls change the local desired spec. They do not wait for an operator to suspend or resume pods.

KServe's catalog discovers the predictor child that contains `storageUri`. Its logical `Container` field can therefore resolve to `/spec/predictor/model` or `/spec/predictor/sklearn`. The caller can inspect the selected path:

```go
location, err := editor.ResolveWriteTarget(ctx, tree.Target{
    Component: "predictor",
    Field:     tree.Container,
})
```

For this catalog, a predictor without a matching child has no resolved Container target. Karta returns an error instead of guessing a destination.

The executable examples in `pkg/tree/example_editable_test.go` cover both KServe shapes, an unresolved target, a fixed Deployment path, stable Ray worker IDs, and Job suspension.

</details>

## Files

| File | Purpose |
| --- | --- |
| `main.go` | Runs the same inspection and mutation flow for both workloads |
| `jobset.yaml`, `lws.yaml` | Example workload inputs |
| `../../catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml` | JobSet Karta definition |
| `../../catalog/leaderworkerset-x-k8s-io-leaderworkerset-v1.yaml` | LeaderWorkerSet Karta definition |

The example module uses a local `replace` directive for the repository root. Remove that directive and choose a released module version when adapting the example outside this experimental checkout.
