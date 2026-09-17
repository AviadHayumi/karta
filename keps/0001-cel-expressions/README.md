<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# KEP-0001: CEL expressions

- Status: implementable
- Authors: @AviadHayumi
- Created: 2026-09-10
- Tracking issue: required before merge

CEL reads the workload and finds write locations. The SDK decides what to write and how. No patches in the Karta CR.

Why: jq used the same expression for reading and writing. Separating them allows a computed read without guessing where its result belongs.

## CRD

Before: `podTemplateSpecPath: .spec.template`. Now:

```yaml
apiVersion: run.ai/v1alpha1 # Prototype version; see release note below.
kind: Karta
metadata:
  name: job
spec:
  structureDefinition:
    rootComponent:
      name: job
      kind: {group: batch, version: v1, kind: Job}
      statusDefinition: {}
      specDefinition:
        podTemplateSpec:
          expression: 'object[?"spec"][?"template"].orValue(null)'
          pathWrite: /spec/template
      suspendDefinition:
        pathWrite: /spec/suspend
```

| Field | What it does |
| --- | --- |
| `expression` | Reads a value. Above, missing template returns `null`. Optional for write-only fields. |
| `pathWrite` | Fixed JSON Pointer, e.g. `/spec/template`. `""` means the whole document. |
| `pathWriteExpression` | CEL returning one pointer. Use when the location depends on the CR. |
| `spec.variables` | Reusable CEL expressions, read as `variables.name`. |
| `component.fields` | Custom named accessors. No SDK enum change needed. |
| `instanceIds.expression` | Read-only stable IDs for repeated components. |
| `suspendDefinition` | One write path; suspend writes `true`, resume writes `false`. |

Choose one write-path form. Without either, the accessor is read-only. Paths see `object`, `variables`, and, for instances, `instance` and `index`; available references are readable too. They never receive the caller's value. Bad or unresolved paths fail the write.

CRD validation checks fixed pointer syntax and exclusive choices; the SDK validates computed pointers. All spec/scale accessors use this shape. Selector and condition paths become `expression`; `instanceIdPath` becomes `instanceIds.expression`; `groupByKeyPaths` becomes `groupByExpressions`, without the old grouping `filters`.

<details>
<summary>KServe: find the container, then return its path</summary>

The catalog finds predictor entries containing `storageUri`. For `model`, the destination is `/spec/predictor/model`; for `sklearn`, it is `/spec/predictor/sklearn`. Zero matches reject a write. Multiple matches reject instead of picking one.

```yaml
# Excerpt: spec.variables and the predictor's container accessor.
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

# structureDefinition.childComponents[name=predictor]
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

The replacements escape a literal `~` or `/` inside a key. This is the [catalog's actual selector](https://github.com/AviadHayumi/workload-map/blob/cel-native/pkg/catalog/kartas/kserve.go), not a hardcoded `model` path.

</details>

## SDK

![Karta resolves the target; the SDK edits and publishes locally](accessor-model.png)

[Editable Excalidraw source](accessor-model.excalidraw).

The caller names a component and field, not an operator-specific path. These snippets omit setup and repeated error checks; [runnable examples](https://github.com/AviadHayumi/workload-map/blob/cel-native/pkg/tree/example_editable_test.go) check each error.

```go
editor, err := tree.Open(ctx, kartas.KServe(), workload)
view := editor.Snapshot() // Read-only copy of the extracted tree.
target, err := editor.ResolveWriteTarget(ctx, tree.Target{
    Component: "predictor", Field: tree.Container,
}) // target.Path, target.Exists, target.Value

err = editor.Mutate(ctx, tree.Write{
    Component: "predictor", Field: tree.Container,
    Value: map[string]any{"image": imageFromUser},
})
updated, err := editor.GetResource()
```

For a model with `image: inference:v1` and `storageUri: s3://bucket/model`, passing `imageFromUser = "inference:v2"` changes only the image. The storage URI stays.

`Mutate` defaults to MergePatch + Merge + CreateMapParents. Override per write:

```go
Options: resource.MutationOptions{
    PatchType: resource.PatchTypeJSONPatch, // Or PatchTypeMergePatch.
    Strategy:  resource.Replace,           // Or Merge.
    Parents:   resource.RequireParents,    // Or CreateMapParents.
},
```

Merge keeps unmentioned map keys; Replace owns the whole selected value. Arrays replace in both. JSONPatch keeps literal `null`; MergePatch uses `null` to delete map members. Creating parents only creates maps, never arrays.

For nested edits, keep the raw CR in a draft:

```go
draft, err := tree.BeginEdit(ctx, editor)
defer draft.Abort()
model, err := draft.Target(ctx, tree.Target{
    Component: "predictor", Field: tree.Container,
})
err = model.At("image").Set(imageFromUser)
err = draft.Commit(ctx)
```

This preserves `storageUri` and `modelFormat`, even though `corev1.Container` has neither. Reads may use that Go type; edits do not write its smaller copy back over the model. Drafts do not take a patch policy: `Set`, `Replace`, and `Remove` state the operation.

<details>
<summary>Other calls: custom fields, instances, suspension, lists</summary>

Custom field in any component:

```yaml
fields:
  exampleos:
    expression: object.d.d.c
    pathWrite: /d/d/c
```

```go
err = editor.Mutate(ctx, tree.Write{
    Component: "app", Field: tree.Field("exampleos"), Value: "new",
}) // /d/d/c changes; its neighbors stay.
```

For repeated components, add `Instance: "gpu"` to `Target` or `Write`. It is the catalog's ID, not the displayed tree index.

| Call | Example / purpose |
| --- | --- |
| `editor.IsSuspendable()` | Check root capability before `Suspend(ctx)` or `Resume(ctx)`. |
| `editor.Mutate(ctx, a, b)` | Two different values in one atomic batch; overlapping targets reject. |
| `draft.Snapshot()` | Typed view from the start of the draft. |
| `cursor.At("labels", "example.com/team")` | Literal keys; SDK escapes the pointer. |
| `cursor.Read()` / `ReadInto(&view)` | Raw value with existence flag / typed read. |
| `cursor.Path()` | Current pointer; follows list moves. |
| `cursor.Set("v2")` / `Set(nil)` | Set a scalar / literal null. Not a whole map or list. |
| `cursor.Replace(map[string]any{"team": "ml"})` | Replace the whole selected value. |
| `cursor.Remove()` | Delete the selected field or list item. |
| `list.Items()` / `list.Match("name", "main")` | Item handles / exactly one matching item. |
| `list.InsertBefore(nil, value)` | Append; pass an item handle to insert before it. |
| `item.MoveBefore(nil)` | Move the whole raw item to the end; handle still follows it. |
| `draft.Commit(ctx)` / `Abort()` | Publish locally / discard. |

To copy from the CR, pass the value from `source.Read()` to `destination.Set(value)`. Use `Replace` for an object/list. Reads and selectors use the starting snapshot. A draft error prevents publication; a stale draft must be rebuilt. Missing parents fail unless `tree.WithEditParents(resource.CreateMapParents)` is passed to `BeginEdit`.

Existing `Component.Get*/Update*` methods remain; `resource.WithMutationOptions` sets their policy. Broad typed replacements can still lose unknown fields. `tree.Build` remains read-only. Lower-level `factory.PathWriter()` exposes `ResolveWriteTarget` and `WriteValues`; `resource.Accessor.ApplyPatch` accepts caller-built patches, without CEL. See [draft examples](https://github.com/AviadHayumi/workload-map/blob/cel-native/pkg/tree/example_draft_test.go) and [path writer examples](https://github.com/AviadHayumi/workload-map/blob/cel-native/pkg/resource/example_write_test.go).

</details>

`Open` copies the input. Successful writes publish the local workload and rebuilt tree together. Nothing is sent to Kubernetes; the controller persists `GetResource()` using its normal conflict handling. Universal scheduler discovery is not added.

Release note: this is a breaking schema change. The prototype still uses `run.ai/v1alpha1`; a released `v1alpha2` and migration are not implemented. Unrelated GVK/scheduling renames and references are outside this KEP. Verification covers unit tests, catalog replay, and raw-field preservation; it does not prove live-cluster migration.
