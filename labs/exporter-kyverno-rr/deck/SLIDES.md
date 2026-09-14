<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# slide spec for the exporter + kyverno + runtime rules deck

status: v16, round 15 applied; 40 authored slides, 38 main plus two references.
voice: lowercase titles, plain words, short lines.
every number and output on a slide must trace to a capture or a file in
`exporter-lab/`. tiers: EXECUTED (capture), VERIFIED (source read),
INFERRED (labeled on the slide when shown).
source explanations and illustrative examples must not look like executed
tests. show their tier beside the claim. replay timing is schematic unless
the capture records it. replace beats in place; no scrolling slides.
deck engine: local skeleton-head.html, skeleton-tail.html and
demo-component.js (dark theme, editor + terminal windows, state chips,
per-slide fx via data-fxmode, one interactive demo slide).

per slide: title / one idea / optional inset / up to four projected beats /
visible scope / one visual /
data source / fx mode / non-projected reference notes.

Findings labels for the Kyverno and RR acts (including s17b and s30b):
each `tiers` field specifies placement, not a slide-wide endorsement. Use the existing spans:
`<span class="tier exec">EXECUTED</span>` for saved output;
`<span class="tier ver">VERIFIED</span>` for source or fixture reads;
`<span class="tier inf">INFERRED</span>` for attribution or extrapolation.
Use `.tier.part` with OPERATOR-REPORTED, NOT TESTED, or NOT INSPECTED where
that is the actual evidence limit. An operator account is not a captured
execution. A caption names the capture or source and its scope. Timestamp
arithmetic can carry EXECUTED with "computed from captured timestamps".
Reveal each chip with its claim and retain it in held, print, mobile, and
reduced-motion views. Do not make color the only distinction. Keep chips
outside verbatim terminal text; corrected output is an "edited excerpt".

### publication contract

- Runtime Rules is the kep-0003 proposal. rr-poc is the lab prototype shown
  here. Neither the proposal nor the lab is a released runtime controller.
  Keep proposal, prototype and observed behavior distinct.
- Use the fork, upstream Kyverno/Kubernetes and lab files for evidence
  links. Source key F means a path from the fork repository root. Resolve
  links at publication; do not publish local machine paths or tool aliases.
- Titles stay lowercase. Preserve case in API names and verbatim evidence.
  Required legal comments are file metadata, not projected slide content.

### terminology contract

One glossary for authored text, diagrams, storyboards and static fallbacks.
This is builder reference, not another slide. Expand GCE and UR at their
existing first-use insets, then use the short forms. Preserve raw excerpts,
API fields, source paths, metric labels, object names and DOM keys exactly.
Lowercase titles do not change the case of kind names in slide bodies.

| Term | Use in this deck |
| --- | --- |
| karta description | A workload description stored in a Karta custom resource or catalog manifest. Later prose may shorten it to description. Karta is the exact resource kind; CR means custom resource. |
| statusMappings | Exact field inside statusDefinition. It maps status to normalized names. Do not rename either API field. |
| normalized workload phase | Initializing, Running, Completed, Failed, Degraded, Suspended, Suspending, Resuming, Undefined. These metric phase labels differ from lowercase statusMappings keys and native Job condition type Complete. |
| suspend handle | Plain name for the description's suspendDefinition. Its suspendActions and resumeActions are field/value assignments. The Python prototype interprets a narrower subset; it does not call the Karta Go executor. |
| allowance epoch | A numbered part of rule/workload history; the prototype field is epoch. The proposal resets a condition window on resume. The tested prototype advances its epoch but reArmOnResume=false grants no new action. Neither a metric range nor this counter proves a fresh window. |
| receipt | An RR record stored in a ConfigMap. Its phase is a decision/action-record label, not a workload status or a Kyverno UR state. It is mutable in this prototype. |
| GlobalContextEntry (GCE) | The Kyverno resource configuring cached data. In this lab its controller polls Prometheus. It is not a CEL function. |
| globalContext.Get | The CEL function reading that cache by entry and projection name. It does not evaluate the projection string as a query. Use the qualified name, not bare Get. |
| UpdateRequest (UR) | A persisted Kyverno background work item. Pending, Failed, Completed and Skip are UR states. DELETED is a watch event, not another state or an RR receipt phase. |
| mutate-existing | Kyverno behavior that edits existing objects. The shown resource is a MutatingPolicy; its field is evaluation.mutateExisting.enabled. Reporting is controlled separately. |
| background scan tick | The periodic action-controller trigger implemented by forceReconciliation. It enqueues eligible policies; policy workers submit UR work. Policy creation/spec changes also enqueue work. A tick is not an observed action. |
| reporting scan | Separate report evaluation, with its own scheduler. It is not the action controller's background scan tick. GCE refresh and Prometheus scrape are separate clocks too. |
| workload kinds | Use Job, CronJob, Deployment, JobSet, StatefulSet, ReplicaSet, Pod and LeaderWorkerSet when naming a kind. Keep lowercase resource names in commands and literal labels such as component=job unchanged. |
| evidence tier | EXECUTED, VERIFIED or INFERRED describes evidence. In particular, EXECUTED is not the receipt phase Executed. Observe and Enforce are runner modes, not receipt phases. |

Receipt phase spellings below come from rr-poc/rr_poc.py. Meanings are
VERIFIED source readings, not a claim that every branch was executed.
Use the exact phase label on a phase chip; plain explanations sit beside it.
No renamed or shortened phase label is needed in the animation JSON.

| Receipt phase | Meaning in this runner |
| --- | --- |
| WouldAct | Observe decision; consumes neither allowance nor action budget. |
| Intended | Pre-action intent record; a separate persisted pendingOp also gates the patch. |
| Executed | Patch command returned success; does not attest controller convergence. |
| ResumeDetected | Stored weSuspended followed by an unsuspended observation; actor unknown. |
| EscalatedNeedsHuman | Eligible after detected resume with re-arming disabled; record and log, no notification service. |
| SkippedNoHandle | No suspendDefinition assignment that this translator can interpret. |
| SkippedNotReady | Patch-permission preflight returned false; not a general readiness assessment. |
| UnknownOutcome | Patch timed out, or startup recovery could not confirm a listed suspended target. |
| FailedPrecondition | Failed patch matched the runner's precondition-error text checks. |
| ExecutedVerifiedAfterRestart | Startup recovery found the listed target suspended. Does not establish who patched it or controller convergence. |
| FailedVisible | Other reported patch failure. |
| AbortedStateUnpersisted | Intended was saved, but the pendingOp state save failed. |
| FactsUnavailable | A handled facts-query failure; an uncaught JSON parse failure bypasses this record. |
| TargetsUnavailable | A workload-list error. |
| SkippedBudget | Per-loop successful-patch budget reached. |

Terminology sources (VERIFIED): F keps/0003-runtime-rules/README.md:94-106,200-210;
kep-0004.md:75-106,118-143; src/pkg/api/runai/v1alpha1/types.go:19-47;
src/pkg/api/runai/v1alpha1/structure.go:41-77,268-280;
src/docs/catalog/batch-job-v1.yaml:26-57;
rr-poc/rr_poc.py:138-161,187-194,288-319,372-461;
K pkg/cel/libs/context.go:115-124; K api/kyverno/v2/updaterequest_types.go:26-33,53-75,167-182;
K pkg/policy/policy_controller.go:645-655,699-705;
manifests/05c-kyverno-metric-suspend-gce.yaml:15-27.
Source key K means the Kyverno v1.19.1 tag used throughout this deck.

### narrative contract

- Slide IDs are stable source handles, not the playback order. Follow file
  order within each act. There are 38 main slides and two reference views.
- Main order: act 0 s1-s2; act 1 s3-s11b with companion slides;
  act 2 s12, s19, s14, s13, s15, s16, s17, s17b, s18, s18a;
  act 3 s21, s22, s20, s23; act 4 s24-s30, s30b; act 5 s32.
- Each act starts with a quiet act label on its first slide, not a divider
  slide. Each `handoff` is the outgoing slide's final short line. Replace
  its working area if needed; do not append another screen of recap text.
- The hook poses the repeat-action problem. The exporter act builds its
  fact source. The Kyverno act establishes that the condition works. The
  platform detour separates a broken resume from a policy acting again.
  The RR act tests a different action contract. The ending chooses a build.
- s20 is a once-only Kyverno lane. s27 owns the only interactive replay.
  It enters with held results; the button replays both captured sequences
  on one relative scale. Each run starts at its own resume. No speed race.
- s32 is the only terminal payoff. s31 and s33 are references outside the
  main next/previous, autoplay and main-print sequence. Open them by an
  evidence link in a separate reference tab; return to the held main tab.
  Do not build a summary
  slide or add another closing animation after s32.

### projector economy contract

- Target 16:9 with no scrolling. Each slide has one short title, at most
  four numbered beats, one visual, and its evidence/scope caption.
- Beats are the reading order, not a duplicate bullet column. If a beat
  labels a node, cell or timeline marker, render it there once. One visual
  means one diagram, table, evidence card or timeline, not several windows.
- Keep projected beats under 60 words total. An optional definition uses
  one small inset, at most 25 words; scope uses at most 26 words. The inset
  can introduce the first beat, but never adds another explanation screen.
- The one idea, tier-placement instructions, source paths, code-source
  directives and reference notes are authoring metadata, not extra prose
  to project. Scope is mandatory visible text, including static/print.
- Reference notes retain details for inspection. They must not return as
  a rotating wall of text, extra source-caption rows or hidden slide acts.
  The full source remains linked; a material limit stays beside its claim.
- A handoff replaces the final beat at the transition; it does not add a
  fifth line. Animations reveal the same bounded visual and its final state.
  Fit failures require cutting copy or graphics, not shrinking or scrolling.

### real-data embedding contract

Authoring metadata, not extra slide beats. Each embed field pins inclusive,
one-based source lines. Count logical lines, including the last line when
there is no trailing newline. Paths are relative to exporter-lab unless
marked as source keys K/U/F. The captures are authoritative; data-excerpts.md
is a partial convenience copy, not a complete evidence set.

- Copy pinned raw output verbatim. Preserve names, IDs, timestamps, spacing,
  case and JSON value types. HTML escaping and soft wrapping may change
  presentation, never the text. Separate discontiguous slices visibly;
  never join them into an invented contiguous transcript.
- embed pins the canonical excerpt for the existing visual/evidence link;
  it does not add a terminal panel or more beats. A field/value card, short
  chip or arithmetic label is PARSED/COMPUTED FROM CAPTURE, not a verbatim
  command response. Retain its exact raw source in the evidence detail.
- Keep corrections OUTSIDE raw output. Inaccurate recorder commentary is
  still inaccurate when quoted exactly. Omit the unsafe headers noted
  below or label them recorder commentary with the correction alongside.
  Do not silently rewrite a saved line and call it verbatim.
- Captured timing differences carry EXECUTED plus computed-from-timestamps.
  Source/configuration values and synthetic examples carry VERIFIED with
  their source/example label. Interpretation/extrapolation carries INFERRED.
  Operator setup or timing accounts without a saved measurement carry
  OPERATOR-REPORTED. A recorded banner is not a server-version query.
- Capture 07b contains ANSI control sequences. For that excerpt only, remove
  ANSI styling, retain every printable character, and caption ANSI-STYLING
  REMOVED. The raw file remains linked. No other normalization is allowed.
- A configuration or code excerpt is SOURCE/VERIFIED, never console output.
  Do not prepend reconstructed kubectl commands to captured responses.
  broken/corrected, PARSED and tier captions sit outside code/output text.
- fx-steps offsets, safeCount intermediate values, lane duration/scale, CSS
  sizes, source line numbers and slide counts are presentation metadata.
  They are not lab measurements. Keep replay compression/axis settings
  labeled when visible; do not promote intermediate counts to results.
- data-excerpts.md has only two UR histories, three non-Running trainer
  status rows, and the opening brace of the receipt body. Use the full
  capture ranges below for four URs, real scrape values and complete JSON.

### engine feasibility contract

Authoring instructions, not projected content. VERIFIED against
skeleton-tail.html:54-157,159-199; demo-component.js:10-76; and
skeleton-head.html:118-151,170-190. The code below is proposed builder
work. This review changes the spec, not the engine.

- Copy each fx line's data-fxmode value exactly. Modes are space-separated
  names, not prose or plus signs. Blank means static. Never give an element
  to two text-mutating effects at once.
- Offsets begin at the effect callback. show() queues runFx about 80ms
  after navigation; this delay is not a lab latency. CSS pops take another
  500ms after each reveal starts (skeleton-head.html:120-121).
- popPills selects .fx-pill: starts at 200ms, then 110ms per element.
  popTimelines selects .timeline .chip and .arr: starts at 220ms, then
  140ms per element. DOM order is reveal order. Both run once per entry.
  Keep titles, scope and necessary definitions outside their hidden groups.
  A claim and its evidence chip share the same reveal group. No data-glitch;
  that CSS effect continues pulsing after the reveal.
- storySteps is the shared customFx for fixed diagrams and changing text.
  Its per-slide inert JSON is the storyboard: [ms, revealKeys, textValues].
  Later fields may be omitted. All keys resolve to unique local data-key
  elements. data-reveal marks result groups to hide initially; frame keys
  reveal them. Text slots have data-final and plain text only. Literal
  angle brackets are escaped in authored HTML and assigned by textContent.
- safeCount is the custom replacement for countUp on s1/s7. Use a plain
  text span with data-count and data-final; units are sibling text. It
  keeps all work on fxT so navigation can cancel it. Native countUp reads
  the first child node, starts after 350ms and runs for about 1100ms, but
  its requestAnimationFrame callbacks are not tracked by fxClear. The
  native data-suffix value is read but never appended. Do not use it here.
- Native dotTravel repeats, uses only the first .pipe, and needs BOTH
  section data-errpath and stage data-errstage to turn the dot red. It
  follows DOM order, not branches or wait/retry state. No slide uses it.
  Fixed node highlights are the accepted cheaper fallback for s5/s8/s15.
- No slide requires typeTerm or typeInto. typeTerm needs buildTerms first;
  it types tl-cmd at 14ms/character and adds a prompt. Most unrecognized
  capture rows are classified as commands. If added later, explicitly
  classify captured output as tl-out/tl-err before invoking it. typeInto
  replaces descendant markup until completion; never apply it to a code,
  tier or count container. Reveal the complete excerpt instead.
- Register custom handlers after customFx is initialized and after the demo
  component, before the DOMContentLoaded initDeck registration. The current
  assembler inserts the demo there already (assemble.py:18-19). Putting
  registrations in slide HTML before the tail would be too early.
- s31/s33 are separate reference pages opened by ordinary links in a new
  tab. Mark those links data-reference so the adapter holds the caller
  before the new tab opens. Do not emit references as section.slide in the main page: initDeck includes
  every such element. A reference modal with navigation/print restoration
  is extra work and is not required. The main deck still has 38 slides.

#### shared custom effects

Each new customFx sketch is under 15 lines. These are the only two new
handlers. The lifecycle support below is required once for all effects.

```js
customFx.storySteps = function(sec) {
  const key = k => sec.querySelector('[data-key="' + k + '"]');
  sec.querySelectorAll('[data-reveal]').forEach(n => {
    n.classList.add('story-hide'); n.classList.remove('lit');
  });
  const steps = JSON.parse(sec.querySelector('script.fx-steps').textContent);
  playTimeline(steps.map(([ms, reveal = [], values = {}]) => [ms, () => {
    reveal.forEach(k => { key(k).classList.remove('story-hide'); key(k).classList.add('lit'); });
    Object.entries(values).forEach(([k, v]) => { key(k).textContent = v; });
  }]));
};
```

```js
customFx.safeCount = function(sec) {
  sec.querySelectorAll('[data-count]').forEach(el => {
    const raw = el.dataset.count, target = Number(raw);
    const decimals = (raw.split('.')[1] || '').length;
    const delay = Number(el.dataset.delay || 350);
    el.dataset.final = raw; el.textContent = '0';
    playTimeline(Array.from({length: 23}, (_, i) => [delay + i * 50, () => {
      el.textContent = (target * (i / 22) ** 2).toFixed(decimals);
    }]));
  });
};
```

#### shared lifecycle support

Stock fxClear cancels fxT timers and the active demo, but does not restore
custom text/visibility or cancel native countUp frames. There is no stock
beforeprint/reduced-motion JavaScript hook. Add this small adapter before
initDeck is registered. All selected custom work uses fxT; no extra RAF,
interval, promise callback or activation token is needed for these sketches.

```js
let wowPrinting = false;
const wowMedia = matchMedia('(max-width:900px), (prefers-reduced-motion: reduce)');
const wowStatic = () => wowPrinting || wowMedia.matches;
function wowHold(sec) {
  sec.querySelectorAll('.story-hide,.fx-hide,.fx-in,.lit,.glitch').forEach(n =>
    n.classList.remove('story-hide','fx-hide','fx-in','lit','glitch'));
  sec.querySelectorAll('[data-final]').forEach(n => { n.textContent = n.dataset.final; });
  sec.querySelectorAll('.lane .ev').forEach(n => n.classList.add('on'));
  sec.querySelectorAll('button.playbtn').forEach(n => { n.disabled = wowStatic(); });
}
```

```js
const wowBaseClear = fxClear, wowBaseRun = runFx, wowBaseInit = initDeck;
fxClear = function() {
  wowBaseClear();
  document.documentElement.classList.toggle('fx-static', wowStatic());
  document.querySelectorAll('section.slide').forEach(wowHold);
};
runFx = sec => wowStatic() ? wowHold(sec) : wowBaseRun(sec);
initDeck = function() {
  document.querySelectorAll('[data-fxmode~="dualLane"]').forEach(sec => customFx.dualLane(sec));
  wowBaseInit();
};
```

```js
window.addEventListener('beforeprint', () => { wowPrinting = true; fxClear(); });
window.addEventListener('afterprint', () => { wowPrinting = false; fxClear(); });
wowMedia.addEventListener('change', () => fxClear());
document.addEventListener('click', e => {
  if (e.target.closest('a[data-reference]')) fxClear();
});
```

In the existing navigation keydown handler, add this guard before testing
arrow/space keys (skeleton-tail.html:178-183). It lets keyboard interaction
with the replay button stay on the slide:

```js
if (e.target.closest('button,input,select,textarea,a')) return;
```

The adapter prebuilds the one dualLane even if it was never visited, so
printing sees its chips. Leaving, printing or entering static mode cancels
timers and shows held data. Returning from print/static mode does not
replay; a fresh slide entry or the demo button can do so. Replay is disabled
in static mode. Use real buttons, not clickable divs.

Required style integration (new builder CSS, not present in the skeleton):

```css
.story-hide { opacity:0; visibility:hidden; }
[data-reveal].lit { outline:1px solid var(--acc); outline-offset:2px; }
.motion-static { display:none; }
.fx-static .motion-live { display:none; }
.fx-static .motion-static { display:block; }
.fx-static *, .fx-static *::before, .fx-static *::after { animation:none!important; transition:none!important; }
@media print, (max-width:900px), (prefers-reduced-motion:reduce) {
  .motion-live { display:none!important; }
  .motion-static { display:block!important; }
  *, *::before, *::after { animation:none!important; transition:none!important; }
}
@media (max-width:900px) { section.slide { overflow:hidden; } }
```

Use one compact held layout per slide. If the desktop visual fits unchanged,
it need not have motion-live/static wrappers. Otherwise author one static
replacement from the SAME data; only one is visible. s27 needs the compact
held row/table fallback below. No stacked frames or clipped scrolling views.
The stock mobile rule enables vertical scrolling and must be overridden;
fit still needs rendering. Print CSS alone does not stop active JS timers.

### newcomer inset contract

- Define unfamiliar terms before their first code/example use. The short
  inset is visible, never tooltip-only. A term already taught uses its
  plain label later; do not replay its definition on every slide.
- Keep one inset per slide. Short diagram labels can translate identifiers
  without becoming another card. Preserve necessary definitions in the
  static/mobile/print layout alongside the same bounded visual.
- Definitions of deck terms say "term used here"; source explanations use
  VERIFIED. They do not inherit EXECUTED from a nearby captured value.
- Name the object/policy and recorded run beside evidence. Lab1 and lab2
  are the integration clusters; earlier experiments retain separate labels.
  No invented run metadata or merged histories of different Jobs.

---

## act 0 - the hook

### s1. the rule ran again
- one idea: the rule suspended the resumed Job once more.
- beats:

  1. A policy engine suspended a demo Job.
  2. A human resumed it.
  3. 123.9 seconds later, the engine suspended it again.
- scope: EXECUTED: one recorded re-suspension, lab2.
- visual: One resume-to-suspend strip. Count its recorded endpoint once,
  350-1450ms; hold. This is a display count, not an elapsed live clock.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: captures/30-lab2-kyverno-phase.txt.
- embed: captures/30-lab2-kyverno-phase.txt:5-8.
- number audit: EXECUTED arithmetic: 18:23:14.635 - 18:21:10.688 = 123.947s; 123.9s is
  rounded to one decimal. Keep only one observed repeat. The number
  animation is presentation, not another measurement. Line 6's printed 0.0s
  is not a precise latency; its two markers differ by 95ms.
- fx: `data-fxmode="safeCount"`; shared customFx.safeCount; once, then hold.
- fx DOM: One plain-text span data-count="123.9" data-final="123.9". Put seconds in
  a sibling. No data-typeinto on this span or its parent.

- handoff: trace the fact that made this Job eligible again.
- reference notes: Retained source explanation: "a policy engine suspended a demo Job. a human
  resumed it. 123.9 seconds later the engine suspended it again." hold on
  that concrete moment. save the RR outcome for s27.

### s2. what we built and ran
- one idea: the lab map - what exists, what talks to what.
- inset cards: karta description: a recipe for reading a workload kind, its parts, status
  and supported action fields.
- beats:

  1. The exporter exposes workload identity and status.
  2. Prometheus stores samples for both consumers.
  3. Runtime Rules (RR, kep-0003) is a proposal; its lab prototype acts
      separately from Kyverno.
  4. Reported setup: lab1 Kubernetes v1.34.0; lab2 v1.34.3; both Kyverno v1.19.1.
- scope: Sleep Jobs; no GPU benchmark. RR reads local files, not a RuntimeRule CRD.
  Versions are reported setup, not saved server-query output.
- visual: One topology: exporter -> Prometheus -> Kyverno / RR. Use the beats as node
  captions, not a second text column. Label lab1=kind-kyverno-lab and
  lab2=kind-kyverno-lab2 in the diagram key. Namespace/service names stay in
  the source detail. Label the RR node "RR lab prototype".
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: EXECUTED on captured startup/CRD fields. OPERATOR-REPORTED beside the
  version row. Architecture from source remains VERIFIED; no captured
  server-version attestation is shown.
- data: F keps/0003-runtime-rules/README.md:6-11,18-25;
  INTEGRATION-LOG.md sections 2-3; captures/01-crd.txt;
  manifests/03-workloads.yaml; manifests/06-rr-rbac.yaml;
  rr-poc/rr_poc.py:87-97,333-344.
  src/pkg/api/runai/v1alpha1/types.go:19-55;
  src/pkg/api/runai/v1alpha1/structure.go:20-58;
  src/exporter/pkg/registry/registry.go:57-79; INTEGRATION-LOG.md:6-9,64-72.
- embed: captures/03-exporter-watchers.txt:1-1; captures/01-crd.txt:2-2. These
  substantiate exporter configuration/start and CRD creation, not cluster
  versions.
- number audit: Versions are OPERATOR-REPORTED setup from INTEGRATION-LOG.md:6-9,64-72. The
  capture directory has no saved server-version/Helm-image attestation
  command. Capture 30:1 also states v1.34.3 in a recorder header. Keep the
  setup tier beside the version row. Do not manufacture kubectl version
  output.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Wrap the four topology nodes in .fx-pill, in reading order. Keep the
  cluster key and scope static.

- handoff: start with how the exporter turns workload objects into metric series.
- reference notes: Retained source explanation: karta exporter (from the fork branch) ->
  prometheus -> two consumers: kyverno mutating policies and a runtime-rules
  prototype. lab1 = kind-kyverno-lab, Kubernetes v1.34.0; lab2 = kind-kyverno-lab2, v1.34.3, used for the clean comparison after the platform
  defect. both run Kyverno v1.19.1. kind is the local Kubernetes cluster
  tool. captured runs are marked EXECUTED; source explanations are VERIFIED;
  proposed extensions are INFERRED. The kep-0003 document is provisional;
  the prototype does not establish completion of that proposal.
  setup detail: catalog mode still needs
  the Karta CRD because the exporter always watches Karta objects, even when
  using bundled descriptions. RR reads local catalog YAML and a local rule
  file; it does not use a RuntimeRule CRD. action permissions are separate.
  these are sleep Jobs, not a gpu training or utilization benchmark.

---

## act 1 - inside the exporter

### exporter animation contract

- All motion is once per entry. The per-slide fx-steps JSON is authoritative.
  Offsets are presentation milliseconds, not controller timings.
- Use fixed diagrams and highlighted nodes instead of stock dotTravel.
  This avoids its loop and the extra geometry needed for branch/park/retry.
- Each data-key is local and unique. data-reveal marks initially hidden
  result groups. A text slot has data-final with its held value, and its
  authored text starts with that value. Tiers live beside text slots.
- Print/mobile/reduced mode uses the compact held visual. The shared
  lifecycle adapter in the engine feasibility contract is required builder
  work, not stock behavior.

### s3. connecting pods to workloads
- one idea: a pod-level signal needs a workload and component mapping.
- inset cards: Workload: the described application object containing those parts.
- beats:

  1. Pod metrics need a workload identity.
  2. A component names a type of part; an instance names a particular
     part.
  3. Example: JobSet -> replicatedjob -> decode.
- scope: Illustrative GPU-metric join; not executed in this lab.
- visual: One labeled pod-to-JobSet mapping with component and instance slots. Remove
  the wall of kind names and the second example.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: src/docs/catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml:50-64;
  src/exporter/test/rules/tests.yaml:51-56 (fixture, not a live capture).
- embed: none: source/fixture illustration. If quoting code, use
  src/docs/catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml:50-64 or
  src/exporter/test/rules/tests.yaml:54-55 with VERIFIED source/fixture
  labels.
- number audit: The fixture's decode, workload llm, UID p2 and values 1x10 are not this
  lab's telemetry. No GPU metric or JobSet join was executed in the saved
  captures. The actual lab skipped the JobSet watcher
  (captures/03-exporter-watchers.txt:6-6).
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .fx-pill for Pod, workload and part-label groups. Their positions and
  connecting lines stay fixed.

- reference notes: Retained source explanation: a gpu signal labeled by pod does not itself
  supply Karta's workload and component mapping. answering "utilization of
  the decode part of this JobSet" needs that mapping. in the JobSet catalog,
  replicatedjob is the component; decode is a component instance.
  illustrative motivation: the lab did not collect GPU metrics or execute
  this utilization join.

### s4. the exporter answer: identity as a metric
- one idea: workload identity can be joined onto external telemetry.
- inset cards: Join: match series using shared labels, then attach workload labels.
- beats:

  1. karta_pod_workload_info adds the workload and part labels.
  2. Each attributed pod emits a join series with value 1.
  3. Prometheus can join that identity onto another measurement.
  4. The exporter does not fetch the external telemetry.
- scope: Illustrative join. uid identifies the Pod; workload series have no workload-UID label.
- visual: One join diagram. Show a short label subset; merge a pod metric with the
  identity row. Do not add the full metric-family inventory or label-schema
  table.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: src/exporter/pkg/collector/collector.go:13,45-62,87-99;
  src/docs/Metrics Exporter.md:105-111; captures/02-metrics-raw.txt:112-126.
- embed: captures/02-metrics-raw.txt:125-125 (the complete trainer-a join series).
- number audit: Value 1, component=job, empty instance/replica, Pod name and UID are literal
  captured fields. A shortened label set is PARSED/SELECTED, not a verbatim
  Prometheus line. Do not replace this Job row with the illustrative
  JobSet/decode identity from s3.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .fx-pill for pod metric, identity row and joined result. No label
  morph or traveling series.

- reference notes: Retained source explanation: `karta_pod_workload_info{pod, workload,
  workload_kind, component, component_instance} 1` (label subset) - one
  series per attributed pod, value always 1. uid is the POD UID; workload
  series have no workload-UID label. namespace, workload, workload_kind and
  workload_group identify a workload in the metric surface. workload_version
  and karta appear on karta_workload_info. join external telemetry in
  Prometheus; this collector does not fetch it. the GPU-metric join is
  illustrative.

### s5. events build records and scrapes render them
- one idea: workload events and pod events feed the same metric store.
- inset cards: Informer: object-change cache. Root: described workload.
  jq: expressions over object JSON. Attribution: assigning workload and
  part labels.
- beats:

  1. The registry selects descriptions for watchers.
  2. Workload events build status and component records.
  3. Pod events add workload and part attribution.
  4. A scrape reads the stored snapshot; it runs no jq or object fetch.
- scope: VERIFIED source mechanism. Watch notifications are distinct from stored
  Kubernetes Event objects.
- visual: One event/store/scrape diagram. The workload and pod branches join the
  store; the final branch is a separate scrape. Node highlights replace
  moving dots.
- playback: Once per entry; hold after 3800ms. The fx-steps JSON is authoritative.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: src/exporter/pkg/controller/events.go:124-139,314-351;
  src/exporter/pkg/store/store.go:31-70,81-111,174-188;
  src/exporter/pkg/collector/collector.go:84-126,139-169.
  src/exporter/pkg/controller/controller.go:139-158.
- embed: none: VERIFIED source diagram; no command/output transcript.
- number audit: Stored record names and the pipeline are source concepts. Storyboard ms
  offsets are authored presentation settings. No measured event-processing
  or scrape latency is asserted.
- fx: `data-fxmode="storySteps"`; shared customFx.storySteps; once per entry.
- fx DOM: Reveal keys: registry, workload, owners, selectors, pod-record, scrape.
  Pre-draw branches inside one .pipe; reveal/highlight nodes without a
  moving dot.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,["registry"]],
    [1000,["workload"]],
    [1700,["owners"]],
    [2400,["selectors"]],
    [2750,["pod-record"]],
    [3200,["scrape"]],
    [3800,[]]
  ]
  </script>
  ```

- reference notes: Retained source explanation: registry (which karta descriptions serve which kinds) ->
  watchers (one informer per described kind) -> owner index (who owns whom)
  -> attributor (pod -> workload + component) -> store + collector.
  VERIFIED: workload callbacks rebuild status, generation and component
  instance/replica records keyed by UID. pod records store attribution and
  phase. records are replaced under a lock; a scrape copies a store snapshot
  and counts pods, with no jq or object fetch in the collector.

### s6. which description wins
- one idea: choose one karta description for each workload kind.
- inset cards: CR: a saved Karta custom resource. Catalog: bundled fallback descriptions.
- beats:

  1. Choose one description per root group+kind.
  2. Usable cluster CRs beat the catalog; oldest CR wins, then name
     ascending.
  3. Catalog fallback uses descending names, not semantic versions.
  4. Unchosen valid CRs count as shadowed; overridden catalog entries do
     not.
- scope: VERIFIED selection rules. Bare Pod needs an explicit CR; catalog mode skips
  it.
- visual: One fixed selected-description slot. Replace its text for labeled source
  cases; do not animate card sorting or physically remove candidates.
- playback: Once per entry; hold after 4500ms. The fx-steps JSON is authoritative.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: src/exporter/pkg/registry/registry.go:59-107,136-202;
  src/exporter/pkg/collector/collector.go:193-197.
- embed: none: VERIFIED selection rules; no captured CR-selection contest.
- number audit: Candidate names/ages and the equal-age tie are labeled source examples. Do
  not print fabricated creationTimestamp values or shadowed totals as
  captured output.
- fx: `data-fxmode="storySteps"`; shared customFx.storySteps; once per entry.
- fx DOM: Reveal keys: invalid, newer, older, tie, fallback. Text key selected has
  data-final="catalog". Candidate examples are separate labeled cases, not
  one fabricated watch trace.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,[],{"selected":"catalog"}],
    [600,["invalid"]],
    [1200,["newer"],{"selected":"usable newer CR"}],
    [1900,["older"],{"selected":"oldest CR"}],
    [2700,["tie"],{"selected":"first name in equal-age tie"}],
    [3500,["fallback"],{"selected":"catalog"}],
    [4500,[]]
  ]
  </script>
  ```

- reference notes: Retained source explanation: VERIFIED: selection is per root group+kind, not
  version. validated CR candidates sort by oldest creationTimestamp, then
  name ascending; a successfully built CR entry takes precedence over
  catalog. catalog fallback sorts names descending, not semantic API
  versions. invalid CRs count as invalid; unchosen non-invalid CRs count as
  shadowed. an overridden catalog entry does NOT count as shadowed. removing
  the chosen CR lets another CR or catalog entry take over. bare Pod is
  skipped by --use-catalog and needs an explicit description.

### s7. roots need objects and children need owner links
- one idea: full informers for roots, metadata-only for children,   one trimmed pod
  informer.
- inset cards: GVK means API group, version and kind. Root means described workload;
  children supply owner links.
- beats:

  1. Described roots use full objects; child-only kinds use metadata.
  2. Pod and Karta informers are shared, outside the dynamic count.
  3. EXECUTED: 5 dynamic watchers started.
  4. 19 missing kind mappings were logged and skipped.
- scope: One watcher per required GVK; root wins if also needed as child. Counts
  exclude shared informers.
- visual: One watcher map. Reveal the root/child groups and skipped count; count the
  captured totals once, then hold.
- playback: Once per entry; hold after 3800ms. The fx-steps JSON is authoritative.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: captures/03-exporter-watchers.txt:1-25;
  src/exporter/pkg/controller/controller.go:139-158,213-231,237-320;
  src/exporter/pkg/registry/registry.go:229-245.
- embed: captures/03-exporter-watchers.txt:5-5;
  captures/03-exporter-watchers.txt:14-14;
  captures/03-exporter-watchers.txt:16-16;
  captures/03-exporter-watchers.txt:18-18;
  captures/03-exporter-watchers.txt:25-25. Optional skipped example:
  captures/03-exporter-watchers.txt:6-6.
- number audit: EXECUTED, counted from captures/03-exporter-watchers.txt:1-25: 5
  started-watcher lines and 19 skipping-watcher lines. The started rows are
  Deployment, Job, StatefulSet, CronJob (root=true) and ReplicaSet
  (root=false). data-excerpts.md:7-12 contains only 1 started and 5 skipped
  rows; it cannot supply the totals by itself.
- fx: `data-fxmode="storySteps safeCount"`; customFx.storySteps + shared customFx.safeCount; once per entry.
- fx DOM: Reveal keys: shared, roots, child, skipped, totals. In totals, use
  plain-text spans data-count="5" and data-count="19", matching data-final
  values and data-delay="2300". Labels stay outside the spans.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,["shared"]],
    [350,["roots"]],
    [1100,["child"]],
    [1700,["skipped"]],
    [2300,["totals"]],
    [3800,[]]
  ]
  </script>
  ```

- reference notes: Retained source explanation: VERIFIED: requested GVKs are deduplicated
  across chosen karta descriptions. roots use full unstructured objects; child-only GVKs
  use PartialObjectMetadata. a GVK needed as both uses the root watcher.
  children include additionalChildKinds; child Pods use the shared pod
  informer. EXECUTED: 5 dynamic watchers started - Job, CronJob, Deployment
  and StatefulSet roots, plus ReplicaSet as a child. these counts exclude
  the shared Pod and Karta informers. 19 missing GVK mappings were logged
  and skipped; they were not started watchers.

### s7b. the pod cache is a deliberate tradeoff
- one idea: retained fields and update handling both constrain pod selectors.
- inset cards: podSelector: Karta rules for finding a pod's component, instance or replica.
  Cache trimming does not edit the live Pod.
- beats:

  1. Default cache: selected metadata, nodeName and phase.
  2. Label, annotation or owner changes rerun attribution.
  3. Phase-only updates change stored phase; nodeName-only changes do not
     rerun selectors.
  4. --full-pod-cache retains full Pods and reruns attribution on
     resourceVersion changes.
- scope: VERIFIED configurations, not a live flag toggle. Retaining a field does not
  guarantee reevaluation when it changes.
- visual: One trimmed/full cache table. Highlight each field-change route in place.
  No geometry tracking or animated cache rewrite.
- playback: Once per entry; hold after 4200ms. The fx-steps JSON is authoritative.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: src/exporter/cmd/main.go:45;
  src/exporter/pkg/controller/controller.go:183-209;
  src/exporter/pkg/controller/events.go:197-227.
  src/pkg/api/runai/v1alpha1/structure.go:178-205.
- embed: captures/03-exporter-watchers.txt:1-1. Optional source flag declaration:
  src/exporter/cmd/main.go:45-45, labeled VERIFIED.
- number audit: fullPodCache=false is the captured setting. --full-pod-cache is the exact
  flag from source. The full-cache alternative and rerun routes remain
  VERIFIED, not an executed flag toggle.
- fx: `data-fxmode="storySteps"`; shared customFx.storySteps; once per entry.
- fx DOM: Reveal keys: configs, fields, trim-selectors, phase-node, full-selectors.
  Put each in a fixed routing-table cell; text names the branch result.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,["configs"]],
    [650,["fields"]],
    [1500,["trim-selectors"]],
    [2400,["phase-node"]],
    [3400,["full-selectors"]],
    [4200,[]]
  ]
  </script>
  ```

- reference notes: Retained source explanation: VERIFIED: default trim
  keeps name, namespace, UID, resourceVersion, labels, annotations,
  ownerReferences, creationTimestamp, TypeMeta, spec.nodeName and
  status.phase. it drops other pod fields. --full-pod-cache retains the
  full object and re-runs attribution on resourceVersion changes.
  trimmed mode re-runs selectors for label, annotation or owner changes;
  a phase-only update changes the stored phase. selectors using nodeName
  or phase need care: retaining a field does not mean its change alone
  re-runs selectors in trimmed mode. Exact default retained fields:
  name, namespace, UID, resourceVersion, labels, annotations,
  ownerReferences, creationTimestamp, TypeMeta, spec.nodeName and
  status.phase. managedFields and containers are omitted.

### s8. follow owners to the outermost workload
- one idea: pods reach their workload through the owner index, even   out of order.
- inset cards: Parked means attribution waits for owner data; it does not pause the Pod.
- beats:

  1. Follow controller=true owner links by UID.
  2. Select the outermost described workload.
  3. An incomplete chain waits, even after finding an inner candidate.
  4. Owner arrival retries waiting pods; Pods can also be middle owners.
- scope: VERIFIED bounded owner walk. JobSet is the result only after the chain is
  complete.
- visual: One Pod -> Job -> JobSet chain and a fixed wait/result field. Highlight
  missing-owner, arrival and retry stages. No moving pod, parked dot or
  second owner walk.
- playback: Once per entry; hold after 3800ms. The fx-steps JSON is authoritative.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: src/exporter/pkg/owner/index.go:69-83,93-104,129-179;
  src/exporter/pkg/controller/events.go:112-139,162-172,259-311;
  src/exporter/pkg/owner/index_test.go:100-112 (read, not executed this round).
- embed: none: VERIFIED source example; no captured JobSet owner walk.
- number audit: controller=true and the owner/result fields are diagram inputs. The
  missing-owner, arrival and retry stages are not a saved event trace. Do
  not give them wall-clock timestamps.
- fx: `data-fxmode="storySteps"`; shared customFx.storySteps; once per entry.
- fx DOM: Reveal keys: inner, pending, arrival, retry, resolved. Text keys waiting
  and result have final values "wait resolved" and "JobSet". Keep the
  owner chain fixed.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,[],{"waiting":"owner not yet observed","result":"not resolved"}],
    [500,["inner"]],
    [1100,["pending"],{"waiting":"waiting for owner"}],
    [1900,["arrival"],{"waiting":"owner arrived"}],
    [2600,["retry"]],
    [3300,["resolved"],{"waiting":"wait resolved","result":"JobSet"}],
    [3800,[]]
  ]
  </script>
  ```

- reference notes: Retained source explanation: VERIFIED: follow
  controller=true owner references, using UIDs for edges and group+kind
  for root matches. keep the outermost described root: a Pod below Job
  -> JobSet belongs to the JobSet. an incomplete chain parks even if an
  inner root was found; it does not settle early. owner arrival drains
  pending pod keys for retry. the walk is bounded. pods also register
  their own edges: worker Pod -> worker StatefulSet -> leader Pod ->
  leader StatefulSet -> LeaderWorkerSet. Middle-owner source example: worker Pod ->
  worker StatefulSet -> leader Pod -> leader StatefulSet ->
  LeaderWorkerSet. The leader Pod registers its own controller-owner
  edge.

### s8b. selectors identify the parts
- one idea: workload attribution and component attribution are separate steps.
- inset cards: Leaf: a component with no child parts. Replica labels name repeated copies
  or groups, not pod counts.
- beats:

  1. Owners identify the workload; selectors identify its parts.
  2. One leaf selects directly; multiple leaves need type selectors.
  3. JobSet: match the pod label to stored instance IDs -> decode.
  4. The replica label is separate and can be empty.
- scope: VERIFIED JobSet example. Nearest configured description ancestor supplies
  replica selection; this example has none.
- visual: One fixed input/output mapping. Fill component, instance and replica
  slots; no reanimation of the owner chain.
- playback: Once per entry; hold after 3700ms. The fx-steps JSON is authoritative.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: src/pkg/instructions/pod.go:14-37;
  src/exporter/pkg/controller/events.go:314-328;
  src/exporter/pkg/attribute/attributor.go:34-80,116-140;
  src/docs/catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml:50-64.
- embed: none: VERIFIED source example. Optional source excerpt:
  src/docs/catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml:61-64.
- number audit: The decode/prefill IDs and empty replica slot illustrate the configured
  matcher. They are not labels observed on trainer-a, whose captured
  component is job. Keep source-example labeling on the mapping.
- fx: `data-fxmode="storySteps"`; shared customFx.storySteps; once per entry.
- fx DOM: Reveal keys: inputs, leaf, matcher, replica-note. Text keys component,
  instance, replica have final values replicatedjob, decode, and two
  literal quote characters. Pre-draw all connectors.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,["inputs"],{"component":"not resolved","instance":"not resolved","replica":"not resolved"}],
    [600,["leaf"],{"component":"replicatedjob"}],
    [1300,["matcher"]],
    [2100,[],{"instance":"decode"}],
    [2900,["replica-note"],{"replica":"\"\""}],
    [3700,[]]
  ]
  </script>
  ```

- reference notes: Retained source explanation: VERIFIED: after root lookup, select a leaf
  component. a single leaf is selected directly; multiple leaves use
  component-type selectors. instance IDs normally come from the stored
  workload record; pod jq matches an instance against those IDs. for JobSet:
  component=replicatedjob, component_instance comes from the replicatedjob-name pod label. replica is separate, using the nearest replicaSelector on
  the component or its description ancestors. an empty instance/replica
  label can be normal.

### s9. status is computed, not copied
- one idea: statusMappings turn each kind's status shape into one   vocabulary.
- inset cards: statusMappings: tests inside a Karta statusDefinition that translate object
  fields into common status names.
- beats:

  1. Job Running requires active>0 and ready>0; missing counts become
     zero.
  2. Completed and Suspended require condition status="True".
  3. Keep every matching status: Running and Degraded can both be 1.
  4. Available status emits every normalized phase as 0 or 1.
- scope: VERIFIED synthetic input, not a captured lifecycle. Dense phases do not
  guarantee scrape coverage.
- visual: One synthetic Job input and nine phase cells. Only numeric text changes;
  no live lifecycle or chip morph. Keep active=1, parallelism=2, failed=0
  and conditions=[] fixed.
- playback: Once per entry; hold after 3200ms. The fx-steps JSON is authoritative.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: src/docs/catalog/batch-job-v1.yaml:26-50;
  src/pkg/resource/accessor.go:603-624;
  src/exporter/pkg/state/workload.go:37-48;
  src/exporter/pkg/collector/collector.go:101-114; src/exporter/pkg/collector/names.go:67-76.
  src/pkg/api/runai/v1alpha1/structure.go:268-280.
- embed: none: VERIFIED synthetic input. Optional exact matcher excerpts:
  src/docs/catalog/batch-job-v1.yaml:27-34 and
  src/docs/catalog/batch-job-v1.yaml:43-50.
- number audit: Synthetic values: parallelism=2, active=1, ready 0->1, succeeded 0->1,
  failed=0, conditions=[]. These produce Initializing, then Running, then
  Running+Degraded under the cited matchers. The nine phases come from
  src/exporter/pkg/collector/names.go:67-76. Zero/one phase cells are
  VERIFIED source outputs for this invented input, not captured lifecycle
  samples.
- fx: `data-fxmode="storySteps"`; shared customFx.storySteps; once per entry.
- fx DOM: Text keys ready, succeeded, initializing, running, degraded. Final values:
  1, 1, 0, 1, 1. Keep all nine labeled phase chips visible; the other six
  are always 0. Use text, never interpolated boolean counters.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,[],{"ready":"0","succeeded":"0","initializing":"1","running":"0","degraded":"0"}],
    [1400,[],{"ready":"1","initializing":"0","running":"1"}],
    [2400,[],{"succeeded":"1","degraded":"1"}],
    [3200,[]]
  ]
  </script>
  ```

- reference notes: Retained source explanation: VERIFIED: the batch-job mapping requires
  active>0 and ready>0 for Running, treating missing counts as zero.
  Completed and Suspended require matching condition types with
  status="True". the workload callback evaluates mappings and stores ALL
  matches, not a single winner. Running and Degraded can both match this
  catalog. when HasStatus is true, the collector emits every normalized
  phase as 0 or 1. scrape gaps still matter; status absence is a separate
  case (s9b).

### s9b. undefined is not absent
- one idea: no matching status, no statusDefinition, and failed evaluation differ.
- inset cards: HasStatus records whether a status result was obtained. No series is
  different from a series valued zero.
- beats:

  1. No mapping matches: Undefined=1, other phases=0.
  2. No statusDefinition: no karta_workload_status series.
  3. Status evaluation fails: record status_eval and store the new partial
     record.
  4. Info and generation can remain while status is absent.
- scope: VERIFIED source cases. Failed evaluation does not retain the previous good
  status.
- visual: One three-case table. Reveal each pre-authored result cell. No extra
  diagnostic panel.
- playback: Once per entry; hold after 3500ms. The fx-steps JSON is authoritative.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: src/pkg/resource/component.go:226-235;
  src/exporter/pkg/state/workload.go:26-48,64;
  src/exporter/pkg/controller/events.go:124-130;
  src/exporter/pkg/collector/collector.go:94-114;
  src/exporter/pkg/collector/names.go:62 (unused constant).
- embed: none: VERIFIED source cases. No saved capture demonstrates Undefined=1,
  missing statusDefinition or a status-evaluation failure here.
- number audit: Use the full metric name karta_workload_status. The absence/zero/Undefined
  distinction is verified in src/exporter/pkg/state/workload.go:37-48 and
  collector/collector.go:101-114. Do not use a captured Undefined=0 row as
  proof of the hypothetical Undefined=1 case.
- fx: `data-fxmode="storySteps"`; shared customFx.storySteps; once per entry.
- fx DOM: Reveal keys: no-match, no-definition, eval-error. Each is a complete
  result cell with its row label already visible.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,[]],
    [600,["no-match"]],
    [1600,["no-definition"]],
    [2700,["eval-error"]],
    [3500,[]]
  ]
  </script>
  ```

- reference notes: Retained source explanation: VERIFIED: mappings evaluated but none matched
  -> Undefined=1, other phases=0. no root statusDefinition ->
  HasStatus=false and no workload_status series, while info/generation still
  emit. a root status evaluation error also leaves HasStatus=false: the
  handler logs it, increments status_eval and stores the newly built record.
  it does not keep the old good status. no_status_definition exists as a
  constant but is not emitted by this implementation.

### s10. what a scrape actually returns
- one idea: real series from the live lab, not a mockup.
- beats:

  1. One real scrape identifies trainer-a and its pod.
  2. Running=1 and Suspended=0.
  3. Desired replicas=1; observed Running pods=1.
  4. The same capture also includes infrastructure Deployments.
- scope: EXECUTED: selected fields from one scrape. No lifecycle or deletion sequence
  is depicted.
- visual: One parsed scrape card. Reveal selected fields in place; do not type them
  as terminal commands.
- playback: Once per entry; hold after 3500ms. The fx-steps JSON is authoritative.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: captures/02-metrics-raw.txt:117-126,200,236,299,302;
  src/exporter/pkg/collector/collector.go:116-169;
  src/exporter/pkg/store/store.go:87-95,174-188.
- embed: captures/02-metrics-raw.txt:125-125; captures/02-metrics-raw.txt:200-200;
  captures/02-metrics-raw.txt:236-236; captures/02-metrics-raw.txt:299-299;
  captures/02-metrics-raw.txt:302-302. Infrastructure source rows:
  captures/02-metrics-raw.txt:117-124.
- number audit: All five selected series are from one scrape: trainer-a join=1, observed
  Running pods=1, desired replicas=1, Running=1 and Suspended=0.
  data-excerpts.md:15-18 lacks the required
  Running/Suspended/replica/pod-count rows; read the full capture. Projected
  field/value cards must say PARSED, not present invented shortened metric
  lines as raw output.
- fx: `data-fxmode="storySteps"`; shared customFx.storySteps; once per entry.
- fx DOM: Reveal keys: identity, status, counts, infra. Pre-author selected/parsed
  capture values, with tiers adjacent.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,[]],
    [450,["identity"]],
    [1500,["status"]],
    [2600,["counts"]],
    [3500,["infra"]]
  ]
  </script>
  ```

- reference notes: Retained source explanation: EXECUTED: trainer-a has an attributed pod and
  dense status; Kyverno's controllers, coredns and Prometheus also appear
  via catalog. VERIFIED: desired component replicas come from the workload
  record; observed pod counts are grouped at scrape time. the instance set
  is the union of declared and observed instances, zero-filled across pod
  phases. no replica value means an omitted replicas series, not zero.
  deletion removes stored workload/pod records from subsequent scrapes; this
  exporter is not the historical store.

### s11. unknown parts keep their workload identity
- one idea: Part-label failures can preserve the workload join.
- beats:

  1. Component inference failure: both part labels become <unknown>.
  2. Unmatched instance: preserve component; instance becomes <unknown>.
  3. Both cases keep the workload join.
  4. Reasons distinguish jq_error from unknown_instance.
- scope: VERIFIED source cases after root attribution succeeds. Missing-owner waiting
  is covered on s8.
- visual: One two-row table with a shared workload identity. Reveal each independent
  source case without moving rows or merging outcomes.
- playback: Once per entry; hold after 2700ms. The fx-steps JSON is authoritative.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: src/exporter/pkg/attribute/attributor.go:34-80;
  src/exporter/pkg/controller/events.go:275-311,328-351;
  src/exporter/pkg/collector/collector.go:173-191.
- embed: none: VERIFIED source cases. The literal sentinel and reasons are in
  src/exporter/pkg/collector/names.go:50-59.
- number audit: No captured failing-attribution row is asserted.
  captures/02-metrics-raw.txt:104-106 reports jq_error=0 and
  unknown_instance=0 for that scrape; those zeros do not execute either
  hypothetical failure case. Render <unknown> as text, not an HTML element.
- fx: `data-fxmode="storySteps"`; shared customFx.storySteps; once per entry.
- fx DOM: Reveal keys: jq-row, instance-row. Text keys jq-component, jq-instance and
  instance have final value <unknown>. Escape authored markup; the helper
  uses textContent for updates.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,[]],
    [650,["jq-row"],{"jq-component":"<unknown>","jq-instance":"<unknown>"}],
    [1700,["instance-row"],{"instance":"<unknown>"}],
    [2700,[]]
  ]
  </script>
  ```

- reference notes: Gauge/counter accounting is reference-only: unattributed_pods is current;
  attribution_errors_total accumulates failed evaluations and may count
  retries of one pod. Retained source explanation: VERIFIED: component
  inference failure sets component and instance to <unknown>, with reason
  jq_error. an unmatched instance can preserve the component and set only
  instance=<unknown>, reason unknown_instance. those pods keep their
  workload join. a newly seen pod with a missing owner parks without a join;
  no-controller and depth-limit paths differ. unattributed_pods is a current
  gauge; attribution_errors_total counts evaluation failures, so retries can
  count the same pod again.

### s11b. activity is not a health guarantee
- one idea: a recent exporter event does not certify every workload's facts.
- beats:

  1. /readyz checks startup and initial informer sync.
  2. karta_exporter_last_event_timestamp_seconds marks watch activity.
  3. The marker can advance after a workload evaluation error.
  4. Neither signal certifies current facts for every workload.
- scope: VERIFIED source example. Quiet input may leave the marker unchanged; skipped
  kinds are outside the sync check.
- visual: One readiness/activity/evaluation panel. Change the activity and
  evaluation text; no running clock or readiness-health animation.
- playback: Once per entry; hold after 3700ms. The fx-steps JSON is authoritative.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: src/exporter/pkg/controller/controller.go:116-136,343-344;
  src/exporter/pkg/controller/events.go:95-98,124-130,197-227;
  src/exporter/cmd/main.go:112.
- embed: captures/02-metrics-raw.txt:101-101 if a real metric sample is shown;
  otherwise keep the source example only.
- number audit: The full name is karta_exporter_last_event_timestamp_seconds. The literal
  sample is 1.7894066551482046e+09. T1/T2 and readiness=true in the
  animation are VERIFIED symbolic source-example states, not conversions of
  that sample or a captured /readyz response.
- fx: `data-fxmode="storySteps"`; shared customFx.storySteps; once per entry.
- fx DOM: Text keys activity and evaluation have final values T2 and status_eval
  error. Readiness=true remains fixed. T1/T2 are symbolic source-example
  labels.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,[],{"activity":"no event shown","evaluation":"initial sync complete"}],
    [900,[],{"activity":"T1","evaluation":"ordinary event"}],
    [1800,[],{"evaluation":"quiet interval"}],
    [2800,[],{"activity":"T2","evaluation":"status_eval error"}],
    [3700,[]]
  ]
  </script>
  ```

- handoff: now put the exported Running series into a policy condition.
- reference notes: Retained source explanation: VERIFIED: last_event_timestamp_seconds changes
  when handlers call markEvent, including after workload evaluation errors.
  a quiet cluster need not advance it; filtered updates need not advance it.
  /readyz checks startup and whether Karta, Pod and currently started
  dynamic informers have synced. skipped unmapped kinds are not in that
  check. neither signal proves per-workload freshness, scrape continuity or
  controller convergence.

---

## act 2 - kyverno consumes the facts

Source key for this act: K paths are relative to the Kyverno checkout,
read at tag v1.19.1 (commit 40ec788d48bb28d83dbf85538e962a59db9d45c6).
These source explanations are VERIFIED; capture excerpts are EXECUTED.

### s12. the rule we wanted
- one idea: the metric predicate belongs in the background target gate.
- inset cards: mutate-existing edits stored objects. CEL (Common Expression Language)
  evaluates conditions; PromQL queries metrics. Admission handles incoming
  API writes.
- beats:

  1. Select unsuspended Jobs in ai-team.
  2. Keep names whose available Running samples in the trailing two
     minutes are all 1.
  3. Put those tests in targetMatchConditions.
  4. The background mutation sets /spec/suspend=true; admission is
     disabled.
- scope: Existing samples only; no full-window guarantee. matchConditions does not
  gate these background targets.
- visual: One annotated policy slice. Use plain labels for matchConstraints,
  targetMatchConditions and mutations. The decoded PromQL is a wrapped code
  excerpt within that same diagram; show it instead of the encoded URL.
- code source: URL-DECODED source expression, wrapped without changing its text. It is
  not a captured response or a verbatim YAML line:
  `min_over_time(karta_workload_status{namespace="ai-team",phase="Running",workload_kind="Job"}[2m])==1`
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: manifests/05b-kyverno-metric-suspend-http.yaml:11-36;
  captures/30-lab2-kyverno-phase.txt:1-4; INTEGRATION-LOG.md section 4;
  K pkg/background/mpol/processor.go:111-145,230;
  K pkg/cel/policies/mpol/compiler/policy.go:201-219.
  K pkg/policy/mpol.go:91-98; manifests/05b-kyverno-metric-suspend-http.yaml:29-36.
- embed: SOURCE, not command output:
  manifests/05b-kyverno-metric-suspend-http.yaml:22-31 and
  manifests/05b-kyverno-metric-suspend-http.yaml:32-36. Optional admission
  configuration: manifests/05b-kyverno-metric-suspend-http.yaml:17-21.
- number audit: VERIFIED configuration: [2m], ==1, admission disabled and
  /spec/suspend=true. The decoded query is an explicitly URL-DECODED source
  expression, not a verbatim line in the YAML or captured Prometheus output.
  Code uses http.Get (capital G). Do not embed stale manifest comments at
  lines 3-5, which predate the projection correction.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Wrap matchConstraints, targetMatchConditions and mutations annotations in
  .fx-pill. Keep the wrapped decoded PromQL static. No typeInto over code
  markup.

- handoff: first, the fact source needed to be reachable.
- reference notes: The decoded query is not a verbatim URL. The name-membership check uses
  returned workload labels. Capture 30 records first suspension 67.6s after
  startTime; retain it in the evidence notes rather than add a second timer
  here. Retained source explanation: "suspend an ai-team Job whose available
  samples in the trailing 2 minutes all read Running." condition:
  min_over_time(...[2m]) == 1. the policy sets
  evaluation.admission.enabled=false and
  evaluation.mutateExisting.enabled=true. matchConstraints selects Jobs;
  namespace, not-already-suspended, and metric predicates sit in
  targetMatchConditions. ordinary matchConditions do not gate this target
  evaluation; the reporting path uses them instead (s17). VERIFIED. honesty
  box: min_over_time tests existing samples, not full 2-minute coverage.
  EXECUTED: first suspension observed 67.6s after startTime.

### s19. an earlier rule reached the metrics service
- one idea: an unscoped generate rule reached the metrics namespaces.
- inset cards: GeneratingPolicy creates resources. This earlier rule is separate from the
  Job-mutating policy.
- beats:

  1. An earlier GeneratingPolicy created ingress NetworkPolicies on
     namespace creation.
  2. Its default-deny rule also reached karta-system and monitoring.
  3. Captured: those policies existed; trainer data was returned after
     cleanup.
  4. Timeout and cleanup/healing are the operator account.
- scope: No packet trace was captured. Any causal arrow carries INFERRED; the timeout
  account carries OPERATOR-REPORTED.
- visual: One fixed ingress diagram; reveal evidence nodes. Do not animate packets,
  a measured timeout or cleanup causing recovery.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: VERIFIED on generating-rule scope; EXECUTED on saved policy rows and
  returned trainer data. OPERATOR-REPORTED on timeout/healing; INFERRED on
  causal attribution.
- data: ../kyverno-mastery-deck/demos/d5-gpol.yaml:9-33;
  captures/04-netpol-interference.txt:1-5; captures/05-prom-first-query.txt:1-3;
  INTEGRATION-LOG.md:110-124 (timeout and cleanup narrative).
- embed: captures/04-netpol-interference.txt:1-4;
  captures/05-prom-first-query.txt:1-3.
- number audit: The raw listing supplies two affected namespace rows; the parsed query
  supplies nightly-report 0, trainer-a 1 and trainer-b 1. The listing's AGE
  is age, not timeout duration. Timeout, cleanup ordering and recovery
  narrative are OPERATOR-REPORTED from INTEGRATION-LOG.md:110-124; causal
  attribution remains INFERRED.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .fx-pill on namespace evidence nodes and the qualified causal caption.
  Keep any INFERRED arrow and its tier in the same group.

- handoff: with trainer data available, the policy using a shared cache still did not
  act.
- reference notes: Retained source explanation: VERIFIED: the leftover default-netpol
  GeneratingPolicy matched namespace CREATE without a policy scope
  predicate. it generated an ingress NetworkPolicy named default-deny,
  selecting all pods with no ingress rules. EXECUTED: captured netpols
  include karta-system and monitoring; the query after cleanup returned
  trainer data. the scrape timeout and its resolution by deleting the policy
  and generated netpols are the log's operator account.

### s14. the control narrowed the search
- one idea: the twin acted while the GCE policy did not.
- inset cards: GlobalContextEntry (GCE) configures a named cache in a Kyverno controller
  process; this entry polls Prometheus.
- beats:

  1. The GCE-backed policy did not suspend its targets.
  2. The operator reported 8+ minutes without action.
  3. Without that read, trainer-b was first observed suspended about 171ms
     after applying the comparison policy.
  4. The twin also narrowed its target to trainer-b.
- scope: 8+ minutes: OPERATOR-REPORTED. About 171ms: computed first-observation
  offset. Different target predicates limit the comparison.
- visual: One static two-lane comparison with staggered observation labels. No timer
  or invented scan markers.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: EXECUTED on saved samples and computed 171ms. OPERATOR-REPORTED on the 8+
  minute interval. VERIFIED on differing target predicates.
- data: captures/09-ab-test.txt:1-3;
  captures/08-kyverno-suspend-timeline.txt;
  manifests/91-ab-twin-no-gctx.yaml:17-21;
  manifests/05-kyverno-metric-suspend.yaml:23-31;
  INTEGRATION-LOG.md:171-185; s13 source chain.
  K api/kyverno/v2alpha1/global_context_entry_types.go:38-77;
  K pkg/globalcontext/externalapi/entry.go:86-110.
- embed: captures/09-ab-test.txt:1-3; captures/08-kyverno-suspend-timeline.txt:1-2 as
  a separate initial sample, not an 8-minute trace.
- number audit: EXECUTED arithmetic: 17:36:45.461 - 17:36:45.289935 = 171.065ms, displayed
  about 171ms. t+1 is the recorder's poll label, not one elapsed second. 8+
  minutes is OPERATOR-REPORTED. Do not subtract the 20:31 local-looking
  header from a UTC marker, or use the malformed 17:31:00.3N in capture
  08-policy-apply-time.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .fx-pill on each observation group. The operator-reported interval is
  a labeled band, not events in lane-data.

- handoff: source review separated a successful fetch from a failed read.
- reference notes: gctx in saved labels refers to globalContext.Get access. Keep it verbatim in
  excerpts, with the globalContext.Get label alongside. Retained source
  explanation: the policy reading the GCE did not suspend its targets; the
  8+ minute duration is operator-reported, not a saved continuous timeline.
  EXECUTED: the comparison policy's Job trainer-b was first observed
  suspended 171ms after its apply marker. the twin also restricted its
  target to trainer-b; this was not an identical-target controlled
  comparison. the differing target predicates limit what this comparison
  isolates.

### s13. a projection name is not a query
- one idea: fetching the body and reading a named projection are separate.
- inset cards: Projection: named extracted result. JMESPath selects JSON fields before
  caching; globalContext.Get selects the stored name.
- beats:

  1. The background fetch succeeded and returned a trainer name.
  2. The policy passed a query string where globalContext.Get expects a projection name.
  3. No such named projection existed: "no data available".
  4. Declare workloads, then read that name.
- scope: Fetch output is EXECUTED; the lookup diagnosis and correction are VERIFIED.
  No query runs inside globalContext.Get.
- visual: One fetched-body/projection/lookup diagram. Replace the lookup string and
  map label in fixed slots. This is a source explanation, not a timed
  execution replay.
- code source: Use these exact expressions in the visual, wrapped without changing the
  literal content:
  `globalContext.Get('running-2m', 'data.result[].metric.workload')`
  `globalContext.Get('running-2m', 'workloads')`
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: EXECUTED on the successful fetch excerpt only. VERIFIED on the missing
  projection and corrected read. Whole-body alternative stays source-only.
- data: manifests/04-kyverno-gce.yaml:9-14;
  manifests/04b-kyverno-gce-projected.yaml:9-17;
  manifests/05-kyverno-metric-suspend.yaml:23-31;
  manifests/05c-kyverno-metric-suspend-gce.yaml:20-27;
  captures/07b-gce-poll-v6.txt:1-2; K pkg/cel/libs/context.go:115-124;
  K pkg/globalcontext/externalapi/entry.go:64-73,86-110,116-129,136-165.
- embed: captures/07b-gce-poll-v6.txt:2-2, with ANSI styling removed only. SOURCE
  excerpts: manifests/04b-kyverno-gce-projected.yaml:14-17;
  manifests/05-kyverno-metric-suspend.yaml:27-31;
  manifests/05c-kyverno-metric-suspend-gce.yaml:25-27.
- number audit: The successful poll returned trainer-c, not trainer-a/b. Preserve that name
  if displaying the fetched JSON. Caption ANSI-STYLING REMOVED; all
  printable text stays verbatim. The two globalContext.Get expressions are SOURCE snippets
  with broken/corrected captions outside the code. The real error message is
  also saved in captures/14-gctx-error-event.txt:7-7, from the separate
  reporting policy.
- fx: `data-fxmode="storySteps"`; shared customFx.storySteps; once per entry.
- fx DOM: Reveal keys: fetch, missing, declared. Text keys lookup, projections,
  result have final values from the 2000ms frame. Label the missing-key
  state as the earlier broken case. Tier chips are siblings, never inside
  overwritten text nodes.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,["fetch"],{"lookup":"globalContext.Get('running-2m', 'data.result[].metric.workload')","projections":"none declared","result":"lookup pending"}],
    [1000,["missing"],{"result":"no data available"}],
    [2000,["declared"],{"lookup":"globalContext.Get('running-2m', 'workloads')","projections":"workloads declared","result":"named projection selected"}],
    [2900,[]]
  ]
  </script>
  ```

- handoff: the read failed; follow what became of its error.
- reference notes: refreshInterval=15s is taught on s18a. globalContext.Get(name, '') can read the full body,
  source-verified but untested here; keep this alternative in reference
  notes, not a third demo. Retained source explanation: this GCE has
  refreshInterval: 15s. the broken policy read
  `globalContext.Get('running-2m', 'data.result[].metric.workload')`. the
  second argument is a projection name; this manifest declared none.
  VERIFIED: the poller evaluates declared JMESPath projections when storing
  a response. globalContext.Get reads that map; a missing name returns "no data
  available". EXECUTED: successful background-controller polls included a
  trainer name. fetch success does not validate the policy's projection
  name. source-only note: globalContext.Get(name, '') reads the whole body; untested here.

### s15. the error misses the action record
- one idea: the processor drops a recorded rule error from the action path.
- inset cards: UpdateRequest (UR): saved background work item. nil means no value. Audit
  here means Kyverno Event/report generation.
- beats:

  1. Target CEL fails; the result contains RuleError.
  2. The outer error is nil and PatchedResource is nil.
  3. Metrics record the error; the worker skips update and audit.
  4. With no accumulated failures, the UR becomes Completed, message
     empty.
- scope: VERIFIED chain. Other false conditions can mask errors. Completed does not
  prove that a target changed.
- visual: One source return path. Reveal/highlight nodes in the verified order; the
  metrics branch remains separate from the closed action/audit gate.
- playback: Once per entry; hold after 3250ms. The fx-steps JSON is authoritative.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: All diagram nodes VERIFIED. No EXECUTED chip on a simulated return path.
  Actual aggregate metric evidence remains in source detail and s16.
- data: K pkg/cel/policies/mpol/compiler/policy.go:42-69,201-209;
  K pkg/cel/policies/mpol/engine/engine.go:95-118,224-226;
  K pkg/cel/policies/mpol/engine/metrics.go:18-33;
  K cmd/background-controller/main.go:390-396;
  K pkg/background/mpol/processor.go:230-276,407-415;
  K pkg/background/common/status.go:37-39; K pkg/metrics/mpol.go:34-90;
  captures/18-metrics-surface.txt:6.
  K api/kyverno/v2/updaterequest_types.go:26-33,53-75;
  K pkg/background/mpol/processor.go:308-351.
- embed: none on the projected source-chain diagram. If the reference detail shows
  the original error count, use captures/18-metrics-surface.txt:6-6
  verbatim.
- number audit: RuleError, nil, PatchedResource=nil and empty UR message are VERIFIED
  code-path values, not dumped runtime objects in capture 31. The reference
  count 7 is an aggregate for metric-suspend-running; dbg-gctx has count 3
  at captures/18-metrics-surface.txt:2-2. Neither count is seven/three Jobs
  or per-UR tracing.
- fx: `data-fxmode="storySteps"`; shared customFx.storySteps; once per entry.
- fx DOM: Reveal keys: target-error, rule-error, outer-return, metric-error,
  skip-audit, completed. Label the final node UR state: Completed; keep its
  DOM key completed. Pre-draw the metrics branch and closed action/audit
  gate. Use static error styling, not a traveling error dot.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,["target-error"]],
    [650,["rule-error"]],
    [1300,["outer-return"]],
    [1950,["metric-error"]],
    [2600,["skip-audit"]],
    [3250,["completed"]]
  ]
  </script>
  ```

- reference notes: Capture 18 records count 7 for the broken metric policy. Those are aggregate
  evaluations, not seven Jobs or traced URs. Only the duration histogram
  family was captured; the wrapper also records the result family.
  Kubernetes API audit logs are a different surface. Retained source
  explanation: VERIFIED at v1.19.1: a runtime target-condition error becomes
  EvaluationResult.Error, then a RuleError in response.Policies[].Rules. the
  engine returns PatchedResource=nil and a nil outer Go error. the metrics
  wrapper records the error before returning to the processor. the processor
  checks the outer error, then gates update AND audit on PatchedResource !=
  nil. it never inspects those rule errors. with no other accumulated
  failures, Success writes Completed and clears message. scope note: another
  false condition can mask a condition error as a skip. EXECUTED: the broken
  metric policy's captured error histogram count is 7.

### s16. the error needs no metrics query
- one idea: four URs at Completed, a failed condition, and no suspension.
- inset cards: rr-cel-runtime-error is a Kyverno policy. Its Job name is not an integer;
  the cast fails during condition evaluation.
- beats:

  1. Condition: int(object.metadata.name) > 0 on cel-error-job.
  2. Four URs reached Pending -> Completed -> DELETED in the saved watch.
  3. The Job stayed unsuspended; policy ready=true.
  4. The error histogram was absent before and count 4 afterward.
- scope: Histogram: aggregate corroboration, not per-UR tracing. DELETED is a watch
  event. Trigger sources were not captured; controls remain unexecuted.
- visual: One parsed evidence card. Rows reveal in reading order, not at claimed
  controller timings.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: EXECUTED beside the saved fields; VERIFIED beside the cast explanation and
  readiness meaning. Label ready as reporting permissions. No universal no-log/no-event claim.
- data: captures/31-cel-error-repro.txt:7-28,35-40;
  upstream-drafts.md:141-167,230-231; K pkg/policy/generate.go:31-39;
  K pkg/background/update_request_controller.go:211-219,268-289;
  K pkg/controllers/policystatus/controller.go:330-340,372-380,462-479,558-571.
- embed: captures/31-cel-error-repro.txt:8-24; captures/31-cel-error-repro.txt:25-28;
  captures/31-cel-error-repro.txt:4-6;
  captures/31-cel-error-repro.txt:39-40. SOURCE condition:
  upstream-drafts.md:160-167.
- number audit: Four distinct UR names appear at lines 9,13,17,21. Every one has Pending,
  Completed and DELETED rows; AGE is 0s/1s, not duration. The before
  selection has no series; do not synthesize a 0-valued sample. The after
  count is 4. Header line 7 announces a 150s watch but invents trigger
  attribution; omit it. No exact elapsed watch duration or individual
  trigger was captured. data-excerpts.md contains only the first two UR
  histories.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .fx-pill on each complete UR evidence row, then the Job/policy and
  aggregate-metric cells. No fake per-UR-duration animation.

- reference notes: The watch shows AGE 0s/1s, not measured per-UR duration or UR messages. UR
  deletion is direct, without a TTL wait. Selected log query found no policy
  matches. Policy message was empty; empty UR message is source-only.
  Reporting readiness checks read permissions. Pending controls: false
  condition, true condition, error in mutation expression. Retained source
  explanation: VERIFIED: the Kyverno MutatingPolicy rr-cel-runtime-error
  puts `int(object.metadata.name) > 0` in targetMatchConditions for cel-error-job. EXECUTED: the saved watch shows four URs: blank initial state ->
  Pending -> Completed -> DELETED watch event; displayed AGE was 0s/1s. the
  Job stayed suspend=false. policy ready:true, top-level message empty. the
  selected background log query returned no policy matches. the histogram
  series was absent before and count 4 afterward: aggregate corroboration,
  not per-UR tracing. individual trigger sources were not captured.
  VERIFIED: the UR controller reads the latest status and deletes Completed
  URs directly; no TTL wait is involved. the captured RBACPermissionsGranted
  condition checks reporting read permissions, not target CEL success.

### s17. the reporting path tells a different story
- one idea: The int-cast policy has a separate reporting result.
- inset cards: Reporting scan: checks whether a stored object already matches the simulated
  mutation.
- beats:

  1. Reporting simulates the mutation with target=false.
  2. For rr-cel-runtime-error, it bypasses the failing
     targetMatchConditions.
  3. The simulated patch differs from the stored Job.
  4. Captured Event: "mutation is not applied".
- scope: This is a separate reporting result, not a relabeled acting-path error.
  Policy name stays visible.
- visual: One rr-cel-runtime-error reporting path: matchConditions=[] -> simulated
  patch -> difference -> report Fail. No dbg-gctx content on this slide.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: VERIFIED on the reporting path and gate choice; EXECUTED on the Event
  excerpt from capture 31. No action-success badge.
- data: captures/31-cel-error-repro.txt:9-28,35-36;
  upstream-drafts.md:141-170; K
  pkg/controllers/report/utils/scanner.go:243-277; K
  pkg/cel/policies/mpol/engine/engine.go:158,215-226; K
  pkg/cel/policies/mpol/compiler/policy.go:201-236.
- embed: captures/31-cel-error-repro.txt:35-36.
- number audit: The verbatim message includes policy rr-cel-runtime-error/ fail: mutation is
  not applied. Short quotes may retain the exact message substring.
  target=false and skipped target conditions are VERIFIED mechanism, not
  fields printed by this Event. No dbg-gctx content belongs here.
- fx: `data-fxmode="popTimelines"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .timeline with .chip nodes and .arr connectors for the single policy
  path. Keep the policy name and captured Event quote static. No
  data-glitch.

- reference notes: Acting URs completing are already shown on s16; do not replay their trace
  here. The scanner changes the simulated RulePass to RuleFail when the
  mutation is unapplied. Retained source explanation: VERIFIED: reports call
  Handle with target=false. this evaluates matchConditions, not
  targetMatchConditions, then simulates mutations. rr-cel-runtime-error has
  no matchConditions. its bad int cast sits only in targetMatchConditions,
  so reporting never evaluates it. the simulated suspend patch differs from
  the object; scanner changes that RulePass to RuleFail("mutation is not
  applied"). EXECUTED: those Job Events appear in capture 31 while the
  acting URs complete.

### s17b. a mutation error reaches the report
- one idea: A failing mutation expression can produce a reporting error Event.
- beats:

  1. dbg-gctx tries to write cached workload names into an annotation.
  2. Its bad projection lookup is inside the mutation expression.
  3. Reporting reaches that expression and preserves RuleError.
  4. Captured: a PolicyViolation error Event on trainer-b.
- scope: Separate policy and run. Its trainer-a target condition does not scope the
  target=false reporting scan.
- visual: One dbg-gctx path: annotation expression -> lookup error -> the captured
  trainer-b Event. Keep the policy name and expression location fixed. No
  int-cast lane.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: VERIFIED on expression location and reporting mechanism; EXECUTED on capture
  14 Event. Never attribute this Event to rr-cel-runtime-error.
- data: captures/14-gctx-error-event.txt:7;
  manifests/90-debug-gctx-annotation.yaml:17-25; K
  pkg/controllers/report/utils/scanner.go:243-277; K
  pkg/cel/policies/mpol/engine/engine.go:158,215-226; K
  pkg/cel/policies/mpol/compiler/policy.go:201-236.
- embed: captures/14-gctx-error-event.txt:7-7. SOURCE mutation:
  manifests/90-debug-gctx-annotation.yaml:20-25.
- number audit: This row names job/trainer-b, policy dbg-gctx, PolicyViolation, and no data
  available. Preserve them. Its age/count columns describe that Event row,
  not error latency or total background evaluations. Do not relabel it
  rr-cel-runtime-error.
- fx: `data-fxmode="popTimelines"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .timeline with .chip nodes and .arr connectors for the separate
  dbg-gctx path. Keep the expression location, policy name and Event tier
  static. No data-glitch.

- reference notes: The two policy diagrams were split to avoid two complete examples on one
  slide. This preserves the executed reporting-error surface without
  claiming it diagnosed the int-cast policy.

### s18. two ways that do work
- one idea: background metrics conditions work through HTTP and projected GCE.
- beats:

  1. Direct HTTP condition: suspension first observed 644ms after apply on
     lab1.
  2. Corrected GCE projection: trainer-gce suspended about 2.5 minutes
     after setup.
  3. Both working paths consumed the exporter/Prometheus fact source.
- scope: EXECUTED observations, not executor latency. gce-test is a namespace. Exact
  triggering evaluations were not captured.
- visual: One two-row result card: HTTP / declared GCE projection. Keep distinct time
  origins. No rereading of projection internals or sample-window lesson.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: EXECUTED on both observed suspension rows. Captions name run and timing
  origin; these are first observations, not latency measurements.
- data: captures/10-v2-http-timeline.txt:1-3;
  captures/19-gce-corrected.txt:1-4; captures/30-lab2-kyverno-phase.txt:1-4;
  INTEGRATION-LOG.md section 4; manifests/05b-kyverno-metric-suspend-http.yaml;
  manifests/04b-kyverno-gce-projected.yaml; manifests/05c-kyverno-metric-suspend-gce.yaml.
- embed: captures/10-v2-http-timeline.txt:1-3; captures/19-gce-corrected.txt:1-4.
- number audit: 644ms = 17:45:19.693 - 17:45:19.049. The GCE setup marker is 18:07:28.165;
  its first true observation is 18:09:59 at whole-second precision: about
  151s, or about 2.5 minutes. Do not present 150.835s as equally precise.
  Source detail only: capture 30:1 and :3 give 3.397s, not the printed 3.2s;
  that header also overstates full-window coverage.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use one .fx-pill for each complete result row, including its own time
  origin and evidence tier. Do not count their durations against each
  other.

- reference notes: Use running-2m-gcetest for the isolated executed GCE run; the saved ai-team
  fixture uses running-2m. Lab2 HTTP first observation was 3.397s; retain it
  in evidence detail. The raw capture annotation saying 3.2s is superseded.
  Do not teach that setup interval again on s20. Retained source
  explanation: EXECUTED: with http.Get in targetMatchConditions, suspension
  was first observed 644ms after the apply marker on lab1, and 3.397s on
  lab2. with a declared workloads projection, trainer-gce was observed
  suspended about 2.5 minutes after setup in gce-test. no capture identifies
  the triggering evaluation. these are observation timings, not executor
  latency measurements or proof of a fully populated two-minute sample
  range.

### s18a. shared polling or direct reads
- one idea: refreshing facts does not schedule a mutation.
- inset cards: Scrape collects samples; refresh updates cached results. A background scan tick
  queues eligible policy work.
- beats:

  1. Configured facts: Prometheus scrape 5s; GCE refresh 15s.
  2. GCE refresh updates a per-process cache; it does not queue a
     mutation.
  3. For these policies, creation, spec changes and background scan ticks
     enqueue policy work.
  4. Background scan tick: 60s operator-reported; source default: 1h.
- scope: A background scan tick does not guarantee action. HTTP reads during evaluation;
  reporting has its own scheduler.
- visual: One clock-to-work diagram. Reveal configured clocks; no synchronized
  ticking or extrapolated action events.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: VERIFIED topology, configured 5s/15s and default 1h. OPERATOR-REPORTED on
  60s. No EXECUTED chip identifies any individual tick.
- data: manifests/02-prometheus.yaml:17; manifests/04-kyverno-gce.yaml:14;
  INTEGRATION-LOG.md:6-9; K cmd/background-controller/main.go:191-199,223-238,319-325;
  K pkg/globalcontext/externalapi/entry.go:86-110;
  K pkg/policy/policy_controller.go:234-246,302-319,591-607,645-655,699-705;
  K pkg/policy/mpol.go:17-25,91-98; K pkg/background/mpol/processor.go:111-145;
  K pkg/cel/policies/mpol/compiler/policy.go:42-69,201-209;
  K pkg/cel/policies/mpol/compiler/compiler.go:252-255;
  K pkg/cel/compiler/http.go:19-43; K cmd/reports-controller/main.go:210,321;
  K cmd/background-controller/main.go:390-396; codex-notes-r1.md section A.
- embed: SOURCE configuration: manifests/02-prometheus.yaml:16-22;
  manifests/04b-kyverno-gce-projected.yaml:14-17. Optional observed poll
  pair: captures/07b-gce-poll-v6.txt:1-2, ANSI styling removed only.
- number audit: 5s and 15s are VERIFIED configured intervals, not measured scrape/action
  guarantees. The two poll logs are 15s apart but prove only that pair.
  Default 1h is VERIFIED at Kyverno v1.19.1
  cmd/background-controller/main.go:191-199. 60s stays OPERATOR-REPORTED
  setup. No captured background scan tick is matched to an action.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .fx-pill on the scrape, GCE refresh, background scan tick and
  http.Get branches. Label the tick implementation forceReconciliation in
  source detail. Static labeled intervals only; no implied action per tick.

- handoff: next came resume; first, separate the earlier cluster defect.
- reference notes: BACKGROUND_SCAN_INTERVAL configures the action controller. Reports use
  --backgroundScanInterval; evaluation.background.enabled gates reporting.
  This background-scan eligibility is for the tested policies without
  targetMatchConstraints.expression. Earlier false predicates can skip HTTP.
  cachedHTTPContext caches a client context, not query results.
  execution_cause=background_scan also labels policy-event work. Retained
  source explanation: VERIFIED: http.Get runs when the target evaluation
  reaches that condition; an earlier false predicate skips it. GCE refreshes
  a store shared by evaluations in the same controller process, not one
  cluster-wide cache. saved configuration: Prometheus scrape 5s and GCE
  refreshInterval 15s. the lab log records a 60s background scan tick
  interval, implemented by forceReconciliation (BACKGROUND_SCAN_INTERVAL; source-verified
  binary default 1h). GCE refresh does not enqueue mutation work. for these
  policies, policy creation or spec changes also enqueue work;
  policy reconciliation submits a UR with requestType=cel-mutate
  (Go constant CELMutate); its processor lists targets. Background scan
  ticks enqueue eligible policies, not a guaranteed action per tick.
  evaluation.background.enabled gates reporting, not mutate-existing
  actions.

---

## act 3 - a clean resume and another action

### s21. the old cluster rejected resume status
- one idea: the failed resumes match a known platform validation defect.
- inset cards: The Job controller writes status; Kubernetes API validation checks that
  proposed update.
- beats:

  1. Lab1: 3 Jobs across 5 documented resume attempts had stalled status.
  2. The controller proposed a changed startTime; the old validator
     rejected it.
  3. The symptoms match Kubernetes issue #134521; the fix shipped in
     v1.34.2.
  4. v1.34.3 capture: startTime reset accepted, active=1.
- scope: Observed fields/counts: EXECUTED. Versions: OPERATOR-REPORTED setup. Fix:
  VERIFIED. Defect attribution: INFERRED; repeated rejection is not
  separately logged for every attempt.
- visual: One version comparison; no repeated retries, live upgrade, or typed
  synthetic shell command.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: EXECUTED on the documented run inventory and captured status fields.
  OPERATOR-REPORTED on cluster-version setup. VERIFIED on validation/fix
  source. INFERRED on applying that defect to each stalled attempt.
- data: captures/12-job-wedge-kcm.txt:1-19;
  captures/13-trainer-b-wedged.txt:9-12,45,70,95-106;
  captures/15-trap-take2.txt:1-4; captures/16-trap-trainer-c.txt:1-7;
  captures/16b-trap-take3.txt:1-5; captures/16c-trap-gce.txt:1-3;
  captures/30-lab2-kyverno-phase.txt:5-6; INTEGRATION-LOG.md:278-286,308-325;
  codex-notes-r3.md sections A-B (U/U1/U2 versioned source keys);
  U1 pkg/controller/job/job_controller.go:639-655,1036-1058;
  U1 pkg/apis/batch/validation/validation.go:698-704;
  U2 pkg/registry/batch/job/strategy.go:383-387,405;
  U CHANGELOG/CHANGELOG-1.34.md:954,1037-1039.
- embed: captures/12-job-wedge-kcm.txt:1-3 and captures/12-job-wedge-kcm.txt:18-19
  for repeated rejection; captures/30-lab2-kyverno-phase.txt:5-6 for the
  accepted resume fields. The five-attempt source inventory is in the audit
  note, not an invented combined transcript.
- number audit: Inventory: first trainer-b attempt, captures/11-resume-trap-kyverno.txt:1-2
  and captures/13-trainer-b-wedged.txt:9-12; second b,
  captures/15-trap-take2.txt:1-4; first c,
  captures/16-trap-trainer-c.txt:3-7; second c,
  captures/16b-trap-take3.txt:1-5; gce, captures/16c-trap-gce.txt:1-3. Thus
  3 named Jobs/5 documented attempts, not 5 independent Jobs. Repeated
  identical rejection is logged for b/c, not separately for every attempt.
  Version identity is setup metadata; fix v1.34.2 is VERIFIED release
  source. x15 in capture 13:45 is an aggregated Event count, not human
  resumes. The 16b/16c settle captions are invalid; never teach them as a
  successful settle check. Manual SET-startTime remains OPERATOR-REPORTED,
  explanation INFERRED.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .fx-pill on broken-version evidence, source mechanism/fix, and
  fixed-version evidence. Each keeps its tier. The short rejection excerpt
  is static text.

- handoff: inspect stored status before trusting a successful patch.
- reference notes: Attempts: trainer-b twice, trainer-c twice, trainer-gce once. Resumed x15 is
  an aggregated Event count, not fifteen human resumes. Retried proposals
  recompute from stored status. Manual SET-startTime rejection is operator-reported; its explanation is INFERRED because the payload was not saved.
  v1.34.2 is a source/release fact, not a tested cluster version. Retained
  source explanation: EXECUTED summary: 3 distinct Jobs, 5 resume attempts
  on v1.34.0: trainer-b twice, trainer-c twice, trainer-gce once. these are
  repeated attempts, not five independent Jobs. saved logs show repeated
  rejection; trainer-b's Event row reports Resumed x15, not fifteen user
  resumes. VERIFIED: the affected controller proposes a new non-null
  startTime on resume. the old guard rejects that change while unsuspended.
  each retry recomputes from stored status; it is not replaying a frozen
  request. this is the mechanism reported in #134521; the fix shipped in
  v1.34.2. EXECUTED separately on v1.34.3: startTime reset accepted,
  active=1. manual-patch note: the SET-startTime rejection was operator-reported. hitting the same value-change guard is INFERRED; the payload was
  not saved.

### s22. a patch can succeed while status stays stuck
- one idea: patch acceptance does not establish controller convergence.
- inset cards: Convergence: observed workload state catches up with the requested spec.
- beats:

  1. trainer-b requested spec.suspend=false.
  2. Stored status still had Suspended=True, ready=0 and active absent.
  3. The catalog maps those stored values to Running=0.
  4. Patch acceptance did not establish controller convergence.
- scope: Stored fields: EXECUTED. Running=0: INFERRED from the VERIFIED matcher, not
  a captured metric trace.
- visual: One requested-spec / stored-status / derived-metric diagram for trainer-b.
  Drop the pod-health panel and second trainer-c watch. Never draw
  continuous metric history.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: EXECUTED beside stored fields; VERIFIED beside the catalog matcher; INFERRED
  on the derived metric and ineligibility explanation. No pod-health
  measurement is depicted.
- data: captures/13-trainer-b-wedged.txt:9-12,35-40,70,95-106;
  captures/16-trap-trainer-c.txt:3-7; INTEGRATION-LOG.md:251-256,288-298,327-334;
  INTEGRATION-LOG.md:483,502-506; rr-poc/rr_poc.py:434-461;
  src/pkg/catalog/kartas/batch_job.go:42-44,52;
  src/docs/catalog/batch-job-v1.yaml:31-34,47-50.
- embed: captures/13-trainer-b-wedged.txt:70-70;
  captures/13-trainer-b-wedged.txt:95-106. Optional describe output:
  captures/13-trainer-b-wedged.txt:9-12.
- number audit: The complete stored status block has ready: 0 and no active field. Its
  Suspended condition is status: "True". The describe view renders zero
  active, but do not insert active: 0 into the verbatim YAML. Running=0 is
  INFERRED from the VERIFIED catalog matcher, not a recorded post-resume
  metric sample.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .fx-pill on requested spec, stored status and derived metric groups.
  The inferred Running=0 keeps its chip. No blinking health indicator.

- handoff: with startTime reset and active=1 on lab2, return to the rule.
- reference notes: The pod-running account is operator-reported. Separate trainer-c capture
  shows no re-suspension during its 7m watch and final active
  absent/ready=0; retain it as corroborating source detail, not another
  slide act. Neither executor demonstrated automatic wedge detection; that
  comparison limit has its main home on s30b. Retained source explanation:
  EXECUTED trainer-b snapshot: spec.suspend=false, Suspended=True, ready=0,
  active absent. its pod running is the operator's account. VERIFIED: the
  Job catalog's Running matcher requires active>0 AND ready>0. INFERRED:
  that stored status maps to Running=0 and explains ineligibility; no saved
  metric trace establishes the whole post-resume interval. separate EXECUTED
  trainer-c take: no re-suspension in the captured 7m watch; final
  suspend=false, active absent, ready=0. do not merge the two Jobs into one
  simultaneous observation. inspected report Events say "mutation is not
  applied"; they do not diagnose controller divergence. neither executor
  demonstrated automatic wedge detection. RR's patch-success receipt is not
  a convergence test.

### s20. one resume led to another suspension
- one idea: the tested policy re-applies after a user resume.
- inset cards: Allowance permits an automated action; a cap limits repeated use. A metric
  time range grants no fresh permission.
- beats:

  1. On fixed lab2, the user resumed trainer-a.
  2. The next sample showed active=1 and a reset startTime.
  3. The same policy re-suspended it at +123.947s.
  4. One repeat was observed; indefinite repetition when eligible is
     inferred.
- scope: Tested policy behavior, not a Kyverno impossibility. Exact triggering background scan tick
  unknown. INFERRED stays on any continuation.
- visual: One compressed ordered-observation lane, once on entry. Show resume, the
  next captured status sample, then the recorded re-suspension. Offsets
  below are presentation time, not sample timestamps.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: EXECUTED on captured markers and computed 123.947s; VERIFIED on this policy
  having no allowance cap; INFERRED on recurrence beyond the one captured
  repeat.
- data: captures/30-lab2-kyverno-phase.txt:1-8;
  manifests/05b-kyverno-metric-suspend-http.yaml:10-36;
  INTEGRATION-LOG.md:390-397.
- embed: captures/30-lab2-kyverno-phase.txt:5-8.
- number audit: 123.947s is computed from line 5/7 timestamps, while line 7 prints rounded
  123.9s. The next status sample is 95ms after the user-resume marker; line
  6's 0.0s is the script's imprecise annotation. For the parsed lane use
  active=1 and reset startTime, not a 0.0s latency claim. Initial apply
  lines 1-4 are not part of this resume interval.
- fx: `data-fxmode="storySteps"`; shared customFx.storySteps; once per entry.
- fx DOM: Reveal keys: resumed, sample, repeated. Pre-author +123.947s on repeated.
  A separate-run-follows label may reserve the second lane; no invented RR
  events.
- fx steps: Copy this inert JSON into the slide. Offsets are presentation ms.

  ```html
  <script type="application/json" class="fx-steps">
  [
    [0,["resumed"]],
    [900,["sample"]],
    [2100,["repeated"]],
    [2800,[]]
  ]
  </script>
  ```

- handoff: after the repeated action, inspect the record left behind.
- reference notes: The initial suspension at 3.397s after apply remains in source detail on
  s18. Preserve the capture as an edited excerpt; do not copy its incorrect
  3.2s annotation or full-window inference. Retained source explanation: an
  edited timing excerpt from lab2 - policy applied, both trainers first
  observed suspended at 3.397s, user resumes trainer-a, the next sample
  shows active=1 and a reset startTime, 123.947s after the user-resume
  marker the engine re-suspends it. one re-suspension observed; the rule has
  no cap, so indefinite repetition under recurring eligibility is INFERRED,
  labeled as such. no capture identifies the exact triggering background scan tick.
  this is the configured rule's behavior, not proof that Kyverno cannot host
  an allowance protocol.

### s23. after the workload is gone
- one idea: no action receipt was identified in the inspected records.
- inset cards: Action receipt: a saved record linking a workload action to its decision.
- beats:

  1. After trainer-a deletion, scoped reports went from one row to 0.
  2. URs were absent before and after; Events still counted 7 afterward.
  3. Policy status was inspected before deletion only.
  4. No retained action-to-decision receipt was identified in these
     inspected records.
- scope: Logs, metrics, API audit and external sinks were not inspected. Event expiry
  and post-delete policy status were not captured.
- visual: One before/after table: reports, URs, Events, policy status. Events after
  deletion show only count=7; status after deletion says not inspected. No
  expiry animation.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: EXECUTED on table observations with before/after labels. INFERRED on the
  bounded no-receipt conclusion. NOT INSPECTED labels absent inspections.
- data: captures/17-kyverno-forensics.txt:1-27;
  captures/30-lab2-kyverno-phase.txt:1-8; INTEGRATION-LOG.md:399-410;
  K pkg/background/update_request_controller.go:268-289 (source explanation
  of deletion of URs in Completed, if shown; not observed by capture 17).
- embed: captures/17-kyverno-forensics.txt:2-15;
  captures/17-kyverno-forensics.txt:17-25.
- number audit: One report row before, zero afterward. Seven Event rows are listed BEFORE
  deletion; afterward only count 7 was saved. UR absence appears in both
  selections. ready=true/message empty is policy status BEFORE deletion, not
  a post-delete read or a UR message. Do not quote the recorder's universal
  no-surviving-surface gloss at lines 26-27 as measurement.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use one .fx-pill per complete before/after row: reports, URs, Events,
  status. No countDown and no erasure of Events.

- handoff: the prototype makes resume allowance and an action record explicit.
- reference notes: Before deletion, Event rows include Job-controller Suspended/Resumed plus a
  policy-naming reporting warning. That warning does not link a suspend
  action to an evaluation. Do not reconstruct the after-list from its count,
  or describe all possible records as absent. Retained source explanation:
  EXECUTED: trainer-a was deleted after the two lab2 engine actions. scoped
  reports: one pass/success row before deletion, 0 afterward. URs: absent
  both before and after. Events: seven rows listed BEFORE deletion; the
  AFTER check records only the count 7. the earlier rows include Job-controller Suspended/Resumed and one policy-naming reporting warning. that
  warning does not attribute a suspend action to its evaluation. policy
  status was read BEFORE deletion: ready for reporting, no action record in
  that output. INFERRED, bounded conclusion: these inspected records provide
  no retained receipt linking an action to its evaluation. logs, metrics,
  API audit, and external sinks were not inspected for this deletion
  comparison. Event expiry and a post-delete status read were not captured;
  this is not a claim that no evidence can exist elsewhere.

---

## act 4 - the runtime rules lab prototype

### s24. one action and no renewed allowance
- one idea: a new allowance epoch records a resume; it does not renew this rule's cap.
- inset cards: Allowance epoch: a numbered part of the rule/UID history. Advancing it does
  not renew this rule's allowance.
- beats:

  1. State is keyed by rule name and workload UID.
  2. The first successful automated suspend consumes this rule's
     allowance.
  3. Detected resume advances the allowance epoch; the actor remains unknown.
  4. reArmOnResume=false blocks another suspend when eligibility returns.
- scope: VERIFIED for retained state and this configuration. Detection uses
  weSuspended plus suspend=false, not an authenticated actor.
- visual: One state diagram: initial -> suspended -> observed resume -> new allowance epoch, cap
  still consumed. Put rule-name:UID and reArmOnResume=false on the diagram;
  no full YAML editor.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: VERIFIED on state/configuration semantics. The executed transition and
  escalation belong on s27; do not replay receipt names here.
- data: rr-poc/rules.yaml:5-23; rr-poc/rr_poc.py:175-194,333-349,368-403,444-450;
  rr-poc/rr_poc.py:237,253-264; captures/21-rr-enforce.txt:4-7;
  captures/22-rr-anti-trap.txt:6-7.
- embed: none on the state diagram: VERIFIED implementation/configuration. Optional
  source line: rr-poc/rules.yaml:21-21.
- number audit: Initial allowance epoch 0 and the fixed no-rearm gate are source semantics.
  actionsPerEpoch: 1 is read/logged, not a configurable counter
  implementation. The resulting e0/e1 receipt IDs are executed elsewhere; do
  not invent an rr-state dump for this slide.
- fx: `data-fxmode="popTimelines"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .timeline .chip/.arr for the state path. Label the new stage
  allowance epoch. Keep rule-name:UID and reArmOnResume=false static.
  No data-glitch or repeated state cycle.

- handoff: before spending that allowance, persist the intended action.
- reference notes: Initial allowance epoch is 0. observed resume increments externalResumes and attempts
  ResumeDetected. With reArmOnResume=false, later eligibility attempts
  EscalatedNeedsHuman. actionsPerEpoch is read/logged but the gate is
  boolean; no configurable action count or adoption/resume timer is
  implemented. Retained source explanation: the tested local YAML sets
  reArmOnResume: false. state lives in rr-system/rr-state under `<rule
  name>:<workload UID>`, initially allowance epoch 0. after a successful patch,
  weSuspended=true. observing that same UID with suspend=false increments
  epoch and externalResumes fields, and attempts a ResumeDetected receipt. actor
  unknown; managedFields does not decide this transition. at later
  eligibility, externalResumes>0 blocks another suspend and attempts
  EscalatedNeedsHuman. one initial successful automated action per retained
  rule-name/UID state; no automatic re-arm in the tested setup. The
  proposal resets a condition window after resume; this configuration does
  not implement that behavior. Sharing allowance-epoch terminology does not
  imply identical contracts.

### s25. two persisted records before the patch
- one idea: Intended and pendingOp must both persist before this patch runs.
- inset cards: Intended: planned-action receipt. pendingOp: saved unfinished-operation
  marker. Suspend handle: suspendDefinition field/value assignments in the
  karta description.
- beats:

  1. Persist Intended, then pendingOp; either failed write blocks this
     patch.
  2. JSONPatch tests only UID and existing suspend=false, then sets true.
  3. A successful patch attempts a separate Executed receipt.
  4. The captured Intended/Executed pair links through supersedes.
- scope: Executed means patch-command success. Later receipt writes can fail. No
  metric, freshness or resourceVersion guard; fault paths were not injected.
- visual: One write path with two persisted gates, one guarded patch and
  separate Intended/Executed ConfigMap cards. Show the Job recipe
  .spec.suspend=true on the patch node. Restart recovery and failure
  taxonomy belong on s30b.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: EXECUTED beside the captured receipt pair and patch result. VERIFIED on
  ordering and tests; NOT TESTED on failure injection. Never morph Intended
  into Executed.
- data: rr-poc/rr_poc.py:124-166,275-319,364-366,434-463;
  captures/21-rr-enforce.txt:4-11; src/docs/catalog/batch-job-v1.yaml:51-57;
  codex-notes-r4.md M08;
  captures/24-rr-forensics.txt:30-31 (separate outcome links to Intended).
  rr-poc/rr_poc.py:87-115; src/pkg/api/runai/v1alpha1/structure.go:61-89.
- embed: captures/21-rr-enforce.txt:4-5; captures/24-rr-forensics.txt:27-31. Optional
  SOURCE handle: src/docs/catalog/batch-job-v1.yaml:51-57.
- number audit: The named Intended/Executed pair and supersedes link are captured. pendingOp
  persistence and the UID/existing-false tests are VERIFIED in code, not
  printed in the transcript. No full Intended body or executed JSONPatch
  request was saved here; do not fabricate one. The receipt says
  /spec/suspend and true; the catalog source uses .spec.suspend and string
  "true".
- fx: `data-fxmode="popTimelines"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .timeline .chip/.arr for Intended, pendingOp, guarded patch and
  Executed. Receipts are nodes in this one path; keep the supersedes link
  static. No gate physics or restart animation.

- reference notes: Rev2 translates a single plain dotted boolean assignment from local catalog
  YAML; it does not call the Karta Go executor. Intended and outcome are
  separate ConfigMaps. The outcome records supersedes pointing to Intended.
  Source-only write-failure and recovery detail moves to s30b. Retained
  source explanation: for an eligible, ready Enforce target, write an
  Intended ConfigMap; then save pendingOp in rr-state. failure of either
  write prevents that patch. the Job JSONPatch tests UID and an existing
  /spec/suspend=false, then adds true. it does not test resourceVersion,
  metric eligibility, sample age, status, or other fields. a successful
  patch command attempts a separate Executed receipt with supersedes
  pointing to Intended. Executed attests command success, not controller
  convergence.

### s26. observe first, then enforce
- one idea: WouldAct spends neither action allowance nor the action budget.
- inset cards: Observe records decisions; Enforce can patch. Preflight
  checks patch permission; --impersonate selects the check/patch
  identity.
- beats:

  1. Observe: WouldAct x4 across trainer-e/f; both ended unsuspended.
  2. Enforce: each got Intended/Executed; both ended suspended.
  3. WouldAct spends neither action allowance nor per-loop patch budget.
  4. Observe still writes records and runs earlier state checks.
- scope: In this run, preflight precedes WouldAct; actionReady may be
  false. Earlier resume, escalation, handle and recovery checks still
  run.
- visual: One Observe/Enforce comparison card using the captured phase counts and
  final fields. Add a short source gate arrow inside the card. No two full
  terminals or generalized dry-run badge.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: EXECUTED for phase counts/final fields. VERIFIED for allowance, gate order
  and actionReady; its JSON body was not captured.
- data: captures/20-rr-observe.txt:1-11; captures/21-rr-enforce.txt:1-11;
  rr-poc/rr_poc.py:268-272,364-432,463; rr-poc/run-rr-phase.sh:11,34,40.
- embed: captures/20-rr-observe.txt:3-7; captures/20-rr-observe.txt:10-11;
  captures/21-rr-enforce.txt:4-7; captures/21-rr-enforce.txt:10-11.
- number audit: WouldAct x4 = trainer-e twice plus trainer-f twice. The SkippedNoHandle row
  is a fifth receipt, not a fifth WouldAct. Enforce has one
  Intended/Executed pair per trainer. Final suspend fields are false/false,
  then true/true. Spending no cap and the preflight branch are VERIFIED
  semantics; a WouldAct JSON body was not captured.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .fx-pill on complete Observe/Enforce cells and the source gate
  caption. Do not animate a cap being consumed during Observe.

- reference notes: Without --impersonate preflight assumes ready=true. The budget counts
  successful patch calls per loop. WouldAct is before Enforce permission-refusal and budget gates, but after resume/recovery/handle logic. Preserve
  this as a bounded branch demonstration, not proof Observe is read-only.
  Retained source explanation: EXECUTED: Observe logged WouldAct twice for
  trainer-e and twice for trainer-f; both were unsuspended at the final
  check. subsequent Enforce logged an Intended/Executed pair for each and
  both were suspended. VERIFIED: for these supported candidates, preflight
  runs with the supplied --impersonate identity; WouldAct carries
  actionReady, even when false. the Observe branch exits before the
  permission-refusal/budget/write path. it writes receipts and saves state.
  resume detection, no-rearm escalation, missing-handle checks and startup
  pending recovery occur before that gate; Observe is not a read-only or
  unconditional WouldAct mode.

### s27. the next eligible decision escalates
- one idea: after the recorded resume, this rule escalates instead of suspending.
- inset cards: Escalation here is a receipt and log, without notification delivery or an
  approval service.
- beats:

  1. Kyverno: active=1, startTime reset at +0.095s; re-suspended at +123.947s.
  2. RR logged ResumeDetected at +0.692s; actor unknown.
  3. It logged EscalatedNeedsHuman at +124.526s.
  4. At the final +300.147s check: suspend=false, active=1.
- scope: Equivalent Job predicate; wider RR query, shorter polling. No speed
  comparison. Final check is sampled, not continuous health.
- caption: Two sequential runs on different Jobs, replayed on one relative clock with each resume at zero.
- visual: One paired replay on a shared 0..310s axis. Kyverno/trainer-a is
  above RR/trainer-f. Enter with all captured observations held. Play
  replays both lanes once. It does not query the cluster or run the rules.
- fallback: One held two-row table with the same five observations, exact
  offsets, tier chips, caption and scope. No play button, animation frames
  or scrolling. The table replaces the tracks, not an extra visual.
- tiers: Place an EXECUTED chip beside each lane header, qualified as computed
  from captured timestamps. These are observation offsets, not precise
  action latencies. VERIFIED belongs beside the query/polling scope.
  Label the axis/replay as compressed display time. Keep all qualifiers
  visible in held and static views.
- data: captures/22-rr-anti-trap.txt:2-10; captures/30-lab2-kyverno-phase.txt:5-8;
  rr-poc/rr_poc.py:237,253-264,372-403; rr-poc/rules.yaml:12-21;
  rr-poc/run-rr-phase.sh:54; INTEGRATION-LOG.md:6-9,467-471;
  wow/demo-component.js:10-76; wow/skeleton-head.html:138-153.
- embed: captures/30-lab2-kyverno-phase.txt:5-8;
  captures/22-rr-anti-trap.txt:2-3; captures/22-rr-anti-trap.txt:6-7;
  captures/22-rr-anti-trap.txt:9-10. Keep raw lines in evidence detail.
  The track chips below are parsed summaries, not verbatim console output.
- number audit: EXECUTED, computed from captured timestamps. Kyverno's
  origin is 18:21:10.688 UTC: 18:21:10.783 minus origin = 0.095s;
  18:23:14.635 minus origin = 123.947s. RR's origin is 18:28:43.906 UTC:
  18:28:44.598 minus origin = 0.692s; 18:30:48.432 minus origin =
  124.526s; 18:33:44.053 minus origin = 300.147s. The first and last
  state checks are samples, not continuous health. Do not use capture
  30's rounded 0.0s/123.9s narration as exact offsets. duration=12000ms
  and scale=[0,310] are PRESENTATION SETTINGS, not measured outcomes.
- fx: `data-fxmode="dualLane"`; stock customFx.dualLane; held on entry, click to replay both lanes once.
- fx DOM: Embed the script below verbatim. Provide two .lane elements in
  Kyverno/RR order, with classes kyverno and rr and one empty .track each.
  Provide one real button.playbtn with aria-label="Play or reset both runs"
  and adjacent help text "Play/replay; click during playback to reset."
  Use authored .lhead/.lname headers: "kyverno / trainer-a" and
  "rr / trainer-f". Both are lab2 runs. cfg.name is metadata; this engine
  does not render it. Keep headers, axis, caption and scope outside tracks.
- lane data: Copy the complete script into s27. Both lanes use the same
  scale and duration. t is seconds since EACH run's scripted resume
  marker. Resume at t=0 is an axis/header label, not an extra event that
  would collide with the first sample. Every listed event is captured.

  ```html
  <script type="application/json" class="lane-data">
  {
    "duration": 12000,
    "scale": [0, 310],
    "lanes": [
      {
        "name": "Kyverno - trainer-a, lab2",
        "events": [
          {
            "t": 0.095,
            "label": "active=1; startTime reset",
            "cls": "c-run early"
          },
          {
            "t": 123.947,
            "label": "re-suspended",
            "cls": "c-sus"
          }
        ]
      },
      {
        "name": "RR - trainer-f, lab2",
        "events": [
          {
            "t": 0.692,
            "label": "ResumeDetected",
            "cls": "c-run early"
          },
          {
            "t": 124.526,
            "label": "EscalatedNeedsHuman",
            "cls": "c-init"
          },
          {
            "t": 300.147,
            "label": "running at check",
            "cls": "c-run final"
          }
        ]
      }
    ]
  }
  </script>
  ```
- lane layout: Both tracks have equal width and aligned endpoints. Author
  one shared axis labeled "resume = 0s" and "310s" above them. Do not
  insert children into .track; the engine appends event/tmark pairs there.
  Its timestamp marks already print all five exact offsets. Reveal the
  corresponding beat text at those chips, not in a duplicate bullet column.
  No moving playhead or scrubber is supplied or required.
- fx steps (presentation ms, not lab seconds; once per button activation):
  1. Entry: build once and show all five event chips. No autoplay. Headers,
     tiers, time marks, caption and scope are static throughout replay.
  2. Click, 0ms: reset both lanes, hide their chips and start fresh timers.
     The button shows its playing state. Timestamp marks remain visible.
  3. 60ms: both early chips reveal. The engine clamps each timer to at
     least 60ms; +0.095s and +0.692s therefore appear together. Exact
     offsets stay readable. Do not portray this as equal detection speed.
  4. About 4798ms: Kyverno's +123.947s re-suspension chip reveals. About
     4820ms: RR's +124.526s escalation chip reveals. They nearly coincide
     under compression; this is not a latency contest.
  5. About 11619ms: RR's +300.147s final sampled-state chip reveals.
     At 12400ms the button becomes replay. All five chips stay held.
  6. Click while playing: cancel timers and hide both lanes' chips. This
     resets; it does not pause. The next click restarts both from zero.
     Leaving cancels timers; re-entry restores the full held view.
  7. Mobile/reduced-motion/print: the shared lifecycle adapter holds the
     data and disables replay; show the static table in place of tracks.
     Mid-replay mode changes also cancel timers and restore held results.
- lane styles: Use these scoped overrides. Disable the stock pop transform
  so positions stay on their timestamps. Anchor both early labels left;
  anchor only RR's final label right. Neither lane has forced opacity.
  The renderer has no collision avoidance; verify the two-row fit.

  ```css
  #s27 .lane .ev.on { animation:none; }
  #s27 .lane .ev { transition:opacity .18s; }
  #s27 .lane .ev.early { transform:none; }
  #s27 .lane.rr .ev.final { transform:translateX(-100%); }
  #s27 .lane .tmark:nth-child(2) { transform:none; }
  #s27 .lane.rr .tmark:last-child { transform:translateX(-100%); }
  #s27 .playbtn { animation:none; }
  ```
- held end state: Kyverno shows the +0.095s active=1/startTime-reset sample
  and the one +123.947s re-suspension. RR shows +0.692s ResumeDetected,
  +124.526s EscalatedNeedsHuman and the +300.147s suspend=false/active=1
  check. Show observations as points, not continuous state bars. No
  observation at 310s or further Kyverno re-suspension is invented.
- lane fallback: Build the two-row static table from the same lane-data at
  authoring time; include the final suspend=false/active=1 fields from the
  beat. Use motion-live on the tracks/button/help, motion-static on the
  table. Preserve the caption, scope and tiers once, outside both wrappers.
  The shared lifecycle adapter is required; stock dualLane alone does not
  implement print/media-change handling. If projector labels overlap, use
  this held table on desktop too. No new customFx is required.
- handoff: escalation is one recorded decision; inspect the skipped actions next.
- reference notes: This paired replay resolves the hook; s32 remains the
  ending. It supersedes the earlier held-Kyverno/animated-RR choice.
  Resume attribution remains transition-based and actor-unknown. A human
  icon denotes the scripted resume only. RR compares recorded weSuspended
  with current spec; managers of spec.suspend are supporting metadata.
  ResumeDetected records allowance epoch 1 without renewed allowance. Escalation
  means a ConfigMap receipt and a log, not notification delivery, human
  acknowledgement or approval. These are sequential lab2 runs on different
  Jobs. The Job predicate is equivalent; RR queries more kinds and polls
  more often. Neither trace proves continuous health. Engine scheduling
  is max(60, t/310*12000) ms with completion at duration+400; its .tmark
  text is always visible. Compression must not imply a common wall clock,
  a controlled speed comparison, or identical scheduler behavior.

### s28. named reasons to skip an action
- one idea: missing handles and failed permission checks produced receipts.
- beats:

  1. web-app Deployment: SkippedNoHandle.
  2. Job trainer-g's patch grant was removed before evaluation.
  3. It recorded SkippedNotReady.
  4. After restoration: Intended/Executed, then suspend=true.
- scope: EXECUTED skip cases. Preflight checks patch permission, not full readiness.
  No action was in flight at revocation.
- visual: One two-row outcome table: unsupported handle / denied permission then
  restored. Use phase names from captures; do not invent receipt bodies or
  add an earlier Kyverno experiment panel.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: EXECUTED on phase names and restored suspend field. VERIFIED on missing
  handle and preflight meaning. Label the capture header revoked mid-flight
  as superseded if quoted.
- data: captures/20-rr-observe.txt:5; captures/23-rr-rbac.txt:2-15;
  src/docs/catalog/apps-deployment-v1.yaml; rr-poc/rr_poc.py:99-115,268-272,405-428;
  ../03-experiments-executed.md:110-117; rr-poc/run-rr-phase.sh:75-81.
- embed: captures/20-rr-observe.txt:5-5; captures/23-rr-rbac.txt:2-4;
  captures/23-rr-rbac.txt:6-6; captures/23-rr-rbac.txt:10-15.
- number audit: Skip names, restoration and final trainer-g suspend=true are captured. Use
  line 15 even though wc -l reports 14: the last line lacks a newline. Omit
  the inaccurate mid-flight header at line 1; permission was removed before
  this evaluation. No expanded reason JSON or SelfSubjectAccessReview
  response was captured.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .fx-pill on the no-handle row, denied-permission cell and restored
  cell. Names are captured phases, not synthesized receipt JSON.

- reference notes: The earlier Kyverno RBAC silent-stall comparison remains on reference s31
  with its own experiment scope. Both executors require action permissions.
  Capture 23 prints phases, not reason JSON; any expanded schema belongs to
  reference detail. Retained source explanation: EXECUTED: eligible web-app
  Deployment -> SkippedNoHandle. the local catalog has no suspend handle for
  it. trainer-g's action grant was removed before evaluation ->
  SkippedNotReady; after restore -> Intended/Executed, then suspend=true. no
  action was in flight at revocation. VERIFIED: preflight asks only whether
  the impersonated identity can patch that kind in the namespace. source-generated reason includes identity, verb and kind; it is not a full
  admission or readiness check. comparison footnote: an earlier Kyverno
  mutation experiment stalled with ready=true and report drift when RBAC was
  missing. different run, not this RR preflight fault test. both executors
  require action permissions.

### s29. the action record survives deletion
- one idea: the captured Executed receipt survives with its decision evidence.
- inset cards: Receipt phase names a decision outcome, not a workload status. supersedes
  names the earlier Intended record.
- beats:

  1. After trainer-e deletion, four receipt ConfigMaps remained.
  2. They were WouldAct x2, Intended and Executed.
  3. The Executed body retained target identity, query/sample, patch and
     operation linkage.
  4. The record survived this deletion; it is mutable.
- scope: One Executed body, not every phase's schema. One query-result sample, not
  window history. No actor identity or rule revision is recorded.
- visual: One retention card: four surviving names plus selected fields from the
  actual Executed body. Label selected fields; full JSON opens as evidence.
  Remove phase-schema variants and repeated gate/recovery diagrams.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: EXECUTED on surviving names and selected literal fields. VERIFIED on field
  meanings and mutability; no permanent-retention badge.
- data: captures/24-rr-forensics.txt:1-32;
  rr-poc/rr_poc.py:49-50,118-166,210-223,241-248,377-387,415-420,434-447;
  rr-poc/run-rr-phase.sh:34,40; codex-notes-r4.md M10.
- embed: captures/24-rr-forensics.txt:2-8; captures/24-rr-forensics.txt:10-32. For
  compact selected fields use captures/24-rr-forensics.txt:27-31 verbatim
  and label the omission.
- number audit: Four ConfigMaps: two WouldAct, one Intended, one Executed. Full body is
  lines 10-32, including the final brace; data-excerpts.md stops at the
  opening brace. Preserve UID, resourceVersion "1908", epoch 0, evidence
  value "1", sampleTime 1789410393.863 and all other JSON fields exactly if
  shown. Body time 18:26:34.432 is receipt construction time, not the log's
  18:26:34.539 completion marker. The PromQL is the wider RR query, not the
  Job-only Kyverno query.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .fx-pill on the surviving-name group and selected Executed fields.
  Keep the deletion context static; no disappearing workload needed.

- handoff: the receipt survived this deletion; the prototype still has limits.
- reference notes: targetResourceVersion is the pre-patch list value,
  not a guard or post-write version. time is receipt construction time.
  rule and karta contain names, not revision/hash identities.
  phase-specific schemas differ. No workload ownerReference is set.
  Receipts can be deleted directly; reused run IDs can overwrite them. Full field
  list and variants remain source detail. Retained source explanation:
  EXECUTED: after trainer-e deletion, four receipt ConfigMaps remain:
  WouldAct x2, Intended, Executed. the full Executed body carries rule
  name, phase, workload namespace/name/kind/UID, targetResourceVersion,
  time, epoch, evidence {promql, value, sampleTime}, karta,
  patchPointer, patchValue, operation and supersedes. supersedes names
  the separate Intended receipt. VERIFIED: targetResourceVersion is from
  the pre-patch list, not a patch precondition or the post-write
  version. evidence is a PromQL result sample, not the two-minute sample
  history. time is receipt construction time. rule and karta are names;
  no actor identity, rule revision or description hash is recorded. this
  is captured survival, not an immutable archive. Phase variants:
  WouldAct includes actionReady but no operation/supersedes;
  ResumeDetected includes epoch, note and suspendFieldManagers without metric evidence;
  handled fact/list failures have scope=rule and no workload identity.
  These variants are not the captured Executed schema.

### s30. what the runner actually implements
- one idea: The prototype is one narrowly scoped runner.
- inset cards: Ambient identity: the runner's default credentials. Impersonation changes
  only the permission-check and workload-patch identity.
- beats:

  1. A single Python runner loads local rules and catalog YAML at startup.
  2. Reads, state and receipts use ambient credentials; preflight/patch
     may impersonate.
  3. It translates one dotted boolean suspend assignment, keyed by Kind.
  4. It lists Job/CronJob/Deployment; executed actions were Jobs.
- scope: VERIFIED scope. No RuntimeRule watch, leader election or state-writer
  coordination. Without --impersonate, preflight is skipped.
- visual: One runner-boundary diagram: startup files -> Python loop -> reads/records
  and impersonated actions. Keep source-only boundaries labeled; remove the
  four-view tour.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: VERIFIED on implementation/configuration scope. Captured Job actions are
  the action coverage, not every printed handle name.
- data: rr-poc/rr_poc.py:65-81,87-115,124-207,210-250,262-319,333-467;
  rr-poc/rules.yaml:16-23; captures/20-rr-observe.txt:2;
  captures/21-rr-enforce.txt:3; rr-poc/run-rr-phase.sh:34,40,54,78,81;
  codex-notes-r4.md M08/M10; codex-notes-r5.md R5-M03.
- embed: captures/20-rr-observe.txt:2-2 if displaying the runner startup record.
  Otherwise use the VERIFIED source boundary diagram.
- number audit: r2, reArmOnResume=False, actionsPerEpoch=1 and the impersonation identity
  are printed startup configuration. Seven printed handle names do not mean
  seven action kinds were tested. Source list support is
  Job/CronJob/Deployment; the captured action pairs were Jobs. Never convert
  the startup catalog list to an execution coverage count.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .fx-pill on startup inputs, runner and each identity branch. Static
  arrows show the boundary; no pod scheduling animation.

- reference notes: action.type is not dispatched; actionsPerEpoch is read/logged but the gate
  is boolean. State uses rule name/UID, without rule UID/revision or
  description hash. Budget counts successful patches, not attempts. Fixtures
  set suspend=false; an absent field fails that JSONPatch test. These
  configuration limits remain part of the prototype disclosure. Retained
  source explanation: identity/process: reads, state and receipts use
  ambient credentials. --impersonate covers preflight and workload patches
  only; without it, preflight is skipped. one startup-loaded rule, no CRD
  watch, informers, leader election or state-writer coordination. state uses
  rule NAME plus workload UID, without a rule UID/revision or description
  hash. adapter/configuration: a Python translator reads local catalog YAML;
  it does not call the Karta Go executor. lookup uses Kind, not full GVK;
  one plain dotted boolean suspend action. list support is
  Job/CronJob/Deployment; executed actions were Jobs. action.type is not
  dispatched; actionsPerEpoch is read/logged but the gate is boolean. budget
  counts successful patch calls per loop, not attempts. fixtures explicitly
  set suspend=false; an absent suspend field does not satisfy that JSONPatch
  test.

### s30b. where an outcome can be lost
- one idea: The prototype does not guarantee a retained receipt for every failure.
- inset cards: Fault-injected means deliberately forced to fail in a test; these
  failure/recovery branches were not tested that way.
- beats:

  1. Outcome, resume and recovery writes can fail without blocking later
     state changes.
  2. Pending recovery inspects listed UID/spec.suspend; it was not crash-tested.
  3. Handled query errors attempt FactsUnavailable; malformed JSON can
     crash without a receipt.
  4. No automatic wedge detection or distributed exactly-once guarantee
     was demonstrated.
- scope: VERIFIED code limits; fault paths NOT TESTED. Neither executor demonstrated
  wedge detection. Executed attests patch success only.
- visual: One incomplete-outcome diagram: patch result -> best-effort receipt, with
  separate uncertain-state and query-decode exits. Never animate these exits
  as captured successes. No full enum inventory.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: VERIFIED on source limits; NOT TESTED beside nonempty recovery, injected
  failures and wedge detection. No EXECUTED chip on any hypothetical failure
  result.
- data: rr-poc/rr_poc.py:65-81,87-115,124-207,210-250,262-319,333-467;
  rr-poc/rules.yaml:16-23; captures/20-rr-observe.txt:2;
  captures/21-rr-enforce.txt:3; rr-poc/run-rr-phase.sh:34,40,54,78,81;
  codex-notes-r4.md M08/M10; codex-notes-r5.md R5-M03.
- embed: none: VERIFIED source limits and NOT TESTED failure/recovery branches.
- number audit: FactsUnavailable, UnknownOutcome and other failure names are code labels,
  not captured receipts. Do not synthesize receipt IDs, timestamps, JSON
  bodies or a successful recovery transcript for them.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Use .fx-pill on source-limit annotations. The failure exits are already
  drawn and explicitly NOT TESTED; revealing a label must not enact a
  successful recovery.

- handoff: Those limits define the work still to build.
- reference notes: Recovery runs after the first successful facts query. A missing UID,
  including an omitted target after a list failure, yields UnknownOutcome.
  A listed suspended target yields ExecutedVerifiedAfterRestart; that name
  does not establish actor identity or controller convergence.
  Untested paths: timeout/UnknownOutcome, FailedPrecondition, FailedVisible,
  intent/state/outcome write failures, AbortedStateUnpersisted,
  FactsUnavailable, TargetsUnavailable and SkippedBudget. Receipts can be
  mutable or overwritten by reused run IDs. This slide owns failure/recovery
  disclosure; s25 owns the two pre-patch gates.

---

## act 5 - the next build

### s32. build the action protocol
- one idea: propose an explicit action protocol around the shared facts.
- inset cards: Composition connects the systems through shared facts or explicit requests.
- beats:

  1. Build a dedicated runtime action protocol.
  2. Make resume allowance and action records explicit.
  3. Keep optional Kyverno composition.
- scope: INFERRED proposal, not a released controller. The lab does not prove that
  equivalent behavior cannot be built on top of Kyverno.
- visual: Label the box "proposed action protocol"; highlight it once, then hold.
  This is the only main ending.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: INFERRED beside the recommendation, composition arrow and proposed box. No
  completed-governor badge.
- data: INTEGRATION-LOG.md section 6b; captures/21-rr-enforce.txt;
  captures/22-rr-anti-trap.txt; rr-poc/rr_poc.py:372-403,434-463.
- embed: none: INFERRED recommendation, not command output or a measured result.
- number audit: The proposed protocol box and composition arrow must not acquire an EXECUTED
  chip from the preceding examples.
- fx: `data-fxmode="popPills"`; stock handler; reading-order reveal once per entry.
- fx DOM: Only the proposed action-protocol node is .fx-pill. Keep the rest of the
  s2 topology held. No docking/morph between slides.

- reference notes: Retained source explanation: INFERRED recommendation: build a dedicated
  runtime execution protocol with optional Kyverno composition. make resume
  allowance and persisted action records explicit parts of that contract.
  this lab does not prove that an equivalent protocol cannot be built on top
  of Kyverno.

---

## references - outside the main story

These views retain the comparison and reproducibility material. They are
available from evidence links, never as slides after the closing decision.
Open reference pages in a separate tab through data-reference links.
The adapter holds the caller before opening. Returning does not call show()
or replay its effects.

### s31. compare the evidence
- role: reference only; outside the main slide sequence.
- one idea: inspect the evidence tier and scope of each comparison.
- beats:

  1. Choose one comparison question.
  2. Read Kyverno and RR outcomes for that row.
  3. Keep each result's evidence tier and scope.
  4. Return to the calling slide.
- scope: Reference only. Separate runs and the earlier permission experiment retain
  their own labels.
- visual: One selected reference row; switching questions replaces that row
  immediately. No autoplay.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: INTEGRATION-LOG.md section 6; rr-poc/rr_poc.py:138-166,417-463;
  captures/20-rr-observe.txt; captures/24-rr-forensics.txt.
- embed: Metric: captures/10-v2-http-timeline.txt:1-3 and
  captures/21-rr-enforce.txt:4-7. Error:
  captures/31-cel-error-repro.txt:9-28 and
  captures/31-cel-error-repro.txt:39-40. Resume:
  captures/30-lab2-kyverno-phase.txt:5-8 and
  captures/22-rr-anti-trap.txt:2-10. Forensics:
  captures/17-kyverno-forensics.txt:17-25 and
  captures/24-rr-forensics.txt:2-32. Skips: captures/20-rr-observe.txt:5-5
  and captures/23-rr-rbac.txt:2-15. Observe:
  captures/20-rr-observe.txt:3-11. Earlier Kyverno permission row:
  ../03-experiments-executed.md:114-117 is an earlier execution note, not a
  raw transcript here.
- number audit: Metric row: captures/10-v2-http-timeline.txt:1-3 and
  captures/21-rr-enforce.txt:4-7; error row:
  captures/31-cel-error-repro.txt:9-28 and
  captures/31-cel-error-repro.txt:39-40; resume row:
  captures/30-lab2-kyverno-phase.txt:5-8 and
  captures/22-rr-anti-trap.txt:2-10; forensics:
  captures/17-kyverno-forensics.txt:17-25 and
  captures/24-rr-forensics.txt:2-32; skips: captures/20-rr-observe.txt:5-5
  and captures/23-rr-rbac.txt:2-15; Observe:
  captures/20-rr-observe.txt:3-11. Keep empty UR message VERIFIED,
  repetition INFERRED, FactsUnavailable source-only, and outcome
  verification PARTIAL. Do not copy the raw log table's broader evidence
  labels without these limits.
- fx: `data-fxmode=""`; empty mode; static reference, no runFx call required.
- fx DOM: Reference only. Use native radio inputs and CSS :checked selectors to show
  one comparison row. No effect handler or custom event bus. Default to
  the metric-conditioned-action row.

- reference notes: Row payloads remain in the retained source detail below and INTEGRATION-LOG
  section 6. Keep FactsUnavailable source-only with the JSON-parse caveat;
  missing-handle Kyverno burden inferred without a control; Observe non-read-only; outcome verification partial and non-convergent; repetition
  beyond the observed repeat inferred. No cell inherits an EXECUTED tier
  from its neighbor. Retained source explanation: compare the tested
  behaviors, one question per row: metric-conditioned action (both EXECUTED)
  / condition failure (Kyverno EXECUTED; RR FactsUnavailable VERIFIED only,
  JSON-parse caveat visible) / user resume (re-suspend vs escalate) /
  provenance after deletion / missing permission (Kyverno from the earlier
  experiment) / missing handle (Kyverno authoring burden INFERRED, no
  control; RR EXECUTED) / observe (Kyverno reporting semantics VERIFIED, RR
  EXECUTED) / outcome verification (no controller-convergence check
  demonstrated on either; RR PARTIAL: patch success only, untested recovery
  and best-effort later receipt writes). RR observe means WouldAct spends no
  action cap, not a read-only runner. the deletion body is an Executed
  example, not every phase's schema. indefinite repetition stays INFERRED.
  use the log table's evidence with the rev2 qualifications from s24-s30.

### s33. inspect the evidence
- role: reference only; outside the main slide sequence.
- one idea: the record is available, with explicit rerun limits.
- inset cards: Settle test: the old script incorrectly waited for startTime to clear before
  continuing.
- beats:

  1. Inspect the log, captures, manifests and prototype.
  2. Historical scripts need fresh names/state and a corrected settle
     test.
  3. Kyverno filing still needs false/true/mutation-error controls.
  4. The Kubernetes draft is a historical duplicate of #134521.
- scope: Reference only. Controls remain unexecuted. Verify public links at
  publication time.
- visual: Static evidence tree. No reveal or closing animation.
- fallback: The held single visual, the same four-or-fewer beats, and scope. Retain one
  necessary definition. No stacked animation frames or scrolling.
- tiers: Use the evidence tiers stated in beats/scope. Source mechanisms remain
  VERIFIED; an animated example is not an executed test.
- data: INTEGRATION-LOG.md section 7; upstream-drafts.md;
  captures/22-rr-anti-trap.txt; captures/30-lab2-kyverno-phase.txt.
  public branch and Pages links must be verified at publication time.
- embed: none: evidence links and reproduction notes, not an executed terminal. The
  required raw excerpts are pinned on the slides that use them.
- number audit: Reproduction commands in upstream-drafts.md are fixture/source text unless a
  matching capture is explicitly linked. False/true/mutation-error controls
  remain NOT TESTED. #134521 is a prior-art issue identifier, not an
  execution count; source linkage is documented in codex-notes-r3.md:34-54.
- fx: `data-fxmode=""`; empty mode; static reference, no runFx call required.
- fx DOM: Static reference evidence links; no effect attributes on descendants.

- reference notes: Retained source explanation: evidence index: log, captures, manifests, rr-poc and upstream drafts on the fork branch (one kyverno issue ready after
  controls, one k8s historical duplicate of #134521). the scripts are
  historical records: fixed names and run IDs can mix evidence, and the old
  settle gate is invalid. fresh names/state and a corrected gate are needed
  for a clean rerun. false/true/mutation-error controls remain unexecuted
  before filing the Kyverno issue.

---

## build handoff

- Final round 9 count: 40 authored IDs, 38 main slides plus s31/s33
  reference views. New companions: s17b and s30b. Keep the specified order.
- Round 7 splits resolved: s17's second policy is s17b; s30's failure limits
  are s30b. s11 accounting, s25 recovery detail and s29 schema variants are
  source/reference detail rather than extra projected lessons.
- s27 owns the one interactive replay. Preserve its separate time origins,
  final-sample limit and wider-query/shorter-polling qualifier.
- Implement the round 10 lifecycle/static adapter before animation QA.
  Native reference pages avoid a new in-deck modal/navigation subsystem.
- Render at projector size to verify fit. Text budgets are a spec check,
  not proof of pixel fit. Do not restore reference notes as slide content.
- Round 11 pins every capture/source excerpt. Preserve its raw-versus-parsed
  labels and unsafe-recorder-caption exclusions during the build.
- Round 13 checked the spec text. Repeat the public-text and link checks
  on the rendered deck and any embedded excerpts before publication.
