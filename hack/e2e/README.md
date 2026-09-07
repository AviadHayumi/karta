<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta e2e cluster provisioner

This directory provisions a local kind cluster for the Karta e2e suite: it builds
and deploys the Karta operator, installs the dependencies the suite needs, and
installs the upstream workload operators it exercises. Each operator is smoke-tested
as it installs, so a broken install fails provisioning rather than a later run.

## Layout

```
hack/e2e/
  up.sh                      orchestrator: base + selected operators (install then verify)
  down.sh                    tear the cluster down (and its kubeconfig for named clusters)
  install-karta-operator.sh  standalone: installs Karta in the selected webhook route
  global.env                 single source of truth for versions and runtime defaults
  kind-config.yaml           kind cluster shape (1 control-plane + 2 workers)
  operators/
    _common.sh               shared helpers + GitHub Actions logging, sourced by every script
    <name>/
      install.sh             standalone: installs the operator (run as a subprocess)
      verify.sh              standalone: smoke-tests it via run_smoke
      smoke.yaml             the throwaway workload the smoke test applies
      <config>.yaml          optional co-located config (e.g. grove/values.yaml)
```

## Usage

```sh
make e2e-up                          # base + all operators
make e2e-up WORKLOADS="jobset lws"   # base + a subset (one provision, deps resolved once)
make e2e-up WORKLOADS="jobset"       # base + a single operator
make e2e-up WORKLOADS=none           # base only, no workload operators
make e2e-down                        # tear down
./hack/e2e/up.sh --list dynamo       # print the resolved plan and exit (dynamo pulls grove)
./hack/e2e/up.sh --list none         # print the base-only plan, including the cert-manager decision
```

The base is the kind cluster, the fake-gpu-operator, and the Karta operator.
Selecting a subset keeps a run light, and `none` keeps only the base, which is what
the controller e2e wants. Dependencies are added automatically: kserve pulls
knative, dynamo pulls grove.

## Karta install routes

`KARTA_WEBHOOK_MODE` picks which webhook and serving-cert arrangement Karta is
installed with. The three are mutually exclusive, so they are exercised as separate
runs against separate clusters rather than as one cluster reconfigured in place.

| Mode | Webhook | Serving cert | caBundle |
|---|---|---|---|
| `auto` (default) | on | the operator self-signs and rotates it | patched by the operator |
| `cert-manager` | on | issued by cert-manager | injected by cainjector |
| `disabled` | off | none | none |

```sh
make e2e-up WORKLOADS=none CLUSTER_NAME=karta-auto                                  # route auto
make e2e-up WORKLOADS=none CLUSTER_NAME=karta-cm   KARTA_WEBHOOK_MODE=cert-manager  # route cert-manager
make e2e-up WORKLOADS=none CLUSTER_NAME=karta-nowh KARTA_WEBHOOK_MODE=disabled      # route disabled
```

Give each route its own `CLUSTER_NAME`, or tear down between them.

`up.sh` reuses a kind cluster that already exists. Each route leaves state the next
one does not want. Switching off the cert-manager route leaves cert-manager and the
Karta `Certificate` behind. That Certificate keeps reconciling the same Secret the
auto route's operator writes. Two controllers then fight over one key, and the
no-cert-manager claim below quietly stops holding.

CI is unaffected. A fresh runner has no cluster to reuse.

The install itself lives in `install-karta-operator.sh`. `up.sh` runs it last, as a
standalone script, on the same exit-code contract as the workload operators. Karta is
the system under test rather than cluster infrastructure, so it gets its own file.
Run it directly against the current context to reinstall Karta without
reprovisioning the cluster.

The chart deliberately ships no `Issuer` or `Certificate`. `provisionMode: manual`
only mounts the Secret and stamps the injection annotation. Supplying the pair is the
caller's half of the contract, and `install_certificate` is that half.

`install_certificate` has to run before the helm install rather than from a test,
because controller-runtime reads the serving cert at startup. An operator pod that
starts without the Secret crashloops instead of waiting for it.

`wait_for_ca_injection` runs after the install. It blocks until cainjector has
stamped a caBundle onto both webhook configs. Nothing else gates on that.

## cert-manager

`CERT_MANAGER` controls whether cert-manager is installed: `auto` (the default)
installs it only when something needs it, `true` forces it on, `false` refuses and
fails fast if something selected needs it.

Something needs it when `KARTA_WEBHOOK_MODE=cert-manager`, or when a selected
operator needs it. Today that is kserve alone: its bundled manifest ships cert-manager
`Certificate` resources, which is why `operators/kserve/install.sh` has to
`--force-conflicts` over cainjector's caBundle. The check lives in `up.sh` next to
where the install decision is made; extend it if another operator turns out to need it.

Leaving cert-manager out is deliberate rather than only a saving. On a fresh cluster
the `auto` and `disabled` routes run without it. That is what proves the operator's
own cert controller depends on nothing external.

The claim only holds on a cluster the cert-manager route has not already touched.
That is why each route wants its own `CLUSTER_NAME`.

## How up.sh runs an operator

For each selected operator, up.sh runs `operators/<name>/install.sh` then
`operators/<name>/verify.sh` as `bash <script>` subprocesses. The only contract is
the exit code: any non-zero exit is a failed install/smoke and fails provisioning
fast (the `main()` wrapper in the scripts is just convention). up.sh groups each
operator in the CI log (`::group::`) and writes an install summary table to the run's
Summary page (operator, version, install time, smoke time).

## Shared helpers (`operators/_common.sh`)

Sourcing `_common.sh` also loads `global.env`, so any script gets the version
pins in one step. Available helpers:

- `rollout_wait <ns> <resource> [timeout]` - wait for a rollout; resource includes
  the kind, e.g. `deploy/foo` or `statefulset/foo`.
- `apply_with_retry <file|url> [tries] [sleep] [kubectl args...]` - kubectl apply,
  retried past a warming webhook.
- `retry <tries> <sleep> <command...>` - run a command, retried past a transient.
- `run_smoke <manifest> <target> <wait-expr> [timeout] [ns]` - apply a throwaway
  resource, wait for the state, delete it. Used by every verify.sh.
- `preload_image <src-ref> <local-tag>` - pull an image and load it into kind.
- `build_and_load_image <context-dir> <local-tag>` - build an image and load it.
- `ensure_secret <ns> <name> <k=v>...` - idempotently create/update a secret.
- Logging: `group`/`endgroup`, `notice`/`warn`/`fail`, `summary`. When to use each
  is documented at the top of `_common.sh`.

## Adding an operator

Install side (this directory):

1. Create `operators/<name>/install.sh` as a standalone script:

   ```sh
   #!/usr/bin/env bash
   # SPDX-License-Identifier: Apache-2.0
   # Copyright (c) 2026 NVIDIA Corporation
   set -euo pipefail
   MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
   # shellcheck source=/dev/null
   source "${MODULE_DIR}/../_common.sh"

   main() {
     echo "==> <name> ${<NAME>_VERSION}"
     # install with helm/kubectl; wait with rollout_wait; use the shared helpers;
     # reference co-located config via "${MODULE_DIR}/...".
   }

   main "$@"
   ```

2. Create `operators/<name>/smoke.yaml` (a throwaway workload) and
   `operators/<name>/verify.sh`:

   ```sh
   #!/usr/bin/env bash
   # SPDX + copyright, set -euo pipefail, MODULE_DIR, source ../_common.sh
   run_smoke "${MODULE_DIR}/smoke.yaml" "<kind>/<name>-smoke" "<wait-expr>" "<timeout>" default
   ```

   `<wait-expr>` is passed to `kubectl wait --for=`, so both `condition=Ready` and
   `jsonpath={.status.state}=ready` work.

3. Pin the version(s) in `global.env`, and add a `version_of` case in `up.sh` so
   the operator shows in the install summary.
4. Add `<name>` to `ALL_WORKLOADS` in `up.sh` (in install order), and a `deps_of`
   entry only if it depends on another operator.

Before pushing: `make lint-shell` (shellcheck), then provision just that operator to
confirm it installs and smoke-tests clean, for example `make e2e-up WORKLOADS=<name>`.
