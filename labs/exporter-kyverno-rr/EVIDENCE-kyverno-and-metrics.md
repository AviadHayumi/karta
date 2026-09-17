# Is it ok to drive Kyverno from metrics? The evidence

Question from the owner, 2026-09-17: I proposed inside NVIDIA to let
Kyverno read Prometheus and act on workloads. Is that an accepted use of
Kyverno, do serious users do it, or am I alone and abusing the product?

Short answer, in three lines:

- The door is theirs. Kyverno added calls to external services in 1.10
  because it was "one of the most requested features", added the cached
  version (GlobalContextEntry) in 1.12, and the CEL `http` library in the
  new policy types. Reading an external API inside a policy decision is a
  supported, documented feature, not a hack.
- The pattern is published by the people who own the product. Kyverno's
  own policy library ships a Kubecost category: a policy calls the
  Kubecost API (a Prometheus-backed cost model) and denies a Deployment
  when the predicted cost overruns the budget. Nirmata published the same
  with OpenCost. A Kyverno maintainer wrote the Kubecost one.
- Nobody has published raw PromQL in a Kyverno policy, and nobody has
  published Kyverno suspending or deleting workloads from utilization.
  On that exact step we are first. The pieces are all theirs; the
  combination is ours. That is what the KEP's gaps section is about.

Second look by astra (Codex, live web search, same day): same verdict.
"Yes, this is a legitimate use of documented Kyverno features and fits
its stated purpose. He is not first to connect telemetry to policy
decisions. The narrower claim, 'PromQL-driven suspension or deletion
through Kyverno is common in production', is not established by the
public evidence." Its additions are folded into the tables below.

## 1. The feature is intended for this

| claim | source | kind |
|---|---|---|
| Kyverno's own scope statement: "Kyverno uniquely addresses Policy-Based Automation across security, operations, and optimization concerns by providing policy types that map to each phase of resource configuration lifecycle." | https://kyverno.io/docs/guides/evaluating-policy-engines/ | docs |
| "one of the most requested features has been the ability for it to make calls to services other than the Kubernetes API server" (1.10, 2023), with the caveat "It is new and a bit limited at this point so we can get an understanding of how folks intend to use it" | https://kyverno.io/blog/2023/05/30/kyverno-1.10-released/ | maintainers |
| Mutating existing resources and scheduled deletion are documented product features (MutatingPolicy mutate-existing, DeletingPolicy with a schedule). The docs warn mutation is asynchronous, with variable delay. | https://kyverno.io/docs/policy-types/mutating-policy/#mutating-existing-resources and https://kyverno.io/docs/policy-types/deleting-policy/ | docs |
| Inline calls vs cache, the official table: inline `apiCall` for "real-time, volatile data", `GlobalContextEntry` for "static or slowly changing data"; "Policy evaluations may briefly use slightly stale data between refresh cycles"; inline calls under load "can overwhelm API servers" | https://kyverno.io/docs/policy-types/global-context-caching/ | docs |
| GlobalContextEntry: "cache Kubernetes resources or the results of external API calls for later reference within policies", `refreshInterval`, "beware of stale data" | https://main.kyverno.io/docs/policy-types/cluster-policy/external-data-sources/ | docs |
| CEL `http` library: "real-time validation against third-party systems, remote config APIs, or internal services"; namespaced policies need `--allowHTTPInNamespacedPolicies`; URL allow/block lists | https://kyverno.io/docs/policy-types/cel-libraries/ | docs |
| Maintainers in 2026 on scope: "Validate, Mutate, Generate, and Cleanup ... cover a broader range of automation and Policy-as-Code scenarios", "external data calls ... to meet enterprise extension needs", "Scan, report on, mutate, and generate for existing resources" | https://www.cncf.io/blog/2026/03/19/policy-as-code-flexible-kubernetes-governance-with-kyverno/ | maintainers (Dahu Kuang, Lei Hou, Shuting Zhao) |
| Jim Bugwadia (Kyverno creator): "Policy as Code is not just about validation or enforcing security. It's about automation as well. It's about reducing the overload of additional controllers." | https://nirmata.com/2025/04/28/level-up-your-kubernetes-security-automation-with-policy-as-code/ | vendor |
| Nirmata's recommendation for repeated external data: GlobalContextEntry, "tenfold increase in performance", projections doubled it again | https://nirmata.com/2025/02/19/optimizing-kyverno-policy-enforcement-with-global-context-entry-and-projections/ | vendor |

## 2. Telemetry-derived data already drives published Kyverno policies

| what | mechanism | action | source | kind |
|---|---|---|---|---|
| Kubecost proactive cost control: budget + predicted monthly cost of the Deployment | `context.apiCall.service.url: http://kubecost-cost-analyzer.kubecost:9090/model/budgets` and `/model/prediction/speccost` (GET and POST) | deny the Deployment when predicted cost > remaining budget | https://kyverno.io/policies/kubecost/kubecost-proactive-cost-control/kubecost-proactive-cost-control/ and https://github.com/kyverno/policies/tree/main/kubecost | official policy library, Kyverno >= 1.11 |
| Same, told by Kubecost (IBM/Apptio): "If the budget will not be overrun, Kyverno allows the resource to be created in the cluster; if not, it is denied" | apiCall | deny | https://www.apptio.com/blog/kyverno-and-kubecost/ | vendor, author Chip Zoller, Kyverno maintainer |
| OpenCost namespace cost vs allocation | `apiCall.service.url: http://opencost.opencost:9090/model/allocation/compute?window=1d` | deny / audit | https://nirmata.com/2023/05/24/policy-based-cost-management-in-kubernetes-leveraging-opencost-and-kyverno-for-maximum-efficiency/ | vendor |
| Kubecost cost per namespace, 2022 | a cron job writes Kubecost data into a ConfigMap, Kyverno reads the ConfigMap | audit report when cost > threshold | https://nirmata.com/2022/04/06/the-cost-governance-of-cloud-native-workloads-using-kyverno-kubecost/ | vendor, demo |
| Enable Kubecost continuous rightsizing | mutate: annotate Deployments so Kubecost's own engine resizes them from observed utilization | mutate, the acting is Kubecost's | https://github.com/kyverno/policies/blob/main/kubecost/enable-kubecost-continuous-rightsizing/enable-kubecost-continuous-rightsizing.yaml | official policy library |

Kubecost and OpenCost are Prometheus consumers themselves. So "metrics ->
a service with an API -> Kyverno decides" is already in the official
library. What the library does not have is "Prometheus -> Kyverno"
without the service in the middle, and it has no policy that acts on a
running workload from utilization.

One caveat astra caught and I confirmed in the YAML: both the official
Kubecost policy and Nirmata's OpenCost example ship with
`validationFailureAction: Audit`. The prose says "denied", the code
reports. They prove cost data can drive a policy decision. They do not
show autonomous action. Our lab is the first public run where the action
actually happens.

## 2b. Serious users run Kyverno well beyond admission

| who | what | source | kind |
|---|---|---|---|
| adidas | generates default VPAs with Kyverno and runs a scheduled cleanup policy that deletes blocking PodDisruptionBudgets twice a day; their metric-driven autoscaling is KEDA, not Kyverno. Their own warning: the cleanup "might delete these valid PDBs" during upgrades | https://medium.com/adidoescode/reducing-cloud-costs-of-kubernetes-clusters-c8c1e3bdb669 (Iya Lang, 2024-06-28) | end user |
| Coinbase, Mandiant, Vodafone and others in ADOPTERS.md | generation of common objects across namespaces, provisioning during onboarding, "policy enforcement and automation" | https://github.com/kyverno/kyverno/blob/main/ADOPTERS.md | self-reported |
| Frank Jogeleit (Nirmata) and Johannes Sonner (Deutsche Telekom), KubeCon EU 2026, "Advanced Kyverno Patterns: Automating Platform Security & Operations" | closing lesson number one: "Look at Kyverno as a holistic automation tool." Patterns shown: Kyverno creates RoleBindings, `apiCall` reads certificate data from a Secret, Crossplane and Pulumi integration | https://hosted-files.sched.co/kccnceu2026/52/Kubecon%202026%20-%20Kyverno%20Maintainers%20Track%20Session.pdf | conference slides |
| A 2022 user, Kyverno 1.5.x | "the cluster came to a near halt" after a broad policy with API lookups per request | https://github.com/kyverno/kyverno/issues/3465 | issue, historical |

No maintainer statement against operational automation or against
external calls on the background path was found by either of us.

## 3. The two-hop shape the owner asked about is the common one

The published Kyverno examples all put something between Prometheus and
Kyverno: Kubecost, OpenCost, or a cron job writing a ConfigMap. Our
exporter plus the chart's recording rules is exactly that middle layer:
Prometheus holds the samples, the recording rules turn them into one
number per workload, Kyverno reads the number. The KEP's design choice
(thresholds and windows live in the query, the exporter never judges) is
the same split Kubecost makes.

## 4. Acting on Prometheus data is normal in Kubernetes, just not in Kyverno yet

| who | what it does from Prometheus | source |
|---|---|---|
| KEDA (CNCF graduated) | Prometheus scaler: any PromQL query becomes a scaling signal, scale to zero when idle | https://keda.sh/docs/2.20/scalers/prometheus/ |
| kubernetes-sigs/descheduler | LowNodeUtilization can classify nodes from a Prometheus query instead of requests (`metricsUtilization.source: Prometheus`) | https://github.com/kubernetes-sigs/descheduler |
| k8s-cleaner | the closest analogue: a PromQL query per resource, a Lua condition, then `Transform` or `Delete`. Their examples scale a Deployment to zero on error rate and delete pods over a memory threshold | https://gianlucam76.github.io/k8s-cleaner/getting_started/examples/metrics/metric_based_selection/ |
| Robusta | Alertmanager webhook -> automated remediation playbooks | https://docs.robusta.dev/master/playbook-reference/triggers/prometheus.html |
| kube-green | suspends workloads on a schedule, not on metrics, and saves the previous state for restore | https://kube-green.dev/docs/lifecycle/ |
| Nirmata (2026) | cloud agents right-size against live Prometheus data and open PRs | https://nirmata.com/2026/08/12/how-nirmata-saved-40-in-kuberbnetes-cloud-cost/ |

## 5. What nobody has published, and what that means

- No policy anywhere on GitHub with `GlobalContextEntry` + prometheus,
  `globalContext.Get` + prometheus, or `http.Get` + `/api/v1/query`
  (GitHub code search, 2026-09-17: zero hits each).
- No Kyverno issue or discussion asking for Prometheus as a data source.
- No adopter in ADOPTERS.md (53 entries, among them Bloomberg, Spotify,
  LinkedIn, GitHub, Coinbase, Vodafone, Deutsche Telekom, Red Hat,
  Akamai) describes metrics-driven policies.

So the honest sentence for the team is: Kyverno gives us the external
data door and its own vendor uses it for cost data; we are the first to
put PromQL behind that door and the first to suspend a workload with it.
Our lab shows it works and shows exactly where Kyverno stops being
enough: it cannot tell a failed rule from a quiet one, it re-applies over
a human's resume, and it keeps no record. Those three are the reason the
KEP proposes runtime rules for the acting part and keeps Kyverno for
what it is good at.

## 6. How to say it without sounding like an abuse

- Not "we use Kyverno as a metrics engine". Say "Kyverno already reads
  external APIs in its own Kubecost policies; we read Prometheus the same
  way".
- Not "Kyverno suspends idle jobs". Say "we tested whether Kyverno can
  act on the signal, it can, and we measured three gaps that the acting
  part needs to close".
- Not "nobody does this". Say "the pattern is published for cost, the
  metrics-to-Kyverno hop is ours, and it is a one-page policy".
- Not "supported HTTP calls mean it is production ready". Say "Kyverno
  supplies the condition and the action; freshness, failure behavior,
  disruption limits, resume handling and durable records are still on
  us", which is the KEP's section 4 and 5.

Evidence for our own runs: captures 62 to 75 on the lab branch,
INTEGRATION-LOG.md sections 10 to 14.
