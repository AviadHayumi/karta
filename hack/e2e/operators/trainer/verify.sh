#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation

set -euo pipefail

MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../_common.sh
source "${MODULE_DIR}/../_common.sh"

main() {
  run_smoke "${MODULE_DIR}/smoke.yaml" "trainjob/trainer-smoke" "condition=Complete" "240s" default
}

main "$@"
