<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# Troubleshooting Karta Definitions

This catalog maps the errors you are most likely to hit when authoring a Karta
definition to their cause and fix. Errors fall into four groups:

- Structure validation errors, raised by the Karta validator.
- Expression errors, raised when a CEL expression fails to compile or evaluate.
- Accessor errors, raised at runtime when code reads a definition that is
  missing a required part or a pod cannot be matched to an instance.
- Silent mistakes that pass validation but produce wrong behavior.

New to authoring? Start with [Authoring Your First Karta](./Authoring%20Your%20First%20Karta.md).
For the full field reference, see the [Technical Guide](./Technical%20Guide.md).

## Structure validation errors

These come from the validator that runs over the whole definition. Several can
be reported at once; the validator joins them, so fix each named component.

| Error message | Cause | Fix |
|---|---|---|
| `root component must have full kind (group, version, kind)` | The root component is missing one of group, version, or kind. | Provide all three under `kind`. Only the core `Pod` kind may omit the group. |
| `root component must have status definition` | The root component has no `statusDefinition`. | Add a `statusDefinition` to the root. It is required so consumers can read normalized status. |
| `root component cannot have owner ref` | An `ownerRef` was set on the root component. | Remove `ownerRef` from the root. Only child components have owners. |
| `child component '<name>' has no owner ref` | A child component is missing `ownerRef`, or it is empty. | Set `ownerRef` to the `name` of the parent component. |
| `child component '<name>' has owner ref to non-existing component '<owner>'` | `ownerRef` points at a name that is not defined in this Karta. | Correct the `ownerRef` to match an existing component `name`. Watch for typos and renamed components. |
| `component name <name> is not unique` | Two components share the same `name`. | Give every component a unique `name`. |
| `component name is empty` | A component has no `name`. | Add a non-empty `name`. |
| `component '<name>' has multiple pod spec definitions` | A `specDefinition` sets more than one of `podTemplateSpec`, `podSpec`, or `fragmentedPodSpecDefinition`. | Keep exactly one. These three patterns are mutually exclusive per component. |
| `component '<name>' has instance ids but no pod component instance selector` | `instanceIds` is set, but the component's `podSelector` has no `componentInstanceSelector`. | Add a `componentInstanceSelector` so pods can be matched to instances, or remove `instanceIds` if the component is not multi-instance. |
| `component '<name>' has pod component instance selector but no instance ids` | A `componentInstanceSelector` is set, but the component has no `instanceIds`. | Add an `instanceIds` accessor pointing at where instance names live, or remove the instance selector. |
| `ownership cycle detected involving component <name>` | Components form a loop through their `ownerRef` chain instead of reaching the root. | Break the cycle. Every child's owner chain must terminate at the root component. |
| `pod-group member component '<name>' is not defined (should be a root or child component)` | A `gangScheduling` pod-group member names a component that does not exist. | Make each `componentName` match a defined root or child component. |
| `karta is nil` | The validator was given no definition to validate. | Ensure the definition was parsed and loaded before validation. Usually indicates an empty or unreadable file. |

## Expression errors

Every expression in a Karta is [CEL](https://kubernetes.io/docs/reference/using-api/cel/),
evaluated with the resource bound as `object`. These errors name the exact
expression that failed, so search your definition for that string.

| Error message | Cause | Fix |
|---|---|---|
| `compile CEL expression '<expr>': ...` | The expression is not valid CEL, or references an unknown field or function. | Fix the syntax. Common causes: unbalanced brackets or quotes, comparing values of different types, or forgetting the leading `object.`. |
| `evaluate CEL expression '<expr>': ...` | The expression compiled but failed while running against real data, often a missing field. | Make the read null-safe with optional types. Use `object[?"status"][?"phase"].orValue(null)` instead of `object.status.phase` when the field may be absent. |
| `the field has no patch and cannot be written` | A write was attempted through an accessor that defines `expression` but no `patch`. | Add a `patch` expression to the accessor. Reads use `expression`; writes use `patch`. |
| `the definition declares references but the consumer provided no resolved references and no reader` | An expression reads `references.<name>` but the factory got neither `WithReferences` nor `WithReferenceReader`. | Pass resolved values or a reader to the factory, or remove the reference from the definition. |
| `reference "<name>": the reader may not get ...` | The reader implements the permission check and denied the verb. | Grant the missing RBAC to the reader's identity, or drop the reference. The error names the kind and verb that were denied. |

Evaluation is budgeted: an expression whose cost explodes (for example deeply
nested comprehensions over a large workload) is stopped with an evaluation
error instead of running unbounded. A normal definition never notices the
budget.

Tip: test an expression against a real workload manifest with the
[CEL playground](https://playcel.undistro.io/). Paste the manifest as the
variable `object` and iterate on the expression without leaving the browser.

## Accessor errors at runtime

These are raised by the Go Component API when code reads a part of a definition
that is not present, or when a pod cannot be tied to a component instance.

| Error type | Example message | Cause | Fix |
|---|---|---|---|
| `DefinitionNotFoundError` | `component <name> does not have suspendDefinition` | Code asked for a part (spec, scale, status, pod template, pod metadata, fragmented pod spec, or suspend) that the component does not define. | Add the missing definition to the component, or guard the call. Use `errors.As` against `DefinitionNotFoundError` to treat "not defined" as an expected case rather than a failure. |
| `InstanceNotFoundError` | `could not match instance id "<id>". existing instance ids [...]` | A pod's extracted instance id does not match any instance id produced by the component's `instanceIds` accessor. | Confirm the `componentInstanceSelector` reads the same id that `instanceIds` produces. A mismatch usually means the selector points at the wrong pod label or the accessor extracts a different field. |

## Silent mistakes (valid but wrong)

These pass validation but lead to wrong status, missing pods, or no effect. They
are worth checking first when a definition "works" but behaves incorrectly.

- Using `ownerName` instead of `ownerRef`. The API field is `ownerRef`. A child
  written with `ownerName` decodes with no owner set. The validator rejects it
  with `child component '<name>' has no owner ref`, but validation runs only
  where `KartaValidator` is invoked, so a code path that skips validation sees
  a silently unowned child. Use `ownerRef`, and run the validator when loading
  definitions.
- Expecting a `referencedComponents` field. The structure has only
  `rootComponent`, `childComponents`, and `additionalChildKinds`. There is no
  `referencedComponents`. Model owned resources as child components and list
  other managed kinds under `additionalChildKinds`.
- Status conditions that do not match the workload's real API. The definition
  validates, but status never resolves because the controller never sets those
  condition types. Verify condition types and field names against the workload's
  CRD source or documentation.
- Expressions evaluated against the wrong resource. Expressions in
  `specDefinition`, `scaleDefinition`, and `statusDefinition` run against the
  workload object. Expressions in `podSelector` and `optimizationInstructions`
  run against pod manifests. A selector that points at a field on the workload
  object instead of the pod will match nothing.
- A read that works and a write that does not. Reads and writes are separate:
  `expression` reads the field, `patch` writes it. An accessor with only an
  `expression` is read-only, and a write through it fails with
  `the field has no patch and cannot be written`.
- Listing an explicitly defined component's kind under `additionalChildKinds`.
  `additionalChildKinds` is only for managed kinds that are not already a child
  component. Duplicating a defined kind is redundant and flagged by the
  "no duplicated child kinds" check.

## Still stuck

- Re-run the [validation checklist](./Technical%20Guide.md#validation-checklist)
  in the Technical Guide.
- Compare your definition against the closest one in
  [`docs/catalog/`](./catalog/).
- Exercise the definition with the offline quickstart at
  [`docs/examples/quickstart/`](./examples/quickstart/) to see what the uniform
  API reads back.
