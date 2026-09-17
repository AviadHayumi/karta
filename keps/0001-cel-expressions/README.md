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

The old jq path did two jobs: read a value and locate it for writes. Now `expression` reads; `pathWrite` or `pathWriteExpression` locates the write. The patch and new value stay in Go.

This Ray Karta shows where the fields go. `(new)` marks fields or sections added to the jq CRD by this proposal. `suspendDefinition` already existed; its contents changed.

```yaml
apiVersion: run.ai/v1alpha1
kind: Karta
metadata:
  name: ray-example
spec:
  variables: # (new)
    - name: workerGroups
      expression: object.spec.workerGroupSpecs # (new)
  structureDefinition:
    rootComponent:
      name: raycluster
      kind: {group: ray.io, version: v1, kind: RayCluster}
      statusDefinition: {statusMappings: {}}
      suspendDefinition: # Existing field, changed contents.
        pathWrite: /spec/suspend # (new)
      fields: # (new)
        rayVersion:
          expression: object.spec.rayVersion # (new)
          pathWrite: /spec/rayVersion # (new)
    childComponents:
      - name: worker
        kind: {group: "", version: v1, kind: Pod}
        ownerRef: raycluster
        instanceIds: # (new) Replaces instanceIdPath.
          expression: variables.workerGroups.map(g, g.groupName) # (new)
        specDefinition:
          podTemplateSpec: # (new) Replaces podTemplateSpecPath.
            expression: variables.workerGroups.map(g, g.template) # (new)
            pathWriteExpression: >- # (new)
              "/spec/workerGroupSpecs/" + string(index) + "/template"
        podSelector:
          componentTypeSelector:
            expression: 'object.metadata.labels["ray.io/node-type"]' # (new)
            value: worker
          componentInstanceSelector:
            expression: 'object.metadata.labels["ray.io/group"]' # (new)
```

This example covers Ray workers, not the head Pod, and expects `rayVersion` and `workerGroupSpecs` in the workload. The custom `rayVersion` accessor illustrates `fields`; it is not in the shipped catalog.

| In the Karta | Why it exists | Example |
| --- | --- | --- |
| `expression` (new) | Read a workload value with CEL | Read a worker's template. |
| `pathWrite` (new) | Write at a fixed JSON path | `/spec/suspend` |
| `pathWriteExpression` (new) | Calculate a path from the workload | Find the template belonging to `gpu`. |
| `spec.variables` (new) | Reuse a CEL calculation | Give the worker list a shared name. |
| `component.fields` (new) | Expose an operator-specific setting | Ray's `rayVersion`, which has no built-in SDK field. |
| `instanceIds.expression` (new) | Read repeated component IDs; replaces `instanceIdPath` | `gpu` and `cpu` worker groups. |
| `suspendDefinition` (changed) | Existing suspend/resume support, now configured with a write path | Set `spec.suspend` to true/false. |

<details>
<summary>How the Ray definition fits together</summary>

`variables.workerGroups` is the workload's list. `instanceIds` reads its names; the template expression reads its templates. The SDK supplies `index` for the requested name, as explained below. The Pod selectors associate actual Pods with these worker groups; they are not write destinations.

Use `pathWrite` or `pathWriteExpression`, not both. Without either, the field is read-only. Nothing here supplies the desired version, scheduler, or other new value: the controller passes that through the SDK.

</details>

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

Most fields use a path such as `/spec/replicas`. An empty path is useful when the value starts at the top of the workload itself, as it does for a Pod. A field can also have a write path without a read expression. The examples below show both cases.

<details>
<summary>When would pathWrite be empty? The Pod catalog already uses it</summary>

A Deployment keeps its Pod template under `spec.template`, so its Karta writes to `/spec/template`. A Pod has no `spec.template`: its `metadata` and `spec` are already at the top.

The Pod catalog therefore defines the same SDK field like this:

```yaml
# Under spec.structureDefinition.rootComponent.
specDefinition:
  podTemplateSpec:
    expression: object
    pathWrite: ""
```

`expression: object` reads the whole Pod. `pathWrite: ""` says to apply writes starting at that outer object. The empty string is a real destination, not a missing setting.

For example, this Pod needs a team label:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: api
  labels: {app: api}
spec:
  containers:
    - {name: api, image: 'ghcr.io/example/api:v1'}
```

With that Pod loaded into `pod`, the controller can use the built-in definition:

```go
editor, err := tree.Open(ctx, kartas.Pod(), pod)
if err != nil {
    return err
}
if err := editor.Mutate(ctx, tree.Write{
    Component: "pod", Field: tree.PodTemplateSpec,
    Value: map[string]any{
        "metadata": map[string]any{
            "labels": map[string]any{"team": "platform"},
        },
    },
}); err != nil {
    return err
}
```

After the call, `editor.GetResource()` returns a Pod with labels `{app: api, team: platform}`. Its name, image, and other fields stay unchanged. Save that resource with the Kubernetes client as in the reconcile example.

Why was nothing removed if the path selects the whole Pod? The path chooses where the write starts; the SDK's default Merge decides how to apply the supplied value. This call merges one label into the Pod. Choosing Replace at this path would request replacement of the whole document, so a partial value like this is not appropriate for Replace.

Keep the quotes in `pathWrite: ""`. Leaving the write path out makes a field with only `expression` read-only. `pathWrite: "/"` is different too: it selects a field whose name is an empty string, not the whole document.

</details>

<details>
<summary>When can expression be omitted? Set Job parallelism from queue capacity</summary>

Suppose a queue controller has assigned four worker slots to a Job. It already knows the desired parallelism is `4`; it does not need to read the old number through this field first.

The Karta author can expose a write-only custom field:

```yaml
# Add to the Job Karta's spec.structureDefinition.rootComponent.
fields:
  parallelism:
    pathWrite: /spec/parallelism
```

Open the editor with that updated Job Karta, then write the queue's decision:

```go
workerSlots := 4
if err := editor.Mutate(ctx, tree.Write{
    Component: "job", Field: tree.Field("parallelism"), Value: workerSlots,
}); err != nil {
    return err
}
```

`spec.parallelism: 1` becomes `spec.parallelism: 4`. The Job's completions and Pod template stay unchanged. The value comes from `workerSlots`; the Karta supplies its destination.

Without `expression`, Karta does not include `parallelism` in the extracted custom fields. The controller can still write it. If the controller also needs Karta to read this field, add the expression:

```yaml
fields:
  parallelism:
    expression: 'object[?"spec"][?"parallelism"].orValue(1)'
    pathWrite: /spec/parallelism
```

"Write-only" describes this field's configuration, not a security boundary. The value is still present in the workload, and `ResolveWriteTarget` can return its current raw value. Omitting the read expression does not hide it or change Kubernetes permissions.

</details>

<details>
<summary>What does validation reject, and why?</summary>

With the proposed CRD installed, Kubernetes rejects this fixed path when the Karta is submitted:

```yaml
scaleDefinition:
  replicas:
    pathWrite: spec/replicas
```

It is missing the leading `/`. Use `/spec/replicas`. A fixed path is a JSON Pointer, not a jq path such as `.spec.replicas` or a CEL expression such as `object.spec.replicas`.

This is also rejected, even though both entries would point to the same place:

```yaml
scaleDefinition:
  replicas:
    pathWrite: /spec/replicas
    pathWriteExpression: '"/spec/replicas"'
```

One field must have one way to find its write destination. For this fixed location, keep `pathWrite` and remove `pathWriteExpression`. If the location depends on the workload, keep only `pathWriteExpression`; the next section shows KServe's example.

The CRD checks the fixed pointer's spelling and prevents both write-path forms on one field. It does not prove that the destination exists in a particular workload. The SDK checks a computed path after evaluating its expression; a valid-looking path can still fail at write time, for example if it points through a string instead of an object.

</details>

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

Read it with this workload fragment beside it:

```yaml
spec:
  predictor:
    minReplicas: 1
    model:
      image: inference:v1
      storageUri: s3://example-bucket/model
```

| Part of the query | What it does on this input |
| --- | --- |
| `object[?"spec"][?"predictor"].orValue(null)` | Reads the predictor object. Missing fields give null instead of a failed lookup. |
| `[dyn(...)].filter(v, type(v) == map)` | Keeps that value only if it is an object. `dyn` lets CEL check its type at runtime. |
| `+ [{}]` followed by `[0]` | Adds an empty-object fallback, then takes the first object. Missing predictor becomes `{}`. |
| `.map(k, string(k)).sort()` | Lists the object keys in a stable order: `["minReplicas", "model"]`. |
| `.filter(k, ...)` | Keeps object-valued entries with a non-null, non-false `storageUri`: `["model"]`. The number `minReplicas` is not a model. |

That last list is `variables.containerKeys`. The two accessors then do different jobs:

```text
Read expression       -> {image: inference:v1, storageUri: s3://example-bucket/model}
Write-path expression -> /spec/predictor/model
SDK Value             -> {image: inference:v2}, supplied later by the controller
```

`condition ? a : b` means "use a when the condition is true; otherwise use b." One match reads that model and returns its path. No matches give null, so there is no write target. Several matches do not pick the first: the read produces a list that the singular Container reader rejects, and the write path is null.

The URI check selects candidates; it does not validate a URI. For compatibility with the old catalog, an empty string still counts, while null and false do not.

The two `replace` calls encode a key for a JSON Pointer. For a key named `example.com/model`, the path ends in `example.com~1model`. The key in the workload does not change. Ordinary keys such as `model` stay exactly the same.

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

KServe stores the model's location in `storageUri`. Kubernetes' `corev1.Container` has no such field, and Karta has no built-in `tree.StorageUri` constant. A model rollout controller still needs a way to change it by name.

Extend the KServe catalog's predictor component with this named field. This is a proposed catalog extension using a real KServe field, not a field the shipped catalog already exposes. It reuses `containerKeys` from the KServe example above:

```yaml
# Under spec.structureDefinition.childComponents[name=predictor].
fields:
  storageUri:
    expression: >-
      variables.containerKeys.size() == 1 ?
      object.spec.predictor[variables.containerKeys[0]].storageUri : dyn(null)
    pathWriteExpression: >-
      variables.containerKeys.size() == 1 ?
      "/spec/predictor/" +
      variables.containerKeys[0].replace("~", "~0").replace("/", "~1") +
      "/storageUri" : dyn(null)
```

Open the workload with this extended Karta, then write the new model location:

```go
if err := editor.Mutate(ctx, tree.Write{
    Component: "predictor", Field: tree.Field("storageUri"),
    Value: "s3://example-bucket/model-v2",
}); err != nil {
    return err
}
```

| Model field | Before | After |
| --- | --- | --- |
| `storageUri` | `s3://example-bucket/model-v1` | `s3://example-bucket/model-v2` |
| `image` | `inference:v1` | `inference:v1` |
| `modelFormat` | `{name: sklearn}` | `{name: sklearn}` |

The same call works whether KServe keeps this model under `model` or `sklearn`. The Karta author handles that difference once. The controller names the setting; it does not need the JSON path or a new SDK release.

Because this accessor also has `expression`, the extracted predictor instance's `Fields["storageUri"]` contains its current value. Without that read expression, it would be write-only through this named field.

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
      pathWriteExpression: >-
        "/spec/workerGroupSpecs/" + string(index) + "/template"
```

Read `.map(g, g.groupName)` as "for each group `g`, take its `groupName`." On this workload, it returns `["gpu", "cpu"]`. These are the names already in the RayCluster. Karta does not create them.

The template expression reads each group's template in the same order. `index` is not a field in the RayCluster and the controller does not pass it. The SDK supplies it:

```text
Controller asks for Instance: "gpu"
  -> SDK evaluates instanceIds: ["gpu", "cpu"]
  -> SDK finds "gpu" at position 0 (counting starts at zero)
  -> CEL receives instance = "gpu", index = 0
  -> pathWriteExpression returns /spec/workerGroupSpecs/0/template
```

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

<details>
<summary>Why are these names not all enums?</summary>

`tree.PodTemplateSpec` already is a typed constant. The other names belong to the Karta or workload, not to a fixed list in Go:

| SDK field | Type | Why |
| --- | --- | --- |
| `Component` | `string` | The Karta author chose `worker`; another Karta can choose `trainer`. |
| `Instance` | `string` | A user can create a Ray group called `gpu`, `cpu`, or `nightly`. |
| `Field` | `tree.Field`, a string type | Use built-in constants or a custom name such as `tree.Field("storageUri")`. |
| `Value` | `any` | The desired value can be a string, number, map, or list. Here it is part of a Pod template. |

A closed enum would need an SDK release for every new component, group name, or custom field. Unknown component, instance, and field names still return errors; using strings does not mean every name is accepted.

`"spec"` and `"schedulerName"` inside `Value` are actual PodTemplateSpec JSON keys. Karta finds the template, but this call still needs to know the shape of the value it sends. It does not discover a scheduler field universally across all catalogs.

</details>

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

| Bad input or definition | Where it is caught |
| --- | --- |
| Both write-path forms, or a malformed fixed pointer | Karta CRD validation and the Go definition validator. |
| IDs such as `["gpu", "gpu"]`, `[""]`, `[7]`, or null | SDK instance extraction rejects them. It does not turn `7` into a name. |
| Two IDs but only one extracted template, or a template of the wrong type | SDK extraction / tree construction returns an error. |
| A write changes valid data into invalid extracted data | The editor re-extracts before publishing; a failed `Mutate` or draft commit leaves the editor unchanged. |
| Two valid string IDs sorted differently from their two templates | Not automatically detectable. Both lists look valid; the Karta author must keep them aligned. |

There is no `karta validate` CLI command today. Definition validation also cannot inspect a future RayCluster or prove what arbitrary CEL will return. CEL compilation/evaluation and the data checks above happen in the SDK; the workload's own CRD checks its schema when Kubernetes admits it.

For example, the Ray catalog treats a missing or non-list worker list as an empty list. Once its expression returns `[]`, the SDK cannot tell which input produced it. The Ray CRD, or a stricter catalog expression, must reject an incorrectly shaped source list. These checks are not a replacement for the workload's schema.

The tree may sort names for display. `index` uses the ID expression's order, not the display order. In the Ray catalog that is also the workload array order. If an author sorts or filters the IDs, the write expression must find the original array position by `instance` instead of assuming `index` still matches it. A missing requested ID returns an error.

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

### Start with a Deployment controller

A platform wants this Deployment to use `batch-scheduler`. The only workload change should be:

```yaml
# Before: spec.template.spec
schedulerName: default-scheduler
containers: [{name: api, image: 'api:v1'}]

---
# After: spec.template.spec
schedulerName: batch-scheduler
containers: [{name: api, image: 'api:v1'}]
```

Karta locates the template. The controller supplies just its new scheduler setting. The built-in Deployment Karta already declares:

```yaml
specDefinition:
  podTemplateSpec:
    expression: 'object[?"spec"][?"template"].orValue(null)'
    pathWrite: /spec/template
```

<details>
<summary>The reconcile: fetch, edit with Karta, save</summary>

`SchedulerReconciler` embeds `client.Client`. Its `DesiredScheduler` setting is `"batch-scheduler"`. Imports and watch setup are omitted; these are the calls that use Karta:

```go
func (r *SchedulerReconciler) Reconcile(
    ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
    workload := &appsv1.Deployment{}
    if err := r.Get(ctx, req.NamespacedName, workload); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    if workload.Spec.Template.Spec.SchedulerName == r.DesiredScheduler {
        return ctrl.Result{}, nil
    }
    workload.SetGroupVersionKind(
        appsv1.SchemeGroupVersion.WithKind("Deployment"),
    )

    editor, err := tree.Open(ctx, kartas.Deployment(), workload)
    if err != nil {
        return ctrl.Result{}, err
    }
    if err := editor.Mutate(ctx, tree.Write{
        Component: "deployment", Field: tree.PodTemplateSpec,
        Value: map[string]any{
            "spec": map[string]any{"schedulerName": r.DesiredScheduler},
        },
    }); err != nil {
        return ctrl.Result{}, err
    }
    updated, err := editor.GetResource()
    if err != nil {
        return ctrl.Result{}, err
    }
    return ctrl.Result{}, r.Update(ctx, updated)
}
```

`Open` reads the workload using the catalog. `Mutate` merges the supplied setting into its local template. `GetResource` returns the full changed Deployment. Only `r.Update` sends it to Kubernetes.

The early comparison prevents an update on every reconcile. Setting the kind supplies the identity Karta needs: a typed client fetch can leave `apiVersion` and `kind` empty.

If Kubernetes rejects an outdated resource version, returning the error lets controller-runtime retry. The next reconcile fetches again and opens a new editor. Do not reuse the old editor across retries.

</details>

<details>
<summary>How this same update worked with jq</summary>

The old Karta used one jq path for reading and writing:

```yaml
specDefinition:
  podTemplateSpecPath: .spec.template
```

The controller read complete Go templates, changed them, and sent them back. This helper used the historical API:

```go
func setSchedulerJQ(
    ctx context.Context, workload resource.KubernetesObject, scheduler string,
) (resource.KubernetesObject, error) {
    factory := resource.NewComponentFactoryFromObject(
        kartas.Deployment(), workload,
    )
    component, err := factory.GetRootComponent()
    if err != nil {
        return nil, err
    }
    templates, err := component.GetPodTemplateSpec(ctx)
    if err != nil {
        return nil, err
    }
    for id, template := range templates {
        template.Spec.SchedulerName = scheduler
        templates[id] = template
    }
    if err := component.UpdatePodTemplateSpec(ctx, templates); err != nil {
        return nil, err
    }
    return factory.GetResource()
}
```

The old extracted tree was read-only; writes went through `resource.Component`. The new editor combines the local workload and its tree, and accepts a partial value:

```go
err := editor.Mutate(ctx, tree.Write{
    Component: "deployment", Field: tree.PodTemplateSpec,
    Value: map[string]any{"spec": map[string]any{"schedulerName": scheduler}},
})
```

Both examples produce the same scheduler change on this Deployment. The difference is what the controller sends back: the old call resends the Go template; the new call can send only `schedulerName`. Neither version saves to Kubernetes until the controller calls its client.

</details>

<details>
<summary>Editor, draft, and why there are two Snapshot calls</summary>

Most updates need neither snapshot. The Deployment controller above already knows what to write. A snapshot is useful when the controller needs to inspect the tree before deciding.

`editor` is the local workload being edited. A `draft` holds changes that have not reached that editor yet. A `Snapshot` is just a detached read view, not another editor.

For example, a controller can use the editor's tree to inspect a Deployment's current image:

```go
view := editor.Snapshot()
template := view.Root.Instances[0].ExtractedInstance.PodTemplateSpec
fmt.Println(template.Spec.Containers[0].Image) // api:v1
```

The tree gives the controller Go values to inspect. It is not the original CR: for example, a Deployment's `spec.template` appears as `PodTemplateSpec`. Changing `template.Spec.Containers[0].Image` here only changes this returned copy. To write, use `Mutate` or a draft.

Why would a draft need its own snapshot? Suppose a rollout should change every container starting on `api:v1`, but leave the `metrics:v1` sidecar alone. A helper receiving only `*tree.Draft` can inspect its starting tree, choose the matching containers, and stage their image edits. It does not need the editor passed in as a second argument.

<details>
<summary>Example: choose containers by their starting image</summary>

Inside an open Deployment draft with one extracted Pod template, this is the selection and edit loop. The caller commits after the helper succeeds:

```go
start := draft.Snapshot()
if start == nil {
    return tree.ErrDraftClosed
}
pod := start.Root.Instances[0].ExtractedInstance.PodTemplateSpec
template, err := draft.Target(ctx, tree.Target{
    Component: "deployment", Field: tree.PodTemplateSpec,
})
if err != nil {
    return err
}
containers := template.At("spec", "containers")
for _, original := range pod.Spec.Containers {
    if original.Image != "api:v1" {
        continue
    }
    item, err := containers.Match("name", original.Name)
    if err != nil {
        return err
    }
    if err := item.At("image").Set("api:v2"); err != nil {
        return err
    }
}
```

Before: `api: api:v1`, `metrics: metrics:v1`. After commit: `api: api:v2`, `metrics: metrics:v1`. The typed snapshot helped make the decision; the cursor changes only the chosen image in raw JSON.

</details>

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

Use the editor snapshot to inspect the current result, or the draft snapshot to make decisions from the starting tree. Calling both is not a required sequence.

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

`corev1.Container` is Kubernetes' Go struct for a Pod container. It knows fields such as `Name`, `Image`, and `Resources`. KServe's model object also has `storageUri` and `modelFormat`, which that struct cannot hold:

```text
Raw KServe model (map[string]any)       Read as corev1.Container
image: inference:v1                    Image: "inference:v1"
storageUri: s3://example-bucket/model   No Go field for this
modelFormat: {name: sklearn}           No Go field for this
```

Reading this typed view does not delete anything. The loss happens if the controller turns that smaller struct back into JSON and replaces the entire model with it. This was possible with the old jq typed-write flow too.

The new call supplies a `map[string]any` containing only `{"image": "inference:v2"}`. Default Merge leaves the other raw keys alone. A draft's `*tree.Cursor` can also edit just `image`. Changing jq to CEL, or choosing JSONPatch while still replacing the whole model, would not fix the typed-write problem.

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
<summary>Write options: what to keep, how to patch, and missing parents</summary>

These answer three separate questions. They are SDK options, not new Karta CR fields:

| Option | Question | Default for Mutate |
| --- | --- | --- |
| `Strategy` | Keep unmentioned map keys, or replace the selected value? | `resource.Merge` |
| `PatchType` | Which patch rules should the SDK apply? | `resource.PatchTypeMergePatch` |
| `Parents` | May the SDK create missing containing objects? | `resource.CreateMapParents` |

For example, replace all predictor labels with the controller's complete desired map:

```go
if err := editor.Mutate(ctx, tree.Write{
    Component: "predictor", Field: tree.LabelsField,
    Value: map[string]any{"team": "platform"},
    Options: resource.MutationOptions{
        Strategy:  resource.Replace,
        PatchType: resource.PatchTypeJSONPatch,
        Parents:   resource.RequireParents,
    },
}); err != nil {
    return err
}
```

<details>
<summary>Merge or Replace: does the controller own one label or all labels?</summary>

Before, the KServe CR contains:

```yaml
spec:
  predictor:
    labels: {team: ml, owner: alice}
```

The controller supplies `Value: map[string]any{"team": "platform"}`.

| Strategy | After at spec.predictor.labels | Why choose it? |
| --- | --- | --- |
| `Merge` | `{team: platform, owner: alice}` | This controller changes team; it should not remove someone else's owner label. |
| `Replace` | `{team: platform}` | This controller owns the complete labels map; anything it omitted should disappear. |

Both patch formats support these strategies in the Karta SDK. JSONPatch itself has no recursive Merge setting: Karta computes the merged value before generating its operations.

Lists are different. Starting with containers `[api, metrics]`, supplying `[api]` replaces the list under either strategy. Neither option guesses that containers should merge by name. To edit one existing container and keep its siblings, use a draft and `Match("name", "api")`.

</details>

<details>
<summary>MergePatch or JSONPatch: deleting a field versus storing null</summary>

For an ordinary image string or label update, either format works. MergePatch describes a partial object. JSONPatch describes operations on paths. The SDK applies these rules to the local workload; CEL only finds the destination. This does not send an API patch request.

For the team-label Merge above, the intent can be represented as:

```json
{"spec":{"predictor":{"labels":{"team":"platform"}}}}
```

Or as an operation list (assuming labels already exists):

```json
[{"op":"add","path":"/spec/predictor/labels/team","value":"platform"}]
```

Those are equivalent patch examples, not a promise that the SDK emits exactly those bytes.

One meaningful difference is null. Suppose a custom workload permits a nullable `note` field, and its Karta exposes the containing settings object:

```yaml
fields:
  settings:
    pathWrite: /spec/settings
```

Before: `spec.settings: {note: temporary, owner: alice}`. The SDK input is `Value: map[string]any{"note": nil}`, with strategy `Merge`:

| Format | After at spec.settings | Meaning |
| --- | --- | --- |
| `PatchTypeMergePatch` | `{owner: alice}` | A null entry means delete this key. |
| `PatchTypeJSONPatch` | `{note: null, owner: alice}` | Store an actual null while keeping the key. |

Do not try the null example on Kubernetes labels: their values must be strings. A draft makes this distinction explicit with `Remove()` versus `Set(nil)`.

JSONPatch can express both deletion and assignment; MergePatch is not needed for an operation JSONPatch cannot express. Keeping both gives callers the two formats' familiar semantics, especially partial objects and null-as-delete.

</details>

<details>
<summary>Parent policy: what if nodeSelector does not exist yet?</summary>

Suppose a platform exposes a named region setting on a Deployment. This is an example extension to its Karta, not a built-in catalog field:

```yaml
# Under the Deployment Karta's rootComponent.
fields:
  region:
    pathWrite: /spec/template/spec/nodeSelector/region
```

The controller calls `Mutate` with `Component: "deployment", Field: tree.Field("region"), Value: "west"`. The Karta supplies the path, but the workload has no `nodeSelector`:

```yaml
# Before: spec.template.spec
containers: [{name: api, image: 'api:v1'}]
```

`nodeSelector` is the parent object that must hold `region`.

| Parents option | Result |
| --- | --- |
| `RequireParents` | Error; the editor stays unchanged. Useful when an absent object means the controller selected the wrong shape. |
| `CreateMapParents` | Creates `nodeSelector` and writes region. Useful for an optional map that has never been set. |

After the successful write:

```yaml
# After: spec.template.spec
containers: [{name: api, image: 'api:v1'}]
nodeSelector: {region: west}
```

An absent leaf is fine with either option if its parents exist. Neither option creates a missing array or turns a string into an object.

Drafts are stricter by default: `BeginEdit(ctx, editor)` requires parents. To allow missing map parents for `template.At("spec", "nodeSelector", "region").Set("west")`, begin with:

```go
draft, err := tree.BeginEdit(
    ctx, editor, tree.WithEditParents(resource.CreateMapParents),
)
```

Existing null is a separate case in this implementation. `Mutate` with CreateMapParents treats a null parent as an object to create. A draft still rejects an existing null parent, even with this option; it only creates absent maps.

If replacing `nodeSelector: null` is intended, replace that entire value explicitly:

```go
if err := template.At("spec", "nodeSelector").Replace(
    map[string]any{"region": "west"},
); err != nil {
    return err
}
```

This stages `nodeSelector: {region: west}`. Do not first replace it with `{}` and then try selecting children: draft selections still use the starting value.

</details>

</details>

<details>
<summary>When a partial value is not enough: edit with a draft</summary>

Suppose an edit needs to copy an existing label, find the container named `main`, or move an init container without rebuilding the list. Sending a new map alone does not describe those selections. A draft lets the controller select raw values, read them, and stage several edits before committing them together.

For an image-only update, `Mutate` above is enough. Here is the smallest draft example so the three steps are visible: begin, select and edit, commit.

```go
func setImageWithDraft(
    ctx context.Context, editor tree.Editable, image string,
) error {
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

`tree.BeginEdit` returns `*tree.Draft`, a pending edit of this editor. `draft.Target` returns `*tree.Cursor`, a handle to the model selected by the Karta. `model.At("image")` selects its image; `Set` stages the new string. No caller needs to know whether KServe uses `model` or `sklearn`.

With `image = "inference:v2"`:

```yaml
# Before at spec.predictor.model
image: inference:v1
storageUri: s3://example-bucket/model
modelFormat: {name: sklearn}

---
# After draft.Commit(ctx), in the local editor
image: inference:v2
storageUri: s3://example-bucket/model
modelFormat: {name: sklearn}
```

The cursor is not a `corev1.Container`. It keeps the raw JSON, including fields a Go Container cannot represent. Only its selected image changes.

Yes, `defer draft.Abort()` is intentional. If a later step returns an error, it discards pending work. After a successful `Commit`, it is a harmless no-op; it does not undo the image change.

![The Karta finds the model; the draft changes image and keeps model data](accessor-model.png)

`Commit` updates the editor's local CR and tree together. If an edit fails, none of the draft is published. If the editor changed after `BeginEdit`, start a new draft rather than overwrite newer work.

In a reconcile, call this helper instead of `editor.Mutate`, check its error, then use the same `GetResource` and client save steps. Committing a draft still does not send a Kubernetes request.

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
