<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Skipped operator versions

Versions that could not be recorded, with the reason. Everything else under
this tree recorded and replayed green.

- dynamo 1.4.2, 1.4.1, 1.4.0: the operator installs and runs, but no longer
  drives the deprecated v1alpha1 DynamoGraphDeployment to running; the
  workload sits Initializing until the timeout. Recording the 1.4 line needs
  the flows moved to the v1beta1 API.
- dynamo 1.1.1: the operator never becomes ready, while 1.1.0 and 1.2.x both
  work on the same cluster; it looks like a broken release rather than an
  environment mismatch.
- kubeflow v2.x: out of scope rather than skipped; the v2 trainer is a new
  API generation (TrainJob) and no longer serves the PyTorchJob and MPIJob
  APIs these flows record.
