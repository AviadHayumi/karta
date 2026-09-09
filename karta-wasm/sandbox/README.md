<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# karta in a sandbox

run karta inside an isolated wasm instance ( the browser door is documented in [karta-wasm](../README.md) ) instead of in the host process. the
guest cannot reach host memory or state , so it is safer. the cost is time -
every call boots a fresh instance and runs interpreted.

## how it fits

![one core , two doors](../../docs/assets/karta-wasm-doors.png)

the karta-wasm module has one shared core and two doors :

```
karta-wasm/
  core/            the karta calls , plain go , no build tag
  main_js.go       browser door  ( GOOS=js )     - window.karta
  main_wasip1.go   wasi door     ( GOOS=wasip1 ) - stdin -> core.BuildTree -> stdout
  sandbox/         the host that runs the wasi door under wazero ( own go.mod )
```

there is no separate guest with its own protocol. the wasi door is 30 lines : it
reads {definition, workload} on stdin , calls the SAME core.BuildTree the browser
door calls , and writes the tree to stdout. the sandbox host runs that wasm.

## use it

the host has two calls, both run in a fresh isolated instance :

```go
box, _ := sandbox.New(ctx, wasm)   // compile the wasi door once
defer box.Close(ctx)

tree, _ := box.BuildTree(ctx, definitionJSON, workloadJSON)                  // read the tree
obj,  _ := box.Suspend(ctx, definitionJSON, workloadJSON)                    // suspend ( the definition's suspendActions )
obj,  _  = box.Resume(ctx, definitionJSON, string(obj))                      // resume
obj,  _  = box.SetField(ctx, workloadJSON, ".metadata.labels.team", `"ml"`)  // low-level single-field write
```

`BuildTree` returns the {data, error} envelope holding the workload tree.
`SetField` is the low-level write door - it assigns one jq path and returns the
mutated object. the typed, capability-checked UpdatePodTemplate is the
higher-level way.

the example builds the tree, PARSES it by walking every (component, instance),
then mutates a label - all in the sandbox :

```sh
make karta-wasi   # from the repo root : builds karta-wasm/karta-wasi.wasm
cd karta-wasm/sandbox
go run ./example ../karta-wasi.wasm \
  ../../docs/catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml \
  ../../docs/examples/quickstart/jobset.yaml
```

```
status: [Running]
  component replicatedjob  instance leader     pods:true
  component replicatedjob  instance workers    pods:true
after suspend : .spec.suspend = true
after resume  : .spec.suspend = false
after setField .metadata.labels.team=ml -> map[team:ml]
```

suspend and resume flip the workload's suspend intent ( the definition's
suspendActions ). the Suspended phase itself shows up once the operator reports
it in the workload's status.

swap in any catalog definition and workload to try another type.

## what it costs

measured on the pod definition , fresh instance per call , same machine.
sandboxed = run through the wasm , native = call core.BuildTree in process.

| workload | size | sandboxed | native | slower by |
|---|---|---|---|---|
| 1 container | 0KB | 31ms | 0ms | boot floor |
| 100 containers | 8KB | 48ms | 0.3ms | ~150x |
| 1000 containers | 81KB | 252ms | 3ms | ~80x |
| 5000 containers | 417KB | 1.4s | 17ms | ~80x |
| 20000 containers | 1.7MB | 5.2s | 67ms | ~77x |

two things to read here :

- a fixed ~30ms per call to boot a fresh instance - it dominates small workloads.
- once past the boot , the work runs about 80x slower than native go.
- compiling the wasm once at startup is a one-time ~3s.

so : the sandbox buys isolation and pays in time. use it where the safety is worth
it. for the browser the tab is already the isolation , so the browser door runs in
process and skips all of this. the fresh-instance numbers above are the floor - a
warm , reused instance would remove the per-call boot.
