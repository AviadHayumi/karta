<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Can we extend Kyverno without forking it ? yes , and it closes the gaps

One line : Kyverno decides , Karta acts and remembers. Kyverno never writes
the job. Its only write is a small Karta request object , and a Karta
executor spends that request once.

## How it works

- The metric rule moves from MutatingPolicy ( mutate-existing ) to a
  GeneratingPolicy. Same condition , same Prometheus query , 60s scan.
- Instead of patching the job , the generate expression creates one
  `ActionRequest` per job ( CEL `resource.Post` , create-only , keyed by the
  job uid ). If the request already exists , it does nothing.
- A Karta executor watches requests : writes Intended , patches the job
  with uid and resourceVersion tests , writes Executed. One attempt per
  request. A human resume is recorded and the request stays spent , so
  Kyverno's next tick finds the request and does nothing.
- The request has no owner , so it outlives the job and the policy.
- Kyverno needs one new right : get and create on `actionrequests` in one
  namespace. Stock Kyverno 1.19.1 , no patch , no fork.

## What we measured ( stock 1.19.1 , lab2 , 2026-09-17 )

| gap | before | with the seam |
|---|---|---|
| failures look like success | broken rule : work items Completed , nothing logged , job untouched , policy ready | broken rule : work items `Failed` with the trigger named and the real CEL error , retried , a fresh one every tick , PolicyError events , ERR log lines , zero requests , job untouched |
| a user resume gets reverted | re-suspended 124s and 175s after the resume | resumed by hand , recorded 1.4s later , still resumed after 300s of scans , no second request , no second patch |
| nothing remembers | job deleted : no report , no work item | job deleted : the request and its receipt chain ( Intended , Executed , resume detected ) still there |

Raw logs : captures 71 ( happy path , resume , deletion ) and 72 ( the clean
broken-rule control ). Manifests 08 , 09 , 10 and `rr-poc/ar_executor.py`.

## What still stands

- The policy still says `ready: true` while its rule fails every tick.
  Alert on the PolicyError events or the error metric , not on readiness.
- Failed work items are retried a few times and then deleted ; the next
  tick files a fresh one. They show the error , they are not the record.
- CEL `&&` absorbs an error when the other side is false. My first broken
  control ( `int(name) > 0 && dyn(Get) != null ? a : b` ) silently took the
  else branch and created a request until the Get side turned true. Keep
  fallible calls out of `&&` chains ; use the nested ternary.
- "resume detected" is an observation , not an identity. Who resumed needs
  an admission-time stamp ( stage 2 below ).
- The executor is a prototype : it does not resolve an Intended left over
  from a crash , it labels an uncertain patch response as Blocked , it
  appends a receipt every poll after the target is deleted , and it does
  not check the target uid on resume detection. Astra listed these ; they
  are executor bugs , not seam problems.

<details>
<summary>how we got here : the bakeoff</summary>

Six proposals ( five fresh agents with different lenses , plus astra who
knew the lab ) , ten reviewer personas , a council over six repos
( gatekeeper , kueue , keda , cert-manager , crossplane , kyverno main ) ,
one chair. Winner P4 , "one acting contract , Kyverno as one front end" ,
Borda 50 vs 44 ( P5 ) vs 38 ( astra's PA ). The council ranked the same
"intent object seam" first on its own , with cert-manager's
CertificateRequest and kueue's AdmissionCheck as precedents.

What the chair adds over the prototype : two CRDs ( a replaceable
WorkloadAction intent plus immutable ActionReceipts ) , Kyverno's native
generator path ( `generator.Apply` ) instead of `resource.Post` , a Go
actuator inside the Karta operator reusing the definitions' suspend
handles ( JobSet and MPIJob for free , SkippedNoHandle for dynamo ) , a
witness annotation in the same patch for crash recovery , and a stage 2 :
an admission-only MutatingPolicy that stamps `request.userInfo` on a
human resume , so the receipt names who did it.

Astra , after reading the ruling and the run , conceded P4 for the shipped
design and held two points : `resource.Post` is a valid extension point
( the run proves it ) , and at-most-one dispatch with a terminal Unknown
beats a blind retry. Files : `.context/kyverno-extend-bakeoff/RESULT.md` ,
`ASTRA-RECONCILIATION.md` , `proposals/` , `votes/` , `oss-council/` .

</details>

<details>
<summary>what is not covered yet</summary>

- the two-CRD generator.Apply variant the chair chose ( only the one-CRD
  resource.Post variant ran )
- crash recovery and the Unknown contract
- authenticated resume identity ( stage 2 )
- JobSet , and dynamo's missing suspend handle
- scale : one Prometheus call per job per tick today ; the chair suggests
  a GlobalContextEntry for the membership test
- upstream : none of this needs PR 17063 , but that PR still matters for
  anyone who keeps mutate-existing as the writer

</details>
