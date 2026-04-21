# RQ-B1: K8s Meta-Orchestration Architecture Decision Record

**Author:** Raj (Technical Researcher)  
**Date:** 2026-04-21  
**Ticket:** TK-EFA312  
**Status:** DECISION RENDERED  

---

## Decision

**RECOMMENDED: Argo Workflows as child job orchestration layer** over bare K8s client controller pattern.

**Confidence:** HIGH — Argo is the stronger default for this use case. The custom controller path is viable only if the team has operator development experience AND requirements exceed what Argo's DAG model can express.

---

## Context

ChainExecutor needs to be packaged as a K8s Job that spawns and manages child jobs (one per Claude agent skill). Two architectural paths exist:

1. **Bare K8s Client Controller**: A custom controller/operator that uses the Kubernetes client library to directly create, watch, and manage child Job resources
2. **Argo Workflows**: Delegate child job orchestration to Argo's workflow engine, with ChainExecutor submitting Workflow CRDs that Argo's controller manages

This ADR evaluates trade-offs across RBAC requirements, failure modes, resource overhead, and CRD design complexity.

---

## Option 1: Bare K8s Client Controller Pattern

### Architecture

```
ChainExecutor Job Pod
  └── K8s client-go / Python kubernetes-client
       ├── Creates child Job A (Claude skill 1)
       ├── Watches Job A → on completion, creates Job B
       ├── Evaluates gating criteria between jobs
       ├── Creates Job C (conditional on Job B output)
       └── Sets ownerReferences → cascading delete
```

The ChainExecutor pod runs a controller loop that:
1. Creates child K8s Jobs with ownerReferences pointing to itself
2. Watches child Job status via informers/watchers
3. Reads child pod logs or shared volumes for output artifacts
4. Evaluates gating logic (quality scores, pass/fail, budget)
5. Creates next child Job(s) based on gate results

### RBAC Requirements

**Extensive — must be hand-crafted:**

```yaml
rules:
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["create", "get", "list", "watch", "delete"]
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list"]  # For inter-job data passing
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["create", "get", "delete"]  # For shared artifact volumes
```

**Security concerns:**
- Service account needs `create` on Jobs — a powerful permission in multi-tenant clusters
- ownerReference + finalizer interaction requires finalizer permissions: `cannot set blockOwnerDeletion if an ownerReference refers to a resource you can't set finalizers on` (known K8s RBAC gotcha)
- If ChainExecutor pod is compromised, attacker can spawn arbitrary Jobs in the namespace
- No built-in audit trail of what was created and why
- Must enforce `runAsNonRoot`, `readOnlyRootFilesystem`, drop capabilities manually

**RBAC blast radius: MEDIUM-HIGH.** The service account essentially becomes a mini-operator within its namespace.

### Failure Modes

| Failure | Impact | Mitigation Complexity |
|---|---|---|
| ChainExecutor pod OOM/eviction | All orchestration state lost. Child jobs become orphans (ownerRef cascade deletes them, losing partial work) | HIGH — must implement external state persistence |
| Child job timeout | Must implement custom timeout detection + cleanup | MEDIUM — `activeDeadlineSeconds` on child Jobs helps |
| Network partition | Informer/watcher disconnects, misses child completion events | HIGH — must implement resync logic |
| API server rate limiting | Job creation throttled, workflow stalls | MEDIUM — implement backoff |
| ownerReference cascade delete race | Parent deleted before children complete → all work lost | HIGH — requires finalizer implementation |
| Partial failure recovery | No built-in replay — must implement idempotent job creation | HIGH — significant engineering effort |

**Critical gap:** If the ChainExecutor pod dies mid-workflow, there is no replay mechanism. The entire chain must restart from scratch unless the developer builds external state persistence (database, ConfigMap, etc.). This is the fundamental weakness of this pattern for long-running Claude agent chains.

### Resource Overhead

- **Control plane:** Zero additional infrastructure — just the ChainExecutor pod itself
- **Per-chain:** ~50-100Mi memory for the controller loop + K8s client library
- **No CRDs installed** — purely uses native K8s resources
- **Total footprint:** Minimal — just the one extra pod per active chain

### CRD Design

**No custom CRDs required** — this is a pure-K8s-primitives approach. Chain definitions could be stored as ConfigMaps or passed as environment variables. This is both a strength (simplicity) and weakness (no declarative workflow definition, no kubectl-native workflow management).

### Development Effort

- **Initial build:** 2-4 weeks for a production-quality controller with proper error handling, timeout management, cleanup, and state persistence
- **Ongoing maintenance:** Operator development patterns are non-trivial. Bug surface area includes informer caching, resource version conflicts, finalizer edge cases, leader election (if HA needed)
- **Testing:** Requires integration test infrastructure (kind/k3s clusters, mock API servers)
- **Expertise required:** Deep Kubernetes API knowledge, controller-runtime or client-go experience

---

## Option 2: Argo Workflows as Child Job Orchestration Layer

### Architecture

```
ChainExecutor (submitter)
  └── Argo REST API / Hera Python SDK
       └── Creates Workflow CR (DAG definition)
            └── Argo Workflow Controller manages:
                 ├── Pod A (Claude skill 1)
                 ├── Pod B (Claude skill 2, depends on A)
                 ├── Gating evaluation (when: conditions)
                 └── Pod C (conditional, fan-out/fan-in)
```

ChainExecutor becomes a **workflow submitter**, not a controller. It:
1. Constructs a Workflow CR (YAML or via Hera Python SDK)
2. Submits to Argo via REST API or K8s API
3. Polls workflow status or subscribes to events
4. Retrieves outputs from completed workflow

### RBAC Requirements

**Narrow — ChainExecutor only needs Workflow CR permissions:**

```yaml
# ChainExecutor service account
rules:
- apiGroups: ["argoproj.io"]
  resources: ["workflows"]
  verbs: ["create", "get", "list", "watch"]
# No direct Job/Pod creation permissions needed
```

**Argo's own service account** (managed by Argo installation) handles Job/Pod creation. This separation of concerns is a significant security advantage.

**Security benefits:**
- ChainExecutor cannot create arbitrary Jobs — only Workflow CRs that Argo validates
- Argo supports `WorkflowRestrictions` to limit what templates/resources workflows can use
- Built-in SSO integration (OIDC, OAuth2) for the Argo dashboard
- Workflow-level RBAC via Kubernetes RBAC on the `argoproj.io` API group
- Audit trail: every workflow step is recorded in the Workflow CR status

**RBAC blast radius: LOW.** ChainExecutor has minimal permissions. Argo's controller has broad permissions but is a well-audited CNCF graduated project.

### Failure Modes

| Failure | Impact | Mitigation Complexity |
|---|---|---|
| ChainExecutor pod dies | Workflow continues running independently — Argo controller owns it. ChainExecutor can reconnect on restart. | NONE — self-healing |
| Child step OOM/crash | Argo `retryStrategy` auto-retries with backoff. `podSpecPatch` can increase resources on retry. | LOW — built-in |
| Workflow timeout | `activeDeadlineSeconds` on Workflow CR. Exit handlers run cleanup. | LOW — declarative |
| Argo controller crash | HA standby takes over. In-flight workflows resume from last known state. | LOW — built-in HA |
| API server rate limiting | Argo has built-in `resourceRateLimit` and queue management | LOW — configuration |
| Partial failure recovery | `argo retry` reruns only failed steps. Manual `argo resubmit` for full restart. | LOW — built-in CLI |

**Critical advantage:** Argo decouples workflow state from the submitter process. If ChainExecutor dies, the workflow is unaffected. This is the fundamental advantage for long-running Claude agent chains.

### Resource Overhead

**Argo controller footprint (production baseline):**
- **Argo Server:** ~100m CPU, 64-128Mi memory (stateless, lightweight)
- **Workflow Controller:** Scales linearly with active workflows
  - <100 concurrent workflows: 500m CPU, 1Gi memory sufficient
  - 100-1000 concurrent workflows: 1-4Gi memory, 1-2 CPU cores
  - 5000+ concurrent workflows: Can spike to tens of GB (requires TTL strategy, pod GC, workflow archival)
- **CRDs installed:** Workflow, WorkflowTemplate, ClusterWorkflowTemplate, CronWorkflow (4 CRDs)
- **Per-workflow overhead:** ~10-20KB for Workflow CR status (grows with step count)

**CNOE benchmark (EKS, Argo 3.5.4):** Controller processed up to 2,100 workflows/min with 128 workers and 5 shards on a single m5.8xlarge instance.

**For this use case:** We're running <50 concurrent Claude agent chains at most. Argo controller at 500m CPU / 1Gi memory is sufficient. Total infrastructure cost: ~$15-30/month on cloud.

### CRD Design

**Uses Argo's existing CRDs** — no custom CRD design needed. Workflow definitions are declarative YAML:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
spec:
  entrypoint: claude-chain
  templates:
  - name: claude-chain
    dag:
      tasks:
      - name: skill-1
        template: claude-skill
        arguments:
          parameters: [{name: skill, value: "analyze"}]
      - name: gate-check
        template: evaluate-gate
        dependencies: [skill-1]
        arguments:
          parameters: [{name: input, value: "{{tasks.skill-1.outputs.parameters.result}}"}]
      - name: skill-2
        template: claude-skill
        dependencies: [gate-check]
        when: "{{tasks.gate-check.outputs.parameters.pass}} == true"
```

**WorkflowTemplates** enable reusable skill definitions. ChainExecutor constructs the DAG dynamically based on chain configuration, then submits as a Workflow CR.

### Development Effort

- **Initial setup:** 2-3 days for Argo Helm installation + RBAC configuration
- **Integration:** 1-2 weeks to build ChainExecutor submission logic via Hera SDK + status polling
- **Gating logic:** Argo's `when` conditionals + output parameters handle 80% of gating patterns. Custom evaluation steps (Python containers) handle the remaining 20%.
- **Ongoing maintenance:** Argo is a CNCF graduated project with 200+ organizations using it. Maintenance burden is primarily version upgrades and configuration tuning.
- **Expertise required:** YAML workflow definitions, basic K8s CRD understanding. Much lower bar than custom controller development.

---

## Comparative Analysis

| Dimension | Bare Controller | Argo Workflows |
|---|---|---|
| **RBAC blast radius** | MEDIUM-HIGH (direct Job/Pod creation) | LOW (only Workflow CR creation) |
| **Failure recovery** | DIY — must build state persistence | Built-in — retry, resume, resubmit |
| **Submitter crash resilience** | Catastrophic — loses orchestration state | Self-healing — workflow continues |
| **Development effort** | 2-4 weeks + ongoing operator maintenance | 2-3 days setup + 1-2 weeks integration |
| **Resource overhead** | Minimal (no extra infrastructure) | Moderate (~500m CPU, 1Gi RAM for controller) |
| **CRD complexity** | None (uses native K8s) | 4 CRDs (Argo-managed) |
| **Observability** | DIY (Prometheus metrics, custom logging) | Built-in (UI, logging, artifact tracking, metrics) |
| **Gating patterns** | Arbitrary code (maximum flexibility) | Declarative `when` + custom eval steps (90% coverage) |
| **Scalability** | Limited by single controller pod | Proven to 2,100 workflows/min (CNOE benchmark) |
| **Debugging** | kubectl logs + custom tooling | Argo UI + `argo logs` + artifact inspection |
| **Human approval gates** | Custom webhook/polling implementation | Built-in `suspend` step + resume API |
| **Multi-tenancy** | Must implement namespace isolation | Built-in namespace partitioning + WorkflowRestrictions |

---

## Decision Rationale

### Why Argo Workflows wins for ChainExecutor:

1. **Crash resilience is the decisive factor.** Claude agent chains can run for 30-180 minutes. If ChainExecutor dies mid-chain with the bare controller pattern, all progress is lost and expensive LLM calls must be repeated. With Argo, the workflow continues independently. This alone justifies the architectural choice.

2. **RBAC security is materially better.** ChainExecutor doesn't need Job/Pod creation permissions — it only creates Workflow CRs. The attack surface is significantly reduced.

3. **Development velocity.** Building a production-quality controller with proper failure handling, state persistence, cleanup, and observability takes 4-8 weeks. Argo integration takes 1-2 weeks. The team ships faster.

4. **Built-in observability.** Argo's UI, logging, and artifact tracking eliminate the need to build custom monitoring for workflow execution.

5. **Resource overhead is acceptable.** ~$15-30/month for the Argo controller is negligible compared to the Claude API costs of the agent chains themselves.

### When bare controller would be preferred (NOT this use case):

- ChainExecutor needs to manage non-Job K8s resources (StatefulSets, Services, custom CRDs) as part of the chain
- Workflow logic is deeply domain-specific and doesn't map to DAG/step models
- Organization has existing operator expertise and CI/CD for controller lifecycle
- Extreme resource constraints where even 1Gi of overhead matters

### Risk mitigation for Argo path:

- **Community health concern:** <5 active core contributors as of 2025 surveys. Mitigated by: CNCF graduated status, Intuit/BlackRock production usage, Pipekit commercial support.
- **Controller memory at scale:** Implement TTL strategy (delete completed workflows after 24h), pod GC, and workflow archive from day 1.
- **Lock-in:** Workflow definitions are YAML — portable concepts even if Argo is replaced. Hera SDK abstracts submission layer.

---

## Implementation Guidance for Sam

**Recommended implementation steps:**

1. **Install Argo Workflows** via Helm into the prototype cluster (2-3 hours)
2. **Create WorkflowTemplate** for Claude skill execution (reusable across chains)
3. **Build ChainExecutor** as a Python module using Hera SDK:
   - Accept chain definition (ordered skills + gating criteria)
   - Generate Argo Workflow YAML dynamically
   - Submit via Argo REST API
   - Poll status and return results
4. **Implement one gating pattern** — output-based quality threshold — as an Argo `when` conditional reading `status.json` from the Claude skill container
5. **Test crash resilience** — kill ChainExecutor mid-chain, verify workflow completes

**Needs prototype validation:** ⚠️ Inter-step data passing (Claude output → next skill input) via Argo parameters vs. shared volumes. The optimal pattern depends on output size.

---

## Sources

- [Argo Workflows — Official Docs](https://argo-workflows.readthedocs.io/en/latest/)
- [Argo Workflows Controller Scalability Testing — CNOE/AWS](https://cnoe.io/blog/argo-workflow-scalability)
- [K8s Jobs Spawning Child Jobs — GitHub Example](https://github.com/wlk/kubernetes-jobs-and-child-jobs)
- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Kubernetes Finalizers](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/)
- [Operator SDK Best Practices](https://sdk.operatorframework.io/docs/best-practices/best-practices/)
- [RBAC Markers for K8s Operators — OneUpTime](https://oneuptime.com/blog/post/2026-02-09-operator-rbac-markers/view)
- [Kubernetes RBAC Good Practices](https://kubernetes.io/docs/concepts/security/rbac-good-practices/)
- [Kubernetes Operators in 2025 — OuterByte](https://outerbyte.com/kubernetes-operators-2025-guide/)
- [Disregard K8s Jobs, Use Argo — Medium](https://medium.com/@michamarszaek/disregard-kubernetes-jobs-replace-them-with-argo-workflows-becc56d2d96a)
- [Kubernetes Jobs vs Argo Workflows — ContainersSimplified](https://www.containerssimplified.com/learning/kubernetes-vs-argo/)
- [Production Argo Workflows Installation — Pipekit](https://pipekit.io/blog/production-install-of-argo-workflows)
- [Argo Controller Memory Issues — GitHub #11948](https://github.com/argoproj/argo-workflows/issues/11948)
- [Jobs Manager Operator — GitHub](https://github.com/lukaszraczylo/jobs-manager-operator)
