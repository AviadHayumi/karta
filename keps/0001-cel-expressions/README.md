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

`editor` holds a local copy of the workload and its extracted tree. `editor.Mutate` changes that local copy directly. A `draft` lets the controller prepare smaller edits, inspect existing values, and select list items before applying the whole edit with `Commit`.

Neither writes to Kubernetes. The controller saves the result with its Kubernetes client. A simple image update needs only the editor; it does not need a draft or either `Snapshot` call.

<details>
<summary>A controller example: fetch a KServe CR, change its image, save it</summary>

This controller watches InferenceServices. For this example, the user supplies the desired image through an annotation:

```yaml
metadata:
  annotations:
    example.com/desired-image: inference:v2
```

That annotation is an input convention for this example, not a Karta or KServe API field. In a platform controller, the value could instead come from a parent CR or a policy.

The reconcile loop fetches the CR, edits a local copy, and saves only when the result differs:

```go
func (r *ImageReconciler) Reconcile(
    ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
    workload := r.workload()
    if err := r.Get(ctx, req.NamespacedName, workload); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    image := workload.GetAnnotations()["example.com/desired-image"]
    if image == "" {
        return ctrl.Result{}, nil
    }

    editor, err := tree.Open(ctx, kartas.KServe(), workload)
    if err != nil {
        return ctrl.Result{}, err
    }
    if err := editor.Mutate(ctx, tree.Write{
        Component: "predictor", Field: tree.Container,
        Value: map[string]any{"image": image},
    }); err != nil {
        return ctrl.Result{}, err
    }
    updated, err := editor.GetResource()
    if err != nil {
        return ctrl.Result{}, err
    }
    before, err := json.Marshal(workload)
    if err != nil {
        return ctrl.Result{}, fmt.Errorf("encode original workload: %w", err)
    }
    after, err := json.Marshal(updated)
    if err != nil {
        return ctrl.Result{}, fmt.Errorf("encode updated workload: %w", err)
    }
    if bytes.Equal(before, after) {
        return ctrl.Result{}, nil
    }
    return ctrl.Result{}, r.Update(ctx, updated)
}
```

The Karta tells `Open` how to read this workload and find its writable fields. `Mutate` receives only the new image. `GetResource` returns the changed CR, with its Kubernetes identity and resource version. `r.Update` is the only line that saves the edit to Kubernetes.

With `image: inference:v1` and the annotation above, the model ends with `image: inference:v2`; `storageUri` and `modelFormat` stay. On the next reconcile, the JSON is unchanged, so the controller makes no update request. JSON comparison avoids differences caused only by Go number types.

If another controller updated the CR meanwhile, the API rejects the stale resource version. Returning that error lets controller-runtime retry; the next reconcile fetches the CR again and recomputes the edit. Do not keep the old editor across retries.

<details>
<summary>Imports, reconciler type, and watch setup</summary>

Use the method above with this controller boilerplate in the same Go file:

```go
package controller

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"

    "github.com/run-ai/karta/pkg/catalog/kartas"
    "github.com/run-ai/karta/pkg/tree"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/runtime/schema"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=serving.kserve.io,resources=inferenceservices,verbs=get;list;watch;update
type ImageReconciler struct {
    client.Client
}

func (*ImageReconciler) workload() *unstructured.Unstructured {
    workload := &unstructured.Unstructured{}
    workload.SetGroupVersionKind(schema.GroupVersionKind{
        Group: "serving.kserve.io", Version: "v1beta1", Kind: "InferenceService",
    })
    return workload
}

func (r *ImageReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(r.workload()).
        Complete(r)
}
```

Register `ImageReconciler{Client: mgr.GetClient()}` through `SetupWithManager` in the controller's existing manager setup. The KServe CRD and the generated RBAC must be installed. Image validation and which users may request a change belong to the controller's policy.

The watch is KServe-specific because it chooses which resource triggers this controller. Finding the model's location is Karta's job. A different workload uses its corresponding Karta definition.

</details>

To use a draft in this controller, replace the `editor.Mutate` block with the draft helper below and check its error. Keep the same `GetResource`, unchanged-result check, and `r.Update` steps. The other examples are small helpers for that part of a reconcile loop; use the workload's matching Karta when opening its editor.

</details>

<details>
<summary>Editor, draft, and why there are two Snapshot calls</summary>

Use `editor.Snapshot()` to inspect the editor's current tree. For example, after opening a Deployment with one container:

```go
view := editor.Snapshot()
template := view.Root.Instances[0].ExtractedInstance.PodTemplateSpec
fmt.Println(template.Spec.Containers[0].Image) // api:v1
```

The tree gives the controller Go values to inspect. It is not the original CR: for example, a Deployment's `spec.template` appears as `PodTemplateSpec`. Changing `template.Spec.Containers[0].Image` here only changes this returned copy. To write, use `Mutate` or a draft.

`tree.BeginEdit(ctx, editor)` creates a draft from the editor's current workload. Use `draft.Snapshot()` if the edit needs to inspect the tree as it was at that moment. For example, a controller can decide which worker groups to edit from one fixed view, even while building several changes.

The two snapshots answer different questions:

| Call | Question it answers |
| --- | --- |
| `editor.Snapshot()` | What is in the editor now? |
| `draft.Snapshot()` | What was in the editor when this draft began? |

Suppose the image starts as `api:v1`. Each snapshot column below means a new call at that step:

| Step | Image in Kubernetes | editor.Snapshot() | draft.Snapshot() |
| --- | --- | --- | --- |
| Fetch the CR, open the editor, begin a draft | `api:v1` | `api:v1` | `api:v1` |
| Draft sets image to `api:v2` | `api:v1` | `api:v1` | `api:v1` |
| `draft.Commit(ctx)` succeeds | `api:v1` | `api:v2` | Closed; returns `nil` |
| Controller saves `editor.GetResource()` | `api:v2` | `api:v2` | Closed |

`draft.Snapshot()` is not a preview of pending edits. Draft reads use the starting values, so an earlier write cannot change what a later selection matches. After a successful commit, call `editor.Snapshot()` or `editor.GetResource()` to read the result. A previously returned snapshot stays unchanged.

Most simple controllers do not need `draft.Snapshot()`. It lets a helper that receives only the draft inspect its starting tree without also receiving the editor. It is a convenience for those helpers, not a required step in every edit.

</details>

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

Use a draft when the edit needs to work inside the selected value. This helper performs the same image change as `Mutate` above:

```go
func setImageWithDraft(ctx context.Context, editor tree.Editable, image string) error {
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
    if err := model.At("image").Set(image); err != nil {
        return err
    }
    return draft.Commit(ctx)
}
```

In `Reconcile`, replace the `editor.Mutate` call with `setImageWithDraft(ctx, editor, image)` and return any error. For this one-field change, either approach works. A draft becomes useful when the next step needs to read a sibling field or select a list item.

`model` points into the draft's raw JSON. It is not a `corev1.Container`. `At("image")` selects one child field, so `storageUri` and `modelFormat` stay in the saved model.

![The Karta finds the model; the draft changes image and keeps model data](accessor-model.png)

`Commit` updates the editor's local CR and tree together. If an edit fails, none of the draft is published. If the editor changed after `BeginEdit`, start a new draft rather than overwrite newer work.

Drafts use explicit operations instead of a merge policy: `Set` changes a scalar or null, `Replace` replaces an object/list, and `Remove` deletes it. Reads and selectors use the starting snapshot. Missing parents fail by default; pass `tree.WithEditParents(resource.CreateMapParents)` to `BeginEdit` to allow missing maps.

</details>

<details>
<summary>Change two fields together: Mutate(ctx, a, b)</summary>

Suppose a rollout must set both the KServe image and a predictor label. The new image comes from the user; the team comes from the controller's configuration. Call this helper after opening the editor and before saving it:

```go
func configurePredictor(
    ctx context.Context, editor tree.Editable, image, team string,
) error {
    return editor.Mutate(ctx,
        tree.Write{
            Component: "predictor", Field: tree.Container,
            Value: map[string]any{"image": image},
        },
        tree.Write{
            Component: "predictor", Field: tree.LabelsField,
            Value: map[string]any{"team": team},
        },
    )
}
```

With `image = "inference:v2"` and `team = "platform"`, the model's image changes to `inference:v2` and `spec.predictor.labels.team` becomes `platform`. Default Merge keeps the model's storage settings and other labels, such as `owner: alice`.

Why one call? If either write fails, the editor keeps both old values. Two separate `Mutate` calls would leave the first local change applied if the second failed. This is local all-or-nothing behavior; saving to Kubernetes is still the controller's job.

The two destinations must not overlap. For example, writing the whole template and a field inside that template in the same call is rejected. Put the changes in one partial value or use one draft target and its child cursors.

</details>

<details>
<summary>Pause or resume a workload: IsSuspendable, Suspend, Resume</summary>

Suppose a queue controller decides whether a Job may run. Open the Job with `kartas.BatchJob()`, then pass the queue's decision to this helper:

```go
func setPaused(ctx context.Context, editor tree.Editable, pause bool) error {
    if !editor.IsSuspendable() {
        return fmt.Errorf("this Karta does not define how to suspend the workload")
    }
    if pause {
        return editor.Suspend(ctx)
    }
    return editor.Resume(ctx)
}
```

For a Job, `pause = true` sets `spec.suspend: true`; `false` sets it to `false`. The controller does not supply `/spec/suspend`: the Karta's `suspendDefinition` supplies it.

`IsSuspendable()` checks whether the Karta declares this capability. It does not mean the Job is already suspended or that the caller has permission to update it. Call `editor.GetResource()`, then save that resource as in the reconcile example. The Job controller then handles the changed setting.

</details>

<details>
<summary>Copy a label from the workload: Target, At, Read, Set</summary>

A controller wants Pods to carry `example.com/team` with the same value as their template's `team` label. Here the input comes from the CR, not from a user-supplied Go string.

Open a Deployment editor with `kartas.Deployment()`. Its template already has `metadata.labels: {team: ml, owner: alice}`. This helper copies the label:

```go
func copyTeam(ctx context.Context, editor tree.Editable) error {
    draft, err := tree.BeginEdit(ctx, editor)
    if err != nil {
        return err
    }
    defer draft.Abort()

    template, err := draft.Target(ctx, tree.Target{
        Component: "deployment", Field: tree.PodTemplateSpec,
    })
    if err != nil {
        return err
    }
    labels := template.At("metadata", "labels")
    team, exists, err := labels.At("team").Read()
    if err != nil {
        return err
    }
    if !exists {
        return nil
    }
    if err := labels.At("example.com/team").Set(team); err != nil {
        return err
    }
    return draft.Commit(ctx)
}
```

After saving, the template labels are `{team: ml, owner: alice, example.com/team: ml}`. If `team` is absent, the helper does nothing.

`Target` finds the template using the Karta definition. It returns a cursor: a handle pointing at that piece of raw JSON. `At` walks down through literal field names. `At("example.com/team")` selects one label key containing a slash; it does not look for a nested `example.com` object.

`Read()` returns the starting value and a separate `exists` flag. That flag distinguishes a missing field from a present field containing null. Editing a returned map would only edit a copy. Here `Set(team)` stages the write.

Why a draft here? The helper reads one field and writes another inside the selected template, while preserving all other raw fields. It commits only after both operations succeed.

</details>

<details>
<summary>Read into Go, or get the exact path: ReadInto and Path</summary>

In the KServe draft example, `model` is a cursor into the raw model object. Before committing that draft, a controller can inspect familiar Go fields:

```go
var view corev1.Container
if err := model.ReadInto(&view); err != nil {
    return err
}
fmt.Println(view.Image) // inference:v1

path, err := model.At("image").Path()
if err != nil {
    return err
}
fmt.Println(path) // /spec/predictor/model/image
```

`ReadInto` is useful for a decision such as checking the current image before choosing an update. It does not turn the cursor into a typed Container or write anything back. `storageUri` remains in the raw model even though `view` cannot hold it. Assigning `view.Image` alone does nothing to the draft; call `model.At("image").Set(newImage)` to write.

`Path` is useful for logging which field an edit targets or passing its location to another tool. The SDK handles escaping: the Deployment label cursor `template.At("metadata", "labels", "example.com/team")` returns `/spec/template/metadata/labels/example.com~1team`.

`Read` and `ReadInto` read the draft's starting values, even after `Set`. `Path` follows the current staged location, including moves of list items. Keep the cursor if more edits may move it; a saved path string does not follow those moves. All these calls must happen before `Commit` closes the draft.

</details>

<details>
<summary>Change one value, replace a map, or delete a field?</summary>

These are different requests. Suppose a Deployment template starts with labels `{team: ml, owner: alice}`. After selecting its template in a draft, use one of these choices:

To change only the team:

```go
if err := template.At("metadata", "labels", "team").Set("platform"); err != nil {
    return err
}
```

Result: `{team: platform, owner: alice}`. `Set` writes a string, number, boolean, or null. It does not accept a map/list or overwrite an existing map/list; use `Replace` for that.

To keep only the supplied labels:

```go
if err := template.At("metadata", "labels").Replace(map[string]any{"team": "platform"}); err != nil {
    return err
}
```

Result: `{team: platform}`. This removes `owner` because the supplied map is the entire desired labels object. Use it only when the controller owns the complete value.

To remove the owner label while keeping the team:

```go
if err := template.At("metadata", "labels", "owner").Remove(); err != nil {
    return err
}
```

Result: `{team: ml}`. Removing an already absent map key is harmless. Each choice needs `draft.Commit(ctx)` to reach the editor.

What about `Set(nil)`? It writes JSON null; it does not remove the key. For a custom CR field that permits null, `cursor.At("note").Set(nil)` changes `{"note":"temporary"}` to `{"note":null}`. `cursor.At("note").Remove()` gives `{}`. Do not use a null label value: Kubernetes labels require strings.

After replacing an object/list, previously selected child cursors are invalid. Make the intended child edits before replacing it, or supply the whole desired replacement value.

</details>

<details>
<summary>Work with list items: Items, Match, InsertBefore, MoveBefore</summary>

A controller needs to update one init container and put it after the others. For this Deployment, the starting list is:

```yaml
spec:
  template:
    spec:
      initContainers:
        - {name: verify, image: 'verify:v1'}
        - {name: download, image: 'download:v1'}
```

With an editor opened using `kartas.Deployment()`, begin a draft and select its template:

```go
draft, err := tree.BeginEdit(ctx, editor)
if err != nil {
    return err
}
defer draft.Abort()
template, err := draft.Target(ctx, tree.Target{
    Component: "deployment", Field: tree.PodTemplateSpec,
})
if err != nil {
    return err
}
initContainers := template.At("spec", "initContainers")
items, err := initContainers.Items()
if err != nil {
    return err
}
fmt.Println(len(items)) // 2 existing items

verify, err := initContainers.Match("name", "verify")
if err != nil {
    return err
}
if err := verify.At("image").Set("verify:v2"); err != nil {
    return err
}
if err := verify.MoveBefore(nil); err != nil {
    return err
}
path, err := verify.At("image").Path()
if err != nil {
    return err
}
fmt.Println(path) // /spec/template/spec/initContainers/1/image
```

`Items()` gives a handle for each starting item, useful when every init container needs inspection. `Match("name", "verify")` finds exactly one starting item by its name. A missing or duplicate name is an error; it never picks one silently.

The `verify` handle follows the same object when it moves. After committing, the order is `[download, verify]`, verify uses `verify:v2`, and its other fields stay. This matters for init containers because their order controls which runs first. A path containing `/0/` would point at the wrong item after the move.

To insert a new init container before verify, do this in the same draft before committing:

```go
if _, err := initContainers.InsertBefore(verify, map[string]any{
    "name": "prepare", "image": "prepare:v1",
}); err != nil {
    return err
}
```

The order becomes `[download, prepare, verify]`. Pass `nil` instead of `verify` to append at the end. Similarly, `verify.MoveBefore(items[1])` would move verify before the original download item. The values above illustrate the edits; actual init containers also need suitable images and commands.

`Items` and `Match` use the starting list. They do not include newly inserted items or match newly changed names. Supply the complete new object to `InsertBefore`. Its returned handle can be moved, removed, replaced, or asked for its path, but cannot select new child fields with `At` in that draft.

Finish with `draft.Commit(ctx)`, then read the editor for the final result. A draft uses explicit selections; it does not assume that every Kubernetes list is keyed by `name`.

</details>

<details>
<summary>What Commit and Abort protect, and when to retry</summary>

Use this pattern whenever a helper starts a draft:

```go
draft, err := tree.BeginEdit(ctx, editor)
if err != nil {
    return err
}
defer draft.Abort()

// Select targets and make edits, returning any error.
return draft.Commit(ctx)
```

If the helper returns early, `Abort` discards its pending edits. If `Commit` succeeds, the deferred `Abort` is harmless: it does not undo the commit. After either operation, the draft and its cursors are closed.

For example, if changing an image succeeds in the draft but selecting a second container fails, `Commit` cannot publish the first change. Any draft operation error makes the entire draft unusable, even if the caller ignores the error. Start a new draft to try again.

There are two separate kinds of conflict:

| What changed? | What detects it? | What the controller should do |
| --- | --- | --- |
| The local editor changed after `BeginEdit`, for example through `editor.Mutate` | `draft.Commit` returns `tree.ErrStaleDraft` | Begin a new draft from the current editor and recalculate the edit. |
| Another controller changed the CR in Kubernetes after it was fetched | The API update returns a resource-version conflict | Fetch the CR again, open a new editor, and recalculate the edit. |

`Commit` updates the editor's raw workload and extracted tree together. It does not contact Kubernetes or retry automatically. Only the controller's API call saves the result.

</details>

<details>
<summary>Existing and lower-level APIs</summary>

Existing `Component.Get*/Update*` calls remain, with policy set by `resource.WithMutationOptions`. Broad typed replacements can still lose unknown fields. `tree.Build` reads only. Lower-level callers can use `factory.PathWriter()` for `ResolveWriteTarget` / `WriteValues`, or `resource.Accessor.ApplyPatch` for an already-built patch. `ApplyPatch` does not evaluate CEL.

</details>

These calls change a local copy. The controller still saves `GetResource()` to Kubernetes and handles conflicts. Automatic scheduler discovery is not added.

The prototype uses `v1alpha1`. Releasing this breaking schema needs a new CRD version and migration; those are not implemented. GVK/scheduling renames and references are separate work. Checks cover unit tests, recorded workloads, and field preservation, not live-cluster migration.
