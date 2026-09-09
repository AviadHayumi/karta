<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# Karta Anatomy
## Root Component
Every Karta must define a root component:
- Must use full Kubernetes GVK (Group, Version, Kind)
- Must include a `statusDefinition`

```YAML
rootComponent:
  name: pytorchjob
  kind:
    group: kubeflow.org
    version: v1
    kind: PyTorchJob
  statusDefinition:
    statusMappings:
      running:
      - byConditions:
        - type: Running
          status: "True"
```

## Child Components
For resources owned by the root component:
- Must include `ownerRef` (points to parent component)
- Usually include a `specDefinition`
- All expressions evaluate against the full CRD manifest, bound as `object`

```YAML
childComponents:
- name: worker
  ownerRef: pytorchjob
  kind:
    group: ""
    version: v1
    kind: Pod
  specDefinition:
    podTemplateSpec:
      expression: object[?"spec"][?"pytorchReplicaSpecs"][?"Worker"][?"template"].orValue(null)
      patch: '{"spec": {"pytorchReplicaSpecs": {"Worker": {"template": value}}}}'
      replace: true
```

## Expressions
All expressions in a Karta are written in [CEL](https://github.com/google/cel-spec), the expression language used across the Kubernetes API (see [Common Expression Language in Kubernetes](https://kubernetes.io/docs/reference/using-api/cel/)).

Reads and writes are separate. A field is accessed through a pair:
- `expression` reads the value. The workload manifest is bound as `object`.
- `patch` writes the value. It is a CEL expression that constructs the change to apply.

A `patch` can construct one of two shapes:
- A map: applied as a JSON merge patch. Maps merge recursively, any other value replaces, and `null` deletes the field.
- A list: applied as an RFC 6902 operation list, for example `[{"op": "add", "path": "/spec/template", "value": value}]`.

Inside a `patch`, these names are bound:
- `value`: the new value being written.
- `instance` and `index`: the instance id and its position, for instanced components.
- `variables.<name>`: the named expressions defined in `spec.variables`.

Setting `replace: true` makes the write replace the field instead of merging into it: the patch is first applied with `value` bound to `null` (deleting the field) and then with the real value. A pod template update wants this; an annotations update usually does not.

### Null safety
Use optional selection instead of plain field access so a missing field does not fail the evaluation:
- `object[?"spec"][?"template"]` selects optionally at each step (`object.?spec.?template` is equivalent).
- `.orValue(null)` unwraps the optional with a default.

```
object[?"spec"][?"template"].orValue(null)
```

To coalesce a possibly missing value to a default, use the filter idiom:

```
([dyn(object[?"spec"][?"replicas"].orValue(null))].filter(v, v != null && v != false) + [1])[0]
```

It wraps the value in a single-item list, drops it when it is null, appends the default, and takes the first element.

### Variables
`spec.variables` are named CEL expressions, available to every expression in the definition as `variables.<name>`. They are evaluated in order, and a later variable may reference an earlier one. Use them to name a shared sub-expression once instead of repeating it.

```YAML
spec:
  variables:
  - name: specReplicas
    expression: ([dyn(object[?"spec"][?"replicas"].orValue(null))].filter(v, v != null && v != false) + [1])[0]
```

```YAML
scaleDefinition:
  replicas:
    expression: variables.specReplicas
```

## Spec Definitions

There are multiple, mutually exclusive, types of specDefinitions:

| Pattern | Use Case |
| ------- | -------- |
| podTemplateSpec | CRD embeds a full pod template |
| podSpec and metadata | CRD directly embeds a podSpec and/or an objectMeta |
| fragmentedPodSpecDefinition | Pod fields scattered across the CRD |

Full pod template:

```YAML
specDefinition:
  podTemplateSpec:
    expression: object[?"spec"][?"template"].orValue(null)
    patch: '{"spec": {"template": value}}'
    replace: true
```

Fragmented pod fields:

```YAML
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
    schedulerName:
      expression: object[?"spec"][?"components"][?"standalone"][?"schedulerName"].orValue(null)
      patch: '{"spec": {"components": {"standalone": {"schedulerName": value}}}}'
      replace: true
```

## Component Instances

A component's spec definition might point to multiple instance specs (in map/array format). In those cases it is crucial to be able to distinguish between each instance of that component.
To do so, define `instanceIds`: an expression returning the list of instance ids. Every other accessor of the component then returns a list aligned with that order, and its `patch` receives `instance` (the id) and `index` (its position).
For example:

1. Array of specs (each entry carries its own name):

```YAML
instanceIds:
  expression: ([dyn(object[?"spec"][?"replicatedJobs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"name"].orValue(null))
```

A per-index write uses an RFC 6902 patch built with `index`:

```YAML
specDefinition:
  podTemplateSpec:
    expression: ([dyn(object[?"spec"][?"replicatedJobs"].orValue(null))].filter(v, type(v) == list) + [[]])[0].map(x, x[?"template"][?"spec"][?"template"].orValue(null))
    patch: '[{"op": "add", "path": "/spec/replicatedJobs/" + string(index) + "/template/spec/template", "value": value}]'
```

2. Map of specs (the map keys are the instance ids, sorted for a stable order):

```YAML
instanceIds:
  expression: ([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort()
```

A per-instance write uses a merge patch keyed by `instance`:

```YAML
labels:
  expression: ([dyn(object[?"spec"][?"services"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, k).sort().map(k, object.spec.services[k][?"labels"].orValue(null))
  patch: '{"spec": {"services": {instance: {"labels": value}}}}'
  replace: true
```

## Pod Selectors
PodSelector defines how to identify pods belonging to a component / an instance of a component.
A component can define type, instance, and replica selectors.
All selectors of each kind (component, instance) must be mutually exclusive within themselves.

Selector expressions evaluate against the pod's manifest, bound as `object`.

1. Component type selector - an expression (and optional value) that associates a pod with the current component.
```YAML
podSelector:
  componentTypeSelector:
    expression: object[?"metadata"][?"labels"][?"training.kubeflow.org/replica-type"].orValue(null)
    value: master
```
If value is not provided, only a non-null result is checked.

2. Component instance selector - an expression on the pod that yields its matching instance id.

```YAML
podSelector:
  componentInstanceSelector:
    expression: object[?"metadata"][?"labels"][?"jobset.sigs.k8s.io/replicatedjob-name"].orValue(null)
```

3. Replica selector - an expression on the pod that yields the replica index or group it belongs to. Use it when replicas of the same component share an identical sub-structure. Descendant components inherit the replica context from their parent, so define it only where replicas are created.

```YAML
podSelector:
  replicaSelector:
    expression: object[?"metadata"][?"labels"][?"leaderworkerset.sigs.k8s.io/group-index"].orValue(null)
```

## Status Definitions
- Mapping the described CRD's conditions or phases to the Karta generic statuses. Must be based on the actual conditions/phases used by the described CRD.
- For each generic status, the user can provide a definition based on conditions, phases, or a boolean expression. If several are provided in a single matcher, all are validated when evaluating the status.
- If using definition by conditions/phase, you first must include `conditionsDefinition` / `phaseDefinition`, whose `expression` extracts the conditions list / phase string from the workload object.
- Multiple, separate, definitions can be provided for each generic status.
- When providing a definition `byConditions`, all must exist (AND logic)
- Required for the root component

```YAML
statusDefinition:
  conditionsDefinition:
    expression: object[?"status"][?"conditions"].orValue(null)
    typeFieldName: type
    statusFieldName: status
  statusMappings:
    initializing:
    - byConditions:
      - type: Created
        status: "True"
    running:
    - byConditions:
      - type: Running
        status: "True"
    completed:
    - byConditions:
      - type: Succeeded
        status: "True"
    failed:
    - byConditions:
      - type: Failed
        status: "True"
```

`byExpression` matches when a CEL expression over the workload object yields the expected result:

```YAML
statusMappings:
  suspended:
  - byExpression:
      expression: object.?spec.?suspend.orValue(false) == true
      expectedResult: "true"
```

## Suspend Definitions
For frameworks with native, first-class suspension support (e.g. `spec.suspend` for batch/v1 Job), define the patches applied on suspend and resume. Each action is a `patch` expression merged into the workload; actions are applied in order.

```YAML
suspendDefinition:
  suspendActions:
  - patch: '{"spec": {"suspend": true}}'
  resumeActions:
  - patch: '{"spec": {"suspend": false}}'
```

## Additional child kinds
List any GVK of objects created or managed by the CRD that are not mentioned explicitly by any child component.
This is essential for permission management so that your CRD can be managed correctly.

```YAML
  additionalChildKinds:
   - group: apps
     version: v1
     kind: Deployment
   - group: leaderworkerset.x-k8s.io
     version: v1
     kind: LeaderWorkerSet
```

## Optimization Instructions
Used for scheduling.

Expressions in the instructions evaluate against the pod's manifest, bound as `object`.

Currently supported instructions:

- `gangScheduling`: instruct the scheduler how to group pods. Each pod-group definition contains a list of the included members.
Each defined member can provide a list of `groupByExpressions`: distinct keys to group pods by.

```YAML
optimizationInstructions:
  gangScheduling:
    podGroups:
    - name: job
      members:
      - componentName: master
        groupByExpressions:
        - object[?"metadata"][?"labels"][?"training.kubeflow.org/job-name"].orValue(null)
      - componentName: worker
        groupByExpressions:
        - object[?"metadata"][?"labels"][?"training.kubeflow.org/job-name"].orValue(null)
```

Grouping examples:

1. Different hierarchy: The following are equivalent (given that master and worker have the same value for that label)

```YAML
optimizationInstructions:
  gangScheduling:
    podGroups:
    - name: job
      members:
      - componentName: master
        groupByExpressions:
        - object[?"metadata"][?"labels"][?"training.kubeflow.org/job-name"].orValue(null)
      - componentName: worker
        groupByExpressions:
        - object[?"metadata"][?"labels"][?"training.kubeflow.org/job-name"].orValue(null)

optimizationInstructions:
  gangScheduling:
    podGroups:
    - name: job
      members:
      - componentName: job
        groupByExpressions:
        - object[?"metadata"][?"labels"][?"training.kubeflow.org/job-name"].orValue(null)
```

2. Using default values: when multiple groups are possible, coalesce to a default value to cover cases where a single group is used (the used pattern: `<prefix>-{name}-{index}`)

```YAML
optimizationInstructions:
  gangScheduling:
    podGroups:
    - name: group
      members:
      - componentName: group
        groupByExpressions:
        - object[?"metadata"][?"labels"][?"leaderworkerset.sigs.k8s.io/name"].orValue(null)
        - ([dyn(object[?"metadata"][?"labels"][?"leaderworkerset.sigs.k8s.io/group-index"].orValue(null))].filter(v, v != null && v != false) + ["0"])[0]
```

## Best Practices
- Always use full GVK for kinds (group, version, kind)
- Expressions always start from the `object` root
- Avoid duplication: don't list explicitly defined components in `additionalChildKinds`
- Mutually exclusive pod selectors in multi-component workloads
- Null-safe CEL expressions: optional selection (`[?"key"]`) chained into `.orValue(...)` defaults
- Target actual components in optimization instructions, not just the root CRD
- Name repeated sub-expressions once in `spec.variables` and reference them as `variables.<name>`

## Examples
Example 1: Kserve inference service
```YAML
spec:
  structureDefinition:
    rootComponent:
      name: inferenceservice
      kind:
        group: serving.kserve.io
        version: v1beta1
        kind: InferenceService
      statusDefinition:
        conditionsDefinition:
          expression: object[?"status"][?"conditions"].orValue(null)
          typeFieldName: type
          statusFieldName: status
          messageFieldName: message
        statusMappings:
          running:
            - byConditions:
                - type: PredictorReady
                  status: "True"
                - type: RoutesReady
                  status: "True"
                - type: LatestDeploymentReady
                  status: "True"
          failed:
            - byConditions:
                - type: PredictorReady
                  status: "False"
                - type: PredictorConfigurationReady
                  status: "False"
                - type: RoutesReady
                  status: "False"
    childComponents:
      - name: predictor
        kind:
          group: apps
          version: v1
          kind: Deployment
        ownerRef: inferenceservice
        specDefinition:
          fragmentedPodSpecDefinition:
            schedulerName:
              expression: object[?"spec"][?"predictor"][?"schedulerName"].orValue(null)
              patch: '{"spec": {"predictor": {"schedulerName": value}}}'
              replace: true
            labels:
              expression: object[?"spec"][?"predictor"][?"labels"].orValue(null)
              patch: '{"spec": {"predictor": {"labels": value}}}'
              replace: true
            annotations:
              expression: object[?"spec"][?"predictor"][?"annotations"].orValue(null)
              patch: '{"spec": {"predictor": {"annotations": value}}}'
              replace: true
            nodeAffinity:
              expression: object[?"spec"][?"predictor"][?"affinity"][?"nodeAffinity"].orValue(null)
              patch: '{"spec": {"predictor": {"affinity": {"nodeAffinity": value}}}}'
              replace: true
            container:
              expression: 'variables.containerKey != "" ? object.spec.predictor[variables.containerKey] : null'
              patch: 'variables.containerKey != "" ? {"spec": {"predictor": {variables.containerKey: value}}} : {}'
              replace: true
            priorityClassName:
              expression: object[?"spec"][?"predictor"][?"priorityClassName"].orValue(null)
              patch: '{"spec": {"predictor": {"priorityClassName": value}}}'
              replace: true
        scaleDefinition:
          minReplicas:
            expression: object[?"spec"][?"predictor"][?"minReplicas"].orValue(null)
          maxReplicas:
            expression: object[?"spec"][?"predictor"][?"maxReplicas"].orValue(null)
        podSelector:
          componentTypeSelector:
            expression: object[?"metadata"][?"labels"][?"component"].orValue(null)
            value: predictor
      - name: transformer
        kind:
          group: apps
          version: v1
          kind: Deployment
        ownerRef: inferenceservice
        specDefinition:
          podSpec:
            expression: object[?"spec"][?"transformer"].orValue(null)
            patch: '{"spec": {"transformer": value}}'
            replace: true
          metadata:
            expression: object[?"spec"][?"transformer"].orValue(null)
            patch: '{"spec": {"transformer": value}}'
            replace: true
        scaleDefinition:
          minReplicas:
            expression: object[?"spec"][?"transformer"][?"minReplicas"].orValue(null)
          maxReplicas:
            expression: object[?"spec"][?"transformer"][?"maxReplicas"].orValue(null)
        podSelector:
          componentTypeSelector:
            expression: object[?"metadata"][?"labels"][?"component"].orValue(null)
            value: transformer
  optimizationInstructions:
    gangScheduling:
      podGroups:
        - name: service
          members:
            - componentName: predictor
              groupByExpressions:
                - object[?"metadata"][?"labels"][?"serving.kserve.io/inferenceservice"].orValue(null)
            - componentName: transformer
              groupByExpressions:
                - object[?"metadata"][?"labels"][?"serving.kserve.io/inferenceservice"].orValue(null)
  variables:
    - name: containerKey
      expression: (([dyn(object[?"spec"][?"predictor"].orValue(null))].filter(v, type(v) == map) + [{}])[0].map(k, string(k)).sort().filter(k, type(object.spec.predictor[k]) == map && "storageUri" in object.spec.predictor[k] && object.spec.predictor[k]["storageUri"] != null && object.spec.predictor[k]["storageUri"] != false) + [""])[0]
```

## Validation Checklist
Before submitting, confirm:

- All kinds use full GVK
- root component has statusDefinition
- All CEL expressions start from the `object` root and are null-safe (optional selection and `orValue` defaults)
- Expressions target the correct resource: spec/scale/status definitions evaluate against the workload object; podSelector and groupByExpressions evaluate against the pod
- No duplicated child kinds
- Pod selectors are mutually exclusive
- Status conditions match real framework APIs
- All child components have ownerRef directed to existing components and there are no ownership cycles


## Quick-Start Templates
Use these as starting points when creating new workload type definitions.
With these templates, you can register new workload types by simply filling in the blanks instead of starting from scratch.

### Template: Generic Job
A single-component workload with pods defined directly in its spec.

```YAML
spec:
  structureDefinition:
    rootComponent:
      name: job
      kind:
        group: batch
        version: v1
        kind: Job
      specDefinition:
        podTemplateSpec:
          expression: object[?"spec"][?"template"].orValue(null)
          patch: '{"spec": {"template": value}}'
          replace: true
      statusDefinition:
        statusMappings:
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
      suspendDefinition:
        suspendActions:
          - patch: '{"spec": {"suspend": true}}'
        resumeActions:
          - patch: '{"spec": {"suspend": false}}'
  variables:
    - name: statusActive
      expression: ([dyn(object.?status.?active.orValue(null))].filter(v, v != null && v != false) + [0])[0]
    - name: statusReady
      expression: ([dyn(object.?status.?ready.orValue(null))].filter(v, v != null && v != false) + [0])[0]
```

### Template: Deployment
A controlling resource with generated ReplicaSets.

```YAML
spec:
  structureDefinition:
    rootComponent:
      name: deployment
      kind:
        group: apps
        version: v1
        kind: Deployment
      specDefinition:
        podTemplateSpec:
          expression: object[?"spec"][?"template"].orValue(null)
          patch: '{"spec": {"template": value}}'
          replace: true
      scaleDefinition:
        replicas:
          expression: variables.specReplicas
      statusDefinition:
        statusMappings:
          running:
            - byConditions:
                - type: Progressing
                  status: "True"
                  reason: NewReplicaSetAvailable
    childComponents:
      - name: replicaset
        kind:
          group: apps
          version: v1
          kind: ReplicaSet
        ownerRef: deployment
  variables:
    - name: specReplicas
      expression: ([dyn(object[?"spec"][?"replicas"].orValue(null))].filter(v, v != null && v != false) + [1])[0]
```

### Template: Distributed Training (PyTorchJob)
Multi-component workload with role-based pods.
```YAML
spec:
  structureDefinition:
    rootComponent:
      name: pytorchjob
      kind:
        group: kubeflow.org
        version: v1
        kind: PyTorchJob
      statusDefinition:
        statusMappings:
          running:
            - byConditions:
                - type: Running
                  status: "True"
          completed:
            - byConditions:
                - type: Succeeded
                  status: "True"
          failed:
            - byConditions:
                - type: Failed
                  status: "True"
    childComponents:
      - name: master
        kind:
          group: ""
          version: v1
          kind: Pod
        ownerRef: pytorchjob
        specDefinition:
          podTemplateSpec:
            expression: object[?"spec"][?"pytorchReplicaSpecs"][?"Master"][?"template"].orValue(null)
            patch: '{"spec": {"pytorchReplicaSpecs": {"Master": {"template": value}}}}'
            replace: true
        podSelector:
          componentTypeSelector:
            expression: object[?"metadata"][?"labels"][?"training.kubeflow.org/replica-type"].orValue(null)
            value: master
      - name: worker
        kind:
          group: ""
          version: v1
          kind: Pod
        ownerRef: pytorchjob
        specDefinition:
          podTemplateSpec:
            expression: object[?"spec"][?"pytorchReplicaSpecs"][?"Worker"][?"template"].orValue(null)
            patch: '{"spec": {"pytorchReplicaSpecs": {"Worker": {"template": value}}}}'
            replace: true
        podSelector:
          componentTypeSelector:
            expression: object[?"metadata"][?"labels"][?"training.kubeflow.org/replica-type"].orValue(null)
            value: worker
```

### Template: Inference Service (Knative)
Workload that owns a secondary component (Revision).

```YAML
spec:
  structureDefinition:
    rootComponent:
      name: knativeservice
      kind:
        group: serving.knative.dev
        version: v1
        kind: Service
      statusDefinition:
        statusMappings:
          running:
            - byConditions:
                - type: Ready
                  status: "True"
    childComponents:
      - name: revision
        kind:
          group: serving.knative.dev
          version: v1
          kind: Revision
        ownerRef: knativeservice
        specDefinition:
          podTemplateSpec:
            expression: object[?"spec"][?"template"].orValue(null)
            patch: '{"spec": {"template": value}}'
            replace: true
```

## Minimum requirements for defining a Karta

The minimal Karta must contain a rootComponent with name, full GVK (group, version, kind) and statusDefinition.

For example:
```YAML
rootComponent:
  name: minimal
  kind:
    group: minimal.org
    version: v1
    kind: Minimal
  statusDefinition:
    statusMappings:
      running:
      - byConditions:
        - type: Running
          status: "True"
```

## Pro Tips
- Start from the closest template to your workload type.

- Replace the GVK (group, version, kind) with your CRD's.

- Verify status conditions in the CRD source code or documentation.

- Add child/referenced components only if they matter for scheduling or optimization.

- Browse `docs/catalog/` for complete, working definitions of common frameworks.
