<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Final integration review, round 4

Reviewed 2026-09-14. Log read end to end. No new cluster execution.
Line numbers refer to this review snapshot. K = Kyverno checkout
`~/workspace/kyverno`, cited v1.19.1 error/report paths.
EXECUTED = capture; VERIFIED = source; INFERRED = interpretation.

The core result survives review. Capture 30 records one Kyverno re-suspend.
Capture 22 records RR escalation and an unsuspended, active Job at the final
check. The current log and table still overstate several surrounding claims.

## MUST-FIX

1. M01 - Cluster provenance and altered quotations.
   `INTEGRATION-LOG.md:6-8`: "Everything executed on kind-kyverno-lab
   (Kubernetes v1.34.0)" excludes the lab2 results. Rewrite to identify both
   clusters and place the clean comparison on v1.34.3.
   The block at `:359-366` edits capture 30 without marking an excerpt.
   Capture `30-lab2-kyverno-phase.txt:1,3,6` has different literal text.
   Mark it "edited excerpt" or copy exactly. The displayed apply marker to
   first suspension is 3.397s (`:1,3`), not 3.2s. At `:442`, use "3.397s
   from the printed pre-apply marker"; the capture's 3.2s inner timer has
   no separately recorded start. EXECUTED timings are observation bounds.

2. M02 - A range selector is not a minimum runtime allowance.
   `INTEGRATION-LOG.md:122-126`: "reported Running for the last 2 minutes";
   `:368-369`: "exactly the mechanism ... next 60s tick".
   VERIFIED: min_over_time tests available samples; it does not establish
   two minutes of coverage or scrape health. Local Prometheus v0.305.4,
   `promql/functions.go:850-874,885-888`, corroborates the prior caveat in
   `codex-notes-r1.md:254-255`. EXECUTED counterexample: capture 30 records
   initial startTime 18:19:43 and suspension at 18:20:50.608 (`:3-4`), only
   67.608s apart. Rewrite: "All available Running samples in the trailing
   two-minute range equal 1; history coverage is not checked. The observed
   resume delay is consistent with eligibility recovery and reevaluation."
   No scrape/UR timestamps identify the exact triggering tick.

3. M03 - Keep the resume comparison, separate recurrence inference.
   `INTEGRATION-LOG.md:444`: "unbounded while eligibility recurs (capture 30)";
   `:451-453`: "four seconds apart in rule text".
   EXECUTED: Kyverno re-suspend = 123.947s after its resume marker
   (`captures/30-lab2-kyverno-phase.txt:5-7`). RR ResumeDetected = 0.692s;
   escalation = 124.526s; final running check = 300.147s after its marker
   (`captures/22-rr-anti-trap.txt:2,6-10`). Compare action versus escalation
   at renewed eligibility, not 123.9s versus 0.7s as competing latencies.
   Mark unlimited future repetition INFERRED from this configured rule's
   lack of a cap; only one re-suspend was observed. Delete the four-seconds
   phrase. These are sequential runs on lab2, different Jobs, with different
   polling intervals. RR uses the Job-equivalent predicate but a wider kind
   query (`rr-poc/rules.yaml:15`; `manifests/05b-kyverno-metric-suspend-http.yaml:30`).
   State that scope difference instead of implying identical rule text.

4. M04 - The report scan does not relabel the target CEL error.
   `INTEGRATION-LOG.md:348-350` and `upstream-drafts.md:220-222`:
   "an evaluation error misdescribed as non-compliance".
   VERIFIED: K `pkg/controllers/report/utils/scanner.go:243-264` calls
   Handle, which evaluates with target=false
   (`pkg/cel/policies/mpol/engine/engine.go:158`).
   Compiler `pkg/cel/policies/mpol/compiler/policy.go:201-219` therefore
   uses matchConditions, not the failing targetMatchConditions. The scanner
   turns a successful, object-changing simulation into "mutation is not
   applied". It does not convert RuleError to RuleFail here.
   Rewrite: "The independent reporting scan reports unapplied mutation;
   it does not evaluate this target-only error condition." The acted-path
   error discard remains VERIFIED and supported by capture 31.

5. M05 - Bound the CEL reproduction claims to captured fields.
   `INTEGRATION-LOG.md:335-339,345-350,443` and
   `upstream-drafts.md:208-219` overstate "exactly one place", "0 -> 4",
   "exactly one per UR", "no log at v=6", and attributable scan triggers.
   EXECUTED `captures/31-cel-error-repro.txt:9-24`: four URs display
   Pending/Completed/deletion with AGE 0s or 1s. The watch has no message
   column or trigger timestamps. `:4-5,40`: metric series absent before,
   histogram count 4 after. `:37-38`: no matching policy-name log lines;
   this transcript does not attest verbosity or all log output.
   UR empty message is VERIFIED by K `pkg/background/common/status.go:37-39`,
   not shown by this watch. Policy-level empty message IS captured (`:28`).
   Four counts matching four URs is aggregate corroboration, not traced
   per-UR attribution. K `pkg/cel/policies/mpol/engine/metrics.go:29-30`
   records both duration and result metrics. Rewrite with those exact limits.
   No additional experiment is needed to state the narrower result.

6. M06 - Remove superseded GlobalContext conclusions.
   `INTEGRATION-LOG.md:159-190` still says "no event", "all read healthy",
   and presents missing-store nil as the executed explanation. Capture
   `08-kyverno-suspend-timeline.txt:1-2` contains only an initial sample;
   `14-gctx-error-event.txt:7` contains the reporting Event. The saved defect
   was a missing projection, not a missing entry. Retain the eight-minute
   duration as operator-reported unless its full transcript is supplied.
   At `:233-237`, "with projections declared up front" is too restrictive:
   a named projection must be declared; the empty projection returns the
   whole response (K `pkg/globalcontext/externalapi/entry.go:154-164`).
   Capture `19-gce-corrected.txt:1-4` proves the named-projection variant works.

7. M07 - Delete the obsolete Job-race conclusion and universal claims.
   `INTEGRATION-LOG.md:299-310` retains "bookkeeping re-arms", the invalid
   wait-for-clear gate, and "permanently on Kubernetes v1.34". These directly
   contradict `:255-275,320-331` and the fixed v1.34.3 run in capture 30.
   `:273,322-328` also overgeneralizes to every started Job and every engine.
   Rewrite around the recorded failure with retained startTime and a stored
   Suspended=True condition, under the affected validation defaults.
   `codex-notes-r3.md:60-158` contains the source chain and limitations.
   Keep "forever" as an INFERRED retry consequence, not a finite capture.
   At `:266-268`, the manual SET diagnosis remains INFERRED: the raw payload
   is absent. Count distinct Jobs separately from repeat attempts; the
   three/four-job summaries are not established by captures 15/16/16b/16c.
   Those name trainer-b, trainer-c, and trainer-gce, with repeats.

8. M08 - Table cells mix execution, code, and unsupported capability claims.
   `INTEGRATION-LOG.md:438`: "Every cell cites a capture" is false.
   Apply the per-row evidence labels below. In particular:
   - `:443`, RR FactsUnavailable: VERIFIED code for handled query errors,
     not EXECUTED. `rr-poc/rr_poc.py:210-217,356-359` also has an uncaught
     JSON parse failure; do not promise receipts for every source failure.
   - `:448`, "CLI offline apply only": false. Kyverno performs runtime
     report scans (capture 31; K `pkg/controllers/report/utils/scanner.go:225-264`).
     Say "No tested observe gate on the mutate-existing executor; separate
     report evaluation exists with different condition semantics."
   - `:449`, "restart resolve coded and exercised at startup": calling the
     function with no pending operation does not test recovery. No capture
     shows pending-op recovery, timeout, or failed-precondition execution.
   - `:283-286,449`, outcome verification: RR Executed means patch command
     success (`rr-poc/rr_poc.py:444-449`). It does not check Job-controller
     convergence. Restart resolution checks spec.suspend (`:288-319`), not
     healthy status. Neither side demonstrated automatic wedge detection.
     "Raw-spec-patch engines cannot even see it" is an unsupported impossibility.
   - `:408-410`: UID and suspend=false tests protect those fields only.
     They do not revalidate metric eligibility or resourceVersion
     (`rr-poc/rr_poc.py:275-285`). Narrow "already-changed object" accordingly.

9. M09 - Scope deletion forensics to inspected surfaces.
   `INTEGRATION-LOG.md:381-382`: "No surviving surface records what was
   done, when, or by which evaluation". EXECUTED capture 17 shows zero
   target reports, no URs in kyverno, and seven remaining Events (`:19-25`).
   It does not inspect retained logs, metrics, API audit, or external export.
   The before-delete Events include Suspended/Resumed timestamps (`:5-11`),
   although they do not attribute those actions to the policy evaluation.
   Rewrite: "No retained per-action receipt linking the action to its policy
   evaluation was found in the inspected reports, URs, or Events."
   The policy status appears only before deletion (`:14-15`); "unchanged"
   after deletion needs another sample or a source/inference label.

10. M10 - The advertised rerun does not create fresh UIDs reliably.
    `INTEGRATION-LOG.md:489-491`: "fresh jobs get new uids, receipts accumulate".
    VERIFIED `rr-poc/run-rr-phase.sh:16-29,63-74` applies fixed names without
    deleting existing trainer-f/g. `:34,40,54,78,81` reuses fixed run IDs;
    `rr_poc.py:139-141,131` can apply over identically named receipts for the
    same UID. The script also retains the invalid settle gate (`:44-50`).
    Remove the clean-rerun promise or require fresh target names/UIDs and
    unique run IDs, with a bounded, valid status gate. Preserve old evidence.
    This is a documentation blocker; no rerun was attempted in this review.

11. M11 - Exporter counts and completeness claims exceed the evidence.
    `INTEGRATION-LOG.md:91-95,471-475`: "seeded all 20 kinds", "15 ... skipped",
    "exactly as its KEP promises", "sub-scrape freshness".
    EXECUTED `captures/03-exporter-watchers.txt:1-25`: five started watchers
    and nineteen failed GVK mappings. The catalog has twenty definitions,
    including two API versions of one kind (`src/pkg/catalog/catalog.go:36-56`).
    VERIFIED: bare Pod is excluded and catalog entries are selected per
    group/kind (`src/exporter/pkg/registry/registry.go:63-76,188-200`).
    Rewrite using the five/nineteen observed counts. Remove the comprehensive
    KEP-conformance, first-ever-deployment, and freshness claims unless backed
    by tests/history. The opening image-history paragraph already describes
    earlier local exercise (`:27-28`).

## SHOULD

1. S01 - Remove exclusive-provider language: "what only Karta knows"
   (`INTEGRATION-LOG.md:40`), "only ... makes possible" (`:126`). Say the
   lab obtains workload normalization from Karta; authored rules or another
   provider can supply it. The integrations remain demonstrated.
2. S02 - `INTEGRATION-LOG.md:460` lists "three different ways" but names two.
   Use "two paths: direct HTTP and GCE with a named projection".
   `:466`: the prototype file is 471 lines; "about 470 lines" is reproducible.
3. S03 - Replace first-person blame, all-caps, "overturn the human", and
   "executable exhibit" with the concrete behavior. In `:462-466`, the Job
   defect is a platform validation bug, not proof that every problem belongs
   to the missing runtime execution protocol.
4. S04 - Draft 1 `:84-86` still labels the reproduction unexecuted. Rename
   it "Original proposed fixture; execution and differences below". Move
   the lab2 reproduction ahead of the older GCE case. Draft 2 `:323-325`
   should now point to capture 30 or explicitly remain an R3 historical note.
5. S05 - Limit "RBAC revoked mid-flight" to "permission removed before the
   next evaluation" (`captures/23-rr-rbac.txt:1-4`). No action was in flight
   when the RoleBinding was removed. Link the earlier Kyverno RBAC result
   directly instead of "prior lab": `../03-experiments-executed.md:110-117`.

## Table evidence and fairness disposition

These are the labels the existing table needs. They avoid new experiments
except the explicitly outstanding Draft 1 controls.

| Row | Kyverno evidence | RR evidence |
|---|---|---|
| Metric-conditioned action | EXECUTED 10/19/30; correct timer and window language | EXECUTED 21: Intended/Executed plus suspended spec |
| Condition/source failure | EXECUTED 31; diagnostic mechanism VERIFIED; limits M04/M05 | VERIFIED handled-error branch; fault injection unexecuted |
| User resume | EXECUTED one re-suspend in 123.947s; future uncapped repetition INFERRED for this policy | EXECUTED escalation in 124.526s and active at 300.147s; detection in 0.692s |
| Deletion provenance | EXECUTED 17, bounded to inspected surfaces/defaults | EXECUTED 24: four action/observe receipts survive deletion |
| Missing permission | Prior EXECUTED Experiment D; not this same fault-injection run | EXECUTED 23: preflight refusal, then action after restore |
| Missing handle | INFERRED authoring burden; no Kyverno control executed | EXECUTED 20: SkippedNoHandle; dedupe also VERIFIED in code |
| Observe | VERIFIED independent report scan; executor observe equivalence not tested | EXECUTED 20 + 21: WouldAct, unchanged targets, later Enforce succeeds |
| Outcome verification | No automatic controller-convergence check demonstrated | EXECUTED patch receipts; pending recovery/Unknown/precondition branches VERIFIED only |

## Verdicts

Log as the goal deliverable: NOT YET. M01-M11 require correction; the core
executed integration and contract comparison are supported.

Draft 1 as fileable-after-controls: NOT YET. Apply M04/M05 and clarify the
executed fixture. Then complete the declared false/true/mutation-error
controls. The central diagnostic defect has adequate evidence to investigate.

Diff table fairness: NOT YET. The 123.9s-versus-escalation pairing itself
earns SIGN-OFF with M02/M03's scope. The complete table needs the evidence
labels above and the M08/M09 capability/provenance corrections.
