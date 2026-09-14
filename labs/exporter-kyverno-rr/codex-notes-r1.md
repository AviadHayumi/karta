<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Exporter integration: Codex review, round 1

Source review only. No cluster scenarios were executed for these notes.
VERIFIED means source read. INFERRED means a derived behavior or proposal.
K references are relative to `~/workspace/kyverno`, at tag
`v1.19.1`, commit `40ec788d48bb28d83dbf85538e962a59db9d45c6`.
Tag blobs were read, or equality with HEAD was checked for cited files.
E references are relative to `exporter-lab/src`, commit `6a8f418f`.
The suggested `clones/kyverno` path was unavailable; the existing checkout
was used read-only.

## A. GlobalContext on background MutatingPolicy

### Access and scoping

VERIFIED: Yes. A cluster-scoped MutatingPolicy can consume the cached
Prometheus response during background mutate-existing evaluation.

- The background binary creates a GlobalContext store and polling controller:
  K `cmd/background-controller/main.go:223-238`.
- That same store enters the CEL context provider and MutatingPolicy engine:
  K `cmd/background-controller/main.go:319-325,384-396`;
  `pkg/cel/libs/context.go:99-108`.
- Compilation installs the GlobalContext library in the shared environment:
  K `pkg/cel/policies/mpol/compiler/compiler.go:58-60,199-205`.
- Background processing calls `Evaluate`, which selects target mode:
  K `pkg/background/mpol/processor.go:230`;
  `pkg/cel/policies/mpol/engine/engine.go:84-98`.

| CEL location | Background target behavior | Source, VERIFIED |
| --- | --- | --- |
| `matchConditions` | Can compile a GlobalContext reference, but these conditions are NOT evaluated as the background target gate. They serve the trigger/admission path. | K `pkg/cel/policies/mpol/compiler/compiler.go:77-84`; `compiler/policy.go:201-219` in that same mpol directory |
| `targetMatchConditions` | Yes. Evaluated for each target. Put namespace/label guards and the metrics condition here. | K `pkg/cel/policies/mpol/compiler/compiler.go:102-109`; `compiler/policy.go:201-209` |
| `mutations[].jsonPatch.expression` or `applyConfiguration.expression` | Yes. Uses the same extended compiler and runs after the target conditions. | K `pkg/cel/policies/mpol/compiler/compiler.go:135-152`; `compiler/policy.go:223-244` |
| `variables[].expression` | Yes. Bind one lazy variable to `globalContext.Get(...)` and reuse it in the target condition and patch. | K `pkg/cel/policies/mpol/compiler/compiler.go:72`; `compiler/policy.go:72-89,201-203` |

VERIFIED: No `--enableGlobalContext` switch is needed. The background
controller starts unconditionally: K `cmd/background-controller/main.go:443-445`.
The similarly named variable/getter in K `cmd/internal/flag.go:72,328-330`
has no flag registration or caller in this checkout.
Use `MutatingPolicy`, not `NamespacedMutatingPolicy`: namespaced policies
reject GlobalContext references. `--allowHTTPInNamespacedPolicies` only
permits their HTTP calls; it does not permit GlobalContext.
K `pkg/cel/policies/mpol/validate.go:40-47`;
`pkg/cel/compiler/http.go:62-82,90-94`.

VERIFIED: Use resource rules plus `objectSelector` and
`targetMatchConditions` for this scheduled experiment. Do not replace the
target list with `targetMatchConstraints.expression`: periodic requeue
explicitly excludes policies with that expression.
K `pkg/background/mpol/processor.go:111-145`;
`pkg/policy/policy_controller.go:699-705`.
`evaluation.background.enabled: false` does not disable mutation.
K `pkg/policy/policy_controller.go:599-603`.

### RBAC and HTTP reachability

VERIFIED: The field is `spec.apiCall.service.url` on GlobalContextEntry.
`ExternalAPICall` is the Go type, not a separate resource kind.
K `api/kyverno/v2beta1/global_context_entry_types.go:60-74,152-165`.

- VERIFIED: Chart RBAC already grants the background controller access to
  `globalcontextentries` and their status. K at the tag,
  `charts/kyverno/templates/background-controller/clusterrole.yaml:29-49`.
- VERIFIED: `apiCall.service` uses an HTTP client. It does not perform a
  Kubernetes GET of the Service object. `urlPath` instead calls the
  Kubernetes API through the controller client.
  K `pkg/engine/apicall/executor.go:54-77,80-105`.
  INFERRED: An anonymous Prometheus ClusterIP endpoint needs no extra
  Kubernetes resource grant for the HTTP request. It needs DNS and network
  reachability. An authenticated endpoint needs its own HTTP authorization.
  A Kubernetes `urlPath`, including a Service proxy path, needs the matching
  API permissions for the calling controller's ServiceAccount.
- VERIFIED: HTTP blocklist/allowlist also apply to `apiCall.service`.
  Defaults block loopback and metadata addresses, not all private ranges.
  Use the in-cluster Service address, not localhost.
  K `pkg/engine/apicall/executor.go:85-90,213-224`;
  `pkg/toggle/toggle.go:9-16,60-65`.
- VERIFIED: Job mutation separately needs `list`, `get`, and `update` on
  `batch/jobs` for the background ServiceAccount. The final write is an
  UPDATE even when the policy expresses a JSONPatch.
  K `pkg/background/mpol/processor.go:125,245,261`.
  Aggregate additional grants with
  `rbac.kyverno.io/aggregate-to-background-controller: "true"`.
  K at the tag, `charts/kyverno/templates/background-controller/clusterrole.yaml:9-14`.
  INFERRED: Verify both executors' action grants before comparing behavior.

### Refresh, scheduling, and failed calls

VERIFIED: `spec.apiCall.refreshInterval` defaults to `10m`.
There is no documented 1s or 1m minimum. Duration fractions are supported.
The webhook checks for zero, despite its error text saying greater than zero.
Use a positive duration such as `5s`; negative durations are not endorsed.
`retryLimit` defaults to 3 and has minimum 1. Code uses it as backoff steps,
so it bounds total attempts, not three additional retries.
K `api/kyverno/v2beta1/global_context_entry_types.go:154-177`;
`pkg/globalcontext/externalapi/entry.go:169-186`.

VERIFIED: A new entry starts its first call without waiting a full period.
Later periods start after the call/retry cycle completes. Slow calls extend
the effective refresh cadence.
K `pkg/globalcontext/externalapi/entry.go:82-110`;
dependency `k8s.io/apimachinery@v0.36.3/pkg/util/wait/backoff.go:166-172,240-260`
under `~/go/pkg/mod/` (version pinned at K `go.mod:84`).

VERIFIED: Cache refresh does not enqueue a MutatingPolicy or create a UR.
The poller only updates its cache and optional GCE status/events.
Policy events and the background ticker drive the separate mutation path.
K `pkg/globalcontext/externalapi/entry.go:86-110`;
`pkg/policy/policy_controller.go:591-604,645-655`.
The background binary defaults its scan to 1h; the lab's configured 60s
is independent of the GCE interval. K `cmd/background-controller/main.go:191-199`.
INFERRED: Record scrape, GCE refresh, and action times separately. A 5s GCE
refresh does not turn the existing 60s scan into a 5s action loop.

| Cache state | Result of reading the entry | Source, VERIFIED |
| --- | --- | --- |
| Entry not yet in store | Provider returns nil without error. Guard the result before reading fields. | K `pkg/cel/libs/context.go:115-124` |
| Entry exists, first data not ready | `no data available` error. | K `pkg/globalcontext/externalapi/entry.go:116-127` |
| Refresh/retries in progress | Previous successful data remains readable until the result is stored. | K `pkg/globalcontext/externalapi/entry.go:86-100,136-165` |
| Final transport/HTTP failure | Entry stores an error. Reads return that error, not old successful data. Non-2xx is an HTTP failure. | K `pkg/globalcontext/externalapi/entry.go:87-98,120-121,140-141`; `pkg/engine/apicall/executor.go:114-121` |
| Invalid JSON or projection failure | Entry stores an error. A later successful refresh replaces data and clears it. | K `pkg/globalcontext/externalapi/entry.go:143-165` |

VERIFIED: There is no cache-age expiry in `entry.Get`.
GCE `status.lastRefreshTime` is not the background controller's freshness
attestation. Its poller has `shouldUpdateStatus=false`; the admission
binary's poller has true. Status update also follows HTTP success even if
the internal JSON/projection decode failed.
K `cmd/background-controller/main.go:235`;
at the tag, `cmd/kyverno/main.go:536-550`;
`pkg/globalcontext/externalapi/entry.go:99-108,116-129,143-165`.

VERIFIED: An unmasked CEL condition error produces an evaluation error and
no patch. A false target condition produces a skip. A false condition can
also override another condition's error.
K `pkg/cel/policies/mpol/compiler/policy.go:42-69,201-209`;
`pkg/cel/policies/mpol/engine/engine.go:221-226`.
INFERRED, needs execution: For an error returned through a CEL GlobalContext
read, the UR may still complete successfully. `Evaluate` packages RuleError
inside its response and returns nil as the outer error. The processor only
collects that outer error, audits when a patched resource exists, and marks
success if its collected failures are empty. Do not use UR completion as
proof that the metrics condition ran successfully.
K `pkg/cel/policies/mpol/engine/engine.go:95-118,224-226`;
`pkg/background/mpol/processor.go:230-276,407-415`.

## B. Is Prometheus necessary?

VERIFIED: Direct GlobalContextEntry -> exporter `/metrics` does not work
as a metrics parser. The GCE code unconditionally JSON-decodes the body.
Prometheus/OpenMetrics exposition text is not decoded into CEL metric facts.
K `pkg/globalcontext/externalapi/entry.go:143-155`.

Direct CEL HTTP needs a version caveat. VERIFIED: Kyverno wires `http.Get`
(capital G), via its SDK HTTP implementation. The compiler itself adds no
text/OpenMetrics parser. K `pkg/cel/policies/mpol/compiler/compiler.go:252-255`;
`pkg/cel/compiler/http.go:24-43`; function spelling also appears in
K `pkg/toggle/toggle.go:60-61`.
The exact SDK revision pinned by v1.19.1, `68d74afcb07a` (K `go.mod:39`),
is absent from the local module cache.

VERIFIED only for cached SDK `e0dc6fb8661a`: HTTP attempts JSON decoding.
Decode failure yields `body: null` plus `statusCode`, with no raw-text
fallback. It does not necessarily raise an HTTP/CEL error for non-JSON.
Source under `~/go/pkg/mod/github.com/kyverno/`:
`sdk@v0.0.0-20260703121625-e0dc6fb8661a/extensions/cel/libs/http/http.go:286-297,318-340`.
INFERRED for the pinned SDK: the same JSON-only limitation likely applies.
Do not label that exact-version claim VERIFIED until its source or a live
plain-text response test is available. Mock HTTP CLI fixtures cannot prove it.

INFERRED recommendation: Keep tiny Prometheus. A custom text-to-JSON adapter
could remove Prometheus as a format bridge, so Prometheus is not structurally
mandatory. But the exporter emits current snapshots, not running-duration
history. A condition about N minutes needs a history owner in either design.
E `exporter/pkg/collector/collector.go:84-114`.

## C. Minimal honest Runtime Rules demonstrator

All contract choices below are INFERRED proposals, not implemented claims.
Scope: one cluster, Jobs, one active executor. Suspend writes must go through
Karta. Resume detection may be Job-specific in this prototype; say so.

1. Persist allowance state under `(rule UID, workload UID)`, with explicit
   rule revision and epoch counter. Record `openedAt`, observed suspend state,
   and the receipt that consumed the epoch. Pin the rule during the demo;
   define edits as retaining the current allowance, not silently resetting it.
   Open a fresh epoch when the live Job changes from the confirmed suspension
   to resumed state. On restart, reconcile persisted state with the live UID
   before acting. Do not use name, every generation increment, or every
   Running metric transition as an epoch. Call it an observed external resume;
   object state alone does not identify an authenticated human actor.
2. Persist one deterministic action intent per
   `(rule UID, workload UID, epoch, action)`. API create/conflict provides the
   claim; an in-memory set does not. Refuse to write the workload if persisting
   the intent fails. Treat an unresolved prior intent as blocking another
   action. Re-read UID/resourceVersion and mode before committing the action.
   A conflict requires re-evaluation, not transplanting an old decision.
3. Store receipts in `rr-system`, without workload ownerReferences or an
   automatic TTL. Proposed shape: `spec.ruleRef {uid, generation}`,
   `spec.targetRef {apiVersion, kind, namespace, name, uid, resourceVersion}`,
   `spec.epoch`, `spec.action`, `spec.definitionHash`, resolved field changes,
   and evidence `{query, values, sampleTimes, fetchedAt, window, threshold}`.
   Status records `Prepared`, `Applied`, `Skipped`, `Failed`, or `Unknown`,
   with reason, timestamps, and resulting resourceVersion. `Applied` means
   the API write succeeded, not that pods stopped. Namespace/cluster loss
   remains outside this durability claim. ConfigMaps with this payload are
   acceptable for a prototype; do not present them as a shipped receipt CRD.
4. Handle the two-write crash window. An API write and receipt completion
   are not atomic. If interrupted after mutation, recover from live state
   conservatively or report `Unknown`. Already-suspended state alone cannot
   prove which writer caused it. Demonstrate restart without duplicate action;
   do not promise exactly-once effects or general multi-writer arbitration.
5. Observe mode runs selection, fact checks, and eligibility, then persists
   an `Observed/WouldSuspend` assessment without writing the workload or
   consuming its action allowance. Use a separate assessment identity so an
   observe record cannot block a later action intent. Put the effective mode
   gate immediately before intent/action, and recheck before the target write.
6. Skip-and-report must distinguish unsupported handle, missing/stale facts,
   insufficient history, denied access, changed UID, and update conflict.
   An ordinary false condition is ineligible, not an infrastructure error.
   Never equate a nil library error with a completed suspend.
   VERIFIED trap: Karta `Component.Suspend()` silently succeeds when the
   suspend definition is absent. Check `HasSuspendDefinition()` first, apply
   the handle to the manifest, then persist the manifest through the API.
   E `pkg/resource/component.go:310-324`;
   `pkg/resource/accessor.go:329-345`.

### Comparison controls and traps

- INFERRED: Publish one condition definition before running: continuous
  sampled Running for N minutes, or elapsed allowance time while currently
  Running. These are different. Give both engines the same Prometheus query,
  scope, threshold, and freshness rules. A resume opens a new RR allowance;
  old samples cannot consume it immediately.
- INFERRED: Do not predict the earlier 43-second resume trap here. If a
  continuous-window query observes Running=0 during suspension, its predicate
  can become false after resume and naturally delay Kyverno's next action.
  Test both a suspension visible to scrapes and a rapid resume between scrapes.
  Compare measured epochs/receipts, not a manufactured unconditional baseline.
- VERIFIED: Workload metrics omit workload UID. The `uid` on
  `karta_pod_workload_info` is the Pod UID.
  E `exporter/pkg/collector/collector.go:13,45-62,87-99`.
  INFERRED: Use unique names for baseline runs. Test name reuse separately.
  For general behavior, add a shared identity-aware fact source or prove the
  full window belongs to the live UID. An RR receipt keyed by UID alone does
  not fix historical Prometheus series keyed by name.
- VERIFIED: Status is multi-hot and missing status is omitted, not zero.
  The catalog Job Running mapping requires active > 0 and ready > 0.
  E `exporter/pkg/collector/collector.go:101-113`;
  `pkg/catalog/kartas/batch_job.go:37-52`.
  INFERRED: `min_over_time(...[Nm]) == 1` alone does not prove an N-minute
  history when only a few samples exist. Validate window coverage and scrape
  freshness equally. Distinguish empty results, query failure, and false.
- VERIFIED: The exporter's last-event timestamp changes on watch events;
  it is not a periodic heartbeat. `/metrics` gates initial readiness only.
  E `exporter/pkg/controller/controller.go:107-110,343-344`;
  `exporter/pkg/server/server.go:27-38`.
  INFERRED: A quiet healthy workload need not produce recent events. Successful
  scraping alone also does not prove a watch cache is current. Bound the lab
  claim accordingly; neither engine gets a stronger truth source for free.
- VERIFIED: Kyverno can scope background targets and read shared context,
  as established in A. It also emits reports on successful/no-op mutation
  evaluations. K `pkg/background/mpol/processor.go:235-243,270-273`.
  INFERRED: Compare explicit allowance and retained receipt contracts.
  Do not claim Kyverno cannot read metrics, cannot scope, or has no reporting.
  Annotation-based epoch protocols or a companion receipt controller remain
  possible compositions; this test does not disprove them.
- INFERRED: Use disjoint labeled Jobs or run sequentially. Capture live
  pre-state, exact policy/query, all three timing intervals, allowed verbs,
  metric samples, target UID/resourceVersion, and receipts. Minimum trials:
  act; resume; restart; workload deletion with receipt retained; observe;
  missing handle/RBAC/facts. Add intent-write failure and crash-after-action
  before claiming the corresponding guarantees. Report pending tests as such.

The defensible result is a comparison of two authored execution contracts
using the same facts. It is not evidence that Kyverno lacks runtime metric
access or that the prototype already supplies a production governor.
