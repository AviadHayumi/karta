<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# KEP-0001: CEL expressions

- Status: implementable
- Authors: @AviadHayumi
- Created: 2026-09-10
- Tracking issue: required before merge

Replace jq reads with CEL. Karta defines where a value lives. The SDK takes the new value and applies the change.

## CRD

`podTemplateSpecPath: .spec.template` becomes a read expression and a write path:

```yaml
apiVersion: run.ai/v1alpha1
kind: Karta
metadata:
  name: job
spec:
  structureDefinition:
    rootComponent:
      name: job
      kind: {group: batch, version: v1, kind: Job}
      statusDefinition: {statusMappings: {}}
      specDefinition:
        podTemplateSpec:
          expression: 'object[?"spec"][?"template"].orValue(null)'
          pathWrite: /spec/template
      suspendDefinition:
        pathWrite: /spec/suspend
```

Use `pathWrite` or `pathWriteExpression`, not both. Without either, the field is read-only. Patch format, merge policy, and new values belong in the SDK call.

<details>
<summary>expression: read a value from the workload</summary>

Inside a Deployment component in Karta:

```yaml
scaleDefinition:
  replicas:
    expression: 'object[?"spec"][?"replicas"].orValue(1)'
```

`object` is the workload CR. With `spec.replicas: 3`, this returns `3`. If replicas is missing, it returns `1`.

This is why reads and writes are separate. A calculated value such as `1` does not tell the SDK where to write. This accessor is read-only until a write path is added.

</details>

<details>
<summary>pathWrite: write to a fixed location</summary>

The same Deployment accessor, now writable:

```yaml
scaleDefinition:
  replicas:
    expression: 'object[?"spec"][?"replicas"].orValue(1)'
    pathWrite: /spec/replicas
```

An SDK write of `5` sets `spec.replicas: 5`. The template stays unchanged. The location is always the same, so it needs no CEL expression.

`pathWrite: ""` means the whole document. Omitting `expression` makes the field write-only. The CRD rejects invalid pointers and an accessor containing both write-path forms.

</details>

<details>
<summary>pathWriteExpression: find KServe's container location</summary>

KServe can put model settings under `spec.predictor.model` or `spec.predictor.sklearn`. A fixed `/spec/predictor/model` path would miss the second case.

The catalog finds the predictor entry with `storageUri`. Both the read and write use `containerKeys` so they select the same entry:

```yaml
# Under spec.
variables:
  - name: containerKeys
    expression: >-
      ([dyn(object[?"spec"][?"predictor"].orValue(null))]
        .filter(v, type(v) == map) + [{}])[0]
        .map(k, string(k)).sort().filter(k,
          type(object.spec.predictor[k]) == map &&
          "storageUri" in object.spec.predictor[k] &&
          object.spec.predictor[k]["storageUri"] != null &&
          object.spec.predictor[k]["storageUri"] != false)

# Under structureDefinition.childComponents[name=predictor].
specDefinition:
  fragmentedPodSpecDefinition:
    container:
      expression: >-
        variables.containerKeys.size() == 0 ? dyn(null) :
        variables.containerKeys.size() == 1 ?
          object.spec.predictor[variables.containerKeys[0]] :
          variables.containerKeys.map(k, object.spec.predictor[k])
      pathWriteExpression: >-
        variables.containerKeys.size() == 1 ?
          "/spec/predictor/" +
          variables.containerKeys[0].replace("~", "~0").replace("/", "~1") :
          dyn(null)
```

One match gives `/spec/predictor/model` or `/spec/predictor/sklearn`. No match fails the write. Multiple matches fail instead of choosing a model by accident. The `replace` calls escape `~` and `/` inside a key.

The SDK caller uses `Component: "predictor", Field: tree.Container` in both cases. Only the Karta definition knows which path to use.

Write expressions can read `object`, `variables`, available `references`, and the current `instance` / `index`. They do not receive the new value. The SDK checks the returned pointer before writing.

</details>

<details>
<summary>spec.variables: reuse a calculation</summary>

For Ray, the same worker list supplies both the IDs and templates. This Karta excerpt names that list once:

```yaml
spec:
  variables:
    - name: workerGroups
      expression: object.spec.workerGroupSpecs
  structureDefinition:
    childComponents:
      - name: worker
        instanceIds:
          expression: variables.workerGroups.map(g, g.groupName)
        specDefinition:
          podTemplateSpec:
            expression: variables.workerGroups.map(g, g.template)
```

Both reads use `variables.workerGroups`. If the list location changes, its definition changes in one place. Variables are calculated from the CR; they are not values supplied by the SDK caller.

</details>

<details>
<summary>component.fields: add a field without changing the SDK</summary>

Inside an App's Karta root component:

```yaml
name: app
fields:
  exampleos:
    expression: object.d.d.c
    pathWrite: /d/d/c
```

For an editor opened with that App's Karta and workload:

```go
if err := editor.Mutate(ctx, tree.Write{
    Component: "app", Field: tree.Field("exampleos"), Value: "new",
}); err != nil {
    return err
}
```

`d.d.c: old` becomes `d.d.c: new`. Other fields under `d.d` stay. The caller uses `exampleos`; it does not need `/d/d/c` or a new SDK enum.

</details>

<details>
<summary>instanceIds.expression: select Ray's gpu worker group by name</summary>

A RayCluster can have several worker groups. This workload excerpt has two:

```yaml
spec:
  workerGroupSpecs:
    - groupName: gpu
      template:
        spec: {schedulerName: default-scheduler}
    - groupName: cpu
      template:
        spec: {schedulerName: default-scheduler}
```

The worker component in Karta reads their names and templates in the same order:

```yaml
# Under spec.structureDefinition.childComponents.
- name: worker
  instanceIds:
    expression: object.spec.workerGroupSpecs.map(g, g.groupName)
  specDefinition:
    podTemplateSpec:
      expression: object.spec.workerGroupSpecs.map(g, g.template)
      pathWriteExpression: '"/spec/workerGroupSpecs/" + string(index) + "/template"'
```

The IDs are `["gpu", "cpu"]`. They come from `groupName`; Karta does not invent them. These identify worker groups, not individual Pods.

With an editor opened using `kartas.Raycluster()` and that workload, change only `gpu`:

```go
if err := editor.Mutate(ctx, tree.Write{
    Component: "worker", Instance: "gpu", Field: tree.PodTemplateSpec,
    Value: map[string]any{
        "spec": map[string]any{"schedulerName": "batch-scheduler"},
    },
}); err != nil {
    return err
}
```

`gpu` now uses `batch-scheduler`. `cpu` still uses `default-scheduler`.

Why an ID instead of a number? The SDK finds `gpu` in the current ID list and gives its position to the write expression as `index`:

| Workload order | index for gpu | Resolved path |
| --- | --- | --- |
| `[gpu, cpu]` | `0` | `/spec/workerGroupSpecs/0/template` |
| `[cpu, gpu]` | `1` | `/spec/workerGroupSpecs/1/template` |

The SDK call stays the same when the groups move. The tree's display order does not decide where to write. IDs must be unique, nonempty strings and are read-only. Keep the ID and template reads in the same order; sorting only the IDs would pair them with the wrong templates.

These short expressions require the shown list and fields. The catalog also handles missing lists. `instanceIds.expression` replaces jq's `instanceIdPath`; identifying groups is not a new capability.

</details>

<details>
<summary>suspendDefinition: use the same suspend call for different workloads</summary>

Inside a Job's Karta root component:

```yaml
suspendDefinition:
  pathWrite: /spec/suspend
```

For an editor opened with the Job definition:

| SDK call | Result |
| --- | --- |
| `editor.IsSuspendable()` | `true`, because the Karta declares support. |
| `editor.Suspend(ctx)` | Sets `spec.suspend: true`. |
| `editor.Resume(ctx)` | Sets `spec.suspend: false`. |

The controller can use the same calls for another workload. Its Karta supplies the boolean field's location. A computed location can use `pathWriteExpression` instead. This changes the local spec; it does not wait for the workload to stop or resume.

</details>

<details>
<summary>Other jq paths become CEL expressions</summary>

Status, Pod selection, and Pod grouping keep their purpose. Their read fields now contain CEL:

| Karta field | Definition value | Reads |
| --- | --- | --- |
| `statusDefinition.phaseDefinition.expression` | `object[?"status"][?"state"].orValue(null)` | The workload's state. |
| `statusDefinition.conditionsDefinition.expression` | `object[?"status"][?"conditions"].orValue(null)` | The workload's conditions. |
| `podSelector.componentTypeSelector.expression` | `object.metadata.labels["ray.io/node-type"]` | A Pod's Ray role, such as `worker`. |
| `groupByExpressions` | `[ 'object.metadata.labels["ray.io/cluster"]' ]` | Groups Pods by Ray cluster. |

Spec and scale reads use the same accessor shape as the replicas example. `groupByExpressions` replaces `groupByKeyPaths`; the old grouping `filters` are removed.

</details>

## SDK

### Change an image

`workload` is the KServe CR. `imageFromUser` is the new image, for example `inference:v2`.

```go
editor, err := tree.Open(ctx, kartas.KServe(), workload)
if err != nil {
    return err
}

if err := editor.Mutate(ctx, tree.Write{
    Component: "predictor", Field: tree.Container,
    Value: map[string]any{"image": imageFromUser},
}); err != nil {
    return err
}

updated, err := editor.GetResource()
if err != nil {
    return err
}
```

`updated` contains the changed CR. At `spec.predictor.model`:

| Field | Before | After |
| --- | --- | --- |
| `image` | `inference:v1` | `inference:v2` |
| `storageUri` | `s3://bucket/model` | `s3://bucket/model` |
| `modelFormat` | `{name: sklearn}` | `{name: sklearn}` |

<details>
<summary>Why send only image, instead of the whole Container?</summary>

KServe's model has fields that `corev1.Container` cannot hold: `storageUri` and `modelFormat`. Reading into that Go type and replacing the whole model can remove them.

The call above sends only `{"image": "inference:v2"}`. Default Merge keeps the other map fields. This fixes the broad typed-write problem; changing jq to CEL or choosing JSONPatch alone does not fix it.

</details>

<details>
<summary>Get the path without changing anything</summary>

Use this when another part of the controller needs the location:

```go
location, err := editor.ResolveWriteTarget(ctx, tree.Target{
    Component: "predictor", Field: tree.Container,
})
if err != nil {
    return err
}
fmt.Println(location.Path) // /spec/predictor/model
```

This does not write. It returns `Path`, `Exists`, and a copy of `Value`. Changing that copy does not update the workload. A caller can use the path in its own patch; `Mutate` already resolves it internally.

Resolve again after a workload change. For example, a list item may have moved to another index.

</details>

<details>
<summary>Choose Merge or Replace, JSONPatch or MergePatch</summary>

Replace all predictor labels with the supplied map:

```go
if err := editor.Mutate(ctx, tree.Write{
    Component: "predictor", Field: tree.LabelsField,
    Value: map[string]any{"team": "platform"},
    Options: resource.MutationOptions{
        PatchType: resource.PatchTypeJSONPatch,
        Strategy:  resource.Replace,
        Parents:   resource.RequireParents,
    },
}); err != nil {
    return err
}
```

Starting labels: `{team: ml, owner: alice}`. Input: `{team: platform}`.

| Strategy | Result | Use when |
| --- | --- | --- |
| `Merge` | `{team: platform, owner: alice}` | Updating some map fields. |
| `Replace` | `{team: platform}` | The supplied map is the complete desired value. |

Both patch formats support these SDK strategies. Arrays always replace; neither merges containers by name.

`PatchTypeMergePatch` is convenient for partial maps. `{"owner": null}` deletes `owner`. With `PatchTypeJSONPatch`, the same input stores a literal null. Use that for fields whose workload schema allows null.

Defaults are MergePatch, Merge, and CreateMapParents. In this call, `RequireParents` needs `spec.predictor` to exist; `labels` itself may be missing. `CreateMapParents` can create missing parent maps, never arrays. These are SDK options, not fields in the Karta CR.

</details>

<details>
<summary>Edit inside the model with a draft</summary>

Use a draft for several field edits or for selecting and moving list items. For the same image change:

```go
draft, err := tree.BeginEdit(ctx, editor)
if err != nil {
    return err
}
defer draft.Abort()

model, err := draft.Target(ctx, tree.Target{
    Component: "predictor", Field: tree.Container,
})
if err != nil {
    return err
}
if err := model.At("image").Set(imageFromUser); err != nil {
    return err
}
if err := draft.Commit(ctx); err != nil {
    return err
}
```

`model` points into the draft's raw JSON. It is not a `corev1.Container`. `At("image")` selects one child field, so `storageUri` and `modelFormat` stay in the saved model.

![The Karta finds the model; the draft changes image and keeps model data](accessor-model.png)

`Commit` updates the editor's local CR and tree together. If an edit fails, none of the draft is published. If the editor changed after `BeginEdit`, start a new draft rather than overwrite newer work.

Drafts use explicit operations instead of a merge policy: `Set` changes a scalar or null, `Replace` replaces an object/list, and `Remove` deletes it. Reads and selectors use the starting snapshot. Missing parents fail by default; pass `tree.WithEditParents(resource.CreateMapParents)` to `BeginEdit` to allow missing maps.

</details>

<details>
<summary>Other SDK calls</summary>

| Call | What it is for |
| --- | --- |
| `editor.Snapshot()` | Read the extracted tree. Editing this copy does not change the CR. |
| `editor.IsSuspendable()` | Check whether the root supports `Suspend(ctx)` and `Resume(ctx)`. For Job, these set `spec.suspend` to true / false. |
| `editor.Mutate(ctx, a, b)` | Write different values to two fields together. An error applies neither; overlapping destinations reject. |
| `draft.Snapshot()` | Read the tree as it was when the draft started. |
| `cursor.At("labels", "example.com/team")` | Select literal keys; the SDK escapes the slash. |
| `cursor.Read()` / `ReadInto(&view)` | Read raw JSON plus an existence flag / read into a Go type. |
| `cursor.Path()` | Get the current path, including after a list move. |
| `cursor.Set("v2")` / `Set(nil)` | Write a scalar / literal null. |
| `cursor.Replace(value)` / `Remove()` | Replace the whole selected value / delete it. |
| `list.Items()` / `list.Match("name", "main")` | Get list item handles / select exactly one matching starting item. |
| `list.InsertBefore(nil, value)` | Append. Pass an item handle to insert before it. |
| `item.MoveBefore(nil)` | Move the whole item to the end, keeping its unknown fields. |
| `draft.Commit(ctx)` / `Abort()` | Apply the draft / discard it. |

To copy a value from the CR, read it with `source.Read()` and pass it to `destination.Set(value)`. Use `Replace` for an object/list. A typed read is for inspection, not a complete replacement value.

Existing `Component.Get*/Update*` calls remain, with policy set by `resource.WithMutationOptions`. Broad typed replacements can still lose unknown fields. `tree.Build` reads only. Lower-level callers can use `factory.PathWriter()` for `ResolveWriteTarget` / `WriteValues`, or `resource.Accessor.ApplyPatch` for an already-built patch. `ApplyPatch` does not evaluate CEL.

</details>

These calls change a local copy. The controller still saves `GetResource()` to Kubernetes and handles conflicts. Automatic scheduler discovery is not added.

The prototype uses `v1alpha1`. Releasing this breaking schema needs a new CRD version and migration; those are not implemented. GVK/scheduling renames and references are separate work. Checks cover unit tests, recorded workloads, and field preservation, not live-cluster migration.
