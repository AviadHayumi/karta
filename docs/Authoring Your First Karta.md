<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# Authoring Your First Karta

This tutorial walks you through writing a Karta definition from scratch for a
single workload type. By the end you will have a complete, valid definition for
a Kubernetes `batch/v1` Job and you will understand the building blocks well
enough to describe any other workload type.

A Karta is a declarative description of a workload's structure. You write it
once per workload type. After that, any controller or platform that uses the
Karta library can read status, scale, and pod specs for that workload through a
single uniform API, without per-type code.

For the full field reference, see the [Technical Guide](./Technical%20Guide.md).
When something does not validate, see [Troubleshooting](./Troubleshooting.md).

## Prerequisites

- Basic familiarity with Kubernetes custom resources and YAML.
- Basic familiarity with
  [CEL](https://kubernetes.io/docs/reference/using-api/cel/). Every read in a
  Karta is a CEL expression, and every write is a CEL patch expression.
- The target workload's CRD. You need to know its Group, Version, and Kind, the
  conditions or phases it reports in `.status`, and where it stores its pod
  template.

## What you will build

A Karta for the built-in `batch/v1` Job. A Job is the simplest useful example:
one component, one pod template, and a status reported through conditions. The
definitions in [`docs/catalog/`](./catalog/) cover more complex workloads
(JobSet, PyTorchJob, RayCluster, KServe, and more); start from the one closest
to your workload when you write your own.

## Step 1: Understand the model

A Karta describes a workload as a tree of components.

- The root component is the top-level CRD itself. Every Karta has exactly one.
  It must declare a full GVK and a `statusDefinition`.
- Child components are resources owned by the root (for example the Deployments
  a higher-level CRD creates). Each child must declare an `ownerRef` pointing at
  its parent. A Job has no children, so this tutorial uses only the root.

Every read you write is a CEL expression evaluated with the resource bound as
`object`. Every write is a patch expression that constructs the change.
Expressions in `specDefinition`, `scaleDefinition`, and `statusDefinition` are
evaluated against the workload object. Expressions in `podSelector` and in
`optimizationInstructions` are evaluated against pod manifests. Mixing these up
is the most common authoring mistake.

## Step 2: Start from the boilerplate

Create a file named `job.yaml`. Every Karta starts with the same header and a
`structureDefinition` holding a `rootComponent`.

```yaml
apiVersion: run.ai/v1alpha1
kind: Karta
metadata:
  name: batch-v1-job
spec:
  structureDefinition:
    rootComponent:
      name: job
      kind:
        group: batch
        version: v1
        kind: Job
```

`name` is a free-form identifier for the component, unique within the Karta.
`kind` is the workload's full GVK. Always provide all three of group, version,
and kind. (The one exception is the core `Pod` kind, which has an empty group.)

## Step 3: Map the status

`statusDefinition` translates the workload's own conditions or phases into
Karta's normalized statuses (for example `running`, `completed`, `failed`).
This is required on the root component.

A `byConditions` rule lists one or more conditions that must all hold (AND
logic). A `byExpression` rule evaluates a CEL expression and compares its
result, rendered as a string, to `expectedResult`. This is useful when a
workload reports state through status fields rather than conditions. A Job
uses both.

Rules under the same status are ORed: the status matches when any one of its
rules matches, so mixing `byConditions` and `byExpression` rules under one
status means either can resolve it. A workload can match several statuses at
once; each match is returned in `MatchedStatuses`. When no rule matches, the
status resolves to `Undefined`.

```yaml
      statusDefinition:
        conditionsDefinition:
          expression: object[?"status"][?"conditions"].orValue(null)
          typeFieldName: type
          statusFieldName: status
          messageFieldName: message
          reasonFieldName: reason
        statusMappings:
          initializing:
            - byExpression:
                expression: variables.statusActive > 0 && variables.statusReady == 0
                expectedResult: "true"
          running:
            - byExpression:
                expression: variables.statusActive > 0 && variables.statusReady > 0
                expectedResult: "true"
          completed:
            - byConditions:
                - type: Complete
                  status: "True"
          failed:
            - byConditions:
                - type: Failed
                  status: "True"
```

`conditionsDefinition.expression` reads the workload's conditions list. Note
the optional chaining: `object[?"status"][?"conditions"]` produces an optional
value, and `.orValue(null)` supplies the fallback when `.status.conditions`
does not exist yet. A field that may be absent must always be read through
optionals so the expression stays null-safe.

The `variables.*` references are named expressions. Declare them under
`spec.variables`, next to `structureDefinition`, and every expression in the
definition can use them as `variables.<name>`:

```yaml
  variables:
    - name: statusActive
      expression: ([dyn(object.?status.?active.orValue(null))].filter(v, v != null && v != false) + [0])[0]
    - name: statusReady
      expression: ([dyn(object.?status.?ready.orValue(null))].filter(v, v != null && v != false) + [0])[0]
```

The idiom `([dyn(X.orValue(null))].filter(v, v != null && v != false) + [D])[0]`
reads `X` and falls back to the default `D` when the field is missing or null.
`.status.active` does not exist before the Job starts, so `statusActive`
evaluates to `0` instead of erroring. Variables are evaluated in order, and a
later variable may reference an earlier one.

Map only the statuses the workload actually reports, and base each rule on the
real condition types and fields in that workload's API. Inventing condition
types that the controller never sets is a frequent source of a Karta that
validates but never reports the right status.

## Step 4: Point to the pods

`specDefinition` tells Karta where the workload stores its pod template. There
are three mutually exclusive patterns; pick exactly one per component.

- `podTemplateSpec` when the CRD embeds a full pod template.
- `podSpec` (with optional `metadata`) when it embeds a bare pod spec.
- `fragmentedPodSpecDefinition` when pod fields are scattered across the spec.

Each of these is an accessor pair: `expression` is the CEL read, and `patch`
is a CEL expression constructing the write. A Job embeds a full pod template at
`.spec.template`, so use the first pattern.

```yaml
      specDefinition:
        podTemplateSpec:
          expression: object[?"spec"][?"template"].orValue(null)
          patch: '{"spec": {"template": value}}'
          replace: true
```

In the patch expression, `value` is bound to the new value Karta computed. The
patch builds either a partial object that merges into the workload (maps merge
recursively, any other value replaces, and null removes the field) or an RFC
6902 list of operations. `replace: true` makes the write replace the field
instead of merging into it: the patch is first applied with `value` bound to
null (removing the field) and then with the real value. A pod template update
wants this; an annotations update usually does not.

## Step 5: Declare scale

`scaleDefinition` tells Karta how to read replica counts. A Job's parallelism
is its replica count.

```yaml
      scaleDefinition:
        replicas:
          expression: variables.specParallelism
```

With the matching variable under `spec.variables`:

```yaml
    - name: specParallelism
      expression: ([dyn(object[?"spec"][?"parallelism"].orValue(null))].filter(v, v != null && v != false) + [1])[0]
```

The same default idiom keeps the read null-safe: `parallelism` is optional and
defaults to 1 when unset.

## Step 6 (optional, experimental): Add scheduling instructions

`optimizationInstructions` is optional and used by schedulers. For gang
scheduling you declare a pod group and map the workload's components onto it.
The current format is `podGroup`: the group gets a name, each listed component
becomes a subgroup, and topology constraints can be set per subgroup or for
the whole group.

```yaml
  optimizationInstructions:
    gangScheduling:
      podGroup:
        name: job
        subGroups:
          - componentName: job
```

Every `componentName` here must match a component you defined above. This part
of the API is experimental and may change. The older `podGroups` list format
still appears in some samples; it is deprecated in favor of `podGroup`.

## Step 7: Validate against the checklist

Before using a definition, confirm:

- All kinds use a full GVK.
- The root component has a `statusDefinition`.
- All expressions are null-safe (read possibly absent fields through optionals
  with an `orValue` fallback) and reference the correct resource (workload
  object versus pod).
- No duplicated child kinds.
- Pod selectors are mutually exclusive.
- Status conditions match the workload's real API.
- Every child component has an `ownerRef` to an existing component, with no
  ownership cycles.

The Karta library runs these checks for you through its validator when a
definition is loaded. If a check fails, the error message names the component
and the problem. See [Troubleshooting](./Troubleshooting.md) for each message
and its fix.

## Step 8: See it in action

The offline quickstart at [`docs/examples/quickstart/`](./examples/quickstart/)
loads a Karta definition together with a sample workload and exercises the
uniform API: it reads status, scale, and pod template, mutates the pods (it
injects a scheduler name and a label), and writes them back, all without a
cluster.

```bash
go run ./docs/examples/quickstart --print-mutated
```

The quickstart ships with JobSet and LeaderWorkerSet definitions. Use it as the
pattern for loading and exercising the Job definition you just wrote.

To see the definition working in a live environment, install the
[controller-runtime example](./examples/controller-runtime/) into a Kind
cluster and add your new Karta to its watched types. It inspects and mutates
live workloads through the same uniform API, with no per-CRD code.

## Where to go next

- Add child components for workloads that own other resources. Each child needs
  an `ownerRef`. See the multi-component samples such as
  [`docs/catalog/serving-kserve-io-inferenceservice-v1beta1.yaml`](./catalog/serving-kserve-io-inferenceservice-v1beta1.yaml) and
  [`docs/catalog/kubeflow-org-pytorchjob-v1.yaml`](./catalog/kubeflow-org-pytorchjob-v1.yaml).
- Handle workloads with repeated roles or instances (for example the replicated
  jobs of a JobSet) with an `instanceIds` expression and a matching component
  instance selector. See [`docs/catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml`](./catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml).
- Consume a definition from a controller with the Go Component API. See
  [`docs/examples/controller-runtime/`](./examples/controller-runtime/).
- Read the full field reference in the [Technical Guide](./Technical%20Guide.md).
