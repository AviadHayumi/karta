<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Exporter integration: Codex review, round 2

VERIFIED = source read. EXECUTED = supplied capture, not a new Codex run.
INFERRED = explanation or proposed behavior awaiting a discriminating test.
K paths refer to `~/workspace/kyverno`, tag v1.19.1,
commit `40ec788d48bb28d83dbf85538e962a59db9d45c6`.
L paths refer to this `exporter-lab` directory.

## A. The saved GCE condition requests a nonexistent projection

VERIFIED: There is a concrete manifest error before the store/race hypotheses.
L `manifests/05-kyverno-metric-suspend.yaml:30-31` calls:

```cel
object.metadata.name in
globalContext.Get('running-2m', 'data.result[].metric.workload')
```

The second argument is a stored projection name, not a JMESPath expression
evaluated at read time. L `manifests/04-kyverno-gce.yaml:9-14` defines no
projections. The entry stores the full response under the empty-string key.
Named projections are computed only from `spec.projections` during refresh.
Reading the supplied nonexistent key returns `no data available`.
K `pkg/globalcontext/externalapi/entry.go:64-73,116-129,154-164`;
`pkg/cel/libs/context.go:115-124`.

INFERRED: This error explains successful polls, no missing-entry V(2) log,
and a patchless evaluation without requiring different stores. It is not
proof that the entry itself was absent. The nil/null explanation in the log
is an alternative code path, not an observed diagnosis.

Proposed corrected condition, retaining the existing namespace and suspend
guards (not yet executed):

```cel
object.metadata.name in
globalContext.Get('running-2m', '').data.result.map(r, r.metric.workload)
```

Alternatively, add this under the GCE `spec` and call
`globalContext.Get('running-2m', 'workloads')`:

```yaml
projections:
  - name: workloads
    jmesPath: data.result[].metric.workload
```

VERIFIED: Projection fields are `name` and `jmesPath`.
K `api/kyverno/v2beta1/global_context_entry_types.go:182-188`.
INFERRED test order: first use the empty projection and CEL traversal;
then test a named projection. Preserve both the broken and corrected
manifests/captures. If the debug annotation probe also failed with the
empty projection, its exact manifest is needed: no saved debug manifest
was present in L `manifests/` at review time. Moving the same bad lookup
into a mutation would preserve the error.

R1 correction: source wiring supports background GlobalContext reads.
That was not an executed compatibility claim. This experiment establishes
a silent failure for the saved expression; it does not yet establish that
correct GlobalContext reads fail in the background. The source-verified
projection mistake must be removed before making that broader claim.

### Exact error path and discard point

All steps below are VERIFIED in the tag. A target condition error is not
converted to a no-match inside the engine; the caller loses the distinction.

| Step | Result | K source |
| --- | --- | --- |
| Processor calls target engine | Calls `p.engine.Evaluate(...)`. | `pkg/background/mpol/processor.go:230` |
| Engine selects target evaluation | Calls `handlePolicy(..., true)`, then `EvaluateTarget`. | `pkg/cel/policies/mpol/engine/engine.go:95-98,215-219` |
| CEL condition fails | `ContextEval` errors are accumulated; returned unless another condition is false. | `pkg/cel/policies/mpol/compiler/policy.go:42-69` |
| Target evaluator returns error | `EvaluationResult{Error: err}`, without a patched resource. Patch-expression errors use the same shape. | `pkg/cel/policies/mpol/compiler/policy.go:201-209,234-236` |
| Engine preserves error as a rule result | `RuleError("evaluation", ...)`, then returns no patched object. | `pkg/cel/policies/mpol/engine/engine.go:224-226` |
| Outer engine return | Stores the rule result in `response.Policies`; returns `response, nil`. | `pkg/cel/policies/mpol/engine/engine.go:98,118` |
| Processor discards the rule error | Checks only the outer `err`. Its action AND audit block requires `response.PatchedResource != nil`. No inspection of `response.Policies[].Rules` follows. | `pkg/background/mpol/processor.go:230-276` |
| UR succeeds | With no other accumulated failure, calls `statusControl.Success`. | `pkg/background/mpol/processor.go:276,407-415` |

The operative discard is the processor's `if response.PatchedResource != nil`
at line 235 combined with the missing rule-error handling before line 276.
The nil Go error at engine line 118 is consistent with an engine API that
returns rule outcomes in its response; the processor must consume them.
INFERRED fix direction: inspect rule errors independently of the patch;
propagate them to UR failure and emit diagnostics for patchless evaluations.
Keep ordinary no-match/no-op behavior separate. Do not turn every skip into
an error.

VERIFIED caveat to "nothing anywhere": the metrics wrapper inspects rule
results even when no patch exists. It records duration and result status.
K `pkg/cel/policies/mpol/engine/metrics.go:18-33,65-74`;
`cmd/background-controller/main.go:390-396`.
INFERRED: An error counter can remain visible when metrics are enabled.
The saved captures do not inspect that surface. No success receipt or
per-target error message is implied by this counter.

### Same store; no demonstrated early-compile race

VERIFIED: The same `gcstore` instance is captured by the leader closure.
It is created once at K `cmd/background-controller/main.go:223`, passed to
the GCE controller at line 231 and to `NewContextProvider` at line 322.
There is no intervening `gcstore` declaration. The provider stores that
reference and assigns `LibraryContext` at K `pkg/cel/libs/context.go:99-108`.

VERIFIED startup order in K `cmd/background-controller/main.go`:

1. Leader callback enters at line 306.
2. Real library context initializes at lines 319-325.
3. Manager is constructed and started at lines 351-370.
4. MutatingPolicy compiler/provider are created at lines 377-384.
5. Provider registers reconciler controllers, which compile on reconciliation.
   K `pkg/cel/policies/mpol/engine/provider.go:51-53,103-110`;
   `engine/reconciler.go:98,127-129` in that same mpol directory.

VERIFIED: Compilation captures `libs.GetLibsCtx()` in its environment.
The helper can create a fake context when unset, but `NewContextProvider`
overwrites it with the real provider. K `pkg/cel/policies/mpol/compiler/compiler.go:58-60,203-205`;
`pkg/cel/libs/context.go:39-44,107`.
The `contextProvider` argument to `EvaluateTarget` is not used to rebind the
compiled environment: K `pkg/cel/policies/mpol/compiler/policy.go:135-143`.
Passing the correct provider at evaluation time would not repair a wrongly
captured compile-time provider. The startup ordering is the relevant evidence.
INFERRED: The proposed "mpols compile before leadership initializes context"
mechanism is not supported by this startup ordering. A global mutable
variable deserves tests, but it is not evidence of this race.

VERIFIED: GCE polling starts outside leader election; that explains polls
being possible before policy compilation. K `cmd/background-controller/main.go:443-450`.
Admission webhooks run in the separate admission binary, not in this
background process. Its validation compile is also not a compiled program
transferred into the background process. K at the tag,
`cmd/kyverno/main.go:690-706,891-901`;
`pkg/webhooks/policy/handlers.go:61-68`;
`pkg/cel/policies/mpol/validate.go:26-29`.

## B. Upstream search and main comparison

Web searches on 2026-09-14 covered `globalContext background`,
`GlobalContextEntry mutate existing`, `gctx background controller`,
and MutatingPolicy/RuleError/projection variants in kyverno issues and PRs.
Direct `gh api` search could not connect from the shell. Web results are
not an exhaustive issue inventory.

- VERIFIED, web: [issue #13871](https://github.com/kyverno/kyverno/issues/13871)
  reports that newly created GCEs need a reports-controller restart on
  v1.15.0. It is closed as not planned. This concerns a resource-backed GCE
  and reports, not this external-API projection lookup. It is a related
  symptom, not a confirmed duplicate or a v1.20 fix.
- VERIFIED, web: [PR #16099](https://github.com/kyverno/kyverno/pull/16099)
  merged May 14, 2026. It fixes per-loader deferred-loading state for legacy
  `globalReference` injection. It does not fix CEL projection lookup or the
  processor's ignored RuleError. The distinction is visible in
  K `pkg/engine/context/loaders/globalcontext.go:47-62` versus
  `pkg/cel/libs/context.go:115-124`.
- VERIFIED, repo and release page: the GCE lifecycle deadlock fix #16904
  was backported as #17454 and is already in v1.19.1. Local commit
  `91283c385` is an ancestor of the tag. The implementation changes stop
  handling and atomic projection replacement, not result propagation.
  K `pkg/globalcontext/externalapi/entry.go:51-60,132-134,154-165`.
  The [release changelog](https://github.com/kyverno/kyverno/releases/tag/v1.19.1)
  lists the backport. The individual PR pages failed to load during this
  review; the patch and ancestry were checked locally.

VERIFIED, repo: `git diff v1.19.1 HEAD` is empty for the inspected background
entry point, mpol compiler/engine/processor, CEL context provider, and external
API entry files at supplied main `3bb926936656bfb14ae1a6306a6d9208a4b77613`.
The discard point therefore remains in this supplied 1.20-development tree.
INFERRED: No exact upstream issue/fix was identified in the bounded search.
Do not claim that no issue exists or that all future 1.20 builds retain it.

Proposed issue framing: "MutatingPolicy mutate-existing completes UR and
omits per-target diagnostics after a CEL evaluation error." Reproduce with
one isolated Job and a data-dependent invalid conversion such as
`int(object.metadata.name) > 0` for a nonnumeric name. That removes GCE,
Prometheus, and projection configuration from the diagnostic defect.
This is a proposed test, not an executed result.

## C. Corrections to the evidence and comparison wording

EXECUTED: L `captures/10-v2-http-timeline.txt:1-3` brackets policy apply and
the first observed suspended state by 644ms. Call it "suspended by the first
poll, 644ms after the apply marker." It is an observation bound, not a
measured internal action duration or a scan-latency guarantee.

EXECUTED: L `captures/09-ab-test.txt:1-3` records 17:36:45.289935 to
17:36:45.461: about 171.1ms, not 411ms. The three-line capture contains no
managedFields output. Keep any separate actor evidence as a separate capture.
L `captures/08-kyverno-suspend-timeline.txt:1-2` only saves the initial state.
It does not itself document an eight-minute observation period or UR states.
The v=6 body logs, UR listings, and debug annotation manifest described in
the assignment were not present as dedicated captures at review time.
Treat those as operator-reported execution until their artifacts are saved.

INFERRED: The proposed two-rule comparison is fair with these qualifications:

| Rule/contract | Defensible wording |
| --- | --- |
| Point-in-time Kyverno condition | While the condition keeps holding and users repeatedly resume, this rule can re-suspend on later evaluations. It contains no lifetime action cap. |
| Windowed Kyverno condition | If a scrape observes Running=0, the window can delay eligibility after resume. The rule can act again after its condition recovers; it still has no lifetime cap. |
| RR, one action then escalate | The prototype adds a persisted per-rule/per-workload action cap across epochs. Later eligibility produces an escalation receipt instead of another automatic suspend. |

VERIFIED mechanism: periodic requeue and no-op write suppression are at
K `pkg/policy/policy_controller.go:645-655,699-705` and
`pkg/background/mpol/processor.go:235-243`.
INFERRED: "Forever unbounded" requires repeated external resumes and continued
eligibility. Kyverno does not keep writing an already-suspended Job every tick.
"<= scan" is not an SLO: include polling, exporter/Job status lag, queueing,
and API failures. "After the window" requires a sampled false state and
sufficient history; rapid resume between scrapes can behave differently.
`min_over_time(...[2m]) == 1` alone still does not prove two full minutes of
sample coverage. Do not label report flip-flops EXECUTED without report
captures; L `captures/11-resume-trap-kyverno.txt:1-2` only records a resumed
state at this review snapshot.

Use distinct names for `windowSeconds`, `epoch`, and `maxAutomatedActions`.
An epoch can reset the allowance timer while leaving a lifetime action cap
consumed. With cap=1, a later epoch grants observation time before escalation,
not another automatic intervention. That is an explicit refinement of the
earlier fresh-allowance-on-resume contract, not a consequence of epochs alone.
Define when that escalation occurs: immediately on resume or only when the
post-resume allowance and condition are both satisfied.

## D. RR draft: contract review

The Python draft appeared during review. Findings below are VERIFIED against
L `rr-poc/rr_poc.py`, SHA256
`4b9aa7082e9843c2a7322b5a3f90436dea38199ff85791c5e1193bb9d59eeb91`.
Fixes are INFERRED recommendations. No RR scenario was executed by Codex.

### MUST-FIX before claiming the execution contract

1. Intent persistence does not gate the action. `Receipts.write()` returns
   None on failure, but lines 340-344 patch anyway.
   L `rr-poc/rr_poc.py:139-144,340-344`.
   Require confirmed intent creation before the target write. Persist a
   deterministic operation key `(rule identity, workload UID, epoch, action)`;
   run-id plus a local sequence is not a restart-safe claim.

2. Restart and unknown-outcome handling do not enforce one-shot behavior.
   State-load errors are discarded, state-save failures are ignored, and
   state is saved only after action/receipt work. Nothing reloads unresolved
   intents or reconciles successful outcome receipts into state.
   L `rr-poc/rr_poc.py:155-171,340-354`.
   `subprocess.TimeoutExpired` also escapes the string-based Unknown branch;
   a classified Unknown leaves the allowance free to retry next loop.
   L `rr-poc/rr_poc.py:53,350-352`.
   Refuse fresh adoption when persisted state cannot be read. Persist the
   pending operation before action; block another action while it is Unknown.
   A missing response is not proof that the patch failed. Cover crashes after
   API success and before outcome/state save. No atomic two-write claim.

3. Observe consumes the real action allowance and can prevent later Enforce
   from acting. L `rr-poc/rr_poc.py:328-331` sets `actedInEpoch=True`.
   Use a separate assessment marker. WouldAct must not consume the automatic
   action cap, including when writing the assessment fails. Also run preflight
   in Observe if WouldAct is meant to include action readiness; currently the
   mode branch precedes preflight at lines 333-338.

4. The mutation has no UID/resourceVersion precondition or final eligibility
   check. A named Job can be replaced or changed between list and merge patch.
   L `rr-poc/rr_poc.py:196-205,342-344`.
   Re-read the live object and apply an API write guarded by its UID and
   resourceVersion. On conflict, refresh facts and decide again. Do not copy
   a stale decision onto a new version. Include these identities in receipts.

5. State and epoch semantics differ from the assignment. The key is only
   workload UID, not rule plus UID; there is no adoption/resume timestamp;
   `consumedEpochs` increments on resume, not on a successful intervention.
   L `rr-poc/rr_poc.py:161-162,267-290`.
   `actionsPerEpoch` in L `rr-poc/rules.yaml:20` is not read; the one-action
   behavior is hardcoded. Persist a stable rule identity and actual action
   consumption. For now, name the narrower behavior "one initial action,
   then no re-arm after observed external resume." There is no independent
   post-resume time allowance in this draft; only the Prometheus window.

6. Skip-and-report has silent paths. List errors are ignored, query failures
   are log-only, and receipt failures can still set deduplication markers.
   L `rr-poc/rr_poc.py:196-198,261-264,295-314`.
   Emit a durable rule-level failure when targets/facts cannot be enumerated;
   do not require a workload receipt when the workload set is unknown. Advance
   a receipt-deduplication marker only after persistence succeeds. Validate
   Prometheus `status`, result shape, value semantics, and freshness. Currently
   result membership alone is eligibility at lines 179-185 and 285-286.

7. Preflight is conditional on `--impersonate`, not unconditional. Reads,
   state, and receipts still use the ambient identity; only preflight and
   workload patches impersonate. L `rr-poc/rr_poc.py:48-52,157,171,196,224-228,333-344`.
   Either test an explicit actor identity throughout, or disclose this split.
   Check both action and intent-storage capability. A positive access review
   is not a promise that a later write passes admission or still has access.

### Resume detection: manager metadata is supporting evidence only

VERIFIED: The code does not actually require `manager != rr-poc`. It computes
`foreign` only for receipt text; the epoch opens on `weSuspended && !suspended`
regardless. L `rr-poc/rr_poc.py:270-283`.
That transition-based approach is more defensible than treating manager
strings as identities, but label it "external resume detected; actor unknown"
when no actor evidence exists. `resumedBy` currently overstates the metadata.

VERIFIED in the local Kubernetes checkout at `121abd026c869bdb9b3896ac64190efcdb8d70cb`
(paths below relative to `~/workspace/kubernetes`):

- `kubectl get -o json` normally omits managedFields unless
  `--show-managed-fields=true` is used. RR's `kubectl_json` does not set it.
  `staging/src/k8s.io/cli-runtime/pkg/genericclioptions/json_yaml_flags.go:74-87`;
  `staging/src/k8s.io/cli-runtime/pkg/printers/managedfields.go:44-56`;
  L `rr-poc/rr_poc.py:57-61,196`.
  Output omission is not evidence that PATCH erased stored metadata.
- Normal `kubectl patch` defaults its field manager to `kubectl-patch` and
  sends it. Without an explicit manager, the apiserver can infer one from
  User-Agent. `staging/src/k8s.io/kubectl/pkg/cmd/patch/patch.go:143,254-259`;
  `staging/src/k8s.io/apiserver/pkg/endpoints/handlers/create.go:259-270`.
  The [Kubernetes field-manager documentation](https://v1-36.docs.kubernetes.io/docs/reference/using-api/server-side-apply/#field-managers)
  confirms that these identify workflows and can be supplied by the caller.
- managedFields is not an ordered actor audit trail. An entry timestamp can
  update when any of its owned fields changes. Callers can also explicitly
  reset managedFields. `staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/types.go:1390-1407`;
  `staging/src/k8s.io/apimachinery/pkg/util/managedfields/internal/fieldmanager.go:96-105,169-179`.
- The Python search matches any nested `f:suspend` string, not specifically
  `fieldsV1['f:spec']['f:suspend']`, and returns every matching manager without
  identifying the transition. L `rr-poc/rr_poc.py:210-215`.

INFERRED minimum: persist the last confirmed action's UID, observed suspend
state, epoch, and operation receipt. For the same UID, observe true -> false
once and persist a new epoch before further decisions. After restart, compare
the live state with the persisted action and unresolved intent. A poller
cannot reconstruct every transition that happened between polls. Preserve
an unknown actor; require an audited resume request if human attribution is
part of the contract. Test missing metadata, explicit manager `rr-poc`,
another controller's resume, metadata-only updates, and restart while resumed.

### Scope and naming corrections

- VERIFIED: Receipts are new ConfigMaps with `supersedes`, not a single object
  updated from Intended to Executed. They have no workload ownerReference,
  which supports retention after workload deletion. They do not yet contain
  rule UID/revision, target group/version/resourceVersion, or a definition hash.
  L `rr-poc/rr_poc.py:114-149`.
- VERIFIED: The cap is `reArmOnResume=false`, and its emitted phase is
  `EscalatedNeedsHuman`. There is no `SkippedAllowanceConsumed` phase in this
  snapshot. `maxActionsPerLoop` is a separate rate budget, not a lifetime cap.
  L `rr-poc/rr_poc.py:246-247,295-303,324-325`.
  INFERRED: A ConfigMap escalation is a persisted escalation request, not a
  notification delivery or a human acknowledgement.
- VERIFIED: Python reads catalog YAML and translates selected paths itself;
  it does not invoke Karta's Go suspend implementation. Catalog lookup is keyed
  by Kind alone, and the translator only rejects bracket syntax and decodes
  boolean strings. L `rr-poc/rr_poc.py:64-105`.
  INFERRED: Call it a catalog-handle interpreter for the tested Job/CronJob
  paths. Reject every unsupported path/value shape explicitly. General Karta
  actions use JQ selectors and JSON values, including arrays and null; a
  dotted merge-patch translator does not preserve all those semantics.
  Karta source: L `src/pkg/resource/accessor.go:329-343`.
- VERIFIED: `rules.yaml` is read as a local file, not watched as an installed
  RuntimeRule CRD; mode is fixed at startup. L `rr-poc/rr_poc.py:242-247`.
  INFERRED: Describe the gate as a startup mode unless dynamic reload is added.
  Baseline comparisons should use the same Job subset. Deployment no-handle
  and CronJob behavior are additional capability tests, since the Kyverno
  baseline only selects Jobs (L `manifests/05b-kyverno-metric-suspend-http.yaml:13-16`).

Recommendation: rerun the corrected empty-projection GCE expression before
declaring that integration broken. Preserve the separate RuleError diagnostic
finding. The bounded-with-escalation RR story is defensible after persistence,
Observe, and restart gaps are fixed and the action cap is named explicitly.
