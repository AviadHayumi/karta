<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta technical guide (cheatsheet)

A condensed field reference for authoring a Karta definition. It matches the API
types in `pkg/api/runai/v1alpha1/` and the validator in
`pkg/api/runai/v1alpha1/validation.go`. The prose reference is
`docs/Technical Guide.md`.

## Top-level shape

```yaml
apiVersion: run.ai/v1alpha1
kind: Karta
metadata:
  name: <lower-case-name>
spec:
  structureDefinition:
    rootComponent: {}          # required
    childComponents: []        # optional
    additionalChildKinds: []   # optional
  optimizationInstructions: {} # optional
  variables: []                # optional named expressions
```

## Expressions and value accessors

Every expression is CEL, evaluated with the resource bound as `object`.
Expressions in `specDefinition`, `scaleDefinition`, `statusDefinition`, and
`instanceIds` run against the workload object. Expressions in `podSelector` and
`optimizationInstructions` run against pod manifests.

A field is read and written through a value accessor pair:

```yaml
podTemplateSpec:
  expression: object[?"spec"][?"template"].orValue(null)   # read
  patch: '{"spec": {"template": value}}'                   # write
  replace: true
```

- `expression` reads the value from `object`.
- `patch` is a CEL expression that constructs the change. A map is applied as a
  JSON merge patch: maps merge recursively, any other value replaces, and `null`
  deletes the field. A list is applied as an RFC 6902 operation list, for
  example `[{"op": "add", "path": "/spec/template", "value": value}]`.
- Inside a `patch`, these names are bound: `value` (the new value), `instance`
  and `index` (the instance id and its position, for instanced components), and
  `variables.<name>`.
- `replace: true` makes the write replace the field instead of merging into it:
  the patch is applied first with `value` bound to `null` (deleting the field)
  and then with the real value. A pod template update wants this; an annotations
  update usually does not.

An accessor with only an `expression` is read-only. A write through it fails at
runtime with `the field has no patch and cannot be written`. The catalog does
this deliberately where no sane write exists, for example the Dynamo `container`
and `image` accessors in
`docs/catalog/nvidia-com-dynamographdeployment-v1beta1.yaml`. Leave a patch out
only with that intent.

### Null safety

Use optional selection instead of plain field access so a missing field yields
a value rather than an evaluation error:

- `object[?"spec"][?"template"].orValue(null)` selects optionally at each step
  (`object.?spec.?template` is equivalent) and unwraps with a default.
- To coalesce a possibly missing value to a default, use the filter idiom:
  `([dyn(object[?"spec"][?"replicas"].orValue(null))].filter(v, v != null && v != false) + [1])[0]`.
  It wraps the value in a single-item list, drops it when it is null, appends
  the default, and takes the first element.

Evaluation is budgeted: an expression whose cost explodes is stopped with an
evaluation error instead of running unbounded. A normal definition never
notices the budget. Optional types plus the standard list and string extensions
are available; there is no other builtin surface to learn.

### Variables

`spec.variables` are named CEL expressions, available to every expression in
the definition as `variables.<name>`. They are evaluated in order, and a later
variable may reference an earlier one. Name a shared sub-expression once
instead of repeating it.

```yaml
spec:
  variables:
  - name: specReplicas
    expression: ([dyn(object[?"spec"][?"replicas"].orValue(null))].filter(v, v != null && v != false) + [1])[0]
```

```yaml
scaleDefinition:
  replicas:
    expression: variables.specReplicas
```

## Component model

A component is one node in the workload tree.

- Root component: exactly one. Requires a full GVK and a `statusDefinition`. Must
  not have an `ownerRef`.
- Child component: requires an `ownerRef` naming another component (root or
  another child). Owner chains must reach the root with no cycles.
- Component `name` is a free-form identifier, unique within the Karta.
- `kind` is the full GVK. All of group, version, and kind are required. The core
  `Pod` kind is the only kind allowed to omit the group (use `group: ""`).

Virtual components. A component may omit `kind` and `specDefinition` entirely and
exist only to model a level of the tree. Use one when the workload has a grouping
level that owns other components but is not itself a Kubernetes object, and give
it a `scaleDefinition` for the level's count and a `replicaSelector` for the
label that identifies which group a pod belongs to. LeaderWorkerSet is the
canonical case: a `group` component sits between the root and the `leader` and
`worker` components and owns both roles. Without it, `leader` and `worker` have
no shared grouping level and per-group identity is lost.

```yaml
- name: group
  ownerRef: leaderworkerset
  scaleDefinition:
    replicas:
      expression: object[?"spec"][?"leaderWorkerTemplate"][?"size"].orValue(null)
  podSelector:
    replicaSelector:
      expression: object[?"metadata"][?"labels"][?"leaderworkerset.sigs.k8s.io/group-index"].orValue(null)
```

Fields available on a component (`ComponentDefinition`):

| Field | Purpose |
|---|---|
| `name` | Unique identifier. Required. |
| `kind` | Full GVK. Required on root; recommended on children. |
| `ownerRef` | Parent component name. Required on children, forbidden on root. |
| `specDefinition` | Where the pod template lives. |
| `scaleDefinition` | Where replica counts live. |
| `statusDefinition` | Status mapping. Required on root. |
| `suspendDefinition` | Native suspend/resume patches. |
| `instanceIds` | Accessor returning the list of instance ids for multi-instance components. |
| `podSelector` | How pods map to this component and its instances. |

## Spec definitions (mutually exclusive)

Set exactly one of these three per component. Setting more than one fails
validation with `has multiple pod spec definitions`.

| Pattern | Use when | Example read |
|---|---|---|
| `podTemplateSpec` | CRD embeds a full PodTemplateSpec | `object[?"spec"][?"template"].orValue(null)` |
| `podSpec` (+ `metadata`) | CRD embeds a bare PodSpec, metadata separate | `object[?"spec"][?"transformer"].orValue(null)` |
| `fragmentedPodSpecDefinition` | Pod fields scattered across the spec | see below |

Choosing `fragmentedPodSpecDefinition` is not only about a missing pod template.
A CRD can embed a real pod spec and still need fragmented accessors, because the
fields Karta treats as part of the pod live at different levels. Grove
PodCliqueSet is the example: containers and scheduler name are inside each
clique's `spec.podSpec`, but labels and annotations sit one level up on the
clique itself. A single `podSpec` accessor would read the spec and silently drop
the labels and annotations. Check where every field lives, not just the
containers.

`fragmentedPodSpecDefinition` fields (all optional; set only those that exist):
`schedulerName`, `labels`, `annotations`, `resources`, `resourceClaims`,
`podAffinity`, `nodeAffinity`, `containers`, `container` (single container),
`priorityClassName`, `image`. Each is a value accessor.

```yaml
specDefinition:
  fragmentedPodSpecDefinition:
    labels:
      expression: object[?"spec"][?"components"][?"standalone"][?"podLabels"].orValue(null)
      patch: '{"spec": {"components": {"standalone": {"podLabels": value}}}}'
      replace: true
    resources:
      expression: object[?"spec"][?"components"][?"standalone"][?"resources"].orValue(null)
      patch: '{"spec": {"components": {"standalone": {"resources": value}}}}'
      replace: true
```

Write shape follows the field's location. A field at a fixed path writes with a
merge-patch map, as above. A field inside an array of instance specs writes with
an RFC 6902 patch built from `index`, and a field inside a map of instance specs
writes with a merge patch keyed by `instance`:

```yaml
# array of specs: per-index RFC 6902 write (Grove cliques)
labels:
  expression: ([dyn(object[?"spec"][?"template"][?"cliques"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"labels"].orValue(null))
  patch: '[{"op": "add", "path": "/spec/template/cliques/" + string(index) + "/labels", "value": value}]'
```

```yaml
# map of specs: per-instance merge write
labels:
  patch: '{"spec": {"services": {instance: {"labels": value}}}}'
  replace: true
```

## Status definition

Required on the root component. Optional on children. Structure:

```yaml
statusDefinition:
  conditionsDefinition:      # needed only if any rule uses byConditions
    expression: object[?"status"][?"conditions"].orValue(null)
    typeFieldName: type      # defaults: type / status / message / reason
    statusFieldName: status
    reasonFieldName: reason
    messageFieldName: message
  phaseDefinition:           # needed only if any rule uses byPhase
    expression: object[?"status"][?"phase"].orValue(null)
  statusMappings:            # required
    running:
    - byConditions:
      - type: Ready
        status: "True"
```

Normalized statuses (the `ResourceStatus` enum): `Initializing`, `Running`,
`Completed`, `Failed`, `Degraded`, `Suspended`, `Suspending`, `Resuming`.
`Undefined` is the implicit result when no rule matches; do not map to it.

Matcher semantics (`StatusMatcher`):

- `byConditions`: a list of expected conditions, all of which must hold (AND).
  Each entry sets `type` plus at least one of `status` or `reason`.
- `byPhase`: matches a single phase string from `phaseDefinition`.
- `byExpression`: a CEL `expression` plus an `expectedResult` string. Use it
  when the state lives in status fields (for example replica counts) rather
  than conditions or a phase.
- Rules under one status are OR'd: any matching rule resolves the status.
- Several statuses can match at once. Map only what the workload reports.

One matcher may combine kinds. A single `StatusMatcher` can set more than one of
`byPhase`, `byConditions`, and `byExpression` at once, and then all of them must
hold (AND). Use this when a status needs both a phase and an extra field check.
This is distinct from listing separate rules under a status, which are OR'd.

Not every controller has a phase or conditions. Some report only replica counts
or other status fields (for example Grove PodCliqueSet has no aggregate phase).
Do not invent a phase or a condition type the controller never sets: that
produces a definition that validates but never resolves. Match such states with
`byExpression` over the real status fields, for example
`variables.statusAvailableReplicas >= variables.specReplicas` for running.

Example combining expression and condition rules (from `batch-job-v1.yaml`):

```yaml
statusMappings:
  running:
  - byExpression:
      expression: variables.statusActive > 0 && variables.statusReady > 0
      expectedResult: "true"
  completed:
  - byConditions:
    - type: Complete
      status: "True"
```

## Scale definition

```yaml
scaleDefinition:
  replicas:
    expression: variables.specParallelism
  minReplicas:
    expression: object[?"spec"][?"predictor"][?"minReplicas"].orValue(null)
  maxReplicas:
    expression: object[?"spec"][?"predictor"][?"maxReplicas"].orValue(null)
```

All three accessors are optional. Keep them null-safe.

A component's replica count is the number of units at that component's level of
the tree, counted across the whole workload. It is not the number of API objects
of the component's `kind`. The distinction matters because a component's `kind`
often names the controller object that produces the pods rather than the pods
themselves. In LeaderWorkerSet the `leader` component has kind `StatefulSet` and
reads `spec.replicas`, which for three groups resolves to 3, even though the
operator creates a single leader StatefulSet. The count describes the level, not
the object.

Two numbers, two levels. A grouped or replicated workload usually holds both a
group count and a members-per-group count, and picking the wrong one is a valid
expression that returns the wrong number, so the validator cannot catch it.
LeaderWorkerSet is the trap: `spec.replicas` is the number of groups and
`spec.leaderWorkerTemplate.size` is pods per group. The `leader` component
scales on `variables.specReplicas` (one leader per group) and `worker` on the
derived
`variables.specReplicasFloat * (variables.specLeaderWorkerTemplateSize - 1.0)`.
A nested level multiplies by its parent's count the same way: JobSet's
`replicatedjob` uses
`([dyn(object[?"spec"][?"replicatedJobs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(j, j.replicas * j.template.spec.parallelism)`.

For a multi-instance component, `replicas` returns a list aligned with the
`instanceIds` order, one count per instance. RayCluster's `worker` maps each
worker group to its own `replicas`, `minReplicas`, and `maxReplicas`.

Two self-checks. Sibling components that model the same level should resolve to
the same count, and the numbers should add up against a real manifest: if the CR
declares 3 groups of 4, the components should report 3, 3, and 9, not 4. The
`hack/karta-verify` README walks the LeaderWorkerSet quickstart manifest through
exactly this sibling check.

Look for autoscaling bounds explicitly. `minReplicas` and `maxReplicas` are easy
to miss because they usually live somewhere other than the replica field itself,
for example KServe's `spec.predictor.minReplicas` or Grove's per-clique
`spec.autoScalingConfig.minReplicas`. Search the CRD for an autoscaling or
elastic policy block before deciding the workload has none.

## Suspend definition

For workloads with native suspend support (for example `spec.suspend` on a
Job). Both action lists require at least one entry. Each action is a `patch`
expression merged into the workload; actions are applied in order.

```yaml
suspendDefinition:
  suspendActions:
  - patch: '{"spec": {"suspend": true}}'
  resumeActions:
  - patch: '{"spec": {"suspend": false}}'
```

## Pod selectors (expressions run against pod manifests)

```yaml
podSelector:
  componentTypeSelector:        # maps a pod to this component type
    expression: object[?"metadata"][?"labels"][?"training.kubeflow.org/replica-type"].orValue(null)
    value: worker               # optional; if omitted, only a non-null result is checked
  componentInstanceSelector:    # splits one component into named instances
    expression: object[?"metadata"][?"labels"][?"ray.io/group"].orValue(null)
  replicaSelector:              # distinguishes replicas of the same sub-structure
    expression: object[?"metadata"][?"labels"][?"leaderworkerset.sigs.k8s.io/group-index"].orValue(null)
```

`componentInstanceSelector` must pair with a component-level `instanceIds`
accessor, and vice versa. Selectors of the same kind must be mutually exclusive
across components. Descendant components inherit the replica context from their
parent, so define `replicaSelector` only where replicas are created.

Role-label keys are framework-specific, and differ even between operators from
the same project. Do not copy a selector key from the nearest sample without
checking the target controller's real pod labels. For example the Kubeflow
training-operator (PyTorchJob, TFJob) labels role with
`training.kubeflow.org/replica-type` (values `master`, `worker`), while the
Kubeflow mpi-operator (MPIJob v2beta1) labels role with
`training.kubeflow.org/job-role` (values `launcher`, `worker`). Read the actual
pod labels the controller sets before writing the selector.

Disambiguating roles that share a label. When two components would match the
same pod label, a plain value match is not mutually exclusive. Separate them by
matching on a key that only one role carries, using key existence (omit `value`).
LeaderWorkerSet is the canonical case: both leader and worker pods carry
`leaderworkerset.sigs.k8s.io/worker-index`, so the leader is matched by that
label with `value: "0"`, and the worker is matched by the existence of the
`leaderworkerset.sigs.k8s.io/leader-name` annotation, which only worker pods
have.

```yaml
# leader: value match on the shared label
componentTypeSelector:
  expression: object[?"metadata"][?"labels"][?"leaderworkerset.sigs.k8s.io/worker-index"].orValue(null)
  value: "0"
# worker: key existence of a role-specific annotation (no value)
componentTypeSelector:
  expression: object[?"metadata"][?"annotations"][?"leaderworkerset.sigs.k8s.io/leader-name"].orValue(null)
```

## Multi-instance components

When one component holds several specs (an array or a map), give it an
`instanceIds` accessor returning the list of instance ids, and a matching
`componentInstanceSelector` on the pod side. Every other accessor of the
component then returns a list aligned with that order, and its `patch` receives
`instance` and `index`.

```yaml
# array of specs (each entry carries its own name)
instanceIds:
  expression: ([dyn(object[?"spec"][?"workerGroupSpecs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"groupName"].orValue(null))
# map of specs (the map keys are the instance ids, sorted for a stable order)
instanceIds:
  expression: ([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort()
```

## Additional child kinds

List GVKs the workload creates or manages that are not modeled as components.
Used for RBAC. Avoid duplicating a kind already declared as a component, unless
the kind must also be listed here for RBAC or owner traversal (the validator does
not reject it).

Component or additional kind. Model a kind as a component when something needs to
be read from it or written to it: a pod template, a replica count, a status, or a
selector that maps pods to it. Otherwise list it here. A component is also the
right choice when it is only a placeholder in the ownership chain that another
component must hang off, in which case it carries a `kind` and an `ownerRef` and
nothing else. The CronJob definition does exactly that for the `batch/v1` Job it
creates. Do not list a kind here merely because the workload creates it, if a
component already covers it.

```yaml
additionalChildKinds:
- group: apps
  version: v1
  kind: Deployment
```

## Optimization instructions (expressions run against pod manifests)

Optional, used by schedulers. Two formats exist. `podGroup` is current;
`podGroups` is marked deprecated in the API but is what every catalog definition
still uses, so expect to read it. Every member or subgroup `componentName` must
name a defined component.

```yaml
# current format
optimizationInstructions:
  gangScheduling:
    podGroup:
      name: job
      subGroups:
      - componentName: worker
```

```yaml
# deprecated format, used throughout the catalog
optimizationInstructions:
  gangScheduling:
    podGroups:
    - name: job
      members:
      - componentName: worker
        groupByExpressions:
        - object[?"metadata"][?"labels"][?"training.kubeflow.org/job-name"].orValue(null)
```

`groupByExpressions` are CEL expressions evaluated against individual pod
manifests, whose values decide which pods share a gang. Use them when pods of
one component must be split into several gangs, typically by owner name plus a
replica index. When omitted, grouping falls back to owner reference traversal.
Each expression must return a single non-empty value for every pod, or grouping
fails at runtime, so coalesce to a default. The LeaderWorkerSet definition
groups by name plus group index, defaulting the index to `"0"`:

```yaml
groupByExpressions:
- object[?"metadata"][?"labels"][?"leaderworkerset.sigs.k8s.io/name"].orValue(null)
- ([dyn(object[?"metadata"][?"labels"][?"leaderworkerset.sigs.k8s.io/group-index"].orValue(null))].filter(v, v != null && v != false) + ["0"])[0]
```

These are pod-level expressions, not workload expressions. When copying a
catalog definition as a skeleton, copy the format it uses rather than converting
it, and check the `groupByExpressions` label keys against the target
controller's real pod labels the same way as `podSelector` keys.

## Validation checklist

- All kinds use a full GVK (only `Pod` may omit the group).
- Root has a `statusDefinition` and no `ownerRef`.
- Every child has an `ownerRef` to an existing component; no ownership cycles.
- Component names are unique and non-empty.
- No component sets more than one spec pattern.
- `instanceIds` and `componentInstanceSelector` are both present or both absent.
- Every expression starts from the `object` root and is null-safe (optional
  selection chained into `.orValue(...)` defaults).
- Pod selectors reference pod fields; selectors of the same kind are mutually exclusive across components.
- Status conditions and phases match the workload's real API.
- Every declared `conditionsDefinition` or `phaseDefinition` is referenced by at least one matcher, and every matcher has the definition it needs.
- Replica counts describe the component's level, siblings at the same level agree, and nested levels multiply by the parent count.
- Autoscaling bounds were looked for, not assumed absent.
- Every field consumers may mutate has a `patch`; an expression-only accessor is documented as read-only.
- No redundant duplicate kinds in `additionalChildKinds` (duplicates are allowed only when needed for RBAC or owner traversal).
- Every gang-scheduling member names a defined component, and `groupByExpressions` reference pod fields.
