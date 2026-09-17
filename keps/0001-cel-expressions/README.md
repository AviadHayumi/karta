<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# KEP-0001: CEL expressions

- Status: implementable
- Authors: @AviadHayumi
- Created: 2026-09-10
- Tracking issue: required before merge

Replace jq reads with CEL. The Karta definition tells the SDK where to read and write. The controller supplies the new value.

For example, the controller asks to change the predictor's image. Karta finds the predictor in the workload; the SDK changes its image. The controller does not need to know KServe's JSON path.

## CRD

The old `podTemplateSpecPath: .spec.template` did two jobs: read the template and locate it for writes. The new definition states each job separately:

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

Use `expression` to tell Karta what to read. For a Deployment's replica count, put this in its Karta component:

```yaml
scaleDefinition:
  replicas:
    expression: 'object[?"spec"][?"replicas"].orValue(1)'
```

`object` means the workload passed to Karta. Here it is the Deployment, not the Karta CR.

With `spec.replicas: 3`, the expression returns `3`. The `?` lookups allow missing fields; `orValue(1)` returns `1` when replicas is absent. Reading that default does not add it to the Deployment.

Why separate the write? A result of `1` is just a number. The SDK still needs a location before it can change replicas. That is what `pathWrite` supplies below.

</details>

<details>
<summary>pathWrite: write to a fixed location</summary>

To let the controller change that replica count, add its location:

```yaml
scaleDefinition:
  replicas:
    expression: 'object[?"spec"][?"replicas"].orValue(1)'
    pathWrite: /spec/replicas
```

`/spec/replicas` means "open spec, then write replicas." The controller can now use `Component: "deployment", Field: tree.Replicas, Value: 5` in `editor.Mutate`. Replicas becomes `5`; the template stays unchanged.

Use this when the location is fixed. The caller chooses the new number; the Karta author defines where it goes.

`pathWrite: ""` means the whole document. Omitting `expression` makes the field write-only. The CRD rejects invalid pointers and an accessor containing both write-path forms.

</details>

<details>
<summary>pathWriteExpression: find KServe's container location</summary>

Use `pathWriteExpression` when the location depends on the workload. It returns a path string, not a patch or the new value.

For example, KServe can put model settings in either of these places:

| Where the model is in the workload | Path the expression returns |
| --- | --- |
| `spec.predictor.model` | `/spec/predictor/model` |
| `spec.predictor.sklearn` | `/spec/predictor/sklearn` |

The catalog looks for the predictor entry containing `storageUri`. The controller uses `Component: "predictor", Field: tree.Container` in either case. A fixed path would only handle one of them.

<details>
<summary>The KServe expression and its checks</summary>

`containerKeys` holds the matching entry names, such as `["model"]`. Both the read and the write use this list so they select the same entry:

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

Write expressions can read `object`, `variables`, available `references`, and the current `instance` / `index`. They do not receive the new value. The SDK checks the returned pointer before writing.

</details>

</details>

<details>
<summary>spec.variables: reuse a calculation</summary>

Use a variable when two expressions need the same data. In this Ray example, one expression reads worker names and another reads their templates. Both start from `spec.workerGroupSpecs`:

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

`workerGroups` is a name for that list inside CEL. Each read can use `variables.workerGroups` instead of repeating its location. If the location changes, only the variable definition needs changing.

This does not add a field to the RayCluster. It also does not supply a new value for a write; that still comes from `Value` in the SDK call.

</details>

<details>
<summary>component.fields: add a field without changing the SDK</summary>

Suppose an App stores a setting at `d.d.c`, and the controller should call it `exampleos`. Add this to the App's Karta root component:

```yaml
name: app
fields:
  exampleos:
    expression: object.d.d.c
    pathWrite: /d/d/c
```

After opening that Karta and workload with `tree.Open`, the controller writes by name:

```go
if err := editor.Mutate(ctx, tree.Write{
    Component: "app", Field: tree.Field("exampleos"), Value: "new",
}); err != nil {
    return err
}
```

`d.d.c: old` becomes `d.d.c: new`. Other fields under `d.d` stay. `tree.Field("exampleos")` accepts the name from the Karta definition. Adding a setting does not require adding a Go constant or teaching the controller its JSON path.

</details>

<details>
<summary>instanceIds.expression: how to change gpu without changing cpu</summary>

A RayCluster has two worker groups, `gpu` and `cpu`. Suppose only `gpu` should use a different scheduler.

`Component: "worker"` alone cannot say which group to change. `instanceIds.expression` tells Karta how to read each group's name. The controller can then say `Instance: "gpu"`.

The relevant part of the workload, before the change:

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

The Karta author defines how to find the groups once. Controllers using `kartas.Raycluster()` already get these rules from the catalog; they do not need to write this CEL themselves. The shortened definition is:

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

Read `.map(g, g.groupName)` as "for each group `g`, take its `groupName`." On this workload, it returns `["gpu", "cpu"]`. These are the names already in the RayCluster. Karta does not create them.

The template expression reads each group's template in the same order. To write to `gpu`, the SDK finds its position, `0`, and gives that number to `pathWriteExpression` as `index`. The result is `/spec/workerGroupSpecs/0/template`.

The controller does not calculate that path. With the RayCluster loaded into `workload`, it calls:

```go
editor, err := tree.Open(ctx, kartas.Raycluster(), workload)
if err != nil {
    return err
}

if err := editor.Mutate(ctx, tree.Write{
    Component: "worker", Instance: "gpu", Field: tree.PodTemplateSpec,
    Value: map[string]any{
        "spec": map[string]any{"schedulerName": "batch-scheduler"},
    },
}); err != nil {
    return err
}

updated, err := editor.GetResource()
if err != nil {
    return err
}
```

Here, `Component` selects the worker definition, `Instance` selects the `gpu` group, and `Field` selects its template. `Value` supplies the part of that template to change.

The default Merge keeps the rest of the template, including containers and their images. In `updated`:

| Worker group | Scheduler before | Scheduler after |
| --- | --- | --- |
| `gpu` | `default-scheduler` | `batch-scheduler` |
| `cpu` | `default-scheduler` | `default-scheduler` |

The controller still needs to save `updated` to Kubernetes. This edits the worker group's template, not its running Pods directly.

Why use `gpu` instead of position `0`? If the list order changes in the workload passed to Karta, position `0` might belong to `cpu`:

| Workload order | index for gpu | Resolved path |
| --- | --- | --- |
| `[gpu, cpu]` | `0` | `/spec/workerGroupSpecs/0/template` |
| `[cpu, gpu]` | `1` | `/spec/workerGroupSpecs/1/template` |

The same `Instance: "gpu"` call still selects `gpu`. A group is called an "instance" here because several groups share one `worker` definition. These IDs identify groups, not individual Pods.

<details>
<summary>Rules when writing a Karta with repeated components</summary>

IDs must be unique, nonempty strings and are read-only. Keep the ID and template reads in the same order. For example, sorting only `["gpu", "cpu"]` to `["cpu", "gpu"]` would attach the name `cpu` to the first group's template.

The tree may sort names for display. The SDK uses their positions in the workload when finding write paths, not the display order. It returns an error if the requested ID does not exist.

These short expressions require the shown list and fields. The catalog also handles missing lists. `instanceIds.expression` replaces jq's `instanceIdPath`; identifying groups is not a new capability.

</details>

</details>

<details>
<summary>suspendDefinition: use the same suspend call for different workloads</summary>

A controller should be able to ask "can this workload be suspended?" and then suspend it. It should not need to know where each operator keeps that setting.

For a Job, its Karta root component declares:

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

Another workload can use the same SDK calls if its Karta points to the boolean setting that controls suspension. Use `pathWriteExpression` if that location varies. These calls change the local CR; they do not save it to Kubernetes or wait for the workload to stop or resume.

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
