# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
"""Runtime Rules prototype (rr-poc), revision 2 after Codex r2 review.

A small demonstrator of the execution protocol from KEP-0003, consuming the
same exporter facts the Kyverno run used. Revision 2 contract fixes:

- intent receipt persistence GATES the action (no receipt, no patch)
- a pending operation is persisted in state BEFORE the patch; on restart an
  unresolved operation blocks further automated action until verified
- the patch is a JSONPatch with test preconditions on uid and the final
  eligibility state; a failed precondition is a visible receipt
- Observe mode writes WouldAct (with real preflight) and does NOT consume
  the action cap
- facts-unavailable and list failures produce rule-level receipts, and
  deduplication markers advance only after a receipt write succeeded
- cap semantics named precisely: one initial automated action per workload;
  an external resume does NOT re-arm automation (reArmOnResume=false);
  later eligibility escalates instead
- resume detection is transition-based ("external resume detected"); the
  managedFields manager list is supporting evidence only, read with
  --show-managed-fields

Known, disclosed scope limits: reads/state/receipts use the ambient
identity; only preflight and workload patches impersonate --impersonate.
The rules file is read at startup (no CRD watch). The catalog-handle
interpreter supports plain dotted boolean paths (Job/CronJob suspend);
everything else is skip-and-report.
"""

import argparse
import datetime
import json
import os
import pathlib
import subprocess
import sys
import time
import urllib.parse

import yaml

KUBECTL = ["kubectl", f"--context={os.environ.get('RR_CONTEXT', 'kind-kyverno-lab')}"]
FIELD_MANAGER = "rr-poc"
RR_NS = "rr-system"
STATE_CM = "rr-state"


def now_iso():
    return datetime.datetime.now(datetime.UTC).isoformat(timespec="milliseconds")


def log(msg):
    print(f"[{now_iso()}] {msg}", flush=True)


class KubectlResult:
    def __init__(self, returncode, stdout, stderr, timed_out=False):
        self.returncode = returncode
        self.stdout = stdout
        self.stderr = stderr
        self.timed_out = timed_out


def kubectl(args, impersonate=None, input_data=None):
    cmd = list(KUBECTL)
    if impersonate:
        cmd += [f"--as={impersonate}"]
    cmd += args
    try:
        r = subprocess.run(cmd, capture_output=True, text=True, input=input_data, timeout=30)
        return KubectlResult(r.returncode, r.stdout, r.stderr)
    except subprocess.TimeoutExpired:
        return KubectlResult(124, "", "kubectl timeout", timed_out=True)


def kubectl_json(args, impersonate=None):
    result = kubectl(args + ["-o", "json"], impersonate=impersonate)
    if result.returncode != 0:
        return None, result.stderr.strip()
    return json.loads(result.stdout), None


class KartaHandles:
    """Catalog-handle interpreter for the tested dotted-path shapes."""

    def __init__(self, catalog_dir):
        self.handles = {}
        for path in pathlib.Path(catalog_dir).glob("*.yaml"):
            doc = yaml.safe_load(path.read_text())
            root = doc.get("spec", {}).get("structureDefinition", {}).get("rootComponent", {})
            kind_ref = root.get("kind") or {}
            kind = kind_ref.get("kind")
            if not kind:
                continue
            suspend = (root.get("suspendDefinition") or {}).get("suspendActions")
            self.handles[kind] = {"karta": doc["metadata"]["name"], "suspend": suspend}

    def suspend_pointer(self, kind):
        """Return (json_pointer, value, karta_name) or (None, None, karta) on
        unsupported shapes. Only plain dotted paths with boolean string values
        are interpreted; anything else is skip-and-report."""
        entry = self.handles.get(kind)
        if not entry or not entry["suspend"]:
            return None, None, entry["karta"] if entry else None
        if len(entry["suspend"]) != 1:
            return None, None, entry["karta"]
        action = entry["suspend"][0]
        path, value = action["path"], action["value"]
        parts = [p for p in path.lstrip(".").split(".") if p]
        if not parts or any(("[" in p or "]" in p or '"' in p) for p in parts):
            return None, None, entry["karta"]
        if value not in ("true", "false"):
            return None, None, entry["karta"]
        return "/" + "/".join(parts), value == "true", entry["karta"]


class Receipts:
    def __init__(self, rule_name, run_id):
        self.rule = rule_name
        self.run_id = run_id
        self.seq = 0

    def _apply(self, name, receipt, labels):
        body = {
            "apiVersion": "v1",
            "kind": "ConfigMap",
            "metadata": {"name": name, "namespace": RR_NS, "labels": labels},
            "data": {"receipt.json": json.dumps(receipt, indent=1)},
        }
        result = kubectl(["apply", "-f", "-"], input_data=json.dumps(body))
        if result.returncode != 0:
            log(f"RECEIPT WRITE FAILED ({receipt['phase']}): {result.stderr.strip()}")
            return None
        log(f"receipt {receipt['phase']}: {name} ({receipt.get('workload', {}).get('name', '-')})")
        return name

    def write(self, phase, workload, detail):
        self.seq += 1
        uid8 = workload["uid"][:8]
        name = f"rr-{uid8}-e{detail.get('epoch', 0)}-{self.run_id}-{self.seq}"
        receipt = {
            "rule": self.rule,
            "phase": phase,
            "workload": {k: workload[k] for k in ("namespace", "name", "kind", "uid") if k in workload},
            "targetResourceVersion": workload.get("resourceVersion"),
            "time": now_iso(),
            **detail,
        }
        labels = {
            "rr.karta.run.ai/rule": self.rule,
            "rr.karta.run.ai/workload": workload["name"],
            "rr.karta.run.ai/phase": phase,
        }
        return self._apply(name, receipt, labels)

    def write_rule_level(self, phase, detail):
        self.seq += 1
        name = f"rr-rule-{self.rule[:20]}-{self.run_id}-{self.seq}"
        receipt = {"rule": self.rule, "phase": phase, "scope": "rule", "time": now_iso(), **detail}
        return self._apply(name, receipt, {"rr.karta.run.ai/rule": self.rule, "rr.karta.run.ai/phase": phase})

    def update(self, prior_name, phase, workload, detail):
        detail = dict(detail)
        detail["supersedes"] = prior_name
        return self.write(phase, workload, detail)


class State:
    """Per (rule, workload uid) allowance state, persisted every loop.

    A pending operation recorded here before the patch makes one-shot
    behavior restart-safe: an unresolved operation blocks further action."""

    def __init__(self, rule_name):
        self.rule = rule_name
        self.data = None
        existing, err = kubectl_json(["get", "configmap", STATE_CM, "-n", RR_NS])
        if existing is not None and existing.get("data", {}).get("state.json"):
            self.data = json.loads(existing["data"]["state.json"])
        elif err and "NotFound" not in err and "not found" not in err:
            # unreadable state must not silently become fresh adoption
            raise RuntimeError(f"state unreadable, refusing fresh adoption: {err}")
        else:
            self.data = {}

    def entry(self, uid):
        return self.data.setdefault(f"{self.rule}:{uid}", {
            "epoch": 0,
            "actedInEpoch": False,
            "externalResumes": 0,
            "weSuspended": False,
            "pendingOp": None,
        })

    def save(self):
        body = {
            "apiVersion": "v1",
            "kind": "ConfigMap",
            "metadata": {"name": STATE_CM, "namespace": RR_NS},
            "data": {"state.json": json.dumps(self.data, indent=1)},
        }
        result = kubectl(["apply", "-f", "-"], input_data=json.dumps(body))
        if result.returncode != 0:
            log(f"STATE SAVE FAILED (next loop retries): {result.stderr.strip()}")
            return False
        return True


def query_condition_workloads(promql):
    url = "/api/v1/namespaces/monitoring/services/prometheus:web/proxy/api/v1/query?query=" + urllib.parse.quote(promql, safe="")
    result = kubectl(["get", "--raw", url])
    if result.returncode != 0:
        return None, result.stderr.strip()
    payload = json.loads(result.stdout)
    if payload.get("status") != "success" or payload.get("data", {}).get("resultType") != "vector":
        return None, f"unexpected prometheus response shape: {str(payload)[:150]}"
    hits = {}
    for sample in payload["data"]["result"]:
        metric = sample["metric"]
        key = (metric.get("namespace"), metric.get("workload"), metric.get("workload_kind"))
        hits[key] = {"value": sample["value"][1], "sampleTime": sample["value"][0]}
    return hits, None


PLURAL = {"Job": "jobs", "CronJob": "cronjobs", "Deployment": "deployments"}


def list_workloads(namespaces, kinds):
    out, errors = [], []
    for ns in namespaces:
        for kind in kinds:
            resource = PLURAL.get(kind)
            if not resource:
                errors.append(f"{kind}: unsupported by poc")
                continue
            objs, err = kubectl_json(["get", resource, "-n", ns, "--show-managed-fields=true"])
            if err:
                errors.append(f"{kind}/{ns}: {err}")
                continue
            for item in objs.get("items", []):
                out.append({
                    "namespace": ns,
                    "name": item["metadata"]["name"],
                    "kind": kind,
                    "uid": item["metadata"]["uid"],
                    "resourceVersion": item["metadata"]["resourceVersion"],
                    "object": item,
                })
    return out, errors


def suspend_managers(obj):
    managers = []
    for mf in obj["metadata"].get("managedFields", []):
        spec_fields = mf.get("fieldsV1", {}).get("f:spec", {})
        if "f:suspend" in spec_fields:
            managers.append(mf.get("manager", "?"))
    return managers


def is_suspended(obj, kind):
    if kind in ("Job", "CronJob"):
        return bool(obj.get("spec", {}).get("suspend", False))
    return False


def preflight(kind, namespace, impersonate):
    group = "batch" if kind != "Deployment" else "apps"
    result = kubectl(["auth", "can-i", "patch", f"{PLURAL[kind]}.{group}", "-n", namespace],
                     impersonate=impersonate)
    return result.stdout.strip() == "yes"


def guarded_patch(wl, pointer, value, impersonate):
    """JSONPatch with test preconditions: uid identity and final eligibility."""
    patch = [
        {"op": "test", "path": "/metadata/uid", "value": wl["uid"]},
        {"op": "test", "path": pointer, "value": False} if value is True else None,
        {"op": "add", "path": pointer, "value": value},
    ]
    patch = [p for p in patch if p]
    return kubectl(["patch", PLURAL[wl["kind"]], wl["name"], "-n", wl["namespace"],
                    "--type=json", f"--field-manager={FIELD_MANAGER}",
                    "-p", json.dumps(patch)], impersonate=impersonate)


def resolve_pending(state, receipts, workloads_by_uid):
    """After restart: verify any pending operation before acting again."""
    for key, entry in state.data.items():
        pending = entry.get("pendingOp")
        if not pending:
            continue
        uid = key.split(":", 1)[1]
        wl = workloads_by_uid.get(uid)
        if wl is None:
            receipts.write_rule_level("UnknownOutcome", {
                "operation": pending,
                "note": "pending operation found at startup; workload gone; outcome unknown",
            })
            entry["pendingOp"] = None
            entry["actedInEpoch"] = True
            continue
        if is_suspended(wl["object"], wl["kind"]):
            receipts.write("ExecutedVerifiedAfterRestart", wl, {
                "epoch": entry["epoch"], "operation": pending,
                "note": "pending operation found at startup; live object matches the intent",
            })
            entry["pendingOp"] = None
            entry["actedInEpoch"] = True
            entry["weSuspended"] = True
        else:
            receipts.write("UnknownOutcome", wl, {
                "epoch": entry["epoch"], "operation": pending,
                "note": "pending operation found at startup; live object does not match; "
                        "automation blocked for this workload pending human review",
            })
            entry["pendingOp"] = None
            entry["actedInEpoch"] = True


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--rules", default=str(pathlib.Path(__file__).parent / "rules.yaml"))
    parser.add_argument("--catalog", required=True)
    parser.add_argument("--mode", default=None, help="override rule mode (Observe|Enforce)")
    parser.add_argument("--impersonate", default=None)
    parser.add_argument("--interval", type=float, default=10.0)
    parser.add_argument("--max-loops", type=int, default=0)
    parser.add_argument("--run-id", default=time.strftime("%H%M%S"))
    args = parser.parse_args()

    rule = yaml.safe_load(pathlib.Path(args.rules).read_text())
    spec = rule["spec"]
    rule_name = rule["metadata"]["name"]
    mode = args.mode or spec.get("mode", "Enforce")
    allowance = spec.get("allowance", {})
    re_arm = bool(allowance.get("reArmOnResume", False))
    actions_per_epoch = int(allowance.get("actionsPerEpoch", 1))
    budget = spec.get("budget", {}).get("maxActionsPerLoop", 10)

    handles = KartaHandles(args.catalog)
    receipts = Receipts(rule_name, args.run_id)
    state = State(rule_name)

    log(f"rr-poc r2 start rule={rule_name} mode={mode} reArmOnResume={re_arm} "
        f"actionsPerEpoch={actions_per_epoch} impersonate={args.impersonate or '-'} "
        f"(reads/state use ambient identity; actions/preflight impersonate) "
        f"suspend handles: {sorted(k for k, v in handles.handles.items() if v['suspend'])}")

    first_loop = True
    loops = 0
    while True:
        loops += 1
        actions_this_loop = 0
        hits, err = query_condition_workloads(spec["condition"]["promql"])
        if hits is None:
            receipts.write_rule_level("FactsUnavailable", {"error": err[:300],
                                                           "note": "no action without facts (fail closed, visibly)"})
        else:
            workloads, list_errors = list_workloads(spec["filter"]["namespaces"], spec["filter"]["workloadKinds"])
            for list_error in list_errors:
                receipts.write_rule_level("TargetsUnavailable", {"error": list_error[:300]})
            if first_loop:
                resolve_pending(state, receipts, {w["uid"]: w for w in workloads})
                first_loop = False

            for wl in workloads:
                entry = state.entry(wl["uid"])
                suspended = is_suspended(wl["object"], wl["kind"])

                if entry["weSuspended"] and not suspended:
                    entry["epoch"] += 1
                    entry["actedInEpoch"] = False
                    entry["weSuspended"] = False
                    entry["externalResumes"] += 1
                    receipts.write("ResumeDetected", wl, {
                        "epoch": entry["epoch"],
                        "note": "external resume detected (suspend flipped false); actor unknown",
                        "suspendFieldManagers": suspend_managers(wl["object"]),
                    })

                key = (wl["namespace"], wl["name"], wl["kind"])
                if key not in hits or suspended:
                    continue
                detail = {"epoch": entry["epoch"],
                          "evidence": {"promql": spec["condition"]["promql"], **hits[key]}}

                if entry["pendingOp"]:
                    continue  # unresolved operation blocks automation
                if entry["actedInEpoch"]:
                    continue  # cap consumed this epoch; receipts already exist

                if entry["externalResumes"] > 0 and not re_arm:
                    if entry.get("escalatedEpoch") != entry["epoch"]:
                        name = receipts.write("EscalatedNeedsHuman", wl, {
                            **detail,
                            "note": "eligible again after an external resume; automated cap "
                                    "consumed (one initial action; no re-arm); human decision required",
                        })
                        if name:
                            entry["escalatedEpoch"] = entry["epoch"]
                    continue

                pointer, value, karta_name = handles.suspend_pointer(wl["kind"])
                if pointer is None:
                    if entry.get("noHandleReported") != entry["epoch"]:
                        name = receipts.write("SkippedNoHandle", wl, {
                            **detail, "karta": karta_name,
                            "note": "no interpretable suspendDefinition; never guess",
                        })
                        if name:
                            entry["noHandleReported"] = entry["epoch"]
                    continue
                detail.update({"karta": karta_name, "patchPointer": pointer, "patchValue": value})

                ready = preflight(wl["kind"], wl["namespace"], args.impersonate) if args.impersonate else True
                if mode == "Observe":
                    receipts.write("WouldAct", wl, {**detail, "actionReady": ready,
                                                    "note": "observe mode; action cap NOT consumed"})
                    continue

                if not ready:
                    receipts.write("SkippedNotReady", wl, {
                        **detail,
                        "reason": f"identity {args.impersonate} cannot patch {wl['kind']} in {wl['namespace']}",
                    })
                    continue

                if actions_this_loop >= budget:
                    receipts.write("SkippedBudget", wl, {**detail, "budget": budget})
                    continue

                op_key = f"{rule_name}/{wl['uid']}/e{entry['epoch']}/suspend"
                intent = receipts.write("Intended", wl, {**detail, "operation": op_key})
                if intent is None:
                    continue  # no persisted intent, no action
                entry["pendingOp"] = op_key
                if not state.save():
                    receipts.update(intent, "AbortedStateUnpersisted", wl, detail)
                    entry["pendingOp"] = None
                    continue

                result = guarded_patch(wl, pointer, value, args.impersonate)
                entry["pendingOp"] = None
                if result.returncode == 0:
                    receipts.update(intent, "Executed", wl, {**detail, "operation": op_key})
                    entry["actedInEpoch"] = True
                    entry["weSuspended"] = True
                    actions_this_loop += 1
                elif result.timed_out:
                    entry["actedInEpoch"] = True  # unknown outcome blocks retries
                    receipts.update(intent, "UnknownOutcome", wl,
                                    {**detail, "operation": op_key, "error": "kubectl timeout"})
                elif "test failed" in result.stderr.lower() or "does not match" in result.stderr.lower():
                    receipts.update(intent, "FailedPrecondition", wl,
                                    {**detail, "operation": op_key, "error": result.stderr.strip()[:300],
                                     "note": "object changed between decision and write; re-deciding next loop"})
                else:
                    receipts.update(intent, "FailedVisible", wl,
                                    {**detail, "operation": op_key, "error": result.stderr.strip()[:300]})

        state.save()
        if args.max_loops and loops >= args.max_loops:
            log(f"max loops reached ({loops}), exiting")
            return 0
        time.sleep(args.interval)


if __name__ == "__main__":
    sys.exit(main())
