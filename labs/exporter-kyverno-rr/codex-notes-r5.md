<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Sign-off verification, round 5

Reviewed the current log and both drafts on 2026-09-14. References below
use this snapshot's line numbers. No new cluster execution or scope added.
The revised comparison, query caveat, prototype limits, and main table are
acceptable. Three remaining editing fixes carry forward R4 findings.

## Remaining MUST-FIX

1. R5-M01 - Draft 1 still contains the old R4-M05 claims.
   `upstream-drafts.md:211-220` still says "within 0-1s", "The only
   executor-side trace", and "exactly one per UR". Its surrounding
   qualifications at `:78-82,241-244` do not make those claims measured.
   EXECUTED `captures/31-cel-error-repro.txt:9-24` shows rounded AGE values,
   not timestamped transition durations. Lines 4-5 and 40 show an absent
   series followed by count 4, without per-UR correlation. Lines 37-38 show
   no matching policy-name log output. The wrapper records two metric
   families (VERIFIED: Kyverno `pkg/cel/policies/mpol/engine/metrics.go:29-30`).
   Replace those bullets with:

   > Four URs displayed Pending, Completed, and deletion during the
   > 150-second watch, with AGE 0s or 1s. Their individual trigger sources
   > were not captured. The selected background-controller log query returned
   > no policy-name matches. The error-result histogram series was absent
   > before and had count 4 afterward. This matches the UR total as aggregate
   > corroboration, not a traced result for each UR.

   Apply the same trigger qualification to `INTEGRATION-LOG.md:361-362`
   ("one on the policy event, then scan ticks"). The existing empty-message
   qualification in the log is correct. No extra test is needed for this edit.

2. R5-M02 - The old manual-patch certainty remains above its correction.
   `INTEGRATION-LOG.md:274-276` still states that the manual patch failed
   "because it changed the stored timestamp to a different value".
   Lines 285-287 correctly label that explanation inferred and disclose the
   missing payload. Delete the earlier assertion or use the later wording.
   Also replace "forever" in the EXECUTED setup at `:253-254` with "repeatedly
   during the captured period". Lines 283-285 already express the conditional
   indefinite-retry inference correctly. These are missed R4-M07 edits;
   `captures/12-job-wedge-kcm.txt:1-17` remains finite evidence.

3. R5-M03 - Two prose statements still exceed the corrected diagnostic scope.
   - `INTEGRATION-LOG.md:158-162`: "runs the same evaluation". The captured
     error Event names dbg-gctx (`captures/14-gctx-error-event.txt:7`). Its
     bad projection is in a mutation expression
     (`manifests/90-debug-gctx-annotation.yaml:23-25`), so the reporting path
     also encounters it. Replace with: "A separate reporting scan of
     dbg-gctx encounters the same bad projection in its mutation expression
     and emits the captured error Event." Keep the already-correct
     target=false explanation at log `:193-201`. This finishes R4-M04.
   - `INTEGRATION-LOG.md:502-506`: "neither engine's inspected surfaces
     reflected a broken action outcome" implies a broken-outcome experiment
     on both. The RR comparison ran on fixed lab2; its captures demonstrate
     successful patch receipts, not an encounter with the wedge. Use:
     "The inspected Kyverno surfaces did not diagnose controller divergence.
     Neither side demonstrated automatic wedge detection; RR receipts attest
     patch success, not controller convergence." This matches the signed-off
     table at `:483` and closes the remaining R4-M08 scope overstatement.

## Verdicts

The log needs the short prose corrections above. Draft 1 needs R5-M01 before
its existing false/true/mutation-error controls and filing. Those controls
remain pending at `upstream-drafts.md:230-231`; they are not a prerequisite
for accepting the executed integration log. No production features or new
comparison scenarios are required.

The table's 123.947s re-suspension versus 124.526s escalation pairing is fair.
The 0.692s detection and 300.147s final check are correctly distinguished.
Future repetition is explicitly INFERRED for the configured rule. Unexecuted
RR branches are disclosed. The empty UR message in row 477 inherits the
explicit source-only qualification at log `:363-364`.

Log as goal deliverable: NOT YET

Draft 1 fileable after controls: NOT YET

Diff table fairness: SIGN-OFF
