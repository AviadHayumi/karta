<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Exporter + Kyverno + Runtime Rules: the integration lab log

Executed on two kind clusters, both Kyverno v1.19.1 (chart 3.9.1) with
a 60s background scan: kind-kyverno-lab (Kubernetes v1.34.0, where the
Job-resume platform bug lives) and kind-kyverno-lab2 (v1.34.3, fixed),
where the clean side-by-side ran. Quotes are captures unless marked as
edited excerpts or operator-reported. Peer review: Codex gpt-6-astra
(thread 01a08c11), engaged at every milestone; four review rounds, all
applied.

## 1. What happened on aviadhayumi/karta

Two branches on the fork:

- `metrics-exporter` (6a8f418f, 17 commits, ~5,200 added lines): a full
  `exporter/` Go module - `karta-exporter` binary with packages
  attribute, collector, controller, owner, registry, server, state,
  store; unit tests next to each; Dockerfile + Makefile; Helm chart
  templates under `charts/karta/templates/exporter/` (deployment,
  aggregated RBAC, service, servicemonitor, prometheusrule,
  networkpolicy); recording rules + promtool tests; a metrics guide in
  `docs/Metrics Exporter.md`; design doc. Commit trail shows external
  review applied (`fix(exporter): apply external review findings`).
- `kep-metrics-exporter` (c86fa4be): KEP-0004 with three excalidraw
  diagrams, stacked on top of KEP-0003 (runtime rules).

Local docker images `karta-exporter:demo` .. `:demo5` exist - the branch
was built and exercised from this machine before.

What the exporter does (from code + KEP-0004):

- Watches Karta CRs (registry validates, picks one Karta per root
  group+kind, oldest wins; `--use-catalog` seeds the built-in catalog of
  20 kinds as a fallback tier; the bare-Pod entry is skipped).
- One full informer per described root kind, metadata-only informers per
  child kind, one trimmed pod informer (metadata + nodeName + phase).
- Attributes every pod to its outermost described workload by walking
  owner references through an index (parks pods whose owner has not
  arrived yet; pods can be middle owners - LeaderWorkerSet).
- Emits the identity join series `karta_pod_workload_info` plus the
  description-derived state: `karta_workload_status{phase}` (dense over
  9 normalized phases), `karta_workload_component_replicas`,
  `..._component_pods`, `karta_workload_generation`,
  `karta_workload_info`.
- Holds no telemetry values: recording rules join the identity series
  with DCGM/cadvisor series at query time.
- Freshness witness: `karta_exporter_last_event_timestamp_seconds`;
  degradation is explicit (`component="<unknown>"`, attribution error
  counters by reason).

The metric contract (public, additive-only): namespace, workload,
workload_kind, workload_group on every workload-scoped series;
workload_version only on `_info`.

Why this matters for the comparison: the normalized `Running/Suspended/
Failed` phase per workload is not a raw object field - here the Karta
description's statusMappings compute it (authored per-kind queries or
another provider could supply an equivalent). Both integrations below
consume the same fact from the same series.

## 2. Lab topology

```text
kind-kyverno-lab
  kyverno/            admission + background + cleanup + reports (v1.19.1, 60s scan)
  karta-system/       karta-exporter :lab (--use-catalog), Karta CRD installed
  monitoring/         prometheus v2.53.0, 5s scrape of the exporter
  ai-team/            demo workloads (Job trainer-*, CronJob nightly-report)
  rr-system/          runtime-rules prototype (later)
```

Interference cleanup before the runs: `default-job-deadline` and
`lab-suspend-idle` MutatingPolicies from the mastery-deck battery were
deleted; they match batch/v1 Jobs cluster-wide.

## 3. Deploying the exporter

Steps executed (manifests in `manifests/`, outputs in `captures/`):

1. Built the image from the branch worktree: `make -C exporter
   docker-build IMAGE=karta-exporter:lab` (multi-stage, go 1.26.3),
   `kind load docker-image karta-exporter:lab --name kyverno-lab`.
2. Applied the Karta CRD from `charts/karta/crds/run.ai_kartas.yaml`
   (the exporter watches `kartas.run.ai` unconditionally; without the
   CRD its informer never syncs and readiness stays false).
3. Deployed exporter with `--use-catalog` and flat lab RBAC
   (`manifests/01-exporter.yaml`), Prometheus with a 5s scrape
   (`manifests/02-prometheus.yaml`), demo workloads trainer-a,
   trainer-b (Jobs, sleep 86400), nightly-report (CronJob)
   (`manifests/03-workloads.yaml`).

What the startup log showed (`captures/03-exporter-watchers.txt`): five
watchers started for the kinds this cluster has (Job, CronJob,
Deployment, StatefulSet as roots; ReplicaSet as a metadata-only child)
and nineteen catalog GVKs failed their REST mapping and were skipped
with a logged reason each - no crash, no wildcard RBAC. (The catalog
carries twenty definitions, including two API versions of one kind;
the bare-Pod entry stays opt-in by design.)

First real capture (`captures/02-metrics-raw.txt`): both trainer pods
attributed via `karta_pod_workload_info`, dense 9-phase status per
workload, and the whole cluster described for free - kyverno's own four
deployments, coredns, prometheus all appear as workloads with
`workload_kind="Deployment"`. The suspended mastery-deck leftover
`lab-training` correctly reads `Running 0`.

### Finding 1 (executed): the policy engine broke the metrics plane

First Prometheus scrape: target `down`, `dial tcp 10.96.177.206:8080:
i/o timeout`. Cause (`captures/04-netpol-interference.txt`): the
mastery-deck GeneratingPolicy `default-netpol` stamped a default-deny
ingress NetworkPolicy into every namespace created after it - including
`karta-system` and `monitoring`, 86 seconds old at capture time. kind
enforces these now. Deleting the GeneratingPolicy and the generated
netpols fixed the scrape (`captures/05-prom-first-query.txt`:
trainer-a 1, trainer-b 1, nightly-report 0).

Lesson for the log: a cluster-wide generate rule silently applied to
infrastructure namespaces; nothing in the policy's status pointed at
the broken scrape. Scoping generate rules away from platform
namespaces is on the rule author.

## 4. Kyverno consuming exporter facts

The rule, in words: suspend any ai-team Job whose available exporter
samples in the trailing two-minute range all read Running. The fact is
`min_over_time(karta_workload_status{namespace="ai-team",
phase="Running",workload_kind="Job"}[2m]) == 1` - min_over_time tests
the samples that exist; it does not check that the range is fully
covered or that scrapes were healthy (capture 30 shows a first
suspension 67.6s after the job's startTime, on partial-range samples).
The normalized phase itself comes from the Karta description's
statusMappings; the lab obtains it from Karta, though authored queries
or another provider could supply an equivalent.

### Attempt 1: GlobalContextEntry - silent no-op (my bug + their swallow)

Post-review correction (Codex r2, accepted): the root cause of the
no-op is a bug in MY policy, not a missing capability. The second
argument of `globalContext.Get(name, projection)` is the NAME of a
projection declared under the GCE's `spec.projections`; it is not a
JMESPath evaluated at read time. My GCE declared no projections, so
`Get('running-2m', 'data.result[].metric.workload')` asked for a
projection that does not exist, which errors with `no data available`
(kyverno `pkg/globalcontext/externalapi/entry.go:116-129,154-164`).

What stands, and is the finding: Kyverno swallowed that error on the
acting path end to end. Codex pinned the discard (all file:line at tag
v1.19.1, `codex-notes-r2.md` section A): the CEL error becomes
`EvaluationResult{Error}` (`pkg/cel/policies/mpol/compiler/policy.go:201-209`),
the engine stores it as a rule result and returns no patched object with
a nil Go error (`pkg/cel/policies/mpol/engine/engine.go:224-226,98,118`),
and the background processor only inspects `response.PatchedResource`,
never `response.Policies[].Rules` - so the UR completes as a success
(`pkg/background/mpol/processor.go:235,276`). A separate reporting scan
of dbg-gctx encounters the same bad projection in its MUTATION
expression and emits the captured error Event (`failed to get global
reference: no data available`, `captures/14-gctx-error-event.txt`) -
so a diagnosis of this error class can exist on the reporting surface
while the acting surface discards it.

`manifests/04-kyverno-gce.yaml` + `manifests/05-kyverno-metric-suspend.yaml`:
a GlobalContextEntry polls the Prometheus query API every 15s, the
MutatingPolicy's targetMatchConditions check
`object.metadata.name in globalContext.Get('running-2m',
'data.result[].metric.workload')`.

Executed result: no mutation for 8+ minutes across many scans
(operator-reported duration; `captures/08-kyverno-suspend-timeline.txt`
saved only the initial sample). The evidence that narrowed it:

1. The GCE polls worked: `status.lastRefreshTime` advanced and at v=6
   the background controller logged `api call success` every 15s with
   the right body (`captures/07b-gce-poll-v6.txt`).
2. The A/B twin (`captures/09-ab-test.txt`): the identical policy with
   the globalContext condition removed suspended its target 171ms after
   the apply marker (17:36:45.289 -> .461; the writer was
   background-controller per managedFields, checked separately). The
   machinery is fine; the gctx read is the failing part.
3. The debug policy (gctx call moved into the mutation, writing the
   value to an annotation): same outcome, no annotation, URs completed
   (`manifests/90-debug-gctx-annotation.yaml`).

Finding 2, as it settled after review: a CEL evaluation error on the
background acting path is indistinguishable from no-match. The trigger
here was the wrong projection argument (reads error with `no data
available`); any runtime CEL error takes the same path, proven
independently by the minimal repro in capture 31 with no GlobalContext
involved - see its bounded claims below.

Also noted during the broken period: the reporting scan produced
`fail: mutation is not applied` results for trainer-a and for
`kyverno-migrate-resources` in the kyverno namespace. The report scan
evaluates the policy with target=false semantics - matchConditions, not
the failing targetMatchConditions - so it was reporting an unapplied
mutation from its own successful simulation, not relabeling the error
(verified: `pkg/controllers/report/utils/scanner.go:243-264`,
`pkg/cel/policies/mpol/engine/engine.go:158`,
`pkg/cel/policies/mpol/compiler/policy.go:201-219`). The out-of-scope
kyverno-namespace result mirrors Experiment E's asymmetry from the
other side.

### Attempt 2: CEL http.Get (works)

`manifests/05b-kyverno-metric-suspend-http.yaml` replaces the gctx read
with a direct `http.Get(<prometheus query url>)` in the same condition.

Executed (`captures/10-v2-http-timeline.txt`):

```text
v2 apply at 17:45:19.049 UTC
mutatingpolicy.policies.kyverno.io/metric-suspend-running configured
t+1 17:45:19.693 trainer-a suspend=true
```

644ms from policy apply to suspension, driven by the exporter's
normalized Running window. Kyverno + exporter integration works.

Tradeoffs of the working path (vs the GCE design): the HTTP call runs
per evaluation (per target, per scan) with no caching and no shared
refresh; and an HTTP failure is another silent skip of the same class -
there is no failure-mode contract on a condition error.

### Attempt 1c: GlobalContext with a declared projection (works)

Codex's correction executed in an isolated namespace (gce-test, its own
GCE + policy, `captures/19-gce-corrected.txt`): with

```yaml
projections:
  - name: workloads
    jmesPath: data.result[].metric.workload
```

on the GCE and `globalContext.Get('running-2m-gcetest', 'workloads')`
in the condition, the fresh job was suspended at 18:09:59, about 2.5
minutes after setup. Settled capability verdict: GlobalContext works in
background mutate-existing conditions on v1.19.1. A NAMED projection
must be declared before reading it by name; per source, the
empty-string projection returns the whole response body (untested
here). The remaining findings are the silent acting-path error discard
(above) and the ergonomics: a wrong projection name fails silently on
the acting path, and refresh frequency is a GCE-side setting, not per
rule.

### Finding 3 (executed): the suspend/resume cycle wedged the Job controller

While arming the resume-trap capture, trainer-b got stuck in a state
worth the whole lab: `spec.suspend=false`, its pod 1/1 Running for 8+
minutes, but `status.active=0` and a stale `Suspended=True` condition
from the earlier suspension. kube-controller-manager, every 60s,
repeatedly during the captured period (`captures/12-job-wedge-kcm.txt`):

```text
E0914 17:45:32 job_controller.go:652 "Unhandled Error" err="syncing job:
tracking status: adding uncounted pods to status: Job.batch \"trainer-b\"
is invalid: status.startTime: Required value: startTime cannot be
removed for unsuspended job"
```

What actually happens (Codex r3 pinned it, then identified it
upstream): on resume, the Job controller RESETS `status.startTime` to
now - by design, to restart activeDeadline timing
(`pkg/controller/job/job_controller.go:1047-1058` at v1.34.1). Since
v1.32 (JobManagedBy graduated on), status validation rejects ANY change
to a non-null startTime while the job is unsuspended - the check is
value inequality, and its error text ("cannot be removed") is simply
misleading (`pkg/apis/batch/validation/validation.go:698-704`). The
controller's workqueue then retries the same recomputed transition
forever - the Suspended=True condition never flips, so every retry
attempts the same rejected startTime reset (`Resumed x15` events,
`captures/13-trainer-b-wedged.txt`).

This is Kubernetes issue kubernetes/kubernetes#134521 (opened
2025-10-10), fixed by #134769 (master) and backported in #135130 -
shipped in v1.34.2. The lab's kind node is v1.34.0. Under these
defaults, a previously-started Job that gets suspended keeps its old
startTime, and any later resume triggers the rejected reset (executed
five resume attempts across three Jobs, captures 15/16/16b/16c; the
retry loop is inferred to continue indefinitely from the mechanism, the
captures cover minutes). The manual `--subresource=status` patch
rejection most plausibly hit the same value-inequality guard - inferred:
its raw payload was not saved.

Why it matters here: the workload looked resumed (pod Running), its
status said Suspended, and the exporter faithfully mirrored the object
(`karta_workload_status{phase="Running"} 0`). Nothing on the engine's
inspected surfaces reflected that the workload behind its action was
broken - the reporting scan said `fail: mutation is not applied`, which
reads as the opposite problem. A governor that verified action outcomes
(did the resume converge within N seconds?) could have filed an
Unknown-outcome receipt here; the rr-poc implements that only partially
(patch-success receipts, restart verification of spec, not controller
convergence), so this remains a design argument backed by an executed
failure, not an executed detection.

Also surfaced by the same events capture
(`captures/14-gctx-error-event.txt`): the reports controller DOES emit
the real GlobalContext error the background path swallows -
`policy dbg-gctx/evaluation error: failed to evaluate policy: failed to
get global reference: no data available` - as a PolicyViolation event
from `kyverno-scan`. The error surface exists; it is attached to the
reporting path, not the acting path.

The failure showed on three distinct Jobs, across five resume attempts
(some repeats on the same Job):

1. trainer-b, resumed ~8min after an engine suspension (captures
   12/13), and again on the next resume attempt after the
   suspend-again recovery try.
2. trainer-c, when the scripted user resume landed 3 seconds after the
   engine suspension (`captures/16-trap-trainer-c.txt`), and again in
   take 3 (16b).
3. trainer-gce (`captures/16c-trap-gce.txt`), resumed ~8min after a
   quiet engine suspension.

Every affected Job had a retained non-null startTime and a stored
Suspended=True condition at resume time - the exact preconditions of
the upstream validation defect. The working hypothesis at the time was
a bookkeeping race with a settle-gate workaround; r3 replaced both
with the mechanism above and the upstream identification. The takes
are kept as the executed record of how the wrong theory fell.

Side effect worth stating plainly (take 2,
`captures/16-trap-trainer-c.txt` tail): the wedge also blinds the
ENGINE. With the Job stuck at Running=0, the metric condition never
became true again, so Kyverno did not re-suspend either - "no
re-suspension within 7m" with `active=` empty. Neither the engine nor
any surface it owns noticed that the workload it had acted on was
broken; the rule simply saw a workload that never became eligible.

Take 3 (`captures/16b-trap-take3.txt`) and the trainer-gce attempt
(`captures/16c-trap-gce.txt`) confirmed the r3 mechanism: the "settle
gate" was chasing a signal (startTime clearing during suspension) that
this controller version does not produce; an earlier "empty startTime"
reading was a jsonpath typo in the check itself. The consequence, kept
to what was executed: on this v1.34.0 cluster, every attempted resume
of a previously-started, suspended Job broke that Job's status stream,
regardless of which actor did the suspending, and nothing the policy
engine owns surfaced it. The clean side-by-side moved to a second
cluster, kind kyverno-lab2 on v1.34.3 (fix included), same stack
redeployed from the same manifests (captures 30+).

Metrics addendum to Finding 2 (executed,
`captures/18-metrics-surface.txt`): the swallowed acting-path errors
were counted in the background controller's execution histogram -
`kyverno_mutating_policy_execution_duration_seconds_count{policy_name=
"metric-suspend-running",result="error"} 7`. In everything this lab
inspected, that aggregate label - no target identity, no message - was
the only executor-side trace (the wrapper also records a result metric
family not shown in this capture).

Finding 2, minimal executed repro (`captures/31-cel-error-repro.txt`,
lab2, no Prometheus or GlobalContext involved): a policy whose target
condition is `int(object.metadata.name) > 0` against a job named
cel-error-job - a guaranteed CEL runtime error. Watched live over 150s,
bounded to what the transcript shows: four UpdateRequests (individual
trigger sources not captured; the count is consistent with one policy
event plus scan ticks), each Pending -> Completed -> deleted at
age 0-1s (the watch has no message column; the empty UR message is
verified in source, `pkg/background/common/status.go:37-39`); the job
untouched; policy status `ready: true` with an empty message (captured);
no transcript line mentioning the policy beyond creation bookkeeping;
the error-result histogram count absent before and 4 after - aggregate
corroboration matching the UR count, not per-UR tracing. The reports
scan separately produced `fail: mutation is not applied` events - its
own simulation result under different condition semantics (M04 above),
not a relabeling of the error. Codex designed the repro
(`upstream-drafts.md`), this lab executed it; the draft carries the
executed section. Its false/true/mutation-error controls remain
unexecuted.

### The clean run on lab2 (v1.34.3, wedge fixed)

`captures/30-lab2-kyverno-phase.txt` - the Kyverno story in five lines
(edited excerpt; full literal text in the capture):

```text
[18:20:47.211 UTC] LAB2 (k8s v1.34.3). Applying the http metric policy.
[18:20:50.608 UTC] both trainers suspended by the metric rule
                   (3.397s after the printed pre-apply marker)
[18:21:10.688 UTC] USER RESUMES trainer-a
[18:21:10.783 UTC] healthy resume on v1.34.3: active=1 (startTime reset accepted)
[18:23:14.635 UTC] TRAP: engine re-suspended trainer-a 123.947s after the
                   user resume. No new human intent.
```

The 123.947s is consistent with the mechanism: eligibility recovered
(the trailing-range samples read Running again) and a later evaluation
re-applied the rule; no capture identifies the exact triggering tick.
One re-suspension was observed; the rule text contains no cap, so
indefinite repetition under recurring eligibility is inferred, not
captured. Both sides of the upstream bug are now executed: v1.34.0
broke the Job on this exact resume shape; v1.34.3 resumed cleanly.

Forensics after the two engine actions (`captures/17-kyverno-forensics.txt`,
trainer-a deleted at 18:23:25), bounded to the inspected surfaces
(reports, UpdateRequests, namespace events, policy status; logs,
metrics, API audit and external sinks were not inspected):
policyreports scoped to trainer-a - 0 remain; UpdateRequests - none
(deleted on completion); events - 7 remain until their TTL, including
job-controller Suspended/Resumed timestamps, none attributing the
action to the policy (the one event naming the policy is the reporting
scan's "mutation is not applied", from the window when the user resume
was in effect); the policy status read "ready for reporting" before
the deletion, with no per-action record. In the inspected surfaces, no
retained per-action receipt links the action to its policy evaluation.

## 5. The runtime-rules prototype on the same facts

`rr-poc/` - a deliberately small Python demonstrator of the KEP-0003
execution protocol, revision 2 after a full Codex design review
(`codex-notes-r2.md` section D applied). It consumes the SAME fact
stream (the exporter's `karta_workload_status` through the Prometheus
query API) and acts through the SAME kind of handle the descriptions
supply (the catalog's `suspendDefinition`), so the comparison is about
the execution contract, not about who can read metrics.

The rule (`rr-poc/rules.yaml`): filter (ai-team; Job, CronJob,
Deployment), condition (the same Running-2m PromQL), action Suspend,
mode Enforce or Observe, allowance (one automated action per epoch, no
re-arm on external resume), budget (2 actions per loop).

The contract the prototype actually implements:

- Intent gates action: a persisted Intended receipt (ConfigMap in
  rr-system) plus a persisted pending-operation record come before the
  patch; if either write fails, no action. On restart, an unresolved
  pending operation is verified against the live object and becomes
  ExecutedVerifiedAfterRestart or UnknownOutcome - an Unknown blocks
  further automation on that workload (recovery branch verified in
  code; the battery never crashed mid-operation, so it ran only in its
  empty case).
- The patch is a JSONPatch with test preconditions on the workload UID
  and on suspend still being false - a replaced object or a lost race
  on that field fails visibly (FailedPrecondition receipt) instead of
  being overwritten. The preconditions protect those two fields only;
  metric eligibility is not re-checked inside the write.
- Allowance epochs: an external resume (transition-based detection,
  actor unknown, managedFields as supporting evidence) opens a new epoch
  and files a ResumeDetected receipt. With reArmOnResume=false, later
  eligibility produces EscalatedNeedsHuman instead of another suspend -
  one initial automated action per workload, then humans decide.
- Skip-and-report everywhere: no suspendDefinition -> SkippedNoHandle
  (the catalog Deployment has none - deliberately in filter scope);
  RBAC preflight fails -> SkippedNotReady; facts or target listing
  unavailable -> rule-level FactsUnavailable / TargetsUnavailable;
  budget exceeded -> SkippedBudget. Observe mode -> WouldAct with the
  real preflight result, consuming nothing.
- Receipts carry the rule, workload identity (uid + resourceVersion),
  epoch, the PromQL evidence sample, the Karta handle used, and the
  operation key; they have no ownerReference to the workload, so they
  survive it.

Disclosed PoC limits (from the review): reads/state/receipts use the
ambient identity while preflight+patch impersonate the rr-poc
ServiceAccount; rules.yaml is read at startup (no CRD watch); the
catalog-handle interpreter covers the plain dotted boolean paths
(Job/CronJob suspend) and skip-reports everything else; no leader
election or informers.

## 6. Executed differences

Same fact stream (the exporter's Running phase over a trailing 2m
range), same handle family (the described suspend field), same cluster
(lab2), sequential runs on different Jobs. RR's rule queries a wider
kind set than the Kyverno policy (Job+CronJob+Deployment vs Jobs); the
Job predicate is equivalent. What differs is the execution contract.
Each cell carries its evidence tier.

| Dimension | Kyverno (MutatingPolicy mutate-existing) | rr-poc (execution protocol) |
|---|---|---|
| Metric-conditioned action | EXECUTED: suspension via CEL http.Get 644ms after the apply marker on lab1 (capture 10), 3.397s on lab2 (capture 30); via GlobalContext with a named projection (capture 19) | EXECUTED: Intended->Executed on both trainers within ~1.1s of the loop, suspend=true verified (capture 21) |
| Condition/source failure | EXECUTED (capture 31, bounded per section 4): CEL error -> URs complete with empty message, job untouched, aggregate error-count metric the only executor trace inspected | VERIFIED in code only: handled query errors produce a rule-level FactsUnavailable receipt; an unhandled JSON parse failure would crash the loop instead; fault injection not executed |
| User resumes a workload the engine suspended | EXECUTED: one re-suspension 123.947s after the user resume, no new human intent (capture 30); indefinite repetition under recurring eligibility INFERRED (the rule has no cap) | EXECUTED: ResumeDetected 0.692s after resume; at renewed eligibility (124.526s) EscalatedNeedsHuman instead of action; still unsuspended and active at 300.147s (capture 22) |
| Action provenance after workload deletion | EXECUTED (capture 17): in the inspected surfaces - 0 reports remain, no URs, 7 TTL-bound events without action attribution, no per-action record in policy status | EXECUTED (capture 24): WouldAct x2, Intended, Executed receipts survive deletion with uid, resourceVersion, PromQL evidence sample, handle, operation key |
| Action without permission | EXECUTED in the earlier experiment run (silent no-op, ready=true status; ../03-experiments-executed.md, Experiment D); not re-executed in this lab | EXECUTED (capture 23): permission removed before the next evaluation -> SkippedNotReady receipt; after restore, Intended->Executed |
| Kind without a suspend handle | INFERRED authoring burden (per-kind patch logic); no Kyverno control executed for this row | EXECUTED (capture 20): SkippedNoHandle receipt for web-app; per-epoch dedupe VERIFIED in code |
| Observe / dry-run at runtime | VERIFIED: the reporting scan evaluates continuously but under different condition semantics (target=false); no observe gate on the mutate-existing executor was found or tested | EXECUTED (captures 20+21): WouldAct receipts with real preflight, targets verified untouched, later Enforce acts |
| Action outcome verification | No automatic controller-convergence check demonstrated; the v1.34.0 Job breakage was invisible on its inspected surfaces (Finding 3) | PARTIAL: Executed receipts attest patch-command success, not controller convergence; pendingOp recovery / UnknownOutcome / FailedPrecondition branches VERIFIED in code, not exercised. Neither side demonstrated automatic wedge detection |

The pairing to remember (both executed, same cluster, sequential runs):
at renewed eligibility after a user resume, the convergence engine
re-suspended its Job 123.947s after the resume; the execution-protocol
prototype filed an escalation receipt at 124.526s and its Job was still
running at the 300-second check.

## 6b. What this lab adds to the standing recommendation

The gap-verification research (03-gap-verification-results.md) said the
missing piece is one execution protocol, not metric plumbing. This lab
executed that sentence. Kyverno consumed the exporter's facts through
two paths (direct http.Get, and GlobalContext with a named projection)
and acted correctly on them - the facts layer composes fine. What
failed or fell short lived in the execution contract: an evaluation
error indistinguishable from no-match (capture 31), a re-suspension
after a user resume with no bound in the rule (capture 30), no retained
per-action provenance in the inspected surfaces (capture 17). The
platform-level Job breakage (Finding 3, upstream #134521) is a
Kubernetes validation bug, not an engine defect - what it adds here is
that the inspected Kyverno surfaces did not diagnose the controller
divergence, and neither side demonstrated automatic wedge detection
(RR receipts attest patch success, not controller convergence). Outcome
verification is part of the proposed protocol, implemented only
partially even in the prototype. The rr-poc
demonstrated the protocol answers with about 470 lines of Python on the
same facts and handles: receipts, epochs, escalation, preflight,
skip-and-report. The dedicated-project recommendation stands; this lab
supplies its executed evidence.

Also: the fork's metrics-exporter ran on both clusters through all of
this. Observed here: catalog fallback with five watchers started and
nineteen catalog GVKs skipped cleanly on a cluster that lacks them
(the catalog holds twenty definitions including two API versions of one
kind; bare Pod stays opt-in), dense status, and facts that drove two
different engines without either knowing about the other.

## 7. Lab state and how to rerun

- kind-kyverno-lab (v1.34.0): the WEDGE EXHIBIT cluster. Wedged jobs
  trainer-b/trainer-c (ai-team), trainer-gce (gce-test) still there;
  kcm still logging the rejected transition. Kyverno policies
  metric-suspend-running (http variant) + metric-suspend-gce + both
  GCEs still applied. Keep for demos of #134521; delete with
  `kind delete cluster --name kyverno-lab`.
- kind-kyverno-lab2 (v1.34.3): the clean side-by-side cluster.
  Exporter + Prometheus + rr RBAC deployed; no kyverno metric policy
  (deleted after the K phase); trainer-b suspended (kyverno leftover),
  trainer-e deleted (forensics), trainer-f running (post-escalation),
  trainer-g suspended by RR; receipts + rr-state in rr-system. The
  battery script reuses fixed job names and run-ids and still contains
  the invalid settle gate, so a naive rerun mixes evidence - before
  rerunning, delete the trainer-* jobs and rr-state, or use fresh
  names; treat `run-rr-phase.sh` as the record of what ran, not a
  fresh harness.
- Exporter image: `karta-exporter:lab` built from the fork branch
  worktree at `src/` (detached at 6a8f418f).
- Upstream material ready to file: `upstream-drafts.md` (kyverno
  diagnostics issue with executed repro; k8s draft kept as a historical
  duplicate of #134521 - do NOT file without a fixed-version repro).

## 8. Codex peer exchanges

- r1 assignment out: verify globalContext on the background mutate path
  (file:line), confirm no text-format parsing in the CEL http lib,
  design-review the RR prototype contract for fairness. Output lands in
  `codex-notes-r1.md`.
- r6 SIGN-OFF: [codex-notes-r6.md](codex-notes-r6.md). "Log as goal
  deliverable: SIGN-OFF. Draft 1 (pre-controls text): SIGN-OFF." The
  kyverno draft still needs its three declared controls executed before
  filing; that is follow-up work, not part of this log.
- r5 verification: [codex-notes-r5.md](codex-notes-r5.md). Three
  leftover edits (draft bullets, manual-patch certainty + "forever",
  reporting-scan scope) - applied; diff table fairness signed off here.
- r4 final review: [codex-notes-r4.md](codex-notes-r4.md). Verdict NOT
  YET with 11 MUST-FIX + 5 SHOULD items - capture/claim mismatches,
  min_over_time semantics, the report-scan explanation (target=false
  semantics), evidence-tier labels for every table cell, provenance
  bounding, rerun-script honesty, exporter counts. All applied in this
  revision; the core comparison (123.947s re-suspend vs 124.526s
  escalation with the workload alive at 300s) earned sign-off at r4
  with the corrected scope.
- r3 upstream identification: [codex-notes-r3.md](codex-notes-r3.md) +
  [upstream-drafts.md](upstream-drafts.md). The Job wedge is
  kubernetes#134521, fixed in v1.34.2 (mechanism verified at v1.34.1
  source; both sides executed in this lab); the kyverno diagnostics
  draft with a minimal repro this lab then executed (capture 31).
- r2 review complete: [codex-notes-r2.md](codex-notes-r2.md). Caught the
  projection-name bug in my GCE policy (Get's second arg names a declared
  spec.projections entry; mine declared none), pinned the acting-path
  error discard to processor.go:235 + missing rule-error handling with a
  full VERIFIED step table, corrected the A/B timing to 171ms, searched
  upstream (no exact issue; #13871 adjacent; discard present on the
  supplied 1.20-dev tree), proposed a clean upstream repro
  (data-dependent CEL error, no GCE needed), and filed 7 MUST-FIX items
  on the RR draft (intent-gating, restart-safe pending ops, observe
  consuming the cap, uid/eligibility preconditions, state keying,
  silent skip paths, preflight scope) - all applied in rr_poc.py r2,
  plus the resume-detection reframe (transition-based, actor unknown,
  managedFields as supporting evidence read with --show-managed-fields).
- r1 source review complete: [codex-notes-r1.md](codex-notes-r1.md).
  VERIFIED at the v1.19.1 tag: background target conditions and mutations
  can use GlobalContext; no enable flag is needed. GCE refresh defaults to
  10m and is independent of the action scan. Final refresh failure makes
  reads error instead of serving old data. Direct GCE ingestion requires
  JSON. Exact pinned SDK HTTP parsing remains unverified locally; the
  cached older SDK drops non-JSON bodies. INFERRED comparison controls:
  do not assume the unconditional resume trap survives a running-window
  predicate; guard name reuse because workload metrics omit workload UID;
  check Karta's suspend capability before treating a nil error as success.
  No cluster execution was performed for this source review.
- r2 review complete: [codex-notes-r2.md](codex-notes-r2.md).
  VERIFIED manifest correction: the GCE policy passes a JMESPath expression
  as the projection name but defines no projection. Use an empty projection
  plus CEL traversal, or declare a named projection, before attributing the
  no-op to background GlobalContext support. The source shares one store
  and initializes the real context before the mpol provider. VERIFIED
  diagnostic defect: the processor ignores RuleError when no patch exists
  and can complete the UR; the metrics wrapper still records result status.
  The notes include upstream search results and seven MUST-FIX items in the
  RR draft, including action after failed intent persistence, restart gaps,
  and Observe consuming the allowance. No new cluster run was performed.
- r3 review complete: [codex-notes-r3.md](codex-notes-r3.md) and
  [upstream-drafts.md](upstream-drafts.md). VERIFIED correction to Finding 3:
  the Job failure matches Kubernetes #134521, fixed by #134769 and backported
  in v1.34.2 as #135130. The affected controller resets startTime on resume;
  it does not clear it during suspension. The validator rejects changing a
  non-null timestamp with a misleading removal message. A narrow timing race
  is not required. The proposed wait-for-null-startTime gate is invalid for
  v1.34.0. The newer local checkout already has the fix. EXECUTED captures
  establish repeated failures over bounded periods, not literally forever.
  The raw manual SET-startTime request is absent from the supplied captures;
  changing an existing timestamp explains the reported rejection without a
  separate writer. INFERRED: rerun both executors on a fixed patch release
  before comparing their resume contracts. VERIFIED Kyverno qualification:
  metrics still record RuleError even when the acting processor drops it;
  the separate reporting path also emitted the captured error Event.
  The drafts separate those surfaces, label the minimal int-cast test as
  unexecuted, and mark the Kubernetes draft as a known, fixed duplicate.
  No cluster changes or upstream posts were made in this review.

## 9. The workaround hunt (round 17): closing gaps without changing Kyverno

Question asked: can the executed gaps be closed with config, authoring
patterns, or existing CRDs only? Own code pass + Codex r17
(codex-notes-r17.md) + one new executed spike.

EXECUTED - the annotation-epoch spike (captures/50-annotation-epoch-spike.txt,
manifests/07-annotation-epoch.yaml, rebuilt lab2): two mutate-existing
policies form a state machine on one Job annotation. epoch-suspend stamps
we-suspended when it suspends; epoch-mark-resume flips it to user-resumed
when it sees an unsuspended job carrying we-suspended; epoch-suspend
refuses user-resumed jobs. Result: suspension + stamp at 16:58:27, human
resume 16:58:52, allowance consumed 34.5s later, and seven minutes of
refilled metric windows and ticks later the job was still running. The
resume-trap allowance IS expressible in pure policy for this reduced
contract. Codex caveats accepted: the executed predicate has an A/B race
window that the metric refill masked here (fix: an explicit armed state);
the background write path can overwrite a late resume between evaluation
and its refetch+PUT; user-resumed is a label, not authenticated intent;
no escalation record, no re-arm semantics, state dies with the object.

VERIFIED, previously missed (own pass): the LEGACY mutate-existing engine
emits PolicyError events on failure (pkg/background/mutate/mutate.go:250,
event.NewBackgroundFailedEvent); only the new CEL MutatingPolicy path
drops errors silently. Authoring acting rules as legacy ClusterPolicy is
a deprecated-but-real visibility workaround for gap 1.

VERIFIED (r17): kyverno_mutating_policy_results_total{result="error",
policy_name} exists on the background controller - an alertable counter
with policy identity (no target, no message). And a paired Audit
ValidatingPolicy canary running the same expression DOES surface CEL
errors as report result "error" with the message (offline CLI check
executed: 'error: type conversion error from string to int').

Per-gap verdicts without Kyverno code changes (full table in r17):
silent errors PARTIALLY (alert + canary + legacy engine; no flag restores
per-target acting diagnostics); resume trap PARTIALLY (executed spike,
race-hardening authored, no supplied protocol); receipts PARTIALLY
(GeneratingPolicy can persist unsynchronized ConfigMaps; async, cannot
gate the action on the record); wedge detection CLOSABLE externally (the
exporter's own series disagree - alert recipe, not executed); per-policy
timer NOT native (time/token gates and external schedulers are authoring
alternatives); Experiment E scoping CLOSABLE (targetMatchConditions,
executed earlier).

The honest bottom line stands but sharpens: the FACTS and one-shot
allowance slices are reachable with disciplined authoring; what no
authoring supplies is the protocol as a product - pre-action persisted
intent, receipts that gate actions, authenticated human intent,
escalation, and diagnostics on the acting path itself.

## 10. The real dynamo run (2026-09-16)

The KEP rewrite needed the exporter shown on a real distributed workload, not a
batch job. The karta-e2e kind cluster still had the dynamo platform 1.2.1
(operator, etcd, nats) from the e2e suite, so we applied the suite's own smoke
DynamoGraphDeployment (Frontend + a Llama-3.1-8B mocker decode worker), let the
real operator drive it to state successful, and ran the exporter locally against
the cluster with --use-catalog. Both pods were attributed to their component
instances (Frontend, decode) through the real owner chain, and the workload
normalized to Running from .status.state=successful. Full scrape: capture 60.

Follow-up: the run was repeated with a third service, prefill, added to the
smoke DGD (the mocker supports disaggregated mode; the prefill service runs
--is-prefill-worker). The operator drove all three services to successful and
the exporter attributed all three pods (Frontend, decode, prefill). Capture 61.

Follow-up: capture 62 records one clean happy-path run on lab2 (fresh job
trainer-hp, the 05b policy scoped to it, UR watch running): applied 19:56:09,
suspended 19:56:26, work items filed-Pending-Completed-deleted, job controller
wrote Suspended. Notable: the report scanner emitted "mutation is not applied"
during this healthy run too, in the window between apply and patch, the same
message the broken rule produces (capture 31). The epoch policies from section 9
were removed before this run.

Follow-up: capture 63 measures the two-minute window from both sides on lab2.
A resumed job (trainer-hp, suspended earlier by the rule) kept
min_over_time[2m]=0 while the window still held samples from the suspended
period, flipped to 1 at 122s, and was re-suspended at 129s after the resume.
A brand-new job (trainer-young) with the same rule already in place was
suspended 8 seconds after creation: its series held only 1s, so the query was
satisfied on the first sample. The window is real for a resume and not for a
new job; only a coverage check closes that.

Follow-up: capture 64 shows the new-job early pass is fixable inside the rule.
Manifest 05d adds `and count_over_time(...[2m]) >= 22` to the 05b query. Same
setup as the 8-second kill (brand-new job, policy applied as soon as the pod
ran): no suspension at 8s, the window filled 6/12/18/24 samples, suspension at
164s after creation (window full at about 120s, next scan tick after that).

Follow-up: capture 65 is the happy path rerun with the coverage rule (05d) and
the work items watched throughout. Brand-new job, policy applied as soon as the
pod ran: the window filled 1/7/13/19/24 samples, four work-item rounds ran (one
per pass), the first three completed without touching the job, the fourth
suspended it at 167s. Two "mutation is not applied" warnings fired at t+14s and
t+44s, while the window was still filling.

Follow-up: the sample-count fix (05d) couples the rule to the scrape interval:
at a 15s interval a 2-minute window holds 8 samples and `>= 22` never fires.
Manifest 05e replaces it with an interval-free age check, `and
karta_workload_status{...phase="Running"} offset 2m == 1` (the job must already
have been Running two minutes ago). Capture 66, same setup as 63 and 65: the
offset value stayed absent until about t+122s while min[2m] was already 1 from
t+32s, four work-item rounds ran, the job was suspended at t+140s.

Follow-up: capture 67 folds the story into one run with the 05e rule: new job,
policy created as soon as the pod ran, first suspension at t+177s (the offset
value was absent for two minutes, then pointed at the pre-pod 0 sample, then 1),
manual resume at 23:11:02, second suspension at r+175s (min[2m] held the
suspended-time 0s until r+120, the offset value pointed into the suspended
period at r+120 and cleared at r+150). Seven work-item rounds, all identical.

## 11. Are the gaps solvable with a different Kyverno? (2026-09-16)

Upstream has a fix PR for gap 1: kyverno/kyverno#17063 (issue #17062), open
since 2026-08-11, CI green, no review. Its GeneratingPolicy twin #17061 merged
the same day. We rebuilt the background controller from v1.19.1 with that
commit cherry-picked and reran the broken rule (capture 68): the policy-event
work item still completed silently, but every scan-tick work item went Failed
with the real CEL error in status.message ("type conversion error from 'string'
to 'int'"), cycling Failed -> Pending -> Failed with a retry count, and the
controller log gained ERR lines carrying the same text. The job stayed
untouched, the policy stayed ready: true, and no new event appeared; the only
events were still the report scanner's "mutation is not applied". The healthy
offset rule still applied on the patched controller (capture 70, re-suspended
at r+169s). Controller restored to stock v1.19.1 afterwards.

Gap 3 with every opt-in on (capture 69): generateSuccessEvents=true plus the
chart-default reporting.mutateExisting. Before deleting the job there was one
PolicyReport entry for the action, result pass, message "success", owned by the
job, and no Kyverno event on the job at all. After deleting the job: zero
reports, zero work items, only the job controller's own events. The scanner's
"mutation is not applied" warning also fired for trainer-h, a job the policy
never targeted (targetMatchConditions are not applied on the report path).

Gap 2: no issue or change in any version asks for act-once or for respecting a
manual change; #16214 covers only periodic re-evaluation (already merged as
#16255), and #17284 (silent write drops on shared targets) is open with no
comments.

## 12. Extending Kyverno without a fork: the intent-object seam (2026-09-17)

A design bakeoff (six proposals, ten reviewers, a six-repo council, a chair)
converged on one shape: Kyverno decides, a Karta component acts and remembers.
The metric rule moves from MutatingPolicy mutate-existing to a
GeneratingPolicy whose only write is a Karta ActionRequest, created with CEL
resource.Post from the generate expression; a small executor spends the
request once. Prototype on stock Kyverno 1.19.1 (manifests 08, 09, 10;
rr-poc/ar_executor.py):

- Capture 71: request created on the tick after the 2m window, executor
  Intended -> patch with uid and resourceVersion tests -> Executed, one patch.
  Manual resume recorded 1.4s later; the job stayed resumed through 300s of
  60s scans with no second request and no second patch. Job deleted: both
  requests and their receipt chains remained. Kyverno never wrote the job.
- Capture 72: the broken rule (int(object.metadata.name) > 0 as the whole
  generate expression) went Failed from the first work item with the trigger
  named and the CEL error text, retried 1-4, a fresh Failed one every tick,
  PolicyError events on the policy with the job as related, ERR log lines at
  generate_controller.go:172. Job untouched, zero requests. Residual: the
  policy still reports ready: true.
- Lesson recorded in 71: CEL's && absorbs an evaluation error when its other
  operand is false, so `int(name) > 0 && dyn(Get) != null ? a : b` silently
  took the else branch while no request existed and only surfaced the error
  once one did. Keep fallible calls out of && chains in generate expressions.

Follow-up (gap 3, job kept alive): capture 73. With the job and the policy both
present, Kyverno holds one PolicyReport result for the action: policy name,
result pass, message success, source KyvernoMutatingPolicy, the scan timestamp.
No patch, no evidence. Deleting only the policy removed it while the job stayed
suspended; events expire on the API server's one-hour TTL. The record depends
on the policy's life, the job's life, and the clock.

## 13. The created gauge and the three goals (2026-09-17)

Input: the handoff from the exporter worktree (handoff-policy-signals.md) and
its two unpushed commits, 65f4fc44 (duration-signals research) and de18d6df
(karta_workload_created_timestamp_seconds, three recipes, promtool fixtures).
Capture 74, manifest 11-dpol-completed.yaml.

- Rebuilt the exporter image from de18d6df, rolled it on lab2, ran the same
  binary locally against karta-e2e with the dynamo operator. Eleven metric
  families, seven for consumers. The new gauge equals the object's
  creationTimestamp to the second on both clusters (trainer-h 1789577750,
  dynamo-smoke 1789641330).
- Same-name recreation: trainer-again 1789641426, deleted and recreated,
  1789641529. Exporter restart: both values unchanged, 24 of 24 samples still
  in the 2m window, up never dropped.
- Goal B shape (Running for X) at lab scale (2m, 5s scrape, floor 23, age >
  120s) returns trainer-h and the nightly-report CronJob only; suspended
  trainers are absent. Goal C shape (Completed for X) returns
  nightly-report-29826900. Goal A (gpu idle) did not run: no dcgm series with
  pod labels anywhere (lab2 has no gpu; karta-e2e's kwok dcgm exporter emits
  none). The promtool fixtures (tests.yaml + duration-tests.yaml, 16 rules)
  pass.
- Goal C on stock Kyverno 1.19.1: a DeletingPolicy (schedule every minute,
  http.Get of the goal C query, condition name in result) removed a
  seconds-long job at 10:41:00, the first tick where age > 120s and the
  window was fully Completed. Kyverno's dpol compiler has the http and
  resource libraries like mpol; the cleanup controller already had job delete
  rights through the aggregated clusterrole.
- What the delete left: policy status lastExecutionTime and an empty message,
  a once-a-minute "updated deleting policy status" log line that never names
  the job, job-controller events on a deleted object, no policy events, zero
  reports. Gap 3 applies to deletion as it does to suspension.
- Lab state after: DeletingPolicy removed, trainer-again and the exporter at
  karta-exporter:lab2-created left on lab2, dynamo-smoke removed from
  karta-e2e again.
- Docs: KEP-0003 gained a goals section, the seventh table row, the delete
  run and the leftovers block; KEP-0004 (kep-metrics-exporter) said six
  families and now says seven with the gauge line added.
