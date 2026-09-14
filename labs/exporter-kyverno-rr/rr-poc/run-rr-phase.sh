#!/bin/zsh
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
# RR phase battery. Prereq: kyverno metric policy + GCE deleted, wedged jobs
# removed. Every scenario appends to captures/.
set -u
CTX="${CTX:-kind-kyverno-lab}"
export RR_CONTEXT="$CTX"
LAB="$(cd "$(dirname "$0")/.." && pwd)"
CAP="$LAB/captures"
RR="python3 $LAB/rr-poc/rr_poc.py --catalog $LAB/src/docs/catalog --impersonate system:serviceaccount:rr-system:rr-poc"
K() { kubectl --context=$CTX "$@" }
TS() { python3 -c "import datetime; print(datetime.datetime.now(datetime.UTC).strftime('%H:%M:%S.%f')[:-3], 'UTC')" }

echo "===== R0: fresh jobs trainer-e, trainer-f ====="
for j in trainer-e trainer-f; do
  K apply -f - << EOF
apiVersion: batch/v1
kind: Job
metadata: {name: $j, namespace: ai-team, labels: {team: fox}}
spec:
  suspend: false
  backoffLimit: 6
  template:
    spec:
      restartPolicy: Never
      containers: [{name: train, image: "busybox:1.36", command: ["sh", "-c", "sleep 86400"]}]
EOF
done
echo "waiting 150s for the Running window to fill..."
sleep 150

echo "===== R1: OBSERVE mode (2 loops) =====" | tee "$CAP/20-rr-observe.txt"
eval "$RR --mode Observe --max-loops 2 --interval 5 --run-id obs" >> "$CAP/20-rr-observe.txt" 2>&1
K get jobs -n ai-team -o jsonpath='{range .items[*]}{.metadata.name} suspend={.spec.suspend}{"\n"}{end}' >> "$CAP/20-rr-observe.txt"
echo "observe done: no job may be suspended (verify above)"

echo "===== R2: ENFORCE (suspends with receipts) =====" | tee "$CAP/21-rr-enforce.txt"
TS >> "$CAP/21-rr-enforce.txt"
eval "$RR --mode Enforce --max-loops 2 --interval 5 --run-id enf" >> "$CAP/21-rr-enforce.txt" 2>&1
K get jobs -n ai-team -o jsonpath='{range .items[*]}{.metadata.name} suspend={.spec.suspend}{"\n"}{end}' >> "$CAP/21-rr-enforce.txt"

echo "===== R3: user resumes trainer-f; RR must NOT re-suspend =====" | tee "$CAP/22-rr-anti-trap.txt"
# settle gate (Finding 3): resuming inside the Job controller's suspension
# bookkeeping window wedges the Job on k8s v1.34. Wait for startTime to clear.
for i in $(seq 1 60); do
  ST=$(K get job trainer-f -n ai-team -o jsonpath='{.status.startTime}')
  [ -z "$ST" ] && echo "trainer-f suspension settled (startTime cleared) after ${i} checks" >> "$CAP/22-rr-anti-trap.txt" && break
  sleep 2
done
TS | tee -a "$CAP/22-rr-anti-trap.txt"
K patch job trainer-f -n ai-team --type=merge -p '{"spec":{"suspend":false}}' >> "$CAP/22-rr-anti-trap.txt" 2>&1
echo "running RR loop for 300s across the window refill..." >> "$CAP/22-rr-anti-trap.txt"
eval "$RR --mode Enforce --max-loops 30 --interval 10 --run-id trap" >> "$CAP/22-rr-anti-trap.txt" 2>&1 &
RRPID=$!
sleep 300
kill $RRPID 2>/dev/null
TS >> "$CAP/22-rr-anti-trap.txt"
K get job trainer-f -n ai-team -o jsonpath='trainer-f suspend={.spec.suspend} active={.status.active}' >> "$CAP/22-rr-anti-trap.txt"
echo >> "$CAP/22-rr-anti-trap.txt"

echo "===== R4: RBAC revoked mid-flight =====" | tee "$CAP/23-rr-rbac.txt"
K apply -f - << 'EOF'
apiVersion: batch/v1
kind: Job
metadata: {name: trainer-g, namespace: ai-team, labels: {team: fox}}
spec:
  suspend: false
  backoffLimit: 6
  template:
    spec:
      restartPolicy: Never
      containers: [{name: train, image: "busybox:1.36", command: ["sh", "-c", "sleep 86400"]}]
EOF
K delete rolebinding rr-poc-actions -n ai-team >> "$CAP/23-rr-rbac.txt" 2>&1
echo "waiting 150s for trainer-g window..."
sleep 150
eval "$RR --mode Enforce --max-loops 1 --run-id rbac1" >> "$CAP/23-rr-rbac.txt" 2>&1
echo "--- rolebinding restored ---" >> "$CAP/23-rr-rbac.txt"
K apply -f "$LAB/manifests/06-rr-rbac.yaml" >> "$CAP/23-rr-rbac.txt" 2>&1
eval "$RR --mode Enforce --max-loops 1 --run-id rbac2" >> "$CAP/23-rr-rbac.txt" 2>&1
K get job trainer-g -n ai-team -o jsonpath='trainer-g suspend={.spec.suspend}' >> "$CAP/23-rr-rbac.txt"

echo "===== R5: deletion forensics =====" | tee "$CAP/24-rr-forensics.txt"
K delete job trainer-e -n ai-team >> "$CAP/24-rr-forensics.txt" 2>&1
sleep 3
echo "--- receipts for trainer-e after deletion ---" >> "$CAP/24-rr-forensics.txt"
K get cm -n rr-system -l rr.karta.run.ai/workload=trainer-e -o custom-columns='NAME:.metadata.name,PHASE:.metadata.labels.rr\.karta\.run\.ai/phase' >> "$CAP/24-rr-forensics.txt" 2>&1
echo "--- one full receipt body ---" >> "$CAP/24-rr-forensics.txt"
K get cm -n rr-system -l 'rr.karta.run.ai/workload=trainer-e,rr.karta.run.ai/phase=Executed' -o jsonpath='{.items[0].data.receipt\.json}' >> "$CAP/24-rr-forensics.txt" 2>&1

echo "===== RR PHASE DONE ====="
ls "$CAP" | tail -8
