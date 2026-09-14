<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# slide spec review loop

## round 1 - coverage (codex)

Coverage verdict: PASS after edits for the executed lab story. All
requested findings and comparison dimensions have a home. This is a
coverage verdict, not publication or rendering approval. Public evidence
packaging and the choices below remain open.

Read the signed-off INTEGRATION-LOG.md end to end, inventoried all 31
capture files, and checked the cited timelines, error surfaces, and RR
results. Paths below are relative to exporter-lab. No cluster actions or
publication were performed. Source checkouts and the log were not edited.

### changes applied

Preserved s1-s33 and their acts. Added s18a, for 34 slides total. Kept the
title / one idea / beats / visual / data / fx shape. Added a rule to
replace beats in place instead of scrolling. The longer comparison uses
one legible row at a time.

| Topic missing or under-explained | Placement and edit | Evidence |
|---|---|---|
| Lab scope and setup | s2: sleep Jobs, Karta CRD even in catalog mode, executor permissions; replaced "everything in this deck ran" with evidence tiers. s3/s4 mark the GPU join as illustrative. | INTEGRATION-LOG.md:81-93; captures/01-crd.txt:2; manifests/03-workloads.yaml |
| GCE fetch success before read failure | s13: successful polls, including a trainer name. The failing projection read is separate. | EXECUTED: captures/07b-gce-poll-v6.txt:1-2; VERIFIED projection interpretation: codex-notes-r2.md section A |
| Executor metric trace in the original failure | s15: the error also reaches an aggregate histogram, count 7. The trapdoor no longer implies erasure on every surface. | EXECUTED: captures/18-metrics-surface.txt:6 |
| Actual reporting error Event | s17: added dbg-gctx's mutation-expression error. Distinguished it from the minimal repro's successful reporting simulation. Corrected the capture citation. | EXECUTED: captures/14-gctx-error-event.txt:7 and captures/31-cel-error-repro.txt:35-36; VERIFIED: INTEGRATION-LOG.md:158-163,194-204 |
| Cost of the two working metric paths | New s18a: per-evaluation HTTP versus shared GCE refresh; independent scrape, refresh and action clocks. No latency or scale benchmark claimed. | VERIFIED: INTEGRATION-LOG.md:222-247; codex-notes-r1.md section A |
| Wrong wedge hypothesis and failed workaround | s21: a narrow race was unnecessary; waiting for startTime to clear was invalid. Broken and fixed versions remain explicit. | EXECUTED: captures/16b-trap-take3.txt:1-5 and captures/30-lab2-kyverno-phase.txt:4-6; VERIFIED mechanism: codex-notes-r3.md |
| Allowance configuration and attribution limits | s24/s27: per-rule/workload-UID state; a new epoch does not re-arm with reArmOnResume=false. Resume actor is unknown to RR. | EXECUTED transition/escalation: captures/22-rr-anti-trap.txt:5-10; VERIFIED semantics: rr-poc/rules.yaml:19-21 and rr-poc/rr_poc.py:369-403 |
| Unexecuted RR branches and supported handle subset | s25/s30: distinguish captured Intended->Executed from source-only recovery, failed preconditions, query/list failures and budget skips. Preserve the JSON-parse exception and field-only patch guards. | EXECUTED: captures/21-rr-enforce.txt:4-11; VERIFIED: rr-poc/rr_poc.py:99-115,275-319,356-363,430-461; INTEGRATION-LOG.md:437-463,477,483 |
| Fair comparison scope | s27/s31: sequential Jobs, wider RR kind query, equivalent Job predicate, different evaluation intervals, no speed contest. s32 scopes conclusions to the tested rule. | INTEGRATION-LOG.md:467-489; rr-poc/run-rr-phase.sh:54; captures/22 and 30 |
| Rerun and filing limits | s33: historical scripts, fixed names/state, invalid settle gate, pending Kyverno controls and known fixed Kubernetes duplicate. | INTEGRATION-LOG.md:521-543; upstream-drafts.md:230-231 |

### unsupported implications corrected

- s9/s11: "time-in-phase queries never break" and a ticking "freshness
  witness" overstate the evidence. Dense phases do not prevent scrape
  gaps. The timestamp is last processed watch activity. Component-query
  failure after root lookup differs from a missing owner. Corrected the
  animation and scope. VERIFIED: src/exporter/pkg/controller/events.go:
  275-287; controller.go:107-110,343-344; attribute/attributor.go:34-43.
- s14: "same target" and scan marks "forever" are not in the saved A/B
  transcript. It records trainer-b acting after the apply marker. The
  long GCE no-op interval is operator-reported. Removed invented ticks.
  EXECUTED: captures/09-ab-test.txt:1-3; captures/08-kyverno-suspend-timeline.txt
  holds only an initial sample; INTEGRATION-LOG.md:171-185.
- s16: "0 -> 4 in sync" invented per-UR metric observations. Replaced
  with absent-before / count-4-after and aggregate corroboration.
  Trigger sources remain uncaptured. EXECUTED: captures/31-cel-error-repro.txt:
  4-24,37-40. UR Completed is not a compliance verdict.
- s18: "window fill + tick" was an unrecorded causal attribution for the
  corrected GCE timing. Removed it. EXECUTED: captures/19-gce-corrected.txt:
  1-4 contains setup and polls, not the triggering evaluation.
- s22/s23: replaced universal absence and staged TTL expiry with the
  inspected surfaces. Neither executor demonstrated wedge detection.
  Forensics includes surviving Job-controller events; policy status was
  read before deletion. EXECUTED: captures/17-kyverno-forensics.txt:1-27;
  INTEGRATION-LOG.md:399-410,503-508.
- s25/s26/s28/s31: removed "every branch" or "every skip" as test-result
  claims, "touched nothing", and "PARTIAL both" as a copied table verdict.
  Observe writes receipts/state, not workload patches. Failure branches
  retain their source-only tiers. Earlier Kyverno RBAC evidence is labeled
  as earlier. Sources: captures/20,21,23; INTEGRATION-LOG.md:476-483.

### coverage map after edits

| Requested coverage | Slides | Evidence |
|---|---|---|
| Finding 1: generate-policy interference | s19 | captures/04,05; log:110-124 |
| Finding 2: acting-path error discard | s13-s17 | captures/07b,09,14,18,31; log:139-204 |
| Finding 3: platform wedge, both versions | s21-s22; clean resume in s20 | captures/12,13,15,16,16b,16c,30; log:249-345 |
| Broken GCE, HTTP, corrected GCE | s13-s14, s18-s18a | captures/07b,08,09,10,19,30 |
| Minimal CEL repro | s16-s17 | captures/31 |
| Forensics on both sides | s23, s29 | captures/17,24 |
| Observe, intent, enforce | s24-s26 | captures/20,21 |
| Epochs and escalation | s24, s27 | captures/22 |
| Permission and no-handle skips | s28 | captures/20,23; ../03-experiments-executed.md:110-134 |
| All diff-table dimensions | s31, with details in s20-s30 | log:474-483 |
| Exporter startup and actual series | s7, s10; source explanation in s4-s11 | captures/02,03; log:33-108 |

Not every capture needs a slide. Cleanup, initial status snapshots, and
aborted wedge takes support setup or mechanism. They are not additional
successful contract tests. Capture 06 supports the existing PromQL slide.

### flags for arbitration

1. s19's timeout and cleanup causality are described in the signed-off
   log. Captures 04/05 show netpol presence and later query results, not
   the raw timeout, policy-status dump, or deletion commands. The slide
   now says "the log reports". Keep that wording, or supply the missing
   transcript before rendering those claims as terminal output.
2. Act 1's GPU join, registry replacement, JobSet/LWS owner walks and
   degradation animations are source explanations, not lab captures.
   Capture 03 explicitly skips several illustrated CRDs. Keep their
   visible source/illustration labels. No executed GPU utilization,
   CR-replacement or out-of-order-owner test is in this capture set.
3. The strict "every number traces to a capture" rule needs a policy for
   configuration and source counts. Kyverno's version/scan interval are
   recorded in the log; scrape and RR intervals also live in config or
   scripts. They are not runtime attestation outputs in this capture set.
   Use labeled configured values with source links, or capture an
   attestation before making them terminal-backed numbers. Do not treat
   animation durations as measurements.
4. Public source paths still need a final mapping: e.g. exporter/pkg/...
   means src/exporter/pkg/... locally, and captures/10 is shorthand.
   s33 now requires verified branch/Pages links at publication. Publish
   the source-only evidence as well as captures, or omit those claims.
   This round did not verify a public artifact URL.

## round 2 - exporter internals (codex)

Verdict: PASS for the corrected, staged exporter section. The canonical
wow/ files still need the accompanying round-2.patch applied. This
session can write only inside src, so reviewed copies and the patch are
in src/.context/wow-round-2/. The patch changes s3-s11, adds s7b, s8b,
s9b and s11b, updates the total to 38, and appends this report. Other
slides are unchanged. No code, cluster, or public deployment was changed.

Source: src at 4983c10bbb245cf1a754602e466c443373830426. The requested
exporter files, main.go and batch-job catalog have no diff from the lab's
6a8f418f revision. Paths below are relative to src. Findings are VERIFIED
by code reading unless explicitly marked otherwise. Existing test cases
were read for examples; no tests or scenarios were executed this round.

### corrections and depth added

| Slide | Change and verified mechanism | Source |
|---|---|---|
| s3 | Removed the universal claim that pod metrics lack workload identity. JobSet's decode is a component instance; replicatedjob is the component. GPU joining remains an illustrative use case. | docs/catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml:50-64; exporter/test/rules/tests.yaml:51-56 |
| s4 | Added the identity contract: uid on the join is the pod UID. Workload metric identity omits workload UID; workload_version and karta live on workload_info. The collector emits facts but does not fetch DCGM. | exporter/pkg/collector/collector.go:13,45-62,84-99; docs/Metrics Exporter.md:105-111 |
| s5 | Explained what is computed and stored. Workload callbacks rebuild root phases, generation and component records. Pod records carry attribution and phase. Scrapes take a locked store snapshot and aggregate pod counts; they do not run jq or fetch objects. | exporter/pkg/controller/events.go:124-139,314-351; exporter/pkg/state/workload.go:23-64; exporter/pkg/store/store.go:31-70,174-188; exporter/pkg/collector/collector.go:84-169 |
| s6 | Fixed "losers show up as shadowed": overridden catalog entries are excluded. Selection groups by group+kind, not version. CRs sort by timestamp then ascending name; catalog names sort descending. This is not semantic version negotiation. Invalid CRs cannot displace a valid catalog entry. | exporter/pkg/registry/registry.go:59-98,136-202; exporter/pkg/collector/collector.go:193-197 |
| s7 | Full root objects versus metadata-only child GVKs; root wins when the same GVK is needed for both. additionalChildKinds contribute watchers. Shared Pod and Karta informers are outside the five dynamic-watcher count. Captured roots are Job, CronJob, Deployment and StatefulSet; ReplicaSet is the child. | exporter/pkg/controller/controller.go:139-158,213-231,237-320; exporter/pkg/registry/registry.go:229-245; EXECUTED ../captures/03-exporter-watchers.txt:1-25 |
| s7b, new | Listed the exact trimmed fields, rather than saying all metadata survives. Added --full-pod-cache and its update cost: changed resourceVersion triggers selector evaluation. Trim mode has a separate phase-only path. | exporter/pkg/controller/controller.go:183-209; exporter/pkg/controller/events.go:197-227; exporter/cmd/main.go:45 |
| s8 | Owner walk follows controller=true edges by UID. Root matching ignores served version. An incomplete chain parks even above an already found inner root; owner arrival drains pending keys. Corrected the LWS chain's missing leader StatefulSet. | exporter/pkg/owner/index.go:69-83,93-104,129-179; exporter/pkg/controller/events.go:112-139,162-172,259-311; exporter/pkg/owner/index_test.go:100-112 |
| s8b, new | Split owner lookup from component/instance/replica resolution. Single-leaf inference bypasses type selectors. Stored component IDs normally provide the workload side of matching; pod selectors provide the pod side. Replica selectors can be inherited through description ancestors, distinct from Kubernetes ownership. | pkg/instructions/pod.go:14-37; exporter/pkg/attribute/attributor.go:34-80,116-140; exporter/pkg/controller/events.go:314-328 |
| s9 | Condition mappings require status=True, not merely a named condition. Running uses active and ready with missing counts treated as zero. All matches are retained; Running and Degraded can both be true. Dense phases are conditional on HasStatus. | docs/catalog/batch-job-v1.yaml:26-50; pkg/resource/accessor.go:603-624; exporter/pkg/state/workload.go:37-48; exporter/pkg/collector/collector.go:101-114 |
| s9b, new | Separated no match (Undefined=1), no statusDefinition (no status series), and status-evaluation error (no status series plus log/counter). A new partial record replaces the prior record. no_status_definition is an unused constant, not an emitted diagnostic. | pkg/resource/component.go:226-235; exporter/pkg/state/workload.go:26-48,64; exporter/pkg/controller/events.go:124-130; exporter/pkg/collector/collector.go:94-114; exporter/pkg/collector/names.go:62 |
| s10 | Explained desired replicas versus observed pod counts. Counts use the union of declared and observed instances and zero-fill pod phases. A nil replica pointer omits its series. Store deletion removes later exporter output, not Prometheus history. | exporter/pkg/state/workload.go:70-95; exporter/pkg/collector/collector.go:116-169; exporter/pkg/store/store.go:87-95 |
| s11 | Distinguished unknown component from unknown instance. Both preserve the workload join after root resolution. Limited the missing-owner/no-join example to a newly seen pod. Current unattributed gauges differ from cumulative evaluation-error counters. | exporter/pkg/attribute/attributor.go:34-80; exporter/pkg/controller/events.go:275-311,328-351; exporter/pkg/collector/collector.go:173-191 |
| s11b, new | Separated activity, initial informer sync, and actual freshness. Errors can advance the event gauge. Ready checks only current started informers; missing GVKs skipped during mapping are not failed-sync entries. Neither measure establishes ongoing per-workload correctness. | exporter/pkg/controller/controller.go:116-136,285-320,343-344; exporter/pkg/controller/events.go:95-98,124-130,197-227; exporter/pkg/server/server.go:43-49 |

### flags for arbitration

1. Trim-mode selector invalidation has a real limit. A change only to
   nodeName does not pass identityChanged. A phase-only change on an
   existing PodRecord updates Phase without re-running a selector that
   reads phase. VERIFIED: exporter/pkg/controller/events.go:206-223.
   INFERRED runtime effect: attribution can remain stale for such custom
   selectors. No reproducer executed. s7b states the limit; do not claim
   the trim/full switch concerns memory alone. Full-cache mode does
   re-evaluate on resourceVersion changes.
2. An already attributed pod that later reaches a missing owner is
   parked without deleting its previous PodRecord on that branch.
   VERIFIED: exporter/pkg/controller/events.go:275-311. INFERRED runtime
   effect: the old workload join may remain until a later successful
   evaluation or removal. s11 now uses a newly seen pod for the no-join
   example. Do not animate all missing-owner cases as immediate erasure.
3. No-status-definition diagnostics are weaker than the declared reason
   vocabulary suggests. A source-wide usage search found
   ReasonNoStatusDefinition only at exporter/pkg/collector/names.go:62.
   collectSelf emits no corresponding reason; normal absence does not
   increment status_eval. s9b teaches the actual metric surface. This is
   not a requested implementation fix.
4. Two scope limits matter when narrating the source. The owner depth
   cap bounds traversal; it is not unlimited graph discovery. Registry
   recompute tries the first sorted CR candidate and does not try the
   next one if newEntry fails; it can then fall back to catalog.
   VERIFIED: exporter/pkg/owner/index.go:141-161 and
   exporter/pkg/registry/registry.go:172-200. Avoid stronger claims that
   every valid definition is tried or that all owner graphs resolve.

### delivery and checks

The staged spec retains the one-idea / beats / visual / data / fx format.
New mechanism animations are labeled VERIFIED, not live executions.
The round-1 report is preserved verbatim before this appended section.
ASCII, headers, unique slide IDs and lowercase titles were checked.
The patch was applied to temporary copies and compared with the staged
files. No edits outside the configured writable root were attempted.

To land the round, an agent with write access to exporter-lab can run
from that directory:

```sh
patch -p1 < src/.context/wow-round-2/round-2.patch
```

Apply the patch rather than replacing files wholesale if another review
has changed the canonical files since this round started.


## round 3 - exporter animations (codex)

Verdict: PASS for the storyboard specification. Eleven exporter slides
now have concrete element states, numbered timed actions, once-only
playback, held endings, and print/mobile/reduced-motion fallbacks. Every
story has six or seven steps. No new slides this round; total remains 38.
These offsets are presentation choices, not new runtime measurements.
No rendered deck or browser behavior was tested in this spec-only round.

Landing note: the saved round-2 patch was verified against current files
and applied to the canonical wow/ files using plain writes. The writable
root now includes exporter-lab. The round-2 entry above records its
historical staged delivery; its outstanding apply step is now complete.

### storyboards written

| Slides | Concrete sequence | Held/static result |
|---|---|---|
| s5 | Registry configures watchers; workload callback builds a record; a separate pod dot resolves ownership and selectors; a separate scrape renders the snapshot. | Separate event and scrape paths; no registry pass per pod and no jq on scrape. |
| s6 | Catalog selected, invalid CR rejected, valid CRs ordered by age, separate equal-time name fixture, removal restores CR/catalog fallback. | Pick-order and accounting rules; no shadowed count for overridden catalog. |
| s7 | Shared informers stay outside the dynamic set; named root/child chips appear; selected real log rows precede full-capture totals. | Five dynamic watchers and nineteen skipped GVKs, clearly scoped. |
| s7b | Retained-field mask; independent identity, phase and nodeName updates; alternate full-cache configuration and RV-triggered selectors. | Default/full-cache comparison with selector invalidation caveat. |
| s8 | Inner Job candidate, missing JobSet owner, pending UID bucket, arrival and retry; separate full LWS chain. | Outermost-root rule and the LWS chain including the leader StatefulSet. |
| s8b | Known workload -> leaf component -> cached instance IDs plus pod selector -> instance; replica remains a separate label. | JobSet replicatedjob/decode example; normal empty replica label. |
| s9 | Synthetic inputs produce Initializing, Running, Running+Degraded, then an independent Suspended False/True case. | Source-derived examples; boolean values never tween and matches are not exclusive. |
| s9b | Successful no-match, no definition, then replacement of a previously good record on evaluation error. | Undefined versus absent status series, with distinct diagnostics. |
| s10 | Short excerpts and parsed fields from the same saved scrape, with exact row references. | Trainer identity, phase, desired/observed counts and infrastructure examples. |
| s11 | Unknown component, independent unknown-instance case, independent NEW pending pod, then gauge/counter distinction. | Join preservation and exact reason labels; no invented repeated-run totals. |
| s11b | Initial sync, skipped kind outside the set, normal activity, quiet interval, error that still advances activity. | Readiness, activity and evaluation outcome remain separate facts. |

Kept titles, beats and source tiers from the corrected round-2 section.
Replaced its vague visual/fx notes and added playback/fallback fields.
Added one common exporter animation contract to avoid repeating lifecycle
and accessibility requirements on every slide. Later acts are unchanged.

### builder requirements exposed by the skeleton

- Use customFx.exporterS5 through the named exporter effect handlers in
  the spec, driven by playTimeline. Stock dotTravel requeues itself at
  skeleton-tail.html:154. The exporter stories are bounded examples and
  should not enable that stock loop as a second effect driver.
- Scope helper calls to the active view. typeTerm treats many metrics
  and time-prefixed logs as command text; its command branch costs 14ms
  per character. Classify recorded stdout rows as tl-out before invoking
  it. Sources: skeleton-tail.html:31-35,87-109. The spec accounts for
  the output-line start delay and countUp's delay/duration at 118-132.
- fxClear cancels tracked timeouts, but countUp also uses untracked
  requestAnimationFrame callbacks. The builder must track/cancel those
  frames or guard them by activation token, and reset custom classes,
  dots and views. Sources: skeleton-tail.html:54-63,118-132,159-168.
- Print CSS currently exposes fx-hide, while mobile CSS permits vertical
  scrolling. Neither implements the specified compact fallbacks or a
  reduced-motion runtime gate. Add deterministic static rendering and
  beforeprint/afterprint handling when building the deck; override the
  scrolling mobile layout for these views. Sources:
  skeleton-head.html:170-190; skeleton-tail.html:159-183.
- `<unknown>` must be escaped or assigned through textContent. A raw HTML
  insertion would hide the sentinel as a tag. Keep reason words and
  path labels so meanings remain visible without color or motion.

These are implementation requirements for the deck builder, not claims
that the current skeleton already satisfies them. No skeleton HTML was
changed in this round. Animation durations are explicitly schematic;
only source-derived fixtures or the referenced captures supply data.

### checks

Checked the canonical files for ASCII and headers, all 38 slide IDs,
required fields, six-to-seven ordered steps per target slide, playback
and fallback definitions, and exactly one round-3 report entry. Verified
that non-animation fields remain unchanged apart from more precise s10
capture references, and that later acts match the landed round-2 version.
The canonical round-3 entry is present; no staged patch is required.

## round 4 - kyverno mechanics (codex)

Verdict: PASS after the in-place corrections. Checked s12-s19 and s18a
against Kyverno tag v1.19.1, commit
40ec788d48bb28d83dbf85538e962a59db9d45c6. Read tag blobs with git show;
the Kyverno checkout stayed read-only. Re-read the cited manifests and
captures. No new cluster execution. No new slides; total remains 38.
K paths below are relative to that checkout. VERIFIED denotes source;
EXECUTED denotes saved outputs. Operator accounts remain labeled.

### changes applied

| Slide | Correction |
|---|---|
| s12 | Named the exact evaluation fields. Resource constraints select Jobs; targetMatchConditions gates these background targets. Ordinary matchConditions belongs to the other evaluation mode. Kept the partial-sample caveat and measured 67.6s counterexample. |
| s13 | Split fetch/store from projection lookup. JMESPath runs when storing declared projections; Get reads by name. Added the source-only empty-string whole-body option. Replaced abbreviated manifest references with exact paths. |
| s14 | The twin narrows the fault rather than proving an identical-target A/B result. Its target predicate also restricts trainer-b. The 171ms value is first observation after the marker; the long no-op interval remains operator-reported. |
| s15 | Rebuilt the trapdoor as the runtime return path: EvaluationResult.Error -> RuleError + nil patch/outer error -> metrics wrapper -> processor gate -> Success. Both update and audit are inside the non-nil-patch gate. Completed requires no other accumulated failures. A false condition can mask another condition's error. |
| s16 | Separated blank/Pending/Completed status from the DELETED watch event. Added the explicit controller delete and reporting-RBAC meaning of readiness. Empty UR message is source-verified, not visible in the watch. Preserved aggregate-only metric corroboration and unknown trigger sources. |
| s17 | Named rr-cel-runtime-error and dbg-gctx in exclusive views. The former bypasses its bad target condition during reporting and yields a simulated patch difference. The latter errors in its mutation expression. Its trainer-a target guard does not exclude trainer-b from reporting. |
| s18 | Labeled all durations as first-observation timings. Preserved 644ms, timestamp-derived 3.397s, and about 2.5 minutes. Separated the isolated GCE name from the ai-team fixture name. Flagged the raw capture's incorrect 3.2s annotation for the builder. |
| s18a | Added the policy-event/ticker -> policy queue -> CELMutate UR -> target listing chain. Separated the 5s scrape, 15s GCE refresh, 60s action requeue, and reporting scheduler. GCE storage is per process. HTTP runs only when its condition is reached. |
| s19 | Specified namespace CREATE, the unscoped policy predicate, and ingress-only generated policy. Kept captured netpol/query evidence separate from the operator's timeout/cleanup account. |

### verified mechanics and evidence

- GCE: K pkg/globalcontext/externalapi/entry.go:64-73,86-110,116-129,136-165;
  pkg/cel/libs/context.go:115-124. The background poller and CEL context
  receive the same store: cmd/background-controller/main.go:223-238,319-325.
  EXECUTED successful fetches: captures/07b-gce-poll-v6.txt:1-2.
  EXECUTED corrected projected path: captures/19-gce-corrected.txt:1-4.
- Condition selection and error precedence: K
  pkg/cel/policies/mpol/compiler/policy.go:42-69,201-236.
  Target Evaluate and RuleError packaging: K
  pkg/cel/policies/mpol/engine/engine.go:84-118,215-226.
  Processor discard and conditional Success: K
  pkg/background/mpol/processor.go:230-276,407-415;
  pkg/background/common/status.go:37-39.
- The wrapper records both duration and result families before returning
  the response: K pkg/cel/policies/mpol/engine/metrics.go:18-33;
  pkg/metrics/mpol.go:34-90. Only the duration family appears in the cited
  captures. EXECUTED error counts: captures/18-metrics-surface.txt:6 and
  captures/31-cel-error-repro.txt:40. Neither is a per-UR trace.
- Policy add/spec-change and periodic requeue: K
  pkg/policy/policy_controller.go:234-246,302-319,591-607,645-655,699-705.
  Bare CELMutate UR creation and Pending update: K pkg/policy/mpol.go:17-25,91-98;
  pkg/policy/generate.go:31-39. Completed UR deletion follows a fresh status
  GET: K pkg/background/update_request_controller.go:268-289.
  EXECUTED four lifecycles: captures/31-cel-error-repro.txt:9-24.
- The 1h binary default and environment override: K
  cmd/background-controller/main.go:191-199. Reports have a distinct
  --backgroundScanInterval: cmd/reports-controller/main.go:210,321.
  The acting wrapper uses execution_cause=background_scan even for work
  enqueued by policy events: cmd/background-controller/main.go:390-396.
  That label cannot identify an individual triggering tick.
- Reporting simulation: K pkg/controllers/report/utils/scanner.go:243-277;
  pkg/cel/policies/mpol/engine/engine.go:158;
  pkg/cel/policies/mpol/compiler/policy.go:201-236.
  EXECUTED int-cast policy Fail Events: captures/31-cel-error-repro.txt:35-36.
  EXECUTED separate dbg-gctx error Event: captures/14-gctx-error-event.txt:7.
  Expression placement: upstream-drafts.md:141-167 and
  manifests/90-debug-gctx-annotation.yaml:17-25.
- Readiness: K pkg/controllers/policystatus/controller.go:330-340,372-380,
  462-479,558-571. This admission-disabled policy's captured condition is
  reporting read access, not a target-condition execution check.
  EXECUTED status: captures/31-cel-error-repro.txt:28.

### flags for the builder

- Capture 31's header guesses policy-event/scan attribution. Display the
  watched rows, not that header as a measured trigger count. Keep absent
  metric series -> count 4 separate from individual UR animations.
- Capture 30's inline 3.2s/full-window annotation is wrong. Use timestamp
  arithmetic and label any corrected output as an edited excerpt. The
  underlying captures remain unchanged.
- Several old manifest comments retain disproved wording. In particular,
  manifests/05b-kyverno-metric-suspend-http.yaml:5 still describes the GCE
  path as a silent no-op. Its executable condition works; use the corrected
  slide explanation rather than presenting that comment as a current fact.
- Preserve the source/capture distinction in s15 and both policy names in
  s17. Keep clock diagrams schematic. Do not derive per-target latency or
  per-tick execution from these captures.

No remaining mechanics blocker within this round's scope. Checks passed
for ASCII, SPDX headers, all 38 unique slide IDs, required slide fields,
and unchanged slides outside s12-s19/s18a. Changes outside those blocks
are limited to the spec status and the act's pinned source key. The
round report is appended once. No rendering or runtime tests were added.

## round 5 - findings honesty (codex)

Verdict: PASS after corrections. All twelve slides in s13-s23, including
s18a, now specify adjacent tier-chip placement. No new slides; total is 38.
This round reviewed saved evidence and source. No new cluster execution or
rendered-deck check. The round-4 Kyverno mechanism remains intact.

### changes applied

- Added a common placement contract and a `tiers` field on every target
  slide. Use the engine's existing .tier.exec/.ver/.inf classes. Reuse
  .tier.part for OPERATOR-REPORTED, NOT TESTED, and NOT INSPECTED. Chips
  appear with their own claims and survive static/reduced-motion views.
  They do not alter verbatim terminal output. Engine CSS already supports
  these classes: wow/skeleton-head.html:52-57. No engine files changed.
- Changed s14/s15/s16/s19/s20 titles to remove implications of no evidence
  anywhere, total error erasure, an absent deployed metrics stack, captured
  network causality, or repeated executions beyond the recorded run.
- Separated source fixtures, runtime outputs, and explanatory diagrams on
  s13-s19. Captured histogram values are aggregate observations, not a UR
  trace or a count of distinct targets. The minimal repro's three controls
  remain NOT TESTED: upstream-drafts.md:230-231.
- Made s20 a bounded replay. It stops at one observed re-suspension.
  Any continuation carries INFERRED before it appears. Standardized the
  first-observation duration to 3.397s and anchored 123.947s explicitly to
  the user-resume marker. Both come from capture 30's timestamps.
- Recut s21 into capture summary, source mechanism, fixed-version result,
  and a qualified manual-patch footnote. Kept 3 Jobs / 5 attempts, with
  named repeats. Resumed x15 is one aggregate Event row, not fifteen user
  resumes or a captured counter progression. The version toggle compares
  saved runs, not a live upgrade.
- Recut s22 to keep trainer-b's stored object, its operator-reported Pod
  state, the derived Running=0, and trainer-c's bounded watch distinct.
  Removed the unsupported continuous claim that the metric condition never
  re-established. RR is not credited with automatic wedge detection.
- Recut s23 around actual before/after queries. Credited the pass/success
  report before deletion. The post-delete Events query saved a count only;
  its earlier rows are not presented as a captured post-delete listing.
  UR absence is captured; the controller-delete explanation is VERIFIED.
  The no-receipt conclusion is INFERRED and restricted to inspected records.

### critical evidence boundaries

| Claim | Placement and evidence |
|---|---|
| 8+ minute GCE no-op | OPERATOR-REPORTED directly beside the interval throughout s14. Capture 08 has only an initial sample at lines 1-2. Duration comes from INTEGRATION-LOG.md:171-173. The 171ms control sample is EXECUTED: captures/09-ab-test.txt:1-3. |
| Error surfaces | VERIFIED source-return diagram, EXECUTED selected histogram, and separate EXECUTED reporting Events. No universal "only surface" claim. Captures/18-metrics-surface.txt:2,6; 31-cel-error-repro.txt:35-40; 14-gctx-error-event.txt:7. |
| Fact/action clocks | VERIFIED source paths and saved 5s/15s configuration. The lab log's 60s setting is OPERATOR-REPORTED configuration, not an observed tick series. S18a's arrows remain schematic; individual UR trigger sources remain uncaptured. |
| One repeat versus unbounded repeats | EXECUTED first re-suspension at 123.947s after the resume marker: captures/30-lab2-kyverno-phase.txt:5-8. VERIFIED no cap in this rule: manifests/05b-kyverno-metric-suspend-http.yaml:10-36. Further repetition under recurring eligibility is INFERRED, not a Kyverno impossibility claim. |
| Three Jobs, five attempts | EXECUTED take summary: trainer-b twice (captures/12:1-17,13:70,95-106,15:1-4), trainer-c twice (16:1-7,16b:1-5), trainer-gce once (16c:1-3). Named mapping is also in INTEGRATION-LOG.md:308-325. The x15 aggregate is captures/13-trainer-b-wedged.txt:45. |
| Manual status patch | Rejection OPERATOR-REPORTED; same-guard explanation INFERRED. Raw command/payload/response absent. INTEGRATION-LOG.md:284-286; codex-notes-r3.md section B, manual SET-startTime subsection. No fabricated terminal sequence. |
| Broken versus fixed | EXECUTED v1.34.0 symptoms and v1.34.3 accepted reset remain separate. v1.34.2 fix is VERIFIED from release source, not a lab execution. Captures/12,13,30:5-6; codex-notes-r3.md sections A-B. |
| Status, metric, and eligibility | Captured trainer-b spec/status: captures/13:70,95-106. VERIFIED matcher: src/pkg/catalog/kartas/batch_job.go:42-44,52. Runtime metric 0 and its ineligibility explanation are INFERRED from that state. The Pod-running account is in INTEGRATION-LOG.md:251-256. Capture 16:3-7 is a separate trainer-c take, not a continuous metric trace. |
| No controller-convergence proof | RR Executed follows successful patch command: rr-poc/rr_poc.py:444-447, VERIFIED. Neither side demonstrated automatic wedge detection: INTEGRATION-LOG.md:502-506. The RR detection experiment is NOT TESTED. |
| Deletion forensics | EXECUTED report row before: capture 17:2-3; zero after: 20-21. Event rows before: 4-11; count 7 after: 22-23. UR absence before/after: 12-13,24-25. Policy status before only: 14-15. The bounded conclusion and uninspected sinks remain beside these boxes. |

Kubernetes source was re-read locally: U1 (v1.34.1)
pkg/controller/job/job_controller.go:639-655,1036-1058 and
pkg/apis/batch/validation/validation.go:698-704; U2 (v1.34.2)
pkg/registry/batch/job/strategy.go:383-387,405; U checkout
CHANGELOG/CHANGELOG-1.34.md:954,1037-1039. Source-root definitions are in
codex-notes-r3.md:14-26. No new upstream lookup was needed.

### flags for the builder

- Keep OPERATOR-REPORTED visible beside the 8+ minute interval and manual
  patch account. Do not replace either with an EXECUTED banner for the act.
- Raw captures include superseded interpretation: "safe after settle",
  "startTime cleared", the 3.2s/full-window annotation, guessed UR trigger
  counts, and the broad no-surviving-surface conclusion. Use the saved
  observations with the corrected captions, or identify an edited excerpt.
- Keep the three version labels distinct: v1.34.0 failed in the lab;
  v1.34.2 contains the source fix; v1.34.3 resumed in the lab. Do not imply
  the two captured clusters were one cluster upgraded during the animation.
- The printed/static view must retain names, time qualifiers, and chips.
  A source diagram plus an EXECUTED terminal panel cannot share a single
  EXECUTED label across both. No scrolling or additional slides are required.

No remaining honesty blocker in this round's scope. Checks: ASCII and
headers; 38 unchanged slide IDs; twelve tier-placement fields; required
slide fields intact; all slides outside s13-s23/s18a unchanged. Other edits
are the shared label contract and version line. The round report is appended
once. Captures, source checkouts, engine HTML, and the integration log were
not modified.

## round 6 - rr protocol (codex)

Verdict: PASS as a description of the executed rev2 demonstrator after
corrections. No claim of complete KEP implementation, generic action
support, crash-tested recovery, or guaranteed receipt delivery remains in
the reviewed RR story. Source: rr-poc/rr_poc.py, 471 lines, revision-2
header. Read the complete runner, local rule/catalog fixtures, captures
20-24, and the prior r4/r5 findings. No code edits or cluster execution.

### changes applied

| Slide | Correction |
|---|---|
| s24 | State key is rule NAME plus workload UID, not rule UID/revision. Epoch 0 is initial state. Detection uses weSuspended plus current suspend=false; it does not authenticate an actor. A later epoch does not re-arm this rule. No separate allowance timer was implemented. |
| s25 | Intended and pendingOp writes gate the patch. UID and existing suspend=false are the only patch tests. Outcomes are separate receipt objects. Removed the assertion that every failure exit yields a persisted receipt. Recovery is source-only and checks spec, not controller convergence. |
| s26 | WouldAct spends no action cap or budget, but Observe still writes receipts/state and runs earlier recovery/resume/escalation/handle logic. Preflight runs only with --impersonate. Phase logs are captured; actionReady schema is source-verified, not dumped in capture 20. Removed the ambiguous loop-latency shorthand. |
| s27 | Kept detection at 0.692s, escalation at 124.526s, and final check at 300.147s, each relative to the captured resume marker. ResumeDetected is actor-unknown. Escalation is a receipt/log, not delivered notification or human approval. Comparison is sequential and uses different Jobs. |
| s28 | Missing handle and failed preflight are the executed skip cases. Revocation occurred before evaluation. Capture 23 prints phases, not reason JSON. The earlier Kyverno RBAC run is separately identified. |
| s29 | Exact captured Executed fields, separate Intended linkage, and phase-specific schemas. targetResourceVersion is the list snapshot, not a write guard. Evidence is a PromQL result sample, not range history. Removed the unsupported actor-attribution promise. |
| s30 | Four bounded limit views cover identity/process, adapter/configuration, persistence/recovery, and unexercised failures. Added ignored/generalized-knob caveats and best-effort outcome persistence. No green recovery or failure-test badges. |
| s1/s2 | Same fact source and a separate RR run, not identical fact samples. RR reads local files and does not use a RuntimeRule CRD. |
| s31/s32 | Carry the refined observe, receipt and recovery limits into the table and conclusion. Label the RR layer as the tested demo protocol, not a complete governor. |

Extended the adjacent tier-placement contract through s30 and added a
`tiers` field to each RR slide. All 38 slide IDs remain unchanged.

### source facts driving the corrections

All runner references below are VERIFIED, not new fault executions.

- State and allowance: rr-poc/rr_poc.py:175-194,368-403,444-450.
  actionsPerEpoch is parsed/logged at 339,347 but is not used as a counter.
  actedInEpoch is a boolean. reArmOnResume=false is actually checked at
  394. The fixture declares the tested values at rr-poc/rules.yaml:19-23.
  Preserved one initial successful action and subsequent no-rearm behavior;
  did not imply support for arbitrary actionsPerEpoch values.
- Resume identity: rr-poc/rr_poc.py:237,253-264,372-381. It reads managedFields
  explicitly but does not use managers to gate detection. No ordered actor
  audit or human classification is implemented. Detection precedes the hits
  membership test, within a loop that successfully obtained a facts response.
- Write gates and tests: rr-poc/rr_poc.py:275-285,434-461. Only successful
  Intended and pendingOp persistence permits this patch. Existing field
  false is required; missing is not equivalent for JSONPatch test. No
  resourceVersion, PromQL, status or freshness test accompanies the write.
  FailedPrecondition classification depends on matching stderr text;
  remaining patch failures go to FailedVisible.
- Later persistence is not guaranteed: rr-poc/rr_poc.py:124-136,288-319,
  372-381,444-463. Receipt write failure logs and returns None. Outcome,
  resume and recovery callers do not require that return value to succeed
  before changing state. Final state.save failure is also not fatal to the
  loop. This is why the old every-failure-gets-a-receipt storyboard was wrong.
- Pending recovery: rr-poc/rr_poc.py:288-319,364-366. It runs on the first
  successful facts query, using the listed UID map. It checks spec.suspend;
  a missing target, including an omitted target after a list error, takes
  UnknownOutcome. It marks the epoch consumed. No capture exercises a
  nonempty pending operation, timeout, or failed-precondition recovery.
- Observe: rr-poc/rr_poc.py:364-432,463. Its WouldAct branch precedes the
  Enforce permission refusal and budget gate. It carries actionReady=false
  too. Without --impersonate, ready is assumed true at 417. Earlier state
  transitions and recovery still run, so "Observe consumes nothing" must
  describe the WouldAct action allowance, not a read-only state machine.
- Identity: rr-poc/rr_poc.py:65-81,124-131,178,196-203,210-237,268-285,417.
  Fact reads, target reads, state and receipt writes use ambient credentials.
  Only preflight and workload patches receive --impersonate in this runner.
- Adapter/configuration: rr-poc/rr_poc.py:87-115,226-250,333-340,405-450.
  Python translates one local dotted boolean suspend action, keyed by Kind.
  Only Job, CronJob and Deployment have list mappings. Printed handle names
  are not execution coverage. The loop is hardwired to suspend; it does
  not dispatch on action.type. The loop budget increments on successful
  patch return, not each attempted patch. Rules load once at startup.
- Receipt schema: rr-poc/rr_poc.py:118-166,210-223,241-248,377-387,415-420.
  A ConfigMap in rr-system contains data["receipt.json"]. Workload receipts
  carry rule name, phase, workload identity, list-time targetResourceVersion,
  construction time and phase detail. update creates another receipt with
  supersedes; it does not replace Intended's phase. Fact/list failure receipts
  have scope=rule, without workload identity. ResumeDetected does not carry
  the action's evidence/handle/operation fields. No rule UID/revision,
  definition hash, or authenticated action actor is recorded.
- Facts errors: rr-poc/rr_poc.py:210-223,356-363. Handled command/response
  errors attempt FactsUnavailable. json.loads is uncaught and can terminate
  the loop without a receipt. No universal skip-and-report promise is valid.

### executed evidence retained

- Observe: captures/20-rr-observe.txt:3-7 records four WouldAct receipts;
  10-11 records unsuspended trainers. Enforce: captures/21-rr-enforce.txt:4-7
  records two Intended/Executed pairs; 10-11 records suspend=true. The
  successful pair does not fault-test either persistence gate.
- Resume comparison: captures/22-rr-anti-trap.txt:2,6-10 gives the RR offsets;
  captures/30-lab2-kyverno-phase.txt:5-7 gives the 123.947s Kyverno offset.
  The RR final value is a sampled check, not proof of continuous health.
- Skips: captures/20-rr-observe.txt:5 and captures/23-rr-rbac.txt:2-15.
  The Deployment definition has no suspendDefinition:
  src/docs/catalog/apps-deployment-v1.yaml:9-50. Prior Kyverno comparison:
  ../03-experiments-executed.md:110-117.
- Deletion: captures/24-rr-forensics.txt:2-8 lists four surviving receipts;
  10-32 contains one full Executed body. It has the rule/target/evidence/
  patch/operation linkage, not every field on every phase. Mutable names
  and reused run IDs remain a retention limit, per r4 M10 and
  rr-poc/rr_poc.py:139-141; rr-poc/run-rr-phase.sh:34,40.

### flags for the builder

- Do not promote actionsPerEpoch or action.type into generalized controls.
  Rev2 displays those fixture fields without implementing those abstractions.
  This review corrects their presentation; it does not change the runner.
- Do not animate Intended mutating into Executed. Keep two objects and the
  supersedes link. Do not attach an actor name, full sample window, rule
  revision, or write-tested resourceVersion to the captured JSON.
- Keep failure/recovery edges VERIFIED and NOT TESTED. Receipt delivery on
  those edges is best effort. An unhandled JSON error must not land in a
  green FactsUnavailable card as if that outcome had been demonstrated.
- Preserve the four compact scope views/static summaries on s30. The code
  boundaries must remain visible without adding scrolling or implying that
  a successful startup with no pending operations tested restart recovery.

Checks passed: ASCII, headers, 38 unchanged IDs, all required fields,
seven new RR tier fields, local citation ranges, and exact edited-slide
scope. Outside s24-s30, only RR references in s1/s2/s31/s32 changed; shared
label range and spec version were updated. No captures, runner, manifests,
integration log, source checkout, or engine HTML were modified.

## round 7 - narrative (codex)

Narrative verdict: PASS for the revised arc, act handoffs, comparison pair
and single ending. One-idea discipline still needs arbitration on the five
slides below. This is a spec review, not a rendered-deck approval.

Read SLIDES.md top to bottom. Editorial judgments below are not new lab
findings. No new execution or source-mechanism claim was needed.

### changes applied

- s1 keeps the concrete resume/re-suspend moment and its capture-derived
  number. Removed the premature RR escalation reveal and the vague "no new
  human decision existed" line. The outcome now arrives on s27.
- Added a narrative contract with explicit playback order. IDs remain
  stable. Quiet act labels and outgoing handoff lines replace any need
  for chapter-summary or divider slides.
- s4 now shows the join series rather than opening a metric-family
  inventory at the same time. The later scrape walkthrough keeps that role.
- Act 2 now runs s12 -> s19 -> s14 -> s13 -> s15-s18 -> s18a. The earlier
  network-policy obstacle precedes the GCE failure. The observed twin
  result precedes the projection explanation. The act now ends with
  working metrics conditions and their clocks, not an earlier outage.
  s19's title explicitly identifies a different, earlier rule.
- Act 3 now runs s21 -> s22 -> s20 -> s23. The known v1.34.0 defect and
  stored-status consequence precede the healthy v1.34.3 repeat-action
  test. Explicit earlier-lab and clean-comparison labels prevent a viewer
  from attributing the later re-suspension to the platform defect.
- s20 establishes one Kyverno lane and stops at its recorded outcome.
  s27 owns the only interactive replay. It reuses the lane layout and
  adds RR, each aligned to its own resume marker. The separate setup
  interval is not plotted on that resume-relative axis. Detection and
  escalation remain different markers; no speed contest or repeating
  measured loop is introduced. Static fallbacks retain the scope limits.
- s32 is the single terminal decision: build the runtime action protocol
  with optional Kyverno composition. It carries INFERRED and retains the
  prototype/alternative-hosting limits. Removed its findings recap.
- s31's evidence table and s33's evidence index are reference views outside
  the main playback and main-print sequence. Removed s33's final number
  replay. Both return to their caller. Evidence, pending controls and
  rerun warnings remain accessible; neither view becomes another ending.

There are 38 authored IDs: 36 main slides and two reference views. No
slide evidence was deleted. The hook's now-unused RR citation was removed;
all other per-slide data blocks remain unchanged.

### act handoffs

| Boundary | What carries the story forward |
|---|---|
| s2 -> s3 | The lab map hands off to how workload objects become named metric series. |
| s11b -> s12 | The exporter facts become a concrete Running predicate. |
| s18a -> s21 | Working condition evaluation leads to the resume test; account for the earlier platform defect first. |
| s22 -> s20 | Healthy resume on the fixed cluster isolates the next policy decision. |
| s23 -> s24 | The inspected action record leads to an explicit allowance and receipt protocol. |
| s30 -> s32 | Prototype limits define the work still to build; the ending chooses that work. |

The first comparison reveal is s20; its answer is s27. Four intervening
slides supply provenance, allowance, write gates and Observe behavior.
The shared lane and explicit callback preserve the pair without moving
its answer ahead of the protocol explanation. s27 leads onward to skips,
retention and limits; it does not signal that the talk has ended.

### one-idea flags for arbitration

These are presentation splits, not factual corrections. They were not
performed automatically because they change pacing and companion count.
Keep the round 6 limits beside any claims they qualify.

| Slide and exact competing material | Judgment | Suggested placement |
|---|---|---|
| s11: "unknown parts keep their workload identity" plus "unattributed_pods is a current gauge; attribution_errors_total counts evaluation failures". | Two lessons: attribution outcomes and metric accounting. The fifth animation step starts the second lesson. | Keep the three outcomes on s11. Put gauge/counter accounting in an adjacent companion if it needs teaching, or a source caption if it does not. Do not replace the corrected gauge/counter distinction with a fabricated count. |
| s17: "rr-cel-runtime-error has no matchConditions" plus "separate policy: dbg-gctx puts the bad projection lookup in its MUTATION". | Two policy traces and different report results. The current exclusive views are accurate but require the listener to reset the whole example. | Keep the int-cast report explanation on s17. Put the mutation-error Event on a companion immediately afterward if both traces need full diagrams. Keep both examples in the evidence; their contrast prevents an all-surfaces claim. |
| s25: "Intended and pendingOp must both persist" plus "pending recovery is a source-only inset" and the failure taxonomy. | The normal write gate and startup recovery are distinct stories. Error exits are supporting qualifications; a full recovery inset is another mechanism. | Keep the gated write path and outcome-write caveat on s25. Give recovery its own companion only if it is taught in depth; otherwise retain a short NOT TESTED note and its full limits on s30. |
| s29: "the captured Executed receipt survives" plus "source-only variant cards". | One executed retention test becomes a separate phase-schema tour. | Keep the surviving four names and the actual Executed body on s29. Preserve a short warning that schemas differ; make detailed variants selectable source material or a companion. |
| s30: "use four short views" covering identity/process, adapter/configuration, persistence/recovery and unexercised failures. | Four limitation lessons under one umbrella. Replacing screens avoids scrolling but does not make them one idea. Highest-priority pacing choice. | Either retain one compact visible boundary map with the detailed text as source captions, or split before s32 by the boundary taught. In both choices keep ambient identity, startup rules, narrow patch guards, best-effort outcomes, untested recovery and the JSON-parse crash limit available and visibly scoped. |

Other pacing flags:

- The exporter act has 13 main slides before the policy attempt. Its
  new handoff explains why it belongs, but a shorter talk path remains
  an audience choice. No internals coverage was cut in this round.
- s3/s4's GPU-join illustration opens a motivation that this lab never
  executes. Keep its illustrative label visible and pivot promptly back
  to the actual Running series. It must not become a second promised demo.
- s2 is one topology idea if setup/tier/benchmark details stay captions.
  s10 is one scrape example if zero-fill/deletion mechanics stay captions.
  Promoting those captions to full animation acts would add second ideas.
- s21's manual-patch hypothesis should remain a qualified footnote, not a
  second command demo. s28's two skip cases can support one comparison of
  named refusal reasons; splitting their full RBAC setups into live demos
  would need another slide. Neither is silently removed here.
- The builder must honor reference roles and file order. Sorting IDs or
  rendering every section into the main next-slide sequence would restore
  the broken order and competing endings.

### checks

Passed ASCII, SPDX header, unique/stable IDs, all required slide fields,
main/reference counts, and retained per-slide tier instructions. Exporter
animation bodies s5-s11b are unchanged except s11b's handoff. Source/data
blocks match the round 6 baseline except the unused RR citation on s1.
Only SLIDES.md and this append were written in wow. No captures, log,
runner, manifests, source checkout or engine HTML were changed.

## round 8 - pedagogy (codex)

Pedagogy verdict: PASS at the spec level. The requested concepts now have
plain definitions before their first explanatory use in playback order.
The log supplies evidence; it is no longer required to identify the actors,
read the query, or understand the named diagnostic tests. Rendering still
needs to verify that the insets and scope captions fit.

### changes applied

Added `inset cards` fields to 36 slide specifications. These include short
definitions, diagram-label translations and reminders at later examples.
They are not 36 new slides. A shared contract requires visible cards before
code/animation beats, one small card slot, and compact static fallbacks.
No tooltip, separate glossary visit or extra interaction is required.

| Concept | First definition in playback | Plain meaning supplied |
|---|---|---|
| Karta description and catalog | s2 | A description is a recipe for interpreting a workload kind, its parts, status and supported action fields. The catalog supplies bundled descriptions. A cluster Karta CR is another description source. |
| Workload, component, instance | s3 | Application object, type of part, and a named part of that type. The JobSet labels make the distinction concrete. |
| Informer, root, attribution, jq | s5 | A list-and-watch cache; the described workload object; assigning workload/part labels; expressions over object JSON. |
| podSelector | s7b | Karta expressions that find a pod's component, instance or replica. Cache trimming does not edit the live Pod. |
| statusMappings | s9 | Tests inside statusDefinition that map fields/conditions to common status names. Computed workload phases differ from Pod.status.phase. |
| mutate-existing, CEL, PromQL | s12 | A worker edits stored objects. CEL evaluates the condition; PromQL selects metric samples. Admission is the incoming API-write path and is disabled here. |
| GCE | s14, before s13 | GlobalContextEntry configures a named cache inside a controller process. This entry polls Prometheus. Removed the unexplained GCE abbreviation from s19's outgoing line. |
| Projection and JMESPath | s13 | A named extracted result, selected by its name. JMESPath extracts it at storage time. Entry name, projection name and query expression get separate labels. |
| UR | s15, before s16 | UpdateRequest is a saved background work item. Its status is not proof that a target changed. |
| Report versus action receipt | s17 | A reporting scan checks a simulated mutation. An action receipt links a decision to an action. Later s25/s29 explain this prototype's ConfigMap representation. |
| Allowance and epoch | s20, then s24 | Allowance permits automation; a cap limits its use. Epoch numbers this rule/UID's history and advances on detected resume. This rule does not renew the allowance. |
| Intended, pendingOp, suspend handle | s25 | Planned-action receipt, unfinished-operation marker, and a description's field/value recipe. The Job recipe becomes the prototype's guarded JSONPatch. |
| Observe, preflight, action budget | s26 | WouldAct for these candidates, permission check for the selected identity, and successful patches allowed per loop. The allowance remains per rule/UID. |

Other cards explain metric joins, GVK, metadata-only children, parking,
leaf components, gauge/counter accounting, absent series, separate clocks,
controller convergence, escalation, receipt linkage and ambient identity.
Parking means delaying attribution, not pausing a Pod. Explicit labels
separate exporter readiness, policy reporting readiness and RR actionReady.

### made self-contained

- s2 gives each topology box a plain role. It expands Runtime Rules (RR),
  names lab1/lab2 and their versions, and identifies the namespaces. The
  demo Jobs are sleep workloads. Fixture means example input/configuration,
  not evidence that a test ran.
- s3/s4 use "GPU-metric join" for the unexecuted illustration. LeaderWorkerSet
  is spelled out before its later LWS abbreviation. No extra GPU demo is
  implied or promised.
- s12 shows the decoded PromQL and explains the name-membership test. The
  URL, decoded query and policy are separate views. It keeps the partial-
  sample caveat. The decoded expression was checked against the manifest.
- s14 identifies the twin as a comparison policy without the GCE read.
  s13 labels the entry and projection names. s18 explains that gce-test
  is a namespace, not another cluster.
- s15 explains nil and the two places an error can be returned. Its audit
  label names Kyverno's Event/report routine; it is separate from Kubernetes
  API audit logs. Watch notifications and diagnostic Event objects also get
  distinct explanations.
- s16 identifies rr-cel-runtime-error as a Kyverno MutatingPolicy despite
  its name. Its nonnumeric Job name explains the integer-conversion failure.
  The pending controls are spelled out as comparison cases, still unexecuted.
- s17 introduces dbg-gctx as a separate annotation-writing diagnostic policy.
  It keeps both policy names and their different expression locations.
- s20 recalls the actual HTTP rule at the resume test. s23 names the records
  being inspected. s28 identifies the separate permission and missing-handle
  examples. None becomes another stage of the same Job's history.
- s24-s30 translate protocol names without adding guarantees: epochs do not
  re-arm this rule; Executed still means patch-command success; later writes
  can fail; receipts are mutable; recovery and failure branches remain
  source-only where previously marked.
- s31 has its own comparison/tiers key for direct reference access. s33
  explains the old settle gate and pending controls without requiring the
  log. Both remain outside the main playback; s32 remains the only ending.

### evidence checked

VERIFIED against local files. K means the Kyverno checkout read with
`git show v1.19.1:<path>`. These reads are not new executions.

- Description contract: src/pkg/api/runai/v1alpha1/types.go:19-55;
  src/pkg/api/runai/v1alpha1/structure.go:20-58,61-89,178-205,268-280.
  Concrete Job and JobSet fields: src/docs/catalog/batch-job-v1.yaml:9-57;
  src/docs/catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml:50-64.
- Catalog, watch/cache and status vocabulary:
  src/exporter/pkg/registry/registry.go:57-98;
  src/exporter/pkg/controller/controller.go:139-158,183-209;
  src/exporter/pkg/controller/events.go:297-311;
  src/exporter/pkg/state/workload.go:20-48.
- Mutation input and decoded query:
  manifests/05b-kyverno-metric-suspend-http.yaml:11-36;
  K pkg/policy/mpol.go:91-98;
  K pkg/background/mpol/processor.go:111-145.
- GCE/projection: K api/kyverno/v2alpha1/global_context_entry_types.go:38-77;
  K pkg/globalcontext/externalapi/entry.go:86-110,116-164;
  manifests/04b-kyverno-gce-projected.yaml:9-17. Retained the prior
  per-process scope explanation and its s18a source citations.
- UR/audit terminology: K api/kyverno/v2/updaterequest_types.go:26-33,53-75;
  K pkg/background/mpol/processor.go:230-276,308-351. The routine creates
  Events and conditionally reports; it is not the API server audit facility.
- Test identity: upstream-drafts.md:137-167,230-231;
  manifests/90-debug-gctx-annotation.yaml:1-25. The two report paths keep
  their existing source and capture evidence on s17.
- RR words map to implementation: rr-poc/rr_poc.py:87-166,175-194,210-223,
  268-285,333-349,368-450; rr-poc/rules.yaml:10-23.
  Receipt-body teaching still uses captures/24-rr-forensics.txt:10-32.
- Cluster/namespace aliases: INTEGRATION-LOG.md:6-9,64-72.
  The invalid settle test is visible at rr-poc/run-rr-phase.sh:44-49;
  its rerun limitation remains documented at INTEGRATION-LOG.md:529-543.

### checks and remaining flags

Passed ASCII, SPDX headers, required slide fields, all 38 unchanged IDs
and playback order, 36 main slides plus two reference views, preserved
source blocks, prior tier fields, and unchanged animation steps/playback.
The displayed PromQL equals the decoded manifest query after whitespace
normalization. New source citations are additive. No timing result changed.

The five round 7 split decisions remain open. Insets define vocabulary;
they do not make a multi-part slide smaller automatically. The builder
must use the specified replacement slot and retain the compact definitions
in print/mobile views. Do not promote reference-only source schemas or
controls to captured outcomes while simplifying the text.

Only SLIDES.md and this append were changed. No runner, capture, manifest,
source checkout, integration log or engine HTML was modified.

## round 9 - economy (codex)

### verdict and count

PASS for spec economy. Final count: 40 authored slides, 38 main slides
plus two reference views (s31 and s33). Added s17b and s30b. All previous
IDs retain their order. s32 remains the only main ending.

Every slide has at most four beats and one visual. Beats now total 1,270
words, down from 3,141, including the two new slides and both references.
The largest beat block is 43 words. The largest beats + inset + scope
block is 77 words. These are editorial whitespace counts, not lab results;
list ordinals are excluded. Titles, visual identifiers and tier chips are
additional. This is not a rendered pixel-fit test.

### changes

- Added a projector contract. Beats label the visual once, rather than
  becoming a duplicate text column. At most one short definition inset and
  one visible scope caption accompany it. A handoff replaces the last beat.
  Source paths, tier-placement instructions and reference notes stay off
  the projected canvas. No scrolling or smaller type to recover overflow.
- Split s17 by policy. s17 owns the int-cast policy's reporting result;
  s17b owns the separate dbg-gctx mutation-error Event. Each has one
  diagram and its own capture. Source references were partitioned with
  the examples. The two policies no longer compete on one slide.
- Split s30 by question. s30 explains the runner's implementation boundary;
  s30b explains incomplete outcomes and untested recovery/failure paths.
  Kept the ambient identity split, startup rules, limited handle adapter,
  JSON parse crash, best-effort later writes and no demonstrated wedge
  detection. Neither slide presents those branches as executed tests.
- Resolved the other round 7 split flags by cutting projected side lessons.
  s11 teaches the two unknown-part cases; s8 owns owner waiting. s25 owns
  the pre-patch gates; s30b owns recovery. s29 teaches deletion survival
  with selected Executed fields, not every receipt phase's schema.
- Cut repeated inventories and example tours. s2 has one topology, s3 one
  JobSet mapping, s4 one identity join, s8 one owner walk and s10 one parsed
  scrape card. Removed extra terminal/metric/schema panels from the visual
  instructions, not just their bullet copy.
- Simplified exporter timelines to reveal the bounded visual once and hold.
  Each retains numbered presentation offsets and a static fallback. Owner
  retry and selector failure no longer trigger a second full example tour.
  s9b says record status_eval, without implying that metric reason is the
  literal logged message.
- s15 owns the source return chain; s16 owns the executed four-UR repro and
  aggregate histogram. s18 owns the two working paths; s18a owns the clocks.
  Initial suspension timing is not replayed again on s20.
- s21 uses one broken/fixed version comparison. s22 uses one stored-state
  diagram. Attempt breakdown, manual-patch inference and operator accounts
  remain reference detail with their evidence qualifications.
- s20 owns the observed repeat; s27 reuses a muted Kyverno lane only to
  compare the RR outcome. Kept one-observed-repeat versus inferred
  continuation, separate runs, wider RR query, shorter polling and the
  sampled final check. This callback is intentional, not a second lesson.
- s23 and s29 retain separate forensics scopes. No missing-record claim
  becomes universal; no surviving ConfigMap becomes an immutable archive.
  Observe still consumes no action cap but can write state and receipts.
- s31 exposes one comparison row at a time, outside main playback. s33 is
  an evidence tree with rerun/control warnings, not another closing slide.

### per-slide volume audit

Beat words below exclude list ordinals. Detailed source notes retained in
SLIDES.md are non-projected. The visual must reuse these beats as labels
where possible; it must not add another explanation panel.

| Slide | Prior beat words | Final beat words | Final beats | Projected focus |
| --- | ---: | ---: | ---: | --- |
| s1 | 30 | 19 | 3 | One recorded repeat; save RR outcome. |
| s2 | 113 | 31 | 4 | Single topology and run scope. |
| s3 | 54 | 25 | 3 | One JobSet part mapping. |
| s4 | 62 | 33 | 4 | Identity join; no family inventory. |
| s5 | 73 | 33 | 4 | Event-to-store-to-scrape pipeline. |
| s6 | 79 | 37 | 4 | One description-selection slot. |
| s7 | 75 | 32 | 4 | One watcher map with parsed counts. |
| s7b | 70 | 35 | 4 | Trim/full cache tradeoff. |
| s8 | 76 | 32 | 4 | One owner walk; no second LWS tour. |
| s8b | 67 | 37 | 4 | Parts after workload attribution. |
| s9 | 70 | 37 | 4 | Matching status tests and dense output. |
| s9b | 63 | 31 | 4 | Undefined versus absent status. |
| s10 | 75 | 23 | 4 | One real scrape card. |
| s11 | 66 | 26 | 4 | Two unknown-part cases; no accounting panel. |
| s11b | 57 | 29 | 4 | Activity and readiness limits. |
| s12 | 69 | 32 | 4 | Target rule with wrapped decoded query. |
| s19 | 59 | 34 | 4 | Earlier policy interference; causal tier retained. |
| s14 | 60 | 35 | 4 | Control and its comparison limit. |
| s13 | 71 | 35 | 4 | Fetch versus named projection lookup. |
| s15 | 91 | 36 | 4 | Source return chain, without counter replay. |
| s16 | 104 | 34 | 4 | Executed minimal repro evidence card. |
| s17 | 113 | 27 | 4 | Int-cast reporting path only. |
| s17b | new | 33 | 4 | Separate mutation-error reporting path. |
| s18 | 59 | 29 | 3 | Two working paths and observation scope. |
| s18a | 98 | 40 | 4 | Three clocks in one diagram. |
| s21 | 110 | 38 | 4 | Broken/fixed version comparison. |
| s22 | 98 | 27 | 4 | Requested spec versus stored status. |
| s20 | 79 | 33 | 4 | One repeat on the fixed cluster. |
| s23 | 127 | 39 | 4 | Bounded post-deletion inspection. |
| s24 | 72 | 34 | 4 | Retained allowance and detected resume. |
| s25 | 68 | 35 | 4 | Intent and pending marker gate the patch. |
| s26 | 85 | 33 | 4 | Observe/enforce outcomes and gate scope. |
| s27 | 88 | 29 | 4 | One paired replay; no speed race. |
| s28 | 98 | 19 | 4 | Two skip reasons; no second Kyverno panel. |
| s29 | 95 | 33 | 4 | Surviving records with selected fields. |
| s30 | 241 | 39 | 4 | Runner implementation boundary. |
| s30b | new | 43 | 4 | Outcome/failure limits. |
| s32 | 40 | 17 | 3 | Only main ending: proposed action protocol. |
| s31 | 120 | 24 | 4 | One selected reference comparison row. |
| s33 | 66 | 32 | 4 | Reference evidence tree and rerun limits. |

### checks and builder flags

- Checked all 40 IDs and their fields. Maximum four beats, one visual,
  one inset, short scope and bounded fallback per slide. All titles are
  lowercase and at most 48 characters. ASCII and SPDX headers pass.
- All original data blocks are retained, with s17's sources deliberately
  divided between s17 and s17b. The decoded PromQL still matches the
  manifest. No capture, fixture, runner, source checkout or log was edited.
- Prior loop entries are unchanged. Their older slide counts and open
  split decisions are historical; this entry supersedes them.
- Retained exact results, evidence tiers and material scope beside claims.
  Longer source explanations remain non-projected reference notes. Do not
  restore them as more panels, animation screens or microtext.
- Still render at 16:9 to confirm actual fit. Watch s12's query, s15's return
  labels, s27's paired lane and s30b's failure exits. If they overflow, cut
  or shorten the visual labels; do not stack windows or scroll.
- No new coverage blocker. Detailed accounting/schema lessons are available
  in references. Existing numerical/publication review rounds still apply.

## round 10 - feasibility (codex)

### verdict

Feasible with the specified builder support. Every slide now has a literal
fx assignment and DOM contract. The unchanged engine alone does not meet
all prior reset, replay and static-mode promises. Those dependencies are
explicit in SLIDES.md, with code sketches and cheaper alternatives.

Count unchanged: 40 authored views, 38 main slides plus s31/s33 references.
No lab fact, source citation, evidence tier, beat or definition changed.
Only animation instructions, reference access and builder support changed.

### assignments

Copy each value into data-fxmode. Modes are space-separated. The two new
handlers are storySteps and safeCount, each an 11-line sketch in the spec.
There are 14 per-slide storySteps JSON sequences, all at most seven steps.
The other assignments use stock handlers or no animation.

| Slide | data-fxmode | Builder contract |
| --- | --- | --- |
| s1 | `safeCount` | Cancellable endpoint count; seconds outside the text span. |
| s2 | `popPills` | Four topology nodes; labels stay in place. |
| s3 | `popPills` | Pod/workload/part groups; no object morph. |
| s4 | `popPills` | Inputs and joined row; no traveling series. |
| s5 | `storySteps` | Fixed branched pipeline; node highlights replace dots. |
| s6 | `storySteps` | Fixed selection slot and labeled source cases; no sorting engine. |
| s7 | `storySteps safeCount` | Watcher groups, then delayed counts 5/19. |
| s7b | `storySteps` | Fixed cache routing cells. |
| s8 | `storySteps` | Owner/wait/result fields; highlight arrival and retry. |
| s8b | `storySteps` | Fill part slots; connectors do not move. |
| s9 | `storySteps` | Change source-example text and boolean phase values discretely. |
| s9b | `storySteps` | Reveal three pre-authored status result cells. |
| s10 | `storySteps` | Parsed scrape rows; no synthetic commands. |
| s11 | `storySteps` | Independent unknown-part rows; textContent for angle brackets. |
| s11b | `storySteps` | Symbolic T1/T2 activity values and fixed readiness. |
| s12 | `popPills` | Policy annotations pop; wrapped query stays complete. |
| s19 | `popPills` | Evidence nodes with tiers; no packet simulation. |
| s14 | `popPills` | Observation groups; operator account is not timed lane-data. |
| s13 | `storySteps` | Replace lookup/projection text in fixed slots. |
| s15 | `storySteps` | Verified return order and metrics branch; no looping error dot. |
| s16 | `popPills` | Complete UR rows plus separate aggregate cell. |
| s17 | `popTimelines` | One policy path; no data-glitch. |
| s17b | `popTimelines` | Separate policy path; no data-glitch. |
| s18 | `popPills` | Whole result rows with separate time origins. |
| s18a | `popPills` | Clock/read branches; no synchronized ticking. |
| s21 | `popPills` | Version cells with tiers; no retry or upgrade loop. |
| s22 | `popPills` | Spec/status/derived-metric groups; no health pulse. |
| s20 | `storySteps` | One compressed ordered-observation lane; no replay button. |
| s23 | `popPills` | Whole before/after rows; no event expiry animation. |
| s24 | `popTimelines` | State chips and connectors; no repeated cycle. |
| s25 | `popTimelines` | Two gates, patch, outcome; no restart animation. |
| s26 | `popPills` | Captured outcome cells and source gate caption. |
| s27 | `dualLane` | Held on entry; button replays RR with Kyverno held by CSS. |
| s28 | `popPills` | No-handle, denied, restored cells. |
| s29 | `popPills` | Surviving names and selected fields; no object erasure. |
| s30 | `popPills` | Runner boundary and identity branches. |
| s30b | `popPills` | Reveal source-limit labels, not simulated recoveries. |
| s32 | `popPills` | Only the proposed box pops; no cross-slide docking. |
| s31 | empty | Static reference; radio/CSS switches one selected row. |
| s33 | empty | Static reference links. |

### source findings and accepted cheaper choices

VERIFIED below means source read. New handlers and integration sketches
are proposed builder work, not features already installed in the engine.

| Finding | Source evidence | Choice in the spec |
| --- | --- | --- |
| runFx dispatches whitespace-separated tokens; prose and plus signs are not a composition API. | skeleton-tail.html:65-74 | Literal data-fxmode for all 40 views. |
| Pill and timeline pops use fixed stagger times and DOM order. data-glitch enables a continuing CSS pulse. | skeleton-tail.html:76-85; skeleton-head.html:115-121 | Pop complete evidence groups. No data-glitch. Reading-order timing is not execution timing. |
| countUp uses untracked requestAnimationFrame callbacks, mutates childNodes[0], and reads but does not append data-suffix. | skeleton-tail.html:54-63,118-132 | safeCount uses fxT/playTimeline; units are siblings. Cheaper fallback: show the final number with popPills. |
| dotTravel follows the first pipe's DOM order, loops, and needs both error attributes. Its stock positioning is at the stage's top edge. | skeleton-tail.html:137-156; skeleton-head.html:127-137 | No stock dotTravel. s5/s8/s15 use fixed branches and node highlights. No moving-dot geometry or custom routing system. |
| Terminal classification treats many ordinary capture rows as commands. typeInto temporarily replaces descendant markup. | skeleton-tail.html:31-51,87-116 | No terminal typer or typeInto required. Parsed evidence/complete code reveals are cheaper and avoid false prompts or disappearing tiers. |
| dualLane enters with all event chips visible; the button plays both configured lanes. Clicking during play resets instead of pausing. | demo-component.js:35-75 | Explicit held entry. Keep Kyverno visible by lane-specific CSS while RR replays. No live-run claim or pause semantics. |
| Lane names/strong flags are not rendered; event positions and labels are direct, with no collision solver. .ev.on's pop animation also changes transform. | demo-component.js:15-30,46-58; skeleton-head.html:120-121,143-147 | Author headers. Omit a zero-time event beside +0.692s. Keep centering, anchor endpoints and shorten labels. Static held table if still crowded. |
| fxClear resets timers/terms/demo, not all custom text/classes. Only one _demoReset callback is installed. | skeleton-tail.html:54-63; demo-component.js:64 | One demo only. Shared hold/clear adapter restores data-final values and cancels all selected custom work. No new async subsystem. |
| No JavaScript print/reduced-motion guard exists. CSS print exposes slides and fx-hide only; mobile explicitly enables scrolling. | skeleton-tail.html:159-199; skeleton-head.html:170-190 | Required static lifecycle/CSS adapter. Prebuild the single demo for unvisited-slide printing. Compact replacement visual; no scrolling. |
| Navigation takes every section.slide and handles space/arrows without a form-control guard. | skeleton-tail.html:172-183 | s31/s33 are separate reference pages. Mark links data-reference to hold the caller. Add the keydown guard for real replay buttons. |
| Custom registration needs the engine's customFx object first. The assembler already injects demo code before init registration. | skeleton-tail.html:64,199; assemble.py:18-19 | Install shared handlers/adapter after the demo, before registering initDeck. Do not register in earlier slide HTML. |

Other expensive stories were reduced in place. s6 changes a selection slot
instead of moving/sorting cards. s9 updates digits instead of morphing
chips. s13 changes a text slot instead of simulating an editor. s32 pops a
box instead of moving an element between slides. No new layout solver,
SVG path traversal, modal router or event-animation framework is required.

### checks and remaining builder flags

- All 40 IDs, titles and their order are unchanged. Each has one fx line,
  one visual and explicit DOM requirements. All existing beats, scopes,
  definitions, tiers and data fields compare unchanged with round 9.
- All 14 fx-steps payloads and the one lane-data payload parse as JSON.
  Each storyboard has ordered nonnegative presentation offsets and at most
  seven steps. The lane payload retains the four exact capture offsets.
- Both customFx sketches are 11 lines. All six JavaScript blocks pass
  node --check, wrapping the inline keydown guard in a function for parsing.
- A deterministic timer/DOM stub ran the proposed effects with the real
  engine fxT/playTimeline/fxClear. Counts stopped after clear, restarted,
  and reached exact decimal endpoints. All 14 frame sets reached their
  held text/reveal states without requestAnimationFrame. This is a sketch
  check, not a browser, lifecycle/media or projector-render test.
- Highest remaining visual risk: s27 endpoint labels and its compact
  fallback. The stock engine cannot solve their layout. The cheaper final
  choice is the same captured outcomes in a held table, not a denser track.
- Implement the shared lifecycle adapter before claiming print/reduced/
  mobile support. Then test leaving mid-effect, reentry, printing during
  replay, unvisited-slide print, and keyboard interaction with the button.
  Those checks are pending builder work; no engine implementation ran here.
- ASCII and SPDX headers pass. Prior loop entries are unchanged. Only
  SLIDES.md and this appended report changed; source, engine, captures,
  fixtures and the integration log remain untouched.

## round 11 - data audit (codex)

### verdict

PASS for the audited spec. All 40 slides now have embed and number audit
fields. Capture-backed output has inclusive path:line ranges. Source-only
examples explicitly decline a fake transcript. Count remains 40 authored
views: 38 main slides and two references.

Verified text against the original captures, not just the convenience
excerpts. All 74 indented excerpt lines in data-excerpts.md match their
advertised capture lines exactly after removing the four-space Markdown
indent. That establishes copying fidelity, not the truth of recorder
commentary embedded in those lines.

### changes

- Pinned exact raw slices for every captured number/output. Kept raw output,
  parsed cards, timestamp arithmetic, source examples and operator accounts
  distinct. Raw excerpts cannot acquire corrected text silently. The pins
  serve the existing visual/evidence detail, not another projected panel.
- s2: version identities are now OPERATOR-REPORTED setup. This capture
  directory has no saved server-version/Helm-image attestation query.
  A recorder's v1.34.3 banner is not such a query. s21 uses the same tier
  for cluster setup, while the fixed release remains VERIFIED from source.
- s9b/s11b: restored the full output identifiers karta_workload_status and
  karta_exporter_last_event_timestamp_seconds. The short suffixes must not
  masquerade as literal metric names.
- s12: the displayed PromQL now exactly equals the URL-decoded expression.
  Labeled it URL-DECODED source, not a verbatim YAML line or query response.
  s13: removed broken:/corrected: prefixes from inside CEL code; they are
  captions. The actual successful poll payload contains trainer-c.
- s14: the twin result is about 171ms to first observation, not exact action
  latency. The 8+ minute account remains OPERATOR-REPORTED. The separate
  initial sample is not an eight-minute watch trace.
- s16: removed the projected 150s duration claim and pinned all four UR
  histories. The recorder announces that watch budget but records no exact
  elapsed endpoint or per-UR trigger source. Pending/Completed/DELETED and
  the histogram observations remain EXECUTED with the existing limits.
- s21: narrowed the first beat to three Jobs across five documented stalled
  resume attempts. Repeated identical rejection is logged for trainer-b
  and trainer-c; it is not independently logged for every attempt. Kept
  the five-attempt source inventory and defect association separate.
- s23/s29: pinned bounded Kyverno before/after selections and the complete
  surviving Executed receipt. No before-only status read becomes an after
  read; no count-only Event inspection becomes a reconstructed listing.
- s31: added explicit per-row capture pins. The earlier Kyverno permission
  result remains an earlier execution note, not a terminal capture in this
  directory. Empty UR message remains VERIFIED; FactsUnavailable stays
  source-only, recurrence INFERRED and outcome verification PARTIAL.

### measured number ledger

EXECUTED below refers to saved executions. Arithmetic is computed from
printed markers, not new measurements of controller latency.

| Slides | Value | Capture evidence and handling |
| --- | --- | --- |
| s1/s20/s27 | 123.947s; rounded 123.9s | captures/30-lab2-kyverno-phase.txt:5-8. 18:23:14.635 minus 18:21:10.688. Separate from initial policy apply. |
| s7 | 5 started, 19 skipped | Count matching lines across captures/03-exporter-watchers.txt:1-25. Started lines: 5,14,16,18,25. Shared Pod/Karta informers are outside this count. |
| s4/s10 | join 1; pods 1; replicas 1; Running 1; Suspended 0 | captures/02-metrics-raw.txt:125-125,200-200,236-236,299-299,302-302. All refer to trainer-a in the same scrape. |
| s14 | about 171ms | captures/09-ab-test.txt:1-3. Printed-marker difference is 171.065ms; display only about 171ms. t+1 is a poll label. |
| s15 reference | error count 7; separate dbg-gctx count 3 | captures/18-metrics-surface.txt:6-6 and :2-2. Aggregate evaluations, not Job or per-UR counts. |
| s16 | four URs; AGE 0s/1s; absent then count 4 | captures/31-cel-error-repro.txt:4-6,8-28,39-40. Four unique names; no fabricated zero-valued before series. |
| s18 | 644ms | captures/10-v2-http-timeline.txt:1-3. 17:45:19.693 minus 17:45:19.049. First true observation after the apply marker. |
| s18 | about 2.5 minutes | captures/19-gce-corrected.txt:1-4. 18:07:28.165 to the whole-second 18:09:59 observation; about 151s. No millisecond-precision GCE latency claim. |
| s18 reference/s31 | 3.397s | captures/30-lab2-kyverno-phase.txt:1-3 timestamps. The script's 3.2s annotation is wrong. Keep the correction outside raw output. |
| s20/s21 context | next sample at +95ms | captures/30-lab2-kyverno-phase.txt:5-6 timestamps. Its 0.0s text is not a precise resume latency. Projected cards use the actual status fields. |
| s21 | 3 Jobs / 5 documented attempts | First b: captures/11-resume-trap-kyverno.txt:1-2 and captures/13-trainer-b-wedged.txt:9-12; next b: captures/15-trap-take2.txt:1-4; c: captures/16-trap-trainer-c.txt:3-7 and captures/16b-trap-take3.txt:1-5; gce: captures/16c-trap-gce.txt:1-3. Not five distinct Jobs or five saved rejection traces. |
| s23 | report rows 1 -> 0; Events after 7 | captures/17-kyverno-forensics.txt:3-3,5-15,19-25. The after Event count does not preserve the prior list as a proven after-list. |
| s26 | WouldAct x4; two Intended/Executed pairs | captures/20-rr-observe.txt:3-7,10-11 and captures/21-rr-enforce.txt:4-7,10-11. The missing-handle receipt is not a WouldAct. |
| s27 | +0.692s / +124.526s / +300.147s | captures/22-rr-anti-trap.txt:2-3,6-7,9-10. Common RR origin 18:28:43.906; the last observation is sampled. |
| s29 | four surviving records; epoch 0; resourceVersion 1908 | captures/24-rr-forensics.txt:2-8,10-32. Two WouldAct, one Intended, one Executed. JSON version is a string. |
| s29 | sampleTime 1789410393.863; value "1" | captures/24-rr-forensics.txt:22-26. One query-result sample, not the trailing-window history. Receipt construction time at line 20 differs from the logged receipt completion. |

### source numbers and unexecuted examples

- VERIFIED: nine normalized phases in
  src/exporter/pkg/collector/names.go:67-76. The s9 synthetic input and its
  zero/one phase changes follow src/docs/catalog/batch-job-v1.yaml:27-50.
  They are not a captured lifecycle. The Undefined=1 and attribution-error
  examples also stay source-only; captured zero error gauges do not execute
  those failure paths.
- VERIFIED configuration: Prometheus 5s at manifests/02-prometheus.yaml:17;
  GCE 15s at manifests/04b-kyverno-gce-projected.yaml:14. The optional two
  GCE logs are 15s apart, not a guarantee about all polls. The default 1h
  was re-read at Kyverno v1.19.1 cmd/background-controller/main.go:191-199.
  Lab 60s remains OPERATOR-REPORTED setup from INTEGRATION-LOG.md:6-9.
- VERIFIED source/release: Kubernetes fix v1.34.2 at the local Kubernetes
  CHANGELOG/CHANGELOG-1.34.md:954,1037-1039. #134521 is a prior-art identifier,
  not a measured quantity; the earlier source mapping is in codex-notes-r3.
- The runner's logged actionsPerEpoch=1 and seven printed handle names are
  configuration/parser output. They do not establish a configurable action
  counter or seven tested action kinds. Only Job actions were captured.
- Animation offsets, 12000ms replay duration, the [0,310] display scale,
  count interpolation, CSS dimensions and slide totals are presentation
  settings. They are not data points from the lab. No tier turns them into
  measured timings.

### unsafe or incomplete convenience excerpts

| Source | Builder rule |
| --- | --- |
| data-excerpts.md:7-12 | Only one started watcher and five skipped rows. Totals come from the full capture. |
| data-excerpts.md:15-18 | Has join plus Completed/Degraded/Failed zeros. Fetch the actual Running/Suspended and replica/pod-count rows for s10. |
| data-excerpts.md:26-33 | Only the first two UR histories. s16 needs capture 31's four histories. |
| data-excerpts.md:47-54 / capture 30:3,6 | Verbatim copying would preserve the bad 3.2s, full-window implication and imprecise 0.0s narration. Use parsed fields or a visible separate correction. |
| data-excerpts.md:92-97 | Not a complete receipt body. The full JSON is capture 24:10-32. |
| data-excerpts.md:103-106 | Stops before the after Event count at capture 17:23. Do not infer the count from this slice. |
| captures/31-cel-error-repro.txt:7-7 | Omit the invented policy-event/tick attribution. Do not derive trigger count from four URs or background_scan labels. |
| captures/23-rr-rbac.txt:1-1 | Omit mid-flight. Revocation preceded the next evaluation. The final status is logical line 15 despite wc -l reporting 14. |
| captures/16b-trap-take3.txt:3-4; captures/16c-trap-gce.txt:1-1 | Settle/cleared-startTime commentary is invalid. The actual retained startTime and later inactive fields are separate observations. |
| captures/08-policy-apply-time.txt:1-1 | Malformed .3N timestamp. Never use it for arithmetic or replace it with invented precision. |
| captures/07b-gce-poll-v6.txt:1-2 | ANSI styling needs formatting-only removal with that caption. Preserve all printable text, including trainer-c. |
| captures/17-kyverno-forensics.txt:26-27 | Recorder's no-surviving-surface gloss is not a universal observation. Use the bounded inspected rows. |

### checks

- Checked 167 distinct capture/source ranges for existence and inclusive
  logical-line bounds. Files without a trailing newline are handled.
- Recomputed the timing differences and watcher/UR/receipt counts. Parsed
  the complete receipt JSON and checked its key values. Verified the
  displayed query against the decoded manifest URL exactly.
- Preserved 40 IDs/order, one visual, up to four beats, scope/inset budgets,
  all fx assignments and all animation/lane JSON. ASCII and SPDX pass.
- No new cluster execution, source modification or engine change. Captures
  and data-excerpts.md remain unchanged. Prior loop entries are preserved.
  Only SLIDES.md and this append changed.

## round 12 - demo (codex)

### changes

- s27 now contains the exact script.lane-data JSON for two replayed lanes.
  Kyverno/trainer-a: +0.095s active=1/startTime reset, then +123.947s
  re-suspended. RR/trainer-f: +0.692s ResumeDetected, +124.526s
  EscalatedNeedsHuman, then +300.147s running at the final check.
- EXECUTED offsets are computed from captures/30-lab2-kyverno-phase.txt:5-8
  and captures/22-rr-anti-trap.txt:2-3,6-7,9-10. The scripted resume marker
  is zero for each run. Capture 30's printed 0.0s and 123.9s narration is
  not used for exact arithmetic. The five chips are parsed summaries.
- Both tracks use scale=[0,310] and duration=12000. Those are display
  settings, not lab measurements. Headers and the shared zero label are
  authored HTML; the component does not render lane.name. No t=0 chip is
  added on top of an early observation.
- Replaced the old held-Kyverno/animated-RR instruction in s27 and the
  narrative contract. Both lanes now replay with stock customFx.dualLane.
  Removed the Kyverno forced-opacity override. Added endpoint anchoring
  for its early observation as well as RR's.
- Specified entry, play, completion, reset, re-entry and static behavior in
  seven steps. Entry is held, without autoplay. A click replays both lanes;
  a click during playback cancels and resets, rather than pausing. The
  next click starts over. Timestamp marks remain visible throughout.
- Added the visible caption: "Two sequential runs on different Jobs,
  replayed on one relative clock with each resume at zero."
  Retained the wider RR query, shorter polling and no-speed-comparison
  scope. Final running fields are a sample, not continuous health.
- Defined the held five-observation view and its compact two-row static
  replacement. No extra observation at 310s, continuous state bar or
  additional Kyverno re-suspension is implied. The shared lifecycle adapter
  handles static mode changes; no new customFx is needed.

### engine limits for the builder

- VERIFIED: demo-component.js:10-33 pairs JSON lanes with DOM lanes by order
  and creates event/timestamp pairs. Keep .track initially empty.
- VERIFIED: demo-component.js:35-58 resets both lanes and schedules reveals
  at max(60, t/310*12000) ms. Both early events reveal at presentation
  60ms despite distinct captured offsets. Kyverno's repeat and RR's
  escalation reveal at about 4798ms and 4820ms; the final check at about
  11619ms. The replay button returns at 12400ms. These are computed
  presentation timings, not additional measurements.
- VERIFIED: demo-component.js:60-76 enters held and wires one button. A
  playing click resets; there is no pause, playhead, scrubber or live feed.
  Stock print/media-change lifecycle handling is not supplied.
- VERIFIED: skeleton-head.html:143-147 centers chips with a pop animation.
  The scoped CSS disables that transform animation and anchors endpoints.
  Rendered label fit still needs the builder's projector check. The static
  table is the cheaper fallback if labels overlap. No visual render was
  performed in this spec review.

### checks and verdict

- Recomputed all five offsets directly from the captured timestamp pairs.
  Parsed the embedded JSON and checked lane order, event counts, scale
  and duration.
- Ran the unmodified demo-component.js in a DOM/timer harness. Verified
  held entry, all positions/time marks, early timer clamping, both-lane
  playback, completion, mid-play reset, timer cancellation and re-entry
  without duplicated chips or button listeners.
- Preserved 40 slide IDs and their order, four s27 beats, scope/inset
  budgets and ASCII. Earlier loop entries remain intact. Only SLIDES.md
  and this append changed; captures and engine files remain unchanged.
- Verdict: ready for the builder. Exact JSON, replay behavior, static
  results and the separate-run comparison scope are specified. No data
  blocker remains for this slide.

## round 13 - publishability (codex)

### changes

- Replaced the single blocked substring in s5's navigation metaphor with
  its concrete idea: workload events and pod events feed the same store.
  The case-insensitive substring scan now has zero hits across SLIDES.md,
  including code and metadata, not just projected prose.
- Removed two local authoring shortcuts from the header. The spec now
  names its actual engine files and states its voice in plain words.
- Added a publication contract at SLIDES.md:35.
  Runtime Rules is the kep-0003 proposal; rr-poc is this lab's prototype.
  Evidence links are limited to the fork, upstream Kyverno/Kubernetes and
  lab files. F paths resolve from the fork repository root.
- s2 now names the proposal on first introduction and labels the topology
  node RR lab prototype. VERIFIED source: fork
  keps/0003-runtime-rules/README.md:6-11,18-25 identifies the provisional
  proposal. Its prototype does not establish completion of that proposal.
- The act 4 label explicitly names the lab prototype. s32 labels its box
  proposed action protocol and keeps INFERRED plus not a released
  controller beside the recommendation. Existing execution limits remain.
- Replaced the stale publication-review reminder with a build-stage check
  of rendered text, embedded excerpts and resolved public links.

### flags and scope

- No private URLs, machine paths, Hebrew, customer names or personal names
  were found in the authored deck prose. Source paths and fixture object
  names remain exact. No new external URL was introduced.
- Literal-file exception: the existing required copyright notice at
  SLIDES.md:2 names the legal owner. It remains in an HTML comment, outside
  projected content. It was not removed to force a literal zero-company
  scan of legal metadata. No company names were found in authored slide
  prose.
- This checks the spec, not a built Pages site. Source files and captures
  named by embed directives are not copied or altered in this round. Check
  the resulting rendered excerpts and public link targets during the build.

### checks and verdict

- Zero matches for every requested restricted term, using substring rather
  than word-boundary matching. All spec text is ASCII. Every heading is
  lowercase; no emphasis markup occurs in prose. The code's multiplication
  exponent is unchanged.
- All 40 slide IDs/order, code blocks, JSON and replay data remain unchanged.
  Four-beat limits and all beat/scope/inset word budgets pass. Earlier loop
  entries remain intact. Only SLIDES.md and this append changed.
- Verdict: slide prose passes this scrub. The required legal notice is the
  sole named-owner exception in file metadata. Runtime Rules is presented
  as a proposal plus a lab prototype, not a shipped product.

## round 14 - terminology (codex)

### changes

- Added one builder glossary for the whole deck. It covers karta
  description, statusMappings/statusDefinition, suspend handle and
  suspendDefinition, allowance epoch, receipt, GlobalContextEntry,
  globalContext.Get, UpdateRequest/UR, mutate-existing and the separate
  action/report/fact-source clocks. It is not a new projected slide.
- Canonical kind names now appear in authored Job/JobSet references and
  owner-chain diagrams. Expanded LeaderWorkerSet where a second acronym
  was unnecessary. Preserved literal command names, component=job, source
  paths and metric keys. Karta stays the exact resource kind.
- Replaced bare Get with globalContext.Get in s13's inset, beats, scope and
  reference notes. GlobalContextEntry/GCE remains the resource, not a
  function name. The existing expression JSON already had the right case.
- Standardized s18a on background scan tick. forceReconciliation is the
  function implementing that trigger; the tick queues eligible policies,
  and policy reconciliation submits UR work. Reporting scan, GCE refresh
  and Prometheus scrape retain separate names. Preserved field spelling
  evaluation.mutateExisting.enabled and requestType=cel-mutate.
- Standardized s24 and its state-diagram instructions on allowance epoch.
  Kept epoch as the serialized field name. The glossary and source notes
  distinguish the KEP's renewed condition window from this prototype's
  tested reArmOnResume=false behavior. Shared terms do not imply KEP
  completion or an implemented fresh-window timer.
- Bound suspend handle to suspendDefinition in s25. Preserved
  suspendActions/resumeActions and the prototype's narrower translator
  scope. Normalized description-hash wording in the limits.
- Added exact meanings for all 15 phase strings passed to the receipt
  writer. EXECUTED remains an evidence tier; Executed is a receipt phase.
  UR states and normalized workload phases have separate glossary entries.
  DELETED remains a watch event. Complete is the native Job condition;
  Completed is the normalized phase or UR state, depending on its label.
- s15's final diagram node explicitly reads UR state: Completed; its DOM
  key remains completed. s27's phase labels and five captured offsets are
  unchanged. s29 now names suspendFieldManagers rather than a made-up
  managers field. s30 no longer uses Executed Jobs as ambiguous prose.
- s30b now names ExecutedVerifiedAfterRestart in its source notes. The
  phase means recovery found the listed target suspended; it does not
  establish who patched it or controller convergence. It remains untested.

### evidence

- VERIFIED proposal vocabulary: fork
  keps/0003-runtime-rules/README.md:94-106,200-210;
  kep-0004.md:75-106,118-143. The former describes a fresh condition window
  after resume. The latter supplies metric labels and phase spellings.
- VERIFIED API vocabulary: src/pkg/api/runai/v1alpha1/types.go:19-47;
  src/pkg/api/runai/v1alpha1/structure.go:41-77,268-280;
  src/docs/catalog/batch-job-v1.yaml:26-57.
- VERIFIED prototype: rr-poc/rr_poc.py:138-161 defines receipt fields;
  :187-194 initializes epoch; :372-403 handles resume/escalation;
  :377-380 spells suspendFieldManagers; :288-319 names recovery outcomes;
  :405-461 names skip, intent and patch outcomes.
- VERIFIED Kyverno v1.19.1: pkg/cel/libs/context.go:115-124 reads the named
  entry/projection; api/kyverno/v2/updaterequest_types.go:26-33,53-75,167-182
  defines UR fields, request types and states;
  pkg/policy/policy_controller.go:645-655,699-705 implements the scan tick
  and policy eligibility. The CEL spelling and mutateExisting field also
  appear in manifests/05c-kyverno-metric-suspend-gce.yaml:15-27.

### checks and verdict

- Extracted all receipt phase string arguments from the Python AST and
  compared them with the glossary. All 15 match exactly. No branch was run
  or promoted from source evidence to executed evidence.
- Reviewed storyboard labels and parsed every animation JSON block. Code
  blocks and JSON are byte-for-byte unchanged from round 13; no event,
  timing, CSS class or DOM key changed.
- Preserved 40 IDs/order, four-beat limits and scope/inset word budgets.
  ASCII, lowercase headings and the round 13 restricted-substring check
  pass. Prior loop entries remain intact. Only SLIDES.md and this append
  changed; both source checkouts and captures remain unchanged.
- Verdict: terminology is consistent. The proposal/prototype behavior
  difference is explicit rather than hidden by common names.

## round 15 - final gate (codex)

Read SLIDES.md end to end as the complete build spec. The file order is
intentional. No slide was added, removed or reordered.

### requirement coverage

| Requirement | Where the builder will show it |
| --- | --- |
| Exporter internals with animations | s3-s11b: descriptions, registry choice, watchers, Pod cache, owner walk, part selectors, status evaluation, stored records, scrape output, missing labels and activity limits. Concrete fx/JSON and held fallbacks are supplied. |
| Kyverno integration | s12, s14, s13, s18 and s18a: target predicates, failed GCE attempt, projection correction, working HTTP and GCE paths, and separate clocks. |
| Finding 1: earlier policy affected the metrics path | s19: saved NetworkPolicy/query evidence, with the timeout account and causal limits labeled. |
| Finding 2: acting-path error loses per-target diagnostics | s13-s17b: bad projection, source return chain, minimal executed repro, metrics and the two distinct reporting-policy examples. |
| Finding 3: platform defect during resume | s21-s22: earlier rejected status changes, fixed-version observation and the distinction between patch acceptance and convergence. It is not presented as a Kyverno-specific defect. |
| Repeat action and the protocol comparison | s1, s20 and s27: one observed Kyverno repeat paired with the separate RR resume/escalation run. Repetition beyond the capture remains INFERRED. The replay preserves all five exact offsets and the shared relative scale. |
| What RR gives and how this lab uses it | s24-s30b: allowance epoch, no re-arm, Intended/pendingOp gates, suspend handle, Observe/Enforce, escalation, permission/handle skips, surviving receipts, startup files and identity split. Unimplemented or untested behavior stays explicit. |
| Recommendation and supporting evidence | s32 is the single ending. s31 is the scoped comparison reference; s33 provides evidence and rerun/filing limits outside main playback. |

### small residue fixed

- Corrected the exporter animation contract's stale reference to an adapter
  below it. The adapter is in the earlier engine feasibility contract.
- s9 now starts with Initializing=1 for its initial active=1/ready=0
  synthetic input. Removed the brief all-zero frame and its redundant
  later correction. VERIFIED matcher:
  src/docs/catalog/batch-job-v1.yaml:27-34. Running/Degraded transitions
  and the held result are unchanged.
- s11b now says initial sync complete in its first frame. That agrees with
  the fixed readiness=true value already specified there. No new readiness
  measurement is claimed.
- s16 source notes now say saved watch, removing the leftover assertion
  that its duration was measured as 150s. That agrees with its number audit;
  captures/31-cel-error-repro.txt:7 contains recorder narration, not an
  elapsed-time measurement.
- s22's handoff now names startTime reset and active=1 directly instead of
  using a broad healthy-resume label.
- s29 distinguishes direct deletion of a receipt from workload deletion.
  Reused run IDs can overwrite records. This no longer suggests that
  deleting the workload overwrites its receipts. The ConfigMap construction
  has no workload ownerReference: rr-poc/rr_poc.py:124-131; captured survival:
  captures/24-rr-forensics.txt:2-8.
- Renamed the final open-items section to build handoff. Its render,
  lifecycle-adapter and public-link checks are builder work, not unfinished
  slide-spec review.

### final checks

- File order exactly matches the narrative contract: 38 main slides, then
  the two separate reference views. Historical IDs remain unchanged. s32
  is the last main slide. All slide references resolve to authored IDs.
- Every slide has one usable fx, fx DOM, embed and fallback field, plus
  its idea, beats, scope, visual, tiers, data and number audit. Static
  references explicitly use an empty fx mode. No effect handler is missing
  from the engine or the supplied builder sketches.
- Parsed all 15 animation JSON blocks and six builder JavaScript blocks.
  Timeline offsets are ordered. Rechecked every complete s9 synthetic
  state against its cited matchers. s27 replay JSON is unchanged; all five
  relative offsets were recomputed from captures 22 and 30.
- Checked 78 distinct pinned embed ranges for file existence and inclusive
  line bounds. None contains a restricted-text hit. The spec also has zero
  restricted substring matches, no private URLs or machine paths, ASCII
  text, lowercase headings and no prose emphasis markup.
- All beat, scope and inset budgets pass. The glossary's 15 receipt phases
  still match the runner's exact strings. Required legal metadata remains
  outside projected content, as recorded in round 13.
- The proposal, prototype and captured runs remain distinct. Observe is
  not read-only; Executed does not mean controller convergence; a new
  allowance epoch does not re-arm this configured prototype. The GCE
  correction, separate report paths and bounded deletion comparison agree
  across slides.
- Only SLIDES.md and this appended report changed. Captures, engine files
  and source checkouts are untouched. Pixel fit and final Pages links must
  be checked when the HTML is built; no rendered-site approval is implied.

No spec blocker remains. The builder can proceed from this file.

SPEC GATE: SIGN-OFF
