#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
"""Prototype executor for ActionRequests.

Kyverno creates an ActionRequest per eligible job. This loop spends it once:
claim (Intended, written with an optimistic-concurrency status replace),
patch the job with uid and resourceVersion tests, record Executed. A human
resume is recorded as ResumeDetected and never re-acted on. Nothing here
deletes anything, so the record outlives the job.

Env: CTX (kube context), NS (request namespace, default karta-actions),
ENFORCE (1 to act, 0 to record WouldAct only), INTERVAL seconds.
"""
import json
import os
import subprocess
import sys
import time
from datetime import datetime, timezone

CTX = os.environ.get("CTX", "kind-kyverno-lab2")
NS = os.environ.get("NS", "karta-actions")
ENFORCE = os.environ.get("ENFORCE", "1") == "1"
INTERVAL = float(os.environ.get("INTERVAL", "2"))


def now():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"


def log(msg):
    print(f"[{now()}] {msg}", flush=True)


def kubectl(*args, stdin=None, check=True):
    cmd = ["kubectl", "--context", CTX, *args]
    r = subprocess.run(cmd, input=stdin, capture_output=True, text=True)
    if check and r.returncode != 0:
        raise RuntimeError(r.stderr.strip())
    return r


def get_json(*args):
    r = kubectl(*args, "-o", "json", check=False)
    if r.returncode != 0:
        return None
    return json.loads(r.stdout)


def write_status(ar, phase, message="", **fields):
    """Replace the status subresource using the object we read (its
    resourceVersion makes the write conditional). Returns the new object or
    None on conflict."""
    st = dict(ar.get("status") or {})
    st["phase"] = phase
    if message:
        st["message"] = message
    st.update(fields)
    receipts = list(st.get("receipts") or [])
    receipts.append({"phase": phase, "time": now(), "message": message})
    st["receipts"] = receipts
    ar["status"] = st
    r = kubectl("replace", "--subresource=status", "-f", "-", stdin=json.dumps(ar), check=False)
    if r.returncode != 0:
        log(f"status write for {ar['metadata']['name']} -> {phase} rejected: {r.stderr.strip()[:160]}")
        return None
    log(f"receipt {phase}: {ar['metadata']['name']} {message}".rstrip())
    return get_json("get", "actionrequest", "-n", NS, ar["metadata"]["name"])


def target_of(ar):
    t = ar["spec"]["target"]
    return get_json("get", t["kind"].lower(), "-n", t["namespace"], t["name"])


def handle_new(ar):
    """Pending (or no status yet): validate, then WouldAct or Intended+patch."""
    name = ar["metadata"]["name"]
    t = ar["spec"]["target"]
    job = target_of(ar)
    if job is None:
        write_status(ar, "Blocked", "target not found")
        return
    if job["metadata"]["uid"] != t["uid"]:
        write_status(ar, "Blocked", f"target uid changed: {job['metadata']['uid']}")
        return
    if job.get("spec", {}).get("suspend") is True:
        write_status(ar, "Blocked", "target already suspended by someone else")
        return
    if not ENFORCE:
        if (ar.get("status") or {}).get("phase") != "WouldAct":
            write_status(ar, "WouldAct", "observe mode, no patch sent")
        return
    rv = job["metadata"]["resourceVersion"]
    patch = [
        {"op": "test", "path": "/metadata/uid", "value": t["uid"]},
        {"op": "test", "path": "/metadata/resourceVersion", "value": rv},
        {"op": "add", "path": "/spec/suspend", "value": True},
    ]
    claimed = write_status(ar, "Intended", f"claiming the single attempt against resourceVersion {rv}",
                           attempts=1, intendedAt=now(), patch=json.dumps(patch))
    if claimed is None:
        log(f"{name}: lost the claim race, not acting")
        return
    r = kubectl("patch", t["kind"].lower(), "-n", t["namespace"], t["name"],
                "--type=json", "-p", json.dumps(patch), check=False)
    if r.returncode != 0:
        write_status(claimed, "Blocked", f"patch rejected, attempt spent: {r.stderr.strip()[:160]}")
        return
    write_status(claimed, "Executed", "patch accepted: /spec/suspend=true", executedAt=now())


def handle_executed(ar):
    """Watch for a human resume; record it once; never act again."""
    st = ar.get("status") or {}
    if st.get("resumeDetectedAt"):
        return
    job = target_of(ar)
    if job is None:
        if not any(r.get("phase") == "TargetGone" for r in st.get("receipts", [])):
            write_status(ar, "Executed", "target deleted, record kept", **{"receiptsNote": "TargetGone"})
        return
    if job.get("spec", {}).get("suspend") is False:
        write_status(ar, "Executed", "resume detected, allowance stays spent, not re-acting",
                     resumeDetectedAt=now())


def main():
    log(f"executor start ctx={CTX} ns={NS} enforce={ENFORCE}")
    while True:
        lst = get_json("get", "actionrequests", "-n", NS)
        for ar in (lst or {}).get("items", []):
            phase = (ar.get("status") or {}).get("phase")
            try:
                if phase in (None, "Pending", "WouldAct"):
                    if phase == "WouldAct" and not ENFORCE:
                        continue
                    handle_new(ar)
                elif phase == "Executed":
                    handle_executed(ar)
            except Exception as e:  # keep the loop alive, record what happened
                log(f"{ar['metadata']['name']}: {e}")
        time.sleep(INTERVAL)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(0)
