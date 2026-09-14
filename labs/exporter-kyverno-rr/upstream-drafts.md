<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Upstream drafts: exporter lab, round 3

Date: 2026-09-14. Drafts only. Nothing posted upstream.
EXECUTED means supplied lab evidence. VERIFIED means inspected source.
INFERRED marks proposed reproduction or causal attribution.
Local evidence paths below are attachment references for a future report.

## Draft 1: Kyverno

Title: MutatingPolicy mutate-existing completes UpdateRequest without
per-target diagnostics after a CEL evaluation error

### What happened

On Kyverno v1.19.1, a background mutate-existing evaluation can produce
RuleError with no PatchedResource. The processor does not inspect that rule
error. It can complete the UpdateRequest with an empty message, without
generating its action-path events or reports for the failed target.

VERIFIED in tag v1.19.1, commit
`40ec788d48bb28d83dbf85538e962a59db9d45c6`:

- `pkg/cel/policies/mpol/compiler/policy.go:201-209` returns a target
  condition runtime error as EvaluationResult.Error.
- `pkg/cel/policies/mpol/engine/engine.go:224-226` converts it to RuleError
  with no patch. `:95-118` returns that response with nil outer error.
- `pkg/background/mpol/processor.go:230-235` checks only the outer error
  before gating the entire update/audit branch on PatchedResource.
- `pkg/background/mpol/processor.go:276,407-415` treats an empty failures
  list as success. `pkg/background/common/status.go:37-39` sets Completed
  and clears the message.
- `pkg/background/mpol/processor.go:308-344` would generate events and
  optional reports, but the nil-patch error never reaches this audit call.

Source: [v1.19.1 processor](https://github.com/kyverno/kyverno/blob/v1.19.1/pkg/background/mpol/processor.go#L230-L276).
The relevant files are unchanged at supplied main commit `3bb926936656`.

### Observed evidence and limits

Environment reported by the lab operator: kind, Kubernetes server v1.34.0,
Kyverno v1.19.1, chart 3.9.1, BACKGROUND_SCAN_INTERVAL=60s, and explicit
background-controller permissions to read/update Jobs.

The original condition called:

```cel
object.metadata.name in
globalContext.Get('running-2m', 'data.result[].metric.workload')
```

VERIFIED: This was an invalid projection lookup in the lab manifest.
The second argument is a projection name. No such projection was declared.
The intended fault report concerns how this runtime error is surfaced.
It does not claim that background GlobalContext reads are unsupported.
`pkg/globalcontext/externalapi/entry.go:64-73,116-129,154-164`.

EXECUTED attachments:

- `captures/07b-gce-poll-v6.txt:1-2`: successful external API refreshes.
- `captures/18-metrics-surface.txt:2,6`: background duration histogram
  result=error counts of 3 for dbg-gctx and 7 for metric-suspend-running.
  The capture includes no target name, target UID, or error message.
- `captures/14-gctx-error-event.txt:7`: the separate reports controller
  emits a PolicyViolation Event with `failed to get global reference:
  no data available`. Its source is kyverno-scan. The reporting evaluation
  therefore does expose the error through a different path.

The operator also observed Completed URs and no background-executor error
log while the target stayed unsuspended for more than eight minutes.
That observation is recorded in `INTEGRATION-LOG.md:159-176`; a raw UR status
capture and full observation timeline must accompany a filed report.
`captures/08-kyverno-suspend-timeline.txt` alone contains only an initial
sample and does not establish the full duration.

VERIFIED qualification: The metric wrapper records both duration and result
metrics (`pkg/cel/policies/mpol/engine/metrics.go:18-33`;
`pkg/metrics/mpol.go:37-50,54-91`). The captured executor error diagnostic
was the aggregate histogram. This is not a claim that only one metric can
exist, or that no Kyverno Event anywhere can contain the error.

### Original proposed fixture (executed 2026-09-14; results in the EXECUTED section below)

Originally INFERRED, NOT EXECUTED - the run below used this fixture
with two deltas noted there. This removes the API service and projection lookup.
It forces a runtime string-to-int conversion error on a nonnumeric Job name.
Run in an isolated test cluster with the MutatingPolicy feature available.
Do not reuse a target selected by another acting policy.

First apply the fixture and RBAC below, saved as `fixture.yaml`:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: rr-cel-error
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: rr-cel-error-background-jobs
  labels:
    rbac.kyverno.io/aggregate-to-background-controller: "true"
rules:
  - apiGroups: [batch]
    resources: [jobs]
    verbs: [get, list, watch, update]
---
apiVersion: batch/v1
kind: Job
metadata:
  name: cel-error-job
  namespace: rr-cel-error
  labels:
    rr-cel-error: "true"
spec:
  suspend: false
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: worker
          image: busybox:1.36.1
          command: [sh, -c, "sleep 3600"]
```

The aggregation label matches the chart contract at
`charts/kyverno/templates/background-controller/clusterrole.yaml:9-14`.
Wait for aggregation and verify the actual controller service account can
list/get/update Jobs. Missing RBAC is a separate failure and invalidates
this diagnostic experiment.

Then save this as `policy.yaml`:

```yaml
apiVersion: policies.kyverno.io/v1beta1
kind: MutatingPolicy
metadata:
  name: rr-cel-runtime-error
spec:
  matchConstraints:
    namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: rr-cel-error
    objectSelector:
      matchLabels:
        rr-cel-error: "true"
    resourceRules:
      - apiGroups: [batch]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [jobs]
  evaluation:
    admission:
      enabled: false
    mutateExisting:
      enabled: true
  targetMatchConditions:
    - name: deliberate-runtime-error
      expression: int(object.metadata.name) > 0
  mutations:
    - patchType: JSONPatch
      jsonPatch:
        expression: >-
          [JSONPatch{op: 'add', path: '/spec/suspend', value: true}]
```

Suggested capture sequence:

```sh
kubectl apply -f fixture.yaml
# After RBAC aggregation, start this watch in a separate terminal.
kubectl get updaterequests.kyverno.io -A -w -o yaml
# In the first terminal:
kubectl apply -f policy.yaml
kubectl get mutatingpolicy rr-cel-runtime-error -o yaml
kubectl get job -n rr-cel-error cel-error-job -o yaml
kubectl get events -n rr-cel-error -o yaml
```

Record the controller metrics before/after, not only accumulated counts.
Observe a policy-triggered UR and at least one configured scan interval.
Watch URs because completed requests can be transient.

Controls, also unexecuted:

1. Change the condition to the CEL expression `false`: the Job should remain
   unsuspended with a skip result, not an evaluation error.
2. Change it to `true`: the same Job should suspend, proving RBAC and target
   resolution work. Run this control last; no resume is needed.
3. Move the runtime conversion into a mutation expression to cover the
   parallel error path at compiler `policy.go:234-236`.

Do not combine the deliberate error with a false target condition.
`pkg/cel/policies/mpol/compiler/policy.go:60-66` intentionally lets false
conditions take precedence over other match-condition errors.

### EXECUTED 2026-09-14: the proposed reproduction ran clean (Claude)

Environment: kind kyverno-lab2, Kubernetes v1.34.3, Kyverno v1.19.1
(chart 3.9.1), BACKGROUND_SCAN_INTERVAL=60s, background-controller Job
RBAC granted via the aggregation label. Full transcript:
`captures/31-cel-error-repro.txt`. Differences from the proposal:
RBAC pre-existed on the cluster (same aggregation contract), image tag
busybox:1.36.

Result:

- Four URs displayed Pending, Completed, and deletion during the
  150-second watch, with AGE 0s or 1s. Their individual trigger sources
  were not captured. The selected background-controller log query
  returned no policy-name matches. The error-result histogram series
  was absent before and had count 4 afterward. This matches the UR
  total as aggregate corroboration, not a traced result for each UR.
- The Job stayed unsuspended (the mutation never applied).
- Policy status: `ready: true`, `message: ""`, RBACPermissionsGranted.
- The wrapper records duration and result metric families; only the
  duration histogram was captured here.
- The reports controller separately emitted PolicyViolation events
  reading `fail: mutation is not applied`. Per source, the reporting
  scan evaluates with target=false semantics (matchConditions, not the
  failing targetMatchConditions), so this is its own successful
  simulation reporting an unapplied mutation - a coincidentally
  misleading surface, not a relabeling of the acting-path error
  (`pkg/controllers/report/utils/scanner.go:243-264`,
  `pkg/cel/policies/mpol/engine/engine.go:158`).

Controls (condition `false`, condition `true`, error in the mutation
expression) remain unexecuted.

### Expected behavior / proposed fix

INFERRED proposal: Preserve per-target policy errors independently of patch
presence. Add target identity and the error reason to the UR failure path,
or an explicit documented partial-error result. Invoke existing diagnostic
generation for an error response even when no update can be attempted.
Preserve successful targets and ordinary false-condition skips.

Regression tests should distinguish true, false, condition runtime error,
mutation runtime error, and mixed successful/errored targets. Assert UR
status/message and error diagnostic generation. Aggregate metrics alone
cannot link the failure to a specific target execution.

## Draft 2: Kubernetes

Disposition: Historical duplicate of #134521, already fixed. Preserve for
the lab record. Do not file as a new unresolved bug without a fixed-version
reproduction.

Title: Job controller repeatedly rejects status after suspend/resume on
v1.34.0, matching the startTime validation bug fixed in v1.34.2

### What happened

The lab operator reproduced persistent Job status failure three times on
kind with Kubernetes server v1.34.0. The sequence was suspend then resume.
One scripted resume was three seconds after suspension was observed.

EXECUTED: `captures/16-trap-trainer-c.txt:3-5` records that three-second
sequence. It does not itself contain the later controller errors for
trainer-c. The three occurrences are operator-reported in
`INTEGRATION-LOG.md:263-270`. The detailed captured failure is trainer-b:

```text
syncing job: tracking status: adding uncounted pods to status:
Job.batch "trainer-b" is invalid: status.startTime: Required value:
startTime cannot be removed for unsuspended job
```

The message above is line-wrapped for readability.
`captures/12-job-wedge-kcm.txt:1-15` records retries from 17:43:29 to
17:52:32 on 2026-09-14. Lines 16-17 record later recurrences.
`captures/13-trainer-b-wedged.txt:70,95-106` shows spec.suspend=false,
Suspended=True, ready=0, and the original non-null startTime.
Its description shows Resumed x15 (`:45`), while Job status reports zero
active Pods (`:12`). The operator separately observed a healthy running Pod.

### Known upstream resolution

VERIFIED: [Issue #134521](https://github.com/kubernetes/kubernetes/issues/134521)
already reports this error after an ordinary start/suspend/resume sequence.
[PR #134769](https://github.com/kubernetes/kubernetes/pull/134769/files)
merged on 2025-11-04. The fix was backported as #135130 in v1.34.2.
[v1.34.2 release](https://github.com/kubernetes/kubernetes/releases/tag/v1.34.2).
Local source `CHANGELOG/CHANGELOG-1.34.md:954,1037-1039` at Kubernetes
`121abd026c869bdb9b3896ac64190efcdb8d70cb` confirms the release placement.
The local checkout and fetched current master already include the fix.

INFERRED: The lab failure is the same bug, not evidence of a new race.
Neither a fast resume nor incomplete Pod bookkeeping is necessary for the
known failure. No fixed-version reproduction was performed in this review.

### Reproduction for an affected-version comparison

INFERRED, NOT A NEW EXECUTION: Use an isolated v1.34.0 cluster with ordinary
JobManagedBy defaults. Exclude this namespace from external automation.
Kyverno, Prometheus, Karta, and the Runtime Rules prototype are not required
for this proposed Kubernetes-only test.

```sh
kubectl create namespace job-resume-repro
kubectl create job -n job-resume-repro starttime-repro \
  --image=busybox:1.36.1 -- sh -c 'sleep 3600'
kubectl wait -n job-resume-repro --for=condition=Ready pod \
  -l batch.kubernetes.io/job-name=starttime-repro --timeout=120s
kubectl get job -n job-resume-repro starttime-repro -o yaml
kubectl patch job -n job-resume-repro starttime-repro --type=merge \
  -p '{"spec":{"suspend":true}}'
sleep 3
kubectl patch job -n job-resume-repro starttime-repro --type=merge \
  -p '{"spec":{"suspend":false}}'
kubectl get job -n job-resume-repro starttime-repro -o yaml
kubectl get pods -n job-resume-repro -o wide
kubectl get events -n job-resume-repro -o yaml
kubectl logs -n kube-system -l component=kube-controller-manager \
  --since=10m --tail=-1
```

Wait until a Pod exists before the Ready command; kubectl wait does not
wait for an empty selector to acquire resources. Confirm the pre-suspend
Job has a non-null startTime. Capture the intermediate status too.
Repeat with a control that waits for Suspended=True and zero active Pods
before resuming. If a fast resume never persists Suspended=True, it need
not hit the broken transition. Do not wait for startTime to become null:
the affected release keeps it during suspension.

Repeat on v1.34.2 or a later fixed release. Capture both API server and
controller versions. EXECUTED addendum 2026-09-14: on kind v1.34.3 the
same shape converged - suspend, settle, user resume, `active=1`
immediately, startTime reset accepted
(`captures/30-lab2-kyverno-phase.txt`). Both sides of the fix boundary
are now observed in this lab.

### Why it happens

VERIFIED source corroboration: v1.34.0 files were fetched from upstream.
The exact local line references below use the still-affected v1.34.1 module
at `~/go/pkg/mod/k8s.io/kubernetes@v1.34.1`.

- Controller `pkg/controller/job/job_controller.go:1036-1058`: suspension
  preserves startTime. Resume sets a new timestamp and emits Resumed before
  successfully persisting the status transition.
- Validator `pkg/apis/batch/validation/validation.go:698-704`: any change
  to a previously non-null startTime is rejected while unsuspended. The
  message says removal even when the request supplies a new non-null time.
- Strategy `pkg/registry/batch/job/strategy.go:345-346,377,400`: the guard
  has no exemption for the controller's Suspended=True to False transition.
- Controller `pkg/controller/job/job_controller.go:1381-1383,1883-1885`:
  status persistence fails through the common uncounted-Pod flush helper.
- Controller `pkg/controller/job/job_controller.go:639-655,828-856`:
  each retry reloads the still-unmodified status and recomputes the same
  invalid transition. Retry backoff caps at a minute (`:65-68,188`).

The fixed v1.34.2 strategy exempts the resume condition transition:
`pkg/registry/batch/job/strategy.go:383-387,405`.
Its regression test is `test/integration/job/job_test.go:4094-4170`.
This is a validation/controller contract mismatch, not a permanently cached
request that would repair itself merely by waiting longer.

### Manual patch observation

The operator reported a rejected status merge patch that SET startTime.
The raw payload and response are not present in the saved captures.
INFERRED: A different non-null timestamp hits the same guard. No admission
mutation is required to explain that rejection. A SET of the identical
stored timestamp would need separate investigation.

The API server compares its proposed object with the stored object, not
the Job controller's informer copy. Source at local Kubernetes
`121abd026c869bdb9b3896ac64190efcdb8d70cb`:
`staging/src/k8s.io/apiserver/pkg/endpoints/handlers/patch.go:578-590`;
`staging/src/k8s.io/apiserver/pkg/registry/generic/registry/store.go:709-721,825-826`.

### Expected behavior

Resume should persist Suspended=False and a fresh startTime, with Job status
converging to the replacement Pods. That is the behavior the existing fix
and regression test supply. A new issue would need evidence of a distinct
failure on a version containing that fix.
