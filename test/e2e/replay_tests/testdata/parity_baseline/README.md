<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# Parity baseline

These fixtures are the raw output of the jq engine, captured at commit
`a0166142`, the last commit before the CEL adoption. For every recorded
object under `test/e2e/recorded_data`, the generator ran every read the
component API offers and applied one deterministic write per write path:
the pod template, the pod spec, the pod metadata, and every fragmented
field in its own update call, so each declared fragment accessor is
exercised separately and a read-only field fails on its own. It then
serialized the results: per-component reads, per-field write outcomes,
and the full document after writes, after suspend, and after resume.
Both engines refuse a write to a field that has no write definition, so
those outcomes agree too.

`parity_test.go` replays the identical operations through the CEL
engine and compares against these files. The snapshot logic in the test
and in `generator/main.go` is the same function body; an edit to one
requires editing the other and regenerating.

The historical fixtures cover 56 recordings. Five Trainer recordings were
added later; jq had no TrainJob catalog or reference reader. They run in
the status, mutation, tree extraction and Draft suites, not this historical
comparison. `TestParityBaselineCoverage` pins that exception list and fails
if any other recording lacks a baseline or any baseline loses its recording.

## Regenerating

The generator needs the jq engine, which no longer exists on this
branch, so it runs in a worktree of the pre-CEL commit:

```sh
git worktree add ../baseline-src a0166142
mkdir ../baseline-src/paritydump
cp test/e2e/replay_tests/testdata/parity_baseline/generator/main.go ../baseline-src/paritydump/
cd ../baseline-src
go run ./paritydump <repo>/test/e2e/recorded_data <repo>/test/e2e/replay_tests/testdata/parity_baseline
```

## Accepted divergences

The suite requires byte-for-byte agreement except for the corrections below.
Each correction is enforced by a narrow
rule in the test; an unlisted divergence fails.

1. Matched statuses (92 values). The jq catalog answers `Undefined` for
   states it has no mapping for (86 values), and the jq jobset mapping
   calls a jobset with active but not yet ready replicas `Initializing`
   (6 values). Where the engines disagree, the cel answer is required to
   be exactly the state label the recorder captured from the live
   cluster; any other cel answer, including an extra matched status,
   fails. `status_test.go` independently pins the cel statuses.
2. Empty Grove scaling groups (22 states). jq returned errors for instance
   IDs, scale and extracted instances. The corrected catalog returns no IDs
   and empty maps. The suite checks that the recorded scaling-group list is
   absent or empty first. A read error or invented instance fails.
3. Pod document after writes (20 documents). The jq pod definition
   writes the pod template over the whole document (`podTemplateSpecPath: .`),
   dropping `apiVersion` and `kind`, so the jq engine's own `GetResource`
   fails validation after any template write. Against this baseline
   error cel is required to produce a document that still carries
   `apiVersion`, `kind`, and the parity scheduler write; a cel error
   fails. The full pod documents after writes are separately pinned
   byte-for-byte by the golden mutation suite.
4. Null elision (33 keys). Writing a typed template that has no
   containers, the jq engine stores a literal `containers: null` where
   the CEL merge patch removes the key. This suite tolerates that historical
   difference by eliding null-valued keys for comparison. JSON null and an
   absent key are not generally interchangeable. A non-null value against
   a null or a missing key still fails.
5. Retained operator fields. Default Merge keeps Knative's
   `spec.template.spec.containerConcurrency` and `timeoutSeconds`, and
   KServe's `spec.predictor.model.storageUri` and `modelFormat`. The old
   typed replacements dropped them. The expected document restores only
   these paths from the original recorded input, never from the CEL output.
   A changed value, a missing field, or an extra unrelated field fails.
6. KServe transformer scheduler. The scheduler write succeeds before the
   metadata write. jq's broad metadata replacement then loses the scheduler.
   Merge must retain the exact requested `karta-parity-scheduler` value at
   `spec.transformer.schedulerName`.

The historical YAML files and generator are unchanged. The comparison builds
its corrected expectations in memory. The regular mutation goldens separately
check explicit Replace compatibility, and the Draft replay compares complete
objects against independently built leaf-edit results.
