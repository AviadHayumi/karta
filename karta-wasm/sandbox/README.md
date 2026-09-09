<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# karta in a sandbox

run karta inside an isolated wasm instance instead of in the host process. the
guest cannot reach host memory or state , so it is safer. the cost is time -
every call boots a fresh instance and runs interpreted.

## how it fits

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

```go
box, _ := sandbox.New(ctx, wasm)   // compile the wasi door once
defer box.Close(ctx)

tree, _ := box.BuildTree(ctx, definitionJSON, workloadJSON)
// tree is the {data, error} envelope holding the workload tree
```

build the wasi door and run the example :

```sh
cd karta-wasm && GOOS=wasip1 GOARCH=wasm go build -o karta-wasi.wasm .
go run ./sandbox/example karta-wasm/karta-wasi.wasm
```

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
