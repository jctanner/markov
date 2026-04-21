# ADR-001: Bare K8s Client vs Argo Workflows for Child Job Orchestration

**Status:** Proposed  
**Date:** 2026-04-21  
**Author:** Raj (Technical Researcher)  
**Deciders:** Dr. Chen, Sam, Prof. Hayes  
**Assignment:** RQ-B1 (Workstream 2 — HIGH priority)  
**Blocks:** Sam's RQ-C2 (checkpoint/resume prototype)

---

## Context

The YAML Agent Job Chaining architecture (RQ4) needs a mechanism for a chain controller to orchestrate child K8s Jobs — one per chain step. Two primary approaches exist:

1. **Bare K8s client** — use `kubernetes` Python client (or `client-go`) to directly create/watch/delete `batch/v1 Job` resources
2. **Argo Workflows** — submit chain executions as Argo `Workflow` CRDs, letting the Argo controller handle child pod orchestration

This ADR evaluates both against our specific requirements: DAG-based step execution, human gates, TMS propagation, context passing, multi-tenant RBAC, and resume-at-failure.

---

## Decision Drivers

1. **Operational complexity** — how much infrastructure do we add?
2. **RBAC surface area** — privilege escalation risk in multi-tenant clusters
3. **Failure handling** — retry, resume, orphan cleanup
4. **Feature coverage** — DAG deps, artifact passing, parallelism, human gates
5. **Resource overhead** — memory/CPU footprint of the orchestration layer
6. **Time-to-prototype** — how fast can Sam build on this?

---

## Options Evaluated

### Option A: Bare K8s Client (Direct API)

The controller uses the `kubernetes` Python client to:
- `create_namespaced_job()` per chain step
- `watch` Job status via the Watch API
- Pass context between steps via ConfigMaps or a shared volume
- Implement DAG scheduling, retries, and cleanup in application code

**RBAC required (controller ServiceAccount):**
```yaml
rules:
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["create", "get", "list", "watch", "delete"]
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "create", "update", "delete"]
```

**Pros:**
- Zero additional infrastructure — no Argo controller, no CRDs, no UI server
- Full control over orchestration logic — custom behaviors for TMS, human gates, LLM-specific patterns
- Lighter RBAC surface — permissions scoped to exactly what's needed
- Smaller resource footprint — no always-on controller pod
- Simpler for small clusters / edge deployments

**Cons:**
- **Must build from scratch:** DAG dependency resolution, retry with backoff, artifact passing, parallel fan-out/fan-in, timeout handling, orphan Job cleanup on controller crash
- **No built-in UI** for workflow monitoring (must build our own, which we're doing anyway for Topic A)
- **No workflow templates** — reuse patterns must be hand-built
- **Orphan cleanup is hard:** If the controller Job/pod crashes, child Jobs keep running. Need finalizer or garbage collection logic.
- **Testing burden:** Every orchestration edge case (partial failure in parallel block, cascading timeout, etc.) must be tested in our code

**Estimated build cost for parity with Argo:** 4-6 weeks of engineering to reach production-grade orchestration (DAG scheduling, retry, cleanup, context passing, observability).

### Option B: Argo Workflows

The controller translates the chain YAML into an Argo `Workflow` CRD and submits it. Argo handles all pod orchestration.

**Infrastructure required:**
- Argo Workflow Controller deployment (stateful, scales with concurrent workflows)
- Argo Server deployment (stateless, optional — provides UI + API)
- CRD installation (`workflows.argoproj.io`, `workflowtemplates`, `cronworkflows`, etc.)
- PostgreSQL or MySQL for workflow archival (optional, recommended for production)

**RBAC required:**
- Argo controller needs broad namespace permissions (list/get/update workflows, create/get/delete pods, PVCs, PDBs, configmaps, secrets)
- Workflow-level ServiceAccounts need minimum `create`/`patch` on `workflowtaskresults`
- For namespaced install, use `--namespaced` flag to restrict controller scope

**Resource overhead (production baseline):**
| Component | CPU Request | Memory Request | Notes |
|-----------|------------|----------------|-------|
| Argo Controller | 500m–2000m | 256Mi–4Gi | Stateful; scales with live workflows |
| Argo Server | 100m | 64Mi | Stateless; optional |
| Executor sidecar | 100m per pod | 64Mi per pod | Per workflow pod |

**Caution:** At scale (5000+ parallel workflows), the controller has been reported to consume 100GB+ memory. Our workloads will be much smaller (10s-100s concurrent chains), so this is not a concern for us.

**Pros:**
- **DAG orchestration is built-in** — declarative DAG with `dependencies:` field, conditional execution, retry policies
- **Artifact passing** — native S3/GCS/HTTP artifact support between steps
- **Retry/timeout** built-in — `retryStrategy`, `activeDeadlineSeconds` per step
- **Built-in UI** — workflow visualization, log viewing, retry from failed node (`argo retry`)
- **Resume from failure** — `argo retry my-wf --node-field-selector` re-runs only failed nodes
- **Production-proven** — most popular K8s workflow engine, CNCF graduated project
- **Hera Python SDK** — Pythonic workflow definition, aligns with our Python codebase

**Cons:**
- **Infrastructure overhead** — always-on controller pod, CRD installation, RBAC setup
- **RBAC surface is larger** — Argo controller needs broad permissions; multi-tenant isolation requires careful namespace scoping
- **Human gates are not native** — Argo has `suspend` nodes but no built-in approval UI. We'd need to build the approval webhook + UI anyway
- **TMS integration requires custom logic** — Argo doesn't know about Truth Maintenance; we'd need to hook our TMS into Argo's event system
- **Abstraction mismatch** — our chain YAML → Argo Workflow YAML is a translation layer that adds complexity and potential semantic drift
- **Version coupling** — tied to Argo release cycle for bug fixes and features

### Option C: Hybrid (Recommended)

Use **bare K8s client for the core orchestration loop** but adopt **specific Argo patterns** where they add clear value:

- **Core orchestration:** Bare K8s client manages Job lifecycle. Our chain engine already has DAG resolution via topological sort — adding K8s Job creation is incremental, not a rewrite.
- **Retry/backoff:** Implement in application code (already designed in RQ-C1 with `retry_strategy` YAML fields). Gives us LLM-specific retry logic (e.g., retry on rate limit with model fallback) that Argo can't express.
- **Context passing:** PostgreSQL checkpoint store (from Topic C design) — not ConfigMaps or Argo artifacts. This is more durable and queryable.
- **Resume:** From checkpoint store, not Argo retry. Enables our four resume strategies (auto-retry, manual, fork, selective re-run).
- **Human gates:** Event-driven via our own API + WebSocket. This is Topic A scope — Argo's `suspend` node doesn't help here.
- **Optional Argo integration:** For teams that already run Argo, provide an **export** target: `chain export --format argo-workflow my-chain.yaml` that generates an Argo Workflow YAML. This is an adapter, not a dependency.

---

## Decision

**Option C: Hybrid — bare K8s client core + optional Argo export adapter.**

### Rationale

1. **We're already building the hard parts.** The chain engine has DAG resolution, the checkpoint store (Topic C) handles resume, and the dashboard (Topic A) handles visualization. Argo would duplicate, not simplify, these components.

2. **LLM-specific orchestration logic doesn't fit Argo's model.** Token budget management, model fallback on rate limits, TMS propagation, context window tracking — none of these map to Argo's generic container orchestration primitives. We'd spend more time working around Argo than working with it.

3. **RBAC is simpler.** Our controller needs `create/watch/delete` on Jobs and ConfigMaps. Argo's controller needs significantly broader permissions. In enterprise multi-tenant environments, less RBAC surface = faster security review.

4. **Resource efficiency.** No always-on Argo controller. The chain controller (whether Job or Deployment) is the only long-running component.

5. **Argo as export target preserves optionality.** Teams with existing Argo infrastructure can use our chain definitions via export. We're not anti-Argo, just not dependent on it.

### What we give up

- Built-in retry/timeout (must build — estimated 1-2 days, already designed)
- Built-in artifact passing (using Postgres checkpoint store instead — more capable)
- Argo UI for debugging (building our own — Topic A)
- Community Argo templates (not applicable to our agent-specific steps)

### RBAC Recommendation

**Namespace-scoped Roles only.** No ClusterRoles for chain orchestration.

- Each chain namespace gets a dedicated `chain-controller` ServiceAccount with a namespace-scoped Role
- Cross-namespace orchestration is explicitly out of scope for v1
- If needed later, implement via Argo Events cross-namespace trigger pattern (not ClusterRole escalation)

---

## Consequences

### Positive
- Simpler infrastructure — no CRD installation, no Argo operator management
- Full control over LLM-agent-specific orchestration patterns
- Tighter security posture with minimal RBAC
- Checkpoint store serves double duty (resume + context passing)
- Export adapter keeps Argo compatibility as a feature, not a dependency

### Negative
- More application code to maintain for Job lifecycle management
- No free UI from Argo (but we're building our own)
- Orphan Job cleanup must be implemented (finalizers + TTL)
- Must implement our own retry/backoff (already designed)

### Risks
- If chain workloads grow to 1000s of concurrent executions, bare K8s client orchestration may hit Watch API limits. **Mitigation:** At that scale, re-evaluate Argo or JobSet API as the orchestration backend. The export adapter would already exist.

---

## Action Items for Sam (RQ-C2 unblocked)

With this ADR decided, Sam can proceed with the checkpoint/resume prototype using:
1. **Bare K8s client** (`kubernetes` Python library) for Job creation/monitoring
2. **SQLite** (dev) / **PostgreSQL** (prod) for checkpoint store
3. **Namespace-scoped RBAC** with the minimal Role defined above
4. Focus on: (a) Job creation per step, (b) checkpoint persistence, (c) `--resume <run_id>` CLI, (d) orphan Job cleanup via TTL

---

## Sources

- [Kubernetes Jobs vs Argo Workflows](https://www.containerssimplified.com/learning/kubernetes-vs-argo/)
- [Argo Workflows Docs](https://argo-workflows.readthedocs.io/en/latest/)
- [Argo Workflow RBAC](https://argo-workflows.readthedocs.io/en/latest/workflow-rbac/)
- [Argo Service Accounts](https://argo-workflows.readthedocs.io/en/latest/service-accounts/)
- [Demystifying Argo Workflows RBAC](https://blog.argoproj.io/demystifying-argo-workflowss-kubernetes-rbac-7a1406d446fc)
- [Argo Cost Optimization](https://argo-workflows.readthedocs.io/en/latest/cost-optimisation/)
- [Argo Controller Memory Issues (#11948)](https://github.com/argoproj/argo-workflows/issues/11948)
- [Introducing JobSet (March 2025)](https://kubernetes.io/blog/2025/03/23/introducing-jobset/)
- [ZenML Kubernetes Orchestrator](https://docs.zenml.io/stacks/stack-components/orchestrators/kubernetes)
- [Hera Python SDK for Argo](https://github.com/argoproj-labs/hera)
- [Disregard K8s Jobs; Replace with Argo](https://medium.com/@michamarszaek/disregard-kubernetes-jobs-replace-them-with-argo-workflows-becc56d2d96a)



---

## Security Addendum (RQ-B1-SEC)

**Added:** 2026-04-21 per Prof. Hayes gap flag and Dr. Chen assignment.

A full security threat model for the controller-as-Job pattern has been completed: `technical/rq-b1-sec-security-threat-model-controller-as-job-pattern` (24.8KB).

**Key findings:**

1. **Blast radius is MEDIUM-HIGH** — lower than Argo (cluster install) or Tekton defaults, but requires explicit mitigation. The `create Jobs` permission is the critical escalation vector.

2. **Five-layer defense-in-depth specified:**
   - Layer 1: Namespace-scoped RBAC (already in this ADR — validated as correct)
   - Layer 2: Default-deny NetworkPolicy with explicit egress allowlist
   - Layer 3: PodSecurity Standards `restricted` profile on namespace
   - Layer 4: K8s audit logging + runtime detection (Falco rules)
   - Layer 5: Admission control (Kyverno) for image allowlisting + resource limits

3. **Two residual MEDIUM risks:** DNS exfiltration (hard to prevent without L7 inspection) and ConfigMap poisoning (recommend HMAC signing of step outputs).

4. **Secrets architecture specified:** LLM API keys injected via External Secrets Operator → projected volume mounts. Controller SA has NO access to Secrets. This is a critical design constraint.

5. **ADR-001 decision validated:** Bare K8s client with namespace-scoped Roles has a smaller attack surface than Argo Workflows or Tekton in their default configurations.

**Sam:** Implementation checklist for prototype security controls is in Section 7 of the threat model doc.
