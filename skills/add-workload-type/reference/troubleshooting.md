<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Troubleshooting catalog

Match the error text to a row and apply the fix. Messages come from the
validator (`pkg/api/runai/v1alpha1/validation.go`), the CEL evaluator
(`pkg/cel/evaluator.go`), or the Go accessor API at runtime
(`pkg/resource/`). The prose version is `docs/Troubleshooting.md`.

## Structure validation errors

The validator joins several errors at once, so fix every named component.

| Message | Cause | Fix |
|---|---|---|
| `root component must have full kind (group, version, kind)` | Root is missing group, version, or kind. | Provide all three under `kind`. Only the core `Pod` kind may omit the group. |
| `root component must have status definition` | Root has no `statusDefinition`. | Add a `statusDefinition` to the root. It is required. |
| `root component cannot have owner ref` | An `ownerRef` is set on the root. | Remove it. Only child components have owners. |
| `child component '<name>' has no owner ref` | A child is missing `ownerRef`, or it is empty. | Set `ownerRef` to the parent component `name`. |
| `child component '<name>' has owner ref to non-existing component '<owner>'` | `ownerRef` names a component that does not exist. | Correct it to an existing `name`. Watch for typos and renames. |
| `component name <name> is not unique` | Two components share a `name`. | Make every `name` unique. |
| `component name is empty` | A component has no `name`. | Add a non-empty `name`. |
| `component '<name>' has multiple pod spec definitions` | More than one of `podTemplateSpec`, `podSpec`, `fragmentedPodSpecDefinition` is set. | Keep exactly one. They are mutually exclusive. |
| `component '<name>' has instance ids but no pod component instance selector` | `instanceIds` is set without a `componentInstanceSelector`. | Add a `componentInstanceSelector`, or remove `instanceIds`. |
| `component '<name>' has pod component instance selector but no instance ids` | A `componentInstanceSelector` is set without `instanceIds`. | Add an `instanceIds` accessor, or remove the instance selector. |
| `ownership cycle detected involving component <name>` | Owner refs form a loop instead of reaching the root. | Break the cycle. Every owner chain must terminate at the root. |
| `pod-group member component '<name>' is not defined (should be a root or child component)` | A gang-scheduling member names a missing component. | Make each `componentName` match a defined component. |
| `karta is nil` | The validator got no definition. | Ensure the file parsed and loaded before validation. |

## Expression errors

Each message names the exact expression, so search the definition for it.

| Message | Cause | Fix |
|---|---|---|
| `compile CEL expression '<expr>': ...` | The expression is not valid CEL, or references an unknown function. | Fix the syntax: unbalanced brackets or quotes, comparing values of different types, or a missing leading `object.`. |
| `evaluate CEL expression '<expr>': ...` | Compiled but failed against real data, often a missing field. | Make the read null-safe with optional types: `object[?"status"][?"phase"].orValue(null)` instead of `object.status.phase`. |
| `the field has no patch and cannot be written` | A write was attempted through an accessor that defines `expression` but no `patch`. | Add a `patch` expression to the accessor. Reads use `expression`; writes use `patch`. |

Evaluation is budgeted: an expression whose cost explodes (for example deeply
nested comprehensions over a large workload) is stopped with an evaluation
error instead of running unbounded. A normal definition never notices the
budget.

Tip: reproduce what Karta evaluates in the CEL playground at
playcel.undistro.io. Paste the manifest as the variable `object` and iterate on
the expression there.

## Accessor errors at runtime

Raised by the Go Component API when reading a definition.

| Error type | Example | Cause | Fix |
|---|---|---|---|
| `DefinitionNotFoundError` | `component <name> does not have suspendDefinition` | Code asked for a part the component does not define. | Add the missing definition, or guard the call with `errors.As` against `DefinitionNotFoundError`. |
| `InstanceNotFoundError` | `could not match instance id "<id>". existing instance ids [...]` | A pod's extracted instance id matches no id from the `instanceIds` accessor. | Confirm the `componentInstanceSelector` reads the same id the `instanceIds` accessor produces. |

## Silent mistakes (valid but wrong)

These pass validation but behave incorrectly. Check them first when a definition
"works" but reports the wrong thing.

- Using `ownerName` instead of `ownerRef`. The field is `ownerRef`. A child with
  `ownerName` decodes with no owner and is only caught where the validator runs.
- Expecting a `referencedComponents` field. It does not exist. Model owned
  resources as child components; list other managed kinds under
  `additionalChildKinds`.
- Status conditions that do not match the workload's real API. The definition
  validates but status never resolves because the controller never sets those
  types. Verify against the CRD source or docs.
- An expression evaluated against the wrong resource. Spec, scale, and status
  expressions run against the workload object; selector and optimization
  expressions run against pod manifests. A selector pointing at a workload field
  matches nothing.
- A read that works and a write that does not. Reads and writes are separate:
  `expression` reads the field, `patch` writes it. An accessor with only an
  `expression` is read-only, and a write through it fails with
  `the field has no patch and cannot be written`. Leave the patch out only on
  purpose, and say so in a comment next to the accessor.
- Merging where a replace is needed. Without `replace: true` a map patch merges
  into the existing field, so stale keys survive a pod template update. With it,
  the field is deleted and re-set, which is wrong for annotations that should
  merge. Pick per accessor.
- Listing a defined component's kind under `additionalChildKinds`. The list is
  for managed kinds, and duplicating a kind already modeled as a component is
  usually redundant. The validator does not reject it, though, and it is
  legitimate when a kind must also be declared for RBAC or owner traversal (for
  example a scaling-group kind that is both a component and an ancestor to walk).
  Duplicate only with that intent, not by accident.
- A role selector key copied from the nearest sample without checking the target
  controller's real pod labels. Role-label keys are operator-specific (PyTorchJob
  `training.kubeflow.org/replica-type` vs MPIJob `training.kubeflow.org/job-role`),
  so a copied key silently matches nothing. Read the controller's actual pod
  labels.
- Two role components matching the same pod label. A plain value match on a
  shared label is not mutually exclusive. Disambiguate by matching a key only one
  role carries, using key existence (omit `value`), as LeaderWorkerSet does for
  leader versus worker.
- Inventing a phase or condition for a controller that reports neither. Some
  controllers expose only status fields such as replica counts. A `byPhase` or
  `byConditions` rule then never matches. Map those states with `byExpression`
  over the real fields instead.
- Mapping to `Undefined`. It is the implicit no-match result, not a target to
  map. Map only the statuses the workload reports.
- Plain field access on a field that can be absent. `object.status.phase` fails
  evaluation when `status` is missing; `object[?"status"][?"phase"].orValue(null)`
  yields a value. Every expression over an optional field needs the optional
  form, and defaults use the coalesce idiom
  `([dyn(X.orValue(null))].filter(v, v != null && v != false) + [D])[0]`.
