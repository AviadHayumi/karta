#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Install the Karta operator from the local chart, in whichever webhook and
# serving-cert arrangement KARTA_WEBHOOK_MODE selects. This is the system under test,
# not cluster infrastructure: the kind cluster, cert-manager and the fake-gpu-operator
# are up.sh's job, and up.sh runs this last.
#
# Run standalone against the current kubectl context, or through `make e2e-up`.
# shellcheck disable=SC2154  # KARTA_* and IMAGE come from global.env via _common.sh
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/operators/_common.sh"

# REPO_ROOT is exported by up.sh; derive it when this script is run on its own.
REPO_ROOT="${REPO_ROOT:-$(cd "${MODULE_DIR}/../.." && pwd)}"

# Webhook resource names rendered by the chart (charts/karta/templates/_helpers.tpl).
# The service name doubles as the serving-cert SAN, so the Certificate below must
# agree with it.
KARTA_WEBHOOK_SERVICE="karta-operator-webhook"
KARTA_WEBHOOK_SECRET="karta-operator-webhook-cert"
KARTA_WEBHOOK_CONFIGS="mutatingwebhookconfiguration/karta-operator-mutating validatingwebhookconfiguration/karta-operator-validating"
# Name of the cert-manager Certificate created for KARTA_WEBHOOK_MODE=cert-manager.
KARTA_WEBHOOK_CERT="karta-webhook-cert"

# install_certificate issues the webhook serving cert with cert-manager, for the
# cert-manager route. The chart deliberately ships no Issuer or Certificate:
# provisionMode=manual only mounts the Secret and stamps the caBundle annotation, so
# supplying these is the caller's half of the contract.
#
# It has to run before the helm install, not from a test: controller-runtime reads the
# serving cert at startup (certwatcher.New does an initial read and errors when the
# files are missing), so an operator pod that starts without the Secret crashloops
# instead of waiting for it. certwatcher hot-reloads afterwards, which covers rotation
# but not bootstrap.
install_certificate() {
  kubectl create namespace "${KARTA_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  kubectl apply -f - >/dev/null <<EOF
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: karta-selfsigned
  namespace: ${KARTA_NAMESPACE}
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ${KARTA_WEBHOOK_CERT}
  namespace: ${KARTA_NAMESPACE}
spec:
  secretName: ${KARTA_WEBHOOK_SECRET}
  issuerRef:
    name: karta-selfsigned
    kind: Issuer
  dnsNames:
    - ${KARTA_WEBHOOK_SERVICE}.${KARTA_NAMESPACE}.svc
    - ${KARTA_WEBHOOK_SERVICE}.${KARTA_NAMESPACE}.svc.cluster.local
EOF
  kubectl wait --for=condition=Ready "certificate/${KARTA_WEBHOOK_CERT}" \
    -n "${KARTA_NAMESPACE}" --timeout=120s
}

# wait_for_ca_injection blocks until cainjector has stamped a caBundle onto both webhook
# configs. It cannot run before the helm install, because cainjector only acts on configs
# that already carry the inject-ca-from annotation, and helm is what creates them.
#
# Nothing else covers this. In auto mode the operator's own rotator writes the caBundle
# before it reports ready, so rollout_wait is an implicit gate; in manual mode the operator
# never touches it, and cainjector works asynchronously. Without this wait the install can
# report ready while the API server still has an empty caBundle, and the first admission
# call fails with an x509 error that looks nothing like the real cause.
wait_for_ca_injection() {
  local target ca
  for target in ${KARTA_WEBHOOK_CONFIGS}; do
    ca=""
    # The counter is a throwaway: 60 tries at 2s is the 120s budget in the message.
    for _ in $(seq 1 60); do
      ca="$(kubectl get "${target}" -o jsonpath='{.webhooks[0].clientConfig.caBundle}' 2>/dev/null || true)"
      [ -n "${ca}" ] && break
      sleep 2
    done
    if [ -z "${ca}" ]; then
      fail "cainjector did not populate caBundle on ${target} within 120s"
      exit 1
    fi
  done
}

main() {
  echo "==> Karta operator (webhook: ${KARTA_WEBHOOK_MODE})"
  kubectl apply --server-side -f "${REPO_ROOT}/charts/karta/crds/"

  # One --set list per route. Only the webhook and cert values differ; everything
  # else about the install is identical, so a route can never drift in some other way.
  local webhook_values=()
  case "${KARTA_WEBHOOK_MODE}" in
    auto)
      webhook_values=(--set webhook.enabled=true --set webhook.cert.provisionMode=auto)
      ;;
    cert-manager)
      install_certificate
      webhook_values=(
        --set webhook.enabled=true
        --set webhook.cert.provisionMode=manual
        # cainjector reads this annotation off the webhook configs and writes the
        # issuing CA into their caBundle. The operator never touches it in manual mode.
        --set-string "webhook.cert.annotations.cert-manager\.io/inject-ca-from=${KARTA_NAMESPACE}/${KARTA_WEBHOOK_CERT}"
      )
      ;;
    disabled)
      webhook_values=(--set webhook.enabled=false)
      ;;
    *)
      # up.sh validates this before provisioning; repeated here so the script is safe
      # to run on its own.
      echo "error: unknown KARTA_WEBHOOK_MODE '${KARTA_WEBHOOK_MODE}' (want: auto, cert-manager, disabled)" >&2
      exit 2
      ;;
  esac

  helm upgrade -i karta "${REPO_ROOT}/charts/karta" -n "${KARTA_NAMESPACE}" --create-namespace \
    --set image.repository="${IMAGE%:*}" --set image.tag="${IMAGE##*:}" \
    --set resources.limits.memory="${KARTA_OPERATOR_MEMORY}" \
    "${webhook_values[@]}" >/dev/null
  rollout_wait "${KARTA_NAMESPACE}" deploy/karta-operator 120s
  # Only the cert-manager route needs this; see wait_for_ca_injection for why the other
  # two are already covered.
  [ "${KARTA_WEBHOOK_MODE}" = "cert-manager" ] && wait_for_ca_injection
  return 0
}

main "$@"
