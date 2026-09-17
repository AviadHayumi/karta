<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Recorded operator versions

Recorded flow fixtures per supported operator. Coverage is anchored on the
latest stable upstream release: for each operator, the newest stable version
and the three releases before it (the latest patch of each earlier minor
where the operator uses minors). Versions recorded along the way below that
window are kept as extra depth.

Each `<operator>/<version>/` folder holds the same recordings the e2e suite
keeps under `test/e2e/recorded_data/`: every distinct CR the operator
produced during each flow, labelled from the operator's own fields,
replayable through the catalog definitions.

How a folder is produced: `hack/e2e/record-matrix.sh <operator> <version>`
installs the operator at that version on the current cluster, verifies it,
records its flows, and files the fixtures here. Old releases that only run on
their era Kubernetes fail fast with the era-cluster hint (see
`require_k8s_max`); those folders (kserve v0.12.1 and v0.11.2, kubeflow
v1.7.0) were recorded on a kind cluster running kindest/node v1.28.9. A
version that cannot install or record is listed in `SKIPPED.md` with the
reason.

Coverage: jobset, lws, kuberay (RayCluster and RayJob), kubeflow (PyTorchJob
and MPIJob), knative, kserve, milvus, grove, dynamo, nim.
