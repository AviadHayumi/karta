#!/bin/zsh
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
# Capture every audit surface kyverno leaves for $TARGET, then delete the
# job and capture what survives.
set -u
CTX="${CTX:-kind-kyverno-lab}"
TARGET="${TARGET:-trainer-c}"
TARGET_NS="${TARGET_NS:-ai-team}"
POLICY="${POLICY:-metric-suspend-running}"
LAB="$(cd "$(dirname "$0")/.." && pwd)"
CAP="$LAB/captures"
K() { kubectl --context=$CTX "$@" }
OUT="$CAP/17-kyverno-forensics.txt"
export TARGET

echo "=== BEFORE deletion ($TARGET, suspended by the engine) ===" > "$OUT"
echo "--- policyreports in $TARGET_NS scoped to $TARGET ---" >> "$OUT"
K get policyreports -n "$TARGET_NS" -o json | python3 -c "
import json, os, sys
d = json.load(sys.stdin)
t = os.environ['TARGET']
for i in d['items']:
    if i.get('scope', {}).get('name') == t:
        for r in i.get('results', []):
            print(i['metadata']['name'], '|', r['policy'], '|', r['result'], '|', r.get('message', '')[:60])
" >> "$OUT" 2>&1
echo "--- events on $TARGET ---" >> "$OUT"
K get events -n "$TARGET_NS" --field-selector involvedObject.name=$TARGET --no-headers >> "$OUT" 2>&1
echo "--- updaterequests ---" >> "$OUT"
K get updaterequests -n kyverno --no-headers >> "$OUT" 2>&1
echo "--- mpol status ---" >> "$OUT"
K get mutatingpolicy "$POLICY" -o jsonpath='{.status}' >> "$OUT" 2>&1
echo >> "$OUT"

echo "=== DELETING $TARGET ===" >> "$OUT"
python3 -c "import datetime; print('deleted at', datetime.datetime.now(datetime.UTC).isoformat(timespec='seconds'))" >> "$OUT"
K delete job "$TARGET" -n "$TARGET_NS" >> "$OUT" 2>&1
sleep 10

echo "=== AFTER deletion ===" >> "$OUT"
echo "--- policyreports scoped to $TARGET ---" >> "$OUT"
K get policyreports -n "$TARGET_NS" -o json | python3 -c "
import json, os, sys
d = json.load(sys.stdin)
t = os.environ['TARGET']
found = [i for i in d['items'] if i.get('scope', {}).get('name') == t]
print(f'{len(found)} report(s) remain for {t}')
" >> "$OUT" 2>&1
echo "--- events on $TARGET (TTL-bound, not action-coupled) ---" >> "$OUT"
K get events -n "$TARGET_NS" --field-selector involvedObject.name=$TARGET --no-headers 2>/dev/null | wc -l >> "$OUT"
echo "--- updaterequests ---" >> "$OUT"
K get updaterequests -n kyverno --no-headers >> "$OUT" 2>&1
echo "--- durable per-action record of the engine's actions on $TARGET: ---" >> "$OUT"
echo "(no surviving surface records WHAT was done WHEN by WHICH evaluation)" >> "$OUT"
cat "$OUT"
