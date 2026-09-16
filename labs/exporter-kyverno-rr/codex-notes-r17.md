<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Round 17: closing gaps without changing Kyverno code

VERIFIED = source read at the pinned revision. EXECUTED = an existing lab
capture or an explicitly labeled offline CLI check. INFERRED = proposed
behavior or limitation requiring an experiment. No cluster resources were
changed for this review.

Source keys:

- K: Kyverno v1.19.1, peeled commit
  `40ec788d48bb28d83dbf85538e962a59db9d45c6`. Read tag blobs, not main.
- A: github.com/kyverno/api at
  `v0.0.1-alpha.3.0.20260723090831-fb2785727f98`, pinned by K go.mod.
  The v1beta1 policy specs alias v1alpha1 types. A references below are in
  `api/policies.kyverno.io/v1alpha1/` unless stated otherwise.
- E: Karta lab revision `6a8f418f`. Read git blobs: the old src worktree
  now contains a different branch. No checkout was changed.
- L: this exporter-lab directory. R: its parent research directory.

Verdicts cover the executed gap, including its stated contract limits.
They are not claims that a proposed workaround has passed a live test.

## Before the annotation experiment

INFERRED: change A before running it. `state != user-resumed` still allows
A to act when state is `we-suspended`. After resume, both A and B can match.
If A wins, it suspends again before B can mark the resume. There is no
cross-policy order guarantee. Require A's state to be explicitly `armed`.
B changes `we-suspended` to `user-resumed`. Neither consumed state permits A.
Source basis: K `pkg/background/mpol/processor.go:230-261`;
`cmd/background-controller/main.go:127-128,143-144`.

VERIFIED: background JSONPatch tests are local, not API-server tests.
The processor computes an object, fetches the latest resourceVersion, then
PUTs the previously computed object. Test resumes between evaluation and
that refetch. See gap 2 before interpreting a quiet scan as a safety proof.
K `pkg/cel/policies/mpol/compiler/json.go:56-73`;
`pkg/background/mpol/processor.go:230-261`.

VERIFIED: the receipt writer needs
`spec.evaluation.skipBackgroundRequests: false`. It is not a top-level
`spec.skipBackgroundRequests` field. The CEL spelling is `generator.Apply`,
with capital A. A `generating_policy.go:104-124,176-181`;
K `pkg/webhooks/resource/gpol/handler.go:55-76`;
`pkg/cel/policies/gpol/compiler/compiler_test.go:125-152`.

## Gap 1: acting-path CEL errors

Verdict: PARTIALLY. No flag repairs this lost result. Metrics and a paired
validation canary can make the problem visible without patching Kyverno.

### What flags cannot recover

VERIFIED chain:

1. CEL condition errors become EvaluationResult.Error. Another false
   condition can mask them. No logger runs in these branches.
   K `pkg/cel/policies/mpol/compiler/policy.go:42-69,201-219`.
2. The engine constructs RuleError, returns no patched resource, and the
   outer engine call returns a nil Go error. These branches have no logger.
   K `pkg/cel/policies/mpol/engine/engine.go:84-118,215-226`.
3. The processor checks the outer error, then gates both update and audit
   on PatchedResource. The RuleError never reaches audit on this path.
   Its error log calls inside audit handling do not run here.
   K `pkg/background/mpol/processor.go:230-276,308-326,407-415`.

VERIFIED: verbosity does not add a missing log statement. Some underlying
libraries do log other conditions: a missing GCE entry has a V(2) message,
whereas the named-projection read error is simply returned. That does not
recover the generic CEL error or turn this projection error into a log.
K `pkg/cel/libs/context.go:115-124`.

VERIFIED: `--omitEvents=PolicyApplied,PolicySkipped` is not the cause.
No Event reaches the generator at all on the patchless RuleError branch.
The omit filter only filters supplied Events. Removing those omissions,
setting `generateSuccessEvents=true`, or raising `maxQueuedEvents` cannot
create the missing error Event. K `pkg/event/controller.go:82-118`;
`pkg/background/mpol/processor.go:235-276,325-326`.

VERIFIED: there is no completed-UR retention/TTL switch on this path.
Completed causes a direct Delete; Failed is reset to Pending for retry.
The branch does not consult age or retention configuration. Background
flags include workers/events/report limits, not a UR retention interval.
K `pkg/background/update_request_controller.go:268-289`;
`cmd/background-controller/main.go:132-175`; `cmd/internal/flag.go:111-138,180-183`.

INFERRED ops workaround: export a live UR watch to a retained store, as
capture 31 demonstrates is possible. It preserves transitions, not the
already discarded RuleError. Pinning a UR with a finalizer or denying its
deletion would interfere with cleanup and still retain an empty success
record; it is not a diagnostic fix or a supported retention knob.
EXECUTED: L `captures/31-cel-error-repro.txt:8-24` captured four URs through
Pending, Completed and DELETED. DELETED is a watch event, not a UR state.

### Alert on the existing metrics

VERIFIED: the wrapper records BOTH duration and result even when there is
no patch. The counter instrument is `kyverno_mutating_policy_results`;
its Prometheus family is `kyverno_mutating_policy_results_total`.
It carries `policy_name`, `result`, `execution_cause` and other dimensions.
No target name, target UID or error message is recorded.
K `pkg/cel/policies/mpol/engine/metrics.go:18-33`;
`pkg/metrics/mpol.go:34-51,74-91`;
`charts/kyverno/values.yaml:551-554`.

Concrete recipe, INFERRED until deployed:

- Scrape the background controller metrics endpoint. Do not mix it with
  the reports controller: both wrappers can use `background_scan` as the
  execution cause. Add a scrape-job label such as `kyverno-background`.
- Keep metrics enabled, retain `policy_name`, and alert on:

```promql
sum by (policy_name) (
  increase(kyverno_mutating_policy_results_total{
    job="kyverno-background",result="error",execution_cause="background_scan"
  }[5m])
) > 0
```

The five-minute lookback is a proposed alert setting, not a measurement.
For the initial smoke test, inspect the raw counter too: a newly appearing
nonzero series has no earlier zero sample for increase to compare against.
A raw `sum by (policy_name) (...{result="error"}) > 0` also detects that
case, but stays active until the series resets/disappears.

VERIFIED configuration: `--disableMetrics=false` and `--metricsPort=8000`
are defaults. The chart can create the background metrics Service.
K `cmd/internal/flag.go:116,122`;
`charts/kyverno/templates/background-controller/service.yaml:1-31`.
EXECUTED: the histogram count, not this counter family, was captured as 4
for the minimal repro. L `captures/31-cel-error-repro.txt:39-40`.
The duration histogram's `_count{result="error"}` is another alert input.

### Paired Audit ValidatingPolicy canary

VERIFIED: this path does preserve errors. The reports scanner runs the
ValidatingPolicy engine and converts its rules to report results.
Validation-expression errors become RuleError, then report result `error`
with the message. Reporting emits PolicyViolation Events regarding BOTH
the resource and the policy for non-pass/non-skip results.
K `pkg/controllers/report/utils/scanner.go:180-221`;
`pkg/cel/policies/vpol/compiler/policy.go:144-175`;
`pkg/cel/policies/vpol/engine/engine.go:140-144,199-207`;
`pkg/controllers/report/background/controller.go:733-740`;
`pkg/utils/report/results.go:64-71,110-116,296-304`;
`pkg/controllers/report/utils/events.go:10-18,53-70`.

INFERRED recipe for the same minimal failing expression:

```yaml
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: rr-cel-error-canary
spec:
  validationActions: [Audit]
  failurePolicy: Fail
  evaluation:
    admission:
      enabled: false
    background:
      enabled: true
  matchConstraints:
    resourceRules:
      - apiGroups: ["batch"]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["jobs"]
  matchConditions:
    - name: scope
      expression: >-
        object.metadata.namespace == 'rr-cel-error' &&
        object.metadata.name == 'cel-error-job'
  variables:
    - name: probe
      expression: int(object.metadata.name) > 0
  validations:
    - expression: variables.probe || !variables.probe
      message: condition evaluation must yield a boolean
```

EXECUTED, offline CLI only: the installed `kyverno version` reports 1.19.1
with no embedded commit ID. Applying the exact canary YAML above to a local
batch/v1 Job named cel-error-job in rr-cel-error produced:

```yaml
- message: 'error: type conversion error from ''string'' to ''int'''
  policy: rr-cel-error-canary
```

Its report result was `error` and the CLI exited 1. Replacing only the probe
with YAML `expression: 'true'`, then `expression: 'false'` (CEL boolean
constants), produced `pass` and exit 0 in both cases. Command shape: `kyverno apply policy.yaml --resource
job.yaml --policy-report --remove-color`, with local files and no cluster
flag. These are canary-expression checks, not the pending acting-path
controls from Draft 1. Live scanner scheduling and Events remain unexecuted.

INFERRED integration: replace probe with the acting expression for the real
policy. Put harmless scope guards in matchConditions; put the
probe in validations/variables. With failurePolicy Ignore, an error in
matchConditions can become a skip. A false scope condition can also hide
an error. K `pkg/cel/policies/vpol/compiler/policy.go:123-147,201-232`.

VERIFIED prerequisites: reports `--backgroundScan=true`,
`--enableReporting` including validate, `--allowedResults` including error,
and no PolicyViolation omission on the REPORTS controller. All are enabled
by their corresponding defaults except explicitly configured omissions.
Its `--backgroundScanInterval` is separate from the background controller
environment variable; set it explicitly for the experiment.
K `cmd/reports-controller/main.go:313-323`;
`cmd/internal/flag.go:181-182`.

VERIFIED: HTTP and GlobalContext libraries exist for ValidatingPolicy too.
K `pkg/cel/policies/vpol/compiler/compiler.go:243-246,292-296`.
INFERRED limit: this is a second evaluation at another time, with another
controller's cache and credentials. It can diagnose persistent expression
failures, but cannot recover the exact failed action's facts or guarantee
observation of a transient error. Reporting uses a synthetic CREATE, with
no old object or authentic admission actor. Do not copy request-dependent
expressions blindly. K `pkg/controllers/report/utils/scanner.go:195-208`.

## Gap 2: resume trap and allowance memory

Verdict: PARTIALLY. Object annotations can supply real persistent memory.
The proposed two-policy predicate needs correction, and background writes
alone do not establish a race-safe one-action contract.

### Minimal corrected recipe

INFERRED: use dedicated keys per rule/revision, opt-in labels, and an
explicit adoption state. For this lab, initialize the selected Job with
`lab.karta.dev/rr-state=armed` and `lab.karta.dev/rr-epoch=0`. Missing or
unrecognized state must skip, not implicitly grant another allowance.
Both policies use Jobs resource rules, the opt-in objectSelector, and:

```yaml
evaluation:
  admission:
    enabled: false
  mutateExisting:
    enabled: true
variables:
  - name: marks
    expression: object.metadata.?annotations.orValue({})
```

A's targetMatchConditions must all hold:

```cel
object.metadata.namespace == 'ai-team'
!object.spec.?suspend.orValue(false)
'lab.karta.dev/rr-state' in variables.marks &&
  variables.marks['lab.karta.dev/rr-state'] == 'armed'
object.metadata.name in globalContext.Get('running-2m', 'workloads')
```

Keep the existing metric predicate unchanged for the comparison. With the
adoption annotations present, A's JSONPatch expression can be:

```cel
[
  JSONPatch{op: 'add', path: '/spec/suspend', value: true},
  JSONPatch{op: 'add', path: '/metadata/annotations/lab.karta.dev~1rr-state',
            value: 'we-suspended'}
]
```

B uses the same namespace/label scope, no metric condition, and these target
conditions:

```cel
!object.spec.?suspend.orValue(false)
'lab.karta.dev/rr-state' in variables.marks &&
  variables.marks['lab.karta.dev/rr-state'] == 'we-suspended'
```

B's patch expression:

```cel
[
  JSONPatch{op: 'add', path: '/metadata/annotations/lab.karta.dev~1rr-state',
            value: 'user-resumed'},
  JSONPatch{op: 'add', path: '/metadata/annotations/lab.karta.dev~1rr-epoch',
            value: '1'}
]
```

INFERRED: this models one initial action and a non-rearming observed resume,
matching this RR configuration. It does not provide a fresh allowance per
resume or an escalation receipt. `user-resumed` is only a state label:
the background scan did not authenticate a human or observe every transition.
A manually removed/replaced annotation is not durable history.

VERIFIED mechanisms and limitations:

- Multiple JSONPatch operations, including annotation fields, are supported.
  They are applied to one local object and later sent in one UPDATE.
  K `pkg/cel/policies/mpol/compiler/compiler.go:135-145`;
  `compiler/json.go:56-90,111-128` in that directory;
  `pkg/background/mpol/processor.go:255-261`.
- JSON Pointer keys require `/` -> `~1` and `~` -> `~0`. Alternatively use
  `'/metadata/annotations/' + jsonpatch.escapeKey(key)`. The library is
  installed at K `pkg/cel/policies/mpol/compiler/compiler.go:202`.
  A new leaf requires its parent map. Never unconditionally replace an
  existing annotations map with `{}`. The explicit adoption above creates
  the map; a generic policy must conditionally create it when absent.
- A UR selects its own policy by name; A and B are separate work items.
  There are two policy workers and ten background workers by default.
  Policy-name sorting is not a serialization protocol. Even reducing
  genWorkers does not exclude user updates or other controllers.
  K `pkg/background/mpol/processor.go:230`;
  `cmd/background-controller/main.go:127-128,143-144`;
  `pkg/cel/policies/mpol/engine/reconciler.go:133-151`.
- INFERRED: in a settled object, corrected A skips consumed states and B skips after
  stamping its state. That is logical idempotence, not concurrency proof.
  A+B have no explicit cross-policy ordering or transaction.
- The live write is a full UPDATE, not an API JSONPatch. Refetching copies
  only the fresh resourceVersion into the earlier candidate. A newer resume
  or annotations can be overwritten if they arrive before this refetch.
  A conflict after refetch can fail the attempt, but is not a complete
  defense. B also submits its old suspend value even though B's authored
  JSONPatch does not name /spec/suspend.
  K `pkg/background/mpol/processor.go:245-267`;
  `pkg/clients/dclient/client.go:326-334`.
- JSONPatch `test` runs against the earlier local snapshot. A failed test
  returns the unchanged object, not a failed UR. It cannot repair the
  refetch window. K `pkg/cel/policies/mpol/compiler/json.go:56-73`.
- Keep admission disabled on A/B for this test. Otherwise admission uses
  matchConditions, not targetMatchConditions; enabling it with only target
  guards can cause an immediate mutation on a resume request.
  K `pkg/cel/policies/mpol/engine/engine.go:154-158`;
  `pkg/cel/policies/mpol/compiler/policy.go:201-219`;
  `pkg/webhooks/resource/mpol/handler.go:97-107`.

INFERRED stronger no-code pattern: replace polling B with an admission-only
MutatingPolicy. Match UPDATE with old suspend=true and new suspend=false;
stamp the resume in that same admission transaction. Use matchConditions,
admission enabled and mutateExisting disabled on that transition policy.
It has real oldObject and request.userInfo, unlike the synthetic scan.
K `pkg/cel/policies/mpol/compiler/eval.go:35-57`;
`pkg/background/mpol/processor.go:184-228`.

INFERRED guard against stale A: add an admission Deny ValidatingPolicy that
compares old/new stored allowance markers, forbids consumed -> armed
rollback, and rejects the background actor's false -> true transition
once the old state is consumed. Protect removal of the marker and opt-in
scope too. Keep its failurePolicy Fail. This is implementable authoring,
not a supplied runtime protocol; tests must include writer identity,
webhook outage and deliberate re-adoption. No claim of impossibility.
The vpol admission handler evaluates the actual admission request:
K `pkg/webhooks/resource/vpol/handler.go:74-84`.

### Reporting and tests that distinguish success from luck

VERIFIED: the reporting scan calls the non-target evaluator, so target-only
A/B guards do not restrict its simulated patch. It can report mutation drift
even when the background machine intentionally does nothing.
K `pkg/controllers/report/utils/scanner.go:235-265`;
`pkg/cel/policies/mpol/compiler/policy.go:201-219`.

INFERRED recipe: mirror the scope/state gates into matchConditions for
reporting, with admission still disabled; or explicitly disable background
reporting on A/B and use a validation canary. Disabling reporting is not
improved diagnostics and must be disclosed. It does not stop mutate-existing:
K `pkg/policy/policy_controller.go:599-603`.

INFERRED execution checklist:

1. Use a new opted-in UID. Leave a matching but unlabeled control untouched.
2. Capture state plus suspend before A, after A, after resume, after B,
   and after metric eligibility returns across several ticks.
3. Try the original predicate once to expose the A/B race, then the corrected
   armed-only predicate. Do not infer ordering from one policy-name choice.
4. Resume near a tick and during overlapping A work. Preserve resourceVersions
   and both policies' UR watches. A quiet window after B is not this test.
5. Change an unrelated annotation and manually suspend while B is pending.
   Test whether the full UPDATE restores older fields.
6. Restart the controller; remove state; recreate the Job with the same name;
   reapply the policy. Define adoption for each case instead of silently rearming.
7. Test duplicate work and the optional admission guard separately. Count
   accepted suspend transitions, not merely completed URs.

## Gap 3: durable action receipts

Verdict: PARTIALLY. GeneratingPolicy can write retained ConfigMaps without
changing Kyverno. That closes the narrow post-deletion-record absence.
It does not make receipt persistence a prerequisite for the action.

### Concrete writer configuration

INFERRED recipe: create a ledger namespace, grant the background controller
get/create/update on ConfigMaps there and get/list on the triggering Jobs,
then install a cluster-scoped GeneratingPolicy before A. Match batch/v1 Jobs
on UPDATE, the opt-in scope, suspend=true and state=we-suspended. The proper
configuration shape is:

```yaml
spec:
  evaluation:
    admission:
      enabled: true
    skipBackgroundRequests: false
    generateExisting:
      enabled: false
    synchronize:
      enabled: false
    orphanDownstreamOnPolicyDelete:
      enabled: true
```

VERIFIED: these fields belong under evaluation, including the boolean
skipBackgroundRequests. Defaults are skip=true, synchronize=false,
generateExisting=false, orphan=false, admission=true. A
`generating_policy.go:104-181`. The v1beta1 aliases are at A
`../v1beta1/generating_policy.go:10-18`.

INFERRED CEL for `spec.generate[].expression`, after guarding annotation
presence in matchConditions:

```cel
generator.Apply('rr-ledger', [{
  'apiVersion': dyn('v1'),
  'kind': dyn('ConfigMap'),
  'metadata': dyn({
    'name': 'rr-stop-v1-' + object.metadata.uid + '-e' +
            object.metadata.annotations['lab.karta.dev/rr-epoch']
  }),
  'data': dyn({
    'phase': 'ObservedSuspendRequest',
    'rule': 'stop-v1',
    'targetUID': object.metadata.uid,
    'targetNamespace': object.metadata.namespace,
    'targetName': object.metadata.name,
    'epoch': object.metadata.annotations['lab.karta.dev/rr-epoch'],
    'requestedSuspend': 'true'
  })
}])
```

Use a deterministic rule/revision + full UID + action epoch, not a name
based only on Job name. A random name or current time on every evaluation
creates duplicate records. Sanitize any user-controlled name segment.
For the one-action recipe, epoch 0 is the action identity; B advances to 1
without granting a second action. Keep ConfigMap data values strings.

VERIFIED: GeneratingPolicy supports lazy variables, http.Get,
globalContext.Get, JSON helpers and generator.Apply. Use a cluster-scoped
policy for GlobalContext. K
`pkg/cel/policies/gpol/compiler/compiler.go:81-100,116-141`;
`pkg/cel/policies/gpol/compiler/policy.go:127-158`.
INFERRED evidence recipe: bind one metric result in A, then stamp its query,
sample and observation time alongside suspend/state. Copy that saved evidence
into the ConfigMap. A fresh HTTP/GCE read by the generator is later evidence,
not necessarily the facts that caused the action. This example intentionally
labels its record ObservedSuspendRequest, not Executed or Converged.

### Why the trigger chain works, and where it stops

VERIFIED: the mpol background write uses the normal Kubernetes UPDATE client.
The gpol admission handler recognizes the background service account and
skips it by default. Opting in allows it to enqueue a CELGenerate UR.
The handler returns an allowed response and submits work asynchronously.
K `pkg/clients/dclient/client.go:326-334`;
`pkg/webhooks/resource/gpol/handler.go:42-76,128-145`.
INFERRED: with the generated webhook matching these Jobs, no admission
filter excluding the target, and the stated RBAC, A's update can trigger the
receipt writer. This chain still needs the proposed live execution.

VERIFIED: synchronize=false prevents the trigger-deletion cleanup branch
and avoids registering synchronization watchers. The generated object gets
Kyverno tracking labels, not an automatically added workload ownerReference.
An independently constructed ConfigMap with no ownerReference can therefore
outlive Job deletion. Orphan-on-policy-delete explicitly preserves it on
that separate lifecycle too.
K `pkg/webhooks/resource/gpol/handler.go:93-116`;
`pkg/background/gpol/generate_controller.go:185-204`;
`pkg/cel/libs/context.go:237-239,382-402`;
`pkg/policy/policy_controller.go:341-346`.
INFERRED limits: deleting the ledger namespace or the ConfigMap still removes
it. Persistence here means survival of workload deletion, not immutable or
unbounded archival storage.

VERIFIED: synchronize=false does NOT mean append-only/create-once. On a new
matching generation, an existing object managed by this policy/trigger is
updated (or applied with SSA). That code does not check synchronize.
K `pkg/cel/libs/context.go:266-359`.
INFERRED recipe: trigger only on the suspension/epoch transition using
oldObject as well as object, or make repeated content identical. Test
retries, status updates and receipt overwrite. A ConfigMap marked immutable
can reject changed data; it does not itself solve replay or missing receipts.

VERIFIED: generateExisting is a backfill mechanism, not an event journal.
Policy processing can build URs for existing triggers; those requests use a
synthetic CREATE. A receipt policy matching UPDATE only will not match that
synthetic request. To add backfill, explicitly support CREATE and distinguish
reconstructed observations from admission-triggered records.
K `pkg/policy/gpol.go:55-72`;
`pkg/background/gpol/generate_controller.go:107-131`.

VERIFIED: the worker requires a live trigger of the recorded UID before
processing an ordinary UPDATE. Deletion before it processes the UR can
prevent receipt creation. Furthermore, in the admission-triggered branch,
it subsequently evaluates the saved AdmissionRequest, not the freshly
fetched trigger object. A live-UID check is not proof that the requested
suspend update was committed.
K `pkg/background/common/resource.go:79-85,121-140`;
`pkg/background/gpol/generate_controller.go:95-100,107-140`;
`pkg/cel/policies/gpol/engine/engine.go:48-77`.

INFERRED hostile tests: deny A's update in a later validation step while
leaving the Job alive; delete the Job immediately after an accepted update;
resume before the receipt worker runs; revoke receipt-write permission.
Check for both absent receipts and receipts for an uncommitted request.
A post-action live-state/epoch check improves evidence but is still not a
transaction with the original update. The gpol webhook's failure policy is
Ignore: A `../v1beta1/generating_policy.go:67-69`.

INFERRED stronger composition: generate an ActionRequest/intent first and
allow the mutator to act only after reading it. This adds a real gate with
existing CRDs, but still requires explicit phases, retries, validation of
ownership, and completion handling. It is not supplied by a receipt writer
triggered after the suspend update. No claim that a policy pack is impossible.

## Gap 4: externally detecting the status divergence

Verdict: CLOSABLE for a persistent disagreement alert. Action attribution,
automatic repair and a guaranteed outcome protocol are separate requirements.

VERIFIED: the exporter exposes both workload status and component Pod counts
by Pod phase. The Job catalog maps stored Suspended=True to Suspended=1;
Running requires active>0 and ready>0. Pod counts come from the Pod records,
not the Job's active field. E `docs/catalog/batch-job-v1.yaml:31-34,47-50`;
`exporter/pkg/collector/collector.go:51-59,101-112,139-168`;
`exporter/pkg/collector/names.go:15-20,34-45`.

INFERRED alert recipe for this single-cluster lab:

```promql
(karta_workload_status{workload_group="batch",workload_kind="Job",phase="Suspended"} == 1)
and on (namespace, workload_group, workload_kind, workload)
(
  sum by (namespace, workload_group, workload_kind, workload) (
    karta_workload_component_pods{workload_group="batch",workload_kind="Job",phase="Running"}
  ) > 0
)
```

Use an alert `for` duration longer than expected Pod termination and normal
watch/scrape skew; a starting lab setting could be two minutes. That duration
is proposed, not measured. Add scrape/attribution health checks. Include a
cluster identity label on both sides when querying multiple clusters, and
deduplicate replicas before aggregation.

EXECUTED: the two metric families exist in L
`captures/02-metrics-raw.txt:201,300,303`. Those rows are from the ordinary
running state, not a captured wedge alert. Stale Suspended=True and ready=0
are in L `captures/13-trainer-b-wedged.txt:95-106`.
INFERRED: applying the verified mapping to the wedged state makes the alert
plausible; no capture demonstrates it firing. Pod phase Running is not
readiness. Normal suspension can briefly overlap Running/terminating Pods.
The current exporter does not expose spec.suspend directly, so this detects
disagreement, not proof that a particular resume failed. Neither metric
family carries workload UID; same-name replacement is another limit.

## Scheduling and the Experiment E asymmetry

VERIFIED: MutatingPolicy has no native per-policy scan timer. The background
binary reads `BACKGROUND_SCAN_INTERVAL` once, defaulting to one hour, and
passes a shared interval to the policy controller. forceReconciliation uses
one ticker. Eligible mpols with no target expression are requeued together.
K `cmd/background-controller/main.go:191-199`;
`pkg/policy/policy_controller.go:645-655,699-705`.
This does not mean all Kyverno policy types lack schedules: DeletingPolicy
has a cron schedule. A `deleting_policy.go:74-76`.

INFERRED correction to the broad assertion: per-rule timing is not impossible
without changing Kyverno code. Options are:

- Author a time gate such as `time.now() >= timestamp(savedDeadline)` and
  store an action slot/last-action marker. Evaluations still share the global
  scan, and the stale-write caveats remain. VERIFIED library:
  K `pkg/cel/policies/mpol/compiler/compiler.go:240-242`.
- Use a Kubernetes CronJob/ops scheduler to update a per-policy
  `spec.variables` schedule token. Require that token to differ from the
  target's last-processed token, then stamp it with the action. Spec changes
  enqueue policy work independently of the tick. Metadata-only policy
  annotation changes do not trigger this mpol path when spec is unchanged.
  K `pkg/policy/policy_controller.go:302-319,599-603`.

These are policy/ops scheduling recipes, not a per-policy built-in timer.
Periodic checks still run unless separately designed out; a time/token gate
limits actions, not scan cost. The global interval is not a universal latency
floor because policy events and eligible admission triggers also create work.
EXECUTED policy-event first observation: L `captures/09-ab-test.txt:1-3`.

VERIFIED and EXECUTED: putting namespace/label/action predicates in
`targetMatchConditions` fixes the executed background scoping mistake.
Resource rules and objectSelector remain useful target restrictions.
Admission `matchConditions` alone never restricted that target set.
K `pkg/background/mpol/processor.go:111-145`;
`pkg/cel/policies/mpol/compiler/policy.go:201-219`;
R `03-experiments-executed.md:136-167`.
INFERRED authoring rule: targetMatchConditions is sufficient for the
background-only action gate, not for every plane. If admission or reporting
is enabled, deliberately author its matchConditions too. An expression-based
target selector is a separate mode and is excluded from the periodic requeue
above; do not substitute it casually in this test.

## Per-gap verdicts

| Gap | Verdict without Kyverno code changes | Concrete closure and remaining limit |
| --- | --- | --- |
| 1. Silent acting-path CEL errors | PARTIALLY | VERIFIED counter + canary report path; EXECUTED offline canary error/boolean checks; live wiring INFERRED. No flag restores lost per-target acting diagnostics or failed UR status. |
| 2. Resume trap / allowance memory | PARTIALLY | VERIFIED annotation/patch primitives; INFERRED armed-only one-action state machine, admission transition and deny guard. Original A/B predicates race; no supplied race-safe epoch protocol. |
| 3. Durable action receipts | PARTIALLY | VERIFIED opt-in gpol trigger and unsynchronized ConfigMap persistence. INFERRED retained-record recipe. Async writer can miss or misattribute an action; it does not persist intent before permitting it. |
| 4. Outcome blindness / wedge | CLOSABLE | VERIFIED status + Pod-phase metric surface; INFERRED external disagreement alert, not yet executed. Does not prove actor, readiness or controller convergence. |
| Native per-policy mutation timer | NOT | VERIFIED shared timer only. INFERRED time/token gates and external scheduling remain no-code alternatives; blanket impossibility is too strong. |
| Experiment E target scoping | CLOSABLE | EXECUTED targetMatchConditions correction; VERIFIED resource rules/objectSelector also constrain targets. Admission/reporting gates remain separate. |
