#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# JobSet operator (sigs.k8s.io/jobset).
# shellcheck disable=SC2154  # JOBSET_VERSION comes from global.env via _common.sh
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

main() {
  echo "==> JobSet ${JOBSET_VERSION}"
  kubectl apply --server-side -f "https://github.com/kubernetes-sigs/jobset/releases/download/${JOBSET_VERSION}/manifests.yaml"
  # Some release manifests (v0.9.1) pin the garbage-collected staging registry;
  # repoint to the promoted image of the same version.
  local image
  image="$(kubectl get deploy jobset-controller-manager -n jobset-system \
    -o jsonpath='{.spec.template.spec.containers[?(@.name=="manager")].image}')"
  case "${image}" in
    *k8s-staging-images*|*gcr.io/k8s-staging*)
      kubectl set image deployment/jobset-controller-manager -n jobset-system \
        manager="registry.k8s.io/jobset/jobset:${JOBSET_VERSION}" ;;
  esac
  rollout_wait jobset-system deploy/jobset-controller-manager
}

main "$@"
