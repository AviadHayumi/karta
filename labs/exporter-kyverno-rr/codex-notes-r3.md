<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Exporter integration: Codex review, round 3

Date: 2026-09-14. No cluster mutation or new cluster test in this review.
Both issue texts are in [upstream-drafts.md](upstream-drafts.md).

EXECUTED = supplied capture. Operator-only observations are named separately.
VERIFIED = source read or upstream record checked.
INFERRED = causal attribution or proposed test, not a new execution.

Source keys:

- L: this exporter-lab directory.
- K: `~/workspace/kyverno`, v1.19.1,
  `40ec788d48bb28d83dbf85538e962a59db9d45c6`.
  The cited error-handling files have no diff against supplied HEAD
  `3bb926936656bfb14ae1a6306a6d9208a4b77613`.
- U: `~/workspace/kubernetes`,
  `121abd026c869bdb9b3896ac64190efcdb8d70cb`, dated 2026-06-29.
  This is a shallow, newer checkout, not the live v1.34.0 source.
- U1: `~/go/pkg/mod/k8s.io/kubernetes@v1.34.1`.
  Local, still-broken release source used for precise file:line citations.
- U2: `~/go/pkg/mod/k8s.io/kubernetes@v1.34.2`.
  Local fixed release source. Neither module cache was modified.

The v1.34.0 [controller](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.34.0/pkg/controller/job/job_controller.go),
[strategy](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.34.0/pkg/registry/batch/job/strategy.go),
and [validator](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.34.0/pkg/apis/batch/validation/validation.go)
were also fetched. Their relevant branches match the broken mechanism below.
U1 line numbers are explicitly v1.34.1, not browser extraction line numbers.

## A. Known bug, already fixed

VERIFIED: This matches Kubernetes
[issue #134521](https://github.com/kubernetes/kubernetes/issues/134521),
opened 2025-10-10. Its reproduction is start, suspend, resume. It reports
the same validation message and a controller changing a non-null startTime
to another non-null time. A short suspension interval is not required.

VERIFIED: [PR #134769](https://github.com/kubernetes/kubernetes/pull/134769/files)
merged into master on 2025-11-04. Backport #135130 shipped in v1.34.2.
U `CHANGELOG/CHANGELOG-1.34.md:954,1037-1039` places the fix in v1.34.2.
The [release page](https://github.com/kubernetes/kubernetes/releases/tag/v1.34.2)
dates that release to November 12; the cached tag metadata identifies 2025
and commit `8cc511e399b929453cd98ae65b419c3cc227ec79`.
U `CHANGELOG/CHANGELOG-1.35.md:1646,1793-1795,1066-1067` also lists it in
v1.35.0-alpha.3 and v1.35.0. The backport PR page did not load through the
browser. Its inclusion is verified from release source and changelog.

INFERRED disposition: Do not file this as a novel suspension-bookkeeping
race. Preserve the requested Kubernetes draft as a historical duplicate.
Reproduce on a fixed patch release before proposing any new issue.

## B. The controller resets startTime; it does not remove it

VERIFIED sequence in the affected release:

1. The controller reads the Job informer and deep-copies the Job.
   U1 `pkg/controller/job/job_controller.go:828-856`.
2. Suspension sets the Suspended condition to True. That branch does not
   clear startTime. U1 `pkg/controller/job/job_controller.go:1036-1043`.
   The old startTime in the capture is therefore expected after suspension.
3. After `spec.suspend=false`, `manageJob` can create replacement Pods before
   the subsequent Job status write. U1
   `pkg/controller/job/job_controller.go:1008-1011,1795-1796,1816,1833`.
4. The controller changes Suspended=True to False, emits Resumed, and sets
   startTime to the current time. The reset restarts active-deadline timing.
   U1 `pkg/controller/job/job_controller.go:1047-1058`.
5. The common status flush calls UpdateStatus. An error gets the prefix
   `adding uncounted pods to status`, even when the failing field is unrelated
   to uncounted Pods. U1 `pkg/controller/job/job_controller.go:1068-1075,1381-1383,1883-1885`.

VERIFIED: Job status validation rejects any changed, previously non-null
startTime while `spec.suspend` is false. The test is value inequality, not
`new.startTime == nil`. The error text falsely describes this as removal.
U1 `pkg/apis/batch/validation/validation.go:698-704`.
The strategy enables that check on every startTime change, with no resume
exception. U1 `pkg/registry/batch/job/strategy.go:345-346,377,400`.
JobManagedBy enables these checks even for ordinary Jobs without a custom
managedBy value. Its default is true starting in 1.32.
U1 `pkg/features/kube_features.go:1355-1358`.

VERIFIED: The queue retries the Job key, not a frozen status request.
Each retry recomputes from the persisted Job, whose rejected condition
transition never became False. It therefore attempts another startTime
reset and hits the same validation. There is no fallback that keeps the old
timestamp after this error. U1
`pkg/controller/job/job_controller.go:639-655,828-856,1047-1058,1381-1383`.
API retry backoff starts at 1 second and caps at 1 minute.
U1 `pkg/controller/job/job_controller.go:65-68,188`.
That minute is independent of Kyverno's configured 60-second scan.

EXECUTED: L `captures/12-job-wedge-kcm.txt:1-15` records the same rejection
from 17:43:29 through 17:52:32. Lines 16-17 show later recurrences through
17:58:41, with an intervening gap. This proves persistence over the captured
period, not literally forever or an uninterrupted 15-minute run.
L `captures/13-trainer-b-wedged.txt:70,95-106` preserves unsuspended spec,
Suspended=True, ready=0, and the old startTime. `active` is absent in the
YAML; the description renders zero at line 12. The Resumed count is x15
in this capture, not x14 (`:45`; also `14-gctx-error-event.txt:9`).

INFERRED: This validation contradiction fully explains the persistent
symptoms. It needs no narrow timing race, stale-cache theory, or admission
mutation. The independently reported upstream reproduction agrees.
Even a fully settled suspension with an existing startTime can hit it.
Very fast resumes that never persist Suspended=True need not take this path.

### Why the manual SET-startTime patch can fail

VERIFIED: Replacing `T0` with `T1` fails the old guard whenever `T0 != T1`
and spec.suspend=false. Providing a non-null value does not avoid it.
Setting exactly `T0`, or initializing a genuinely absent startTime, does
not fail this particular guard. U1
`pkg/apis/batch/validation/validation.go:698-704`.

VERIFIED: The API server applies merge patch to the currently persisted
object. Its update strategy validates the proposed object against the
storage-side existing object. The Job controller's informer is not passed
to that validation. U
`staging/src/k8s.io/apiserver/pkg/endpoints/handlers/patch.go:323-331,417-427,578-590`;
`staging/src/k8s.io/apiserver/pkg/registry/generic/registry/store.go:709-721,825-826`.
The status strategy retains the stored spec, so status patch does not
independently choose a different suspend value.
U1 `pkg/registry/batch/job/strategy.go:330-341`.

INFERRED: The reported manual SET likely changed the existing timestamp and
hit the same misleading validation. No separate writer is needed to explain
it. However, the supplied captures contain no literal manual status-patch
command, payload, or rejection. Capture 13 contains describe/YAML only.
The observation is operator-reported in L `INTEGRATION-LOG.md:240-243`.
If the payload set exactly the stored timestamp, this explanation would
need revision. Save request payload, before-object, and response for that case.

### What fixed it, and what remains unexecuted

VERIFIED: U2 `pkg/registry/batch/job/strategy.go:383-387,405` exempts a
Suspended=True to False status transition from the timestamp-change guard.
U2 `pkg/registry/batch/job/strategy_test.go:2562-2588` covers that exemption.
U2 `test/integration/job/job_test.go:4094-4170` starts a Job, waits for a
settled suspension, resumes, and checks status convergence with JobManagedBy
enabled. These tests were read, not run in this review.
U `pkg/registry/batch/job/strategy.go:354-358,376` contains the same fix.
The fetched [current master strategy](https://raw.githubusercontent.com/kubernetes/kubernetes/master/pkg/registry/batch/job/strategy.go)
still contains the condition-transition exemption.

INFERRED: The reported temporary recovery after suspending again is not
enough to establish a reliable workaround. A repeated resume still reaches
the broken guard if the old timestamp and Suspended=True remain. No capture
records the complete recovery interleaving.

VERIFIED correction to the proposed settle gate: v1.34.0 does not clear
startTime on suspension in this controller branch. Waiting for it to clear
can wait indefinitely. U1 `pkg/controller/job/job_controller.go:1036-1043`
and the fetched v1.34.0 controller agree. U has a later, feature-gated clear
at `pkg/controller/job/job_controller.go:1168-1169`; that newer behavior must
not be projected backward onto the v1.34.0 lab.

INFERRED next control: repeat both executors' resume scenarios on a fixed
release. Bound waits. Record spec, Suspended condition, active/ready, Pod
state, and Prometheus samples. Do not count a broken Job status stream as
proof of either executor's allowance behavior. The exporter derives Running
from active/ready and Suspended from the Job condition.
L `src/pkg/catalog/kartas/batch_job.go:42-44,52,55-57` (VERIFIED).
A raw patch does not itself verify controller convergence, but neither
architecture is structurally unable to observe it.

## C. Kyverno draft: distinguish acting, metrics, and reporting

VERIFIED: The defect remains the processor's failure to inspect RuleError,
not GlobalContext support or polling. Target CEL errors become
EvaluationResult.Error, then RuleError plus no patch. The outer Evaluate
error remains nil. The processor only checks the outer error and enters
audit when PatchedResource is non-null. With no other failure, it marks the
UR Completed and clears the message.
K `pkg/cel/policies/mpol/compiler/policy.go:42-69,201-209,234-236`;
`pkg/cel/policies/mpol/engine/engine.go:95-118,215-226`;
`pkg/background/mpol/processor.go:230-276,407-415`;
`pkg/background/common/status.go:37-39`.
Audit normally supplies events and optional reports.
K `pkg/background/mpol/processor.go:308-344`.

EXECUTED: Background histogram counts with result=error are 7 for
metric-suspend-running and 3 for dbg-gctx.
L `captures/18-metrics-surface.txt:2,6`. These are aggregate evaluation
counts, not seven distinct Jobs or a trace linking each count to a UR.
The selected capture contains no target name or error message.

VERIFIED qualification: Both duration and result metrics are recorded by
the wrapper. K `pkg/cel/policies/mpol/engine/metrics.go:18-33`;
`pkg/metrics/mpol.go:37-50,54-91`. Thus write "the captured executor error
surface was an aggregate histogram", not "Kyverno has exactly one error
surface". The result-counter family was not shown in this selected capture.

EXECUTED: A separate reports-controller scan emits the exact debug-policy
error as a resource PolicyViolation Event, from kyverno-scan.
L `captures/14-gctx-error-event.txt:7`. This is not an acting-path receipt.
It also falsifies an unqualified claim that no Event anywhere recorded the
error. UR Completed and lack of executor logs remain operator observations
in L `INTEGRATION-LOG.md:159-176`; a raw UR status transcript is still needed
for the standalone upstream report.

VERIFIED: The triggering lab expression requests a nonexistent projection.
L `manifests/04-kyverno-gce.yaml:9-14` and
`manifests/05-kyverno-metric-suspend.yaml:30-31`;
K `pkg/globalcontext/externalapi/entry.go:64-73,116-129,154-164`.
Corrected manifests exist at L `manifests/04b-kyverno-gce-projected.yaml:15-17`
and `manifests/05c-kyverno-metric-suspend-gce.yaml:25-27`.
Their execution is still pending per the operator. Do not claim a successful
corrected-GCE runtime test yet.

INFERRED: The draft's smaller `int(object.metadata.name)` reproduction
should isolate the diagnostic defect without Prometheus or GlobalContext.
It is explicitly unexecuted. A false target condition is a required control:
the matcher lets a false condition override other condition errors.
K `pkg/cel/policies/mpol/compiler/policy.go:60-66`.

## Main-version verdict

VERIFIED fix present; INFERRED non-reproduction: this specific persistent
resume/startTime wedge is fixed in local U and current upstream master,
with the 1.34 backport shipped in v1.34.2; no fixed-version rerun was performed.
