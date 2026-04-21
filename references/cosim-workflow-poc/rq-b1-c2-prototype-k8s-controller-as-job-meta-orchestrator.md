# RQ-B1/C2 Prototype: K8s Controller-as-Job Meta-Orchestrator

## Overview

Implements the controller-as-Job pattern from Raj's ADR-001. The chain executor itself runs as a K8s Job that creates and monitors child Jobs for each chain step, with integrated checkpoint persistence and resume-from-failure.

**Repo:** `yaml-agent-chain-poc` | **Commit:** `4fba8da8`

## Architecture

```
┌─────────────────────────────────────────┐
│  Controller Job (k8s_controller.py)     │
│  ┌───────────────────────────────────┐  │
│  │  K8sJobExecutor                   │  │
│  │  - Reads chain YAML              │  │
│  │  - Dispatches steps as child Jobs │  │
│  │  - Monitors completion via poll   │  │
│  │  - Checkpoints to PVC/SQLite     │  │
│  │  - Runs gates/rules in-process   │  │
│  └─────────┬─────────────────────────┘  │
│            │ ownerReferences             │
└────────────┼────────────────────────────┘
     ┌───────┼───────┬───────────────┐
     ▼       ▼       ▼               ▼
  ┌──────┐┌──────┐┌──────┐     ┌──────────┐
  │Step 1││Step 2││Step 3│ ... │Step N    │
  │ Job  ││ Job  ││ Job  │     │ Job      │
  └──────┘└──────┘└──────┘     └──────────┘
```

## Key Design Decisions

### 1. Bare K8s Client (Not Argo) — per ADR-001
- `kubernetes` Python client for Job lifecycle
- Our chain engine already has DAG resolution, TMS, checkpoint store
- Argo would duplicate, not simplify

### 2. Namespace-Scoped RBAC Only
- ServiceAccount: `chain-executor`
- Role: create/get/list/watch/delete Jobs + get/list pods + get pods/log
- No ClusterRoles, no cross-namespace access
- Blast radius: compromised controller can only manage Jobs in its namespace

### 3. Context Passing via Checkpoint Store
- NOT ConfigMaps (per Raj's decision)
- Registered vars stored as JSONB in checkpoint DB
- Step Jobs write output to stdout as JSON; controller reads via pod logs
- Checkpoint store shared via PVC mount

### 4. Orphan Cleanup via ownerReferences
- Child Jobs get ownerReferences pointing to controller Job
- K8s GC automatically cleans up children when controller deleted
- Explicit `cleanup_orphans()` method for manual cleanup on failure

### 5. NetworkPolicy
- Child step pods: egress restricted to DNS only
- Controller pod: egress to K8s API + DNS only
- No cross-namespace traffic
- Addresses Prof. Hayes' security concern about blast radius

## Step Dispatch Strategy

| Step Type | Execution Mode | Why |
|-----------|---------------|-----|
| Task | K8s Job | Isolation, resource limits, auditable |
| Parallel | Concurrent K8s Jobs | True parallelism via K8s scheduling |
| Gate | In-process | Needs working memory + approval webhook |
| Rule | In-process | Needs GateEvaluator + working memory |
| Block | Mixed | Children dispatch per type; rescue/always in-process |

## Resume Flow

```
1. Original run fails at step 3
2. User: python k8s_controller.py chain.yaml --resume <run_id>
3. Controller loads checkpoints for run_id
4. Steps 1-2: loaded from checkpoint (status: "cached")
5. Steps 3-N: re-dispatched as fresh K8s Jobs
6. New run linked to original via resumed_from
```

## Files

| File | Purpose |
|------|---------|
| `k8s_executor.py` | K8sJobExecutor — dispatches steps as K8s Jobs |
| `k8s_controller.py` | CLI entrypoint for the controller Job |
| `k8s_manifests/rbac.yaml` | Namespace-scoped RBAC (ServiceAccount, Role, RoleBinding) |
| `k8s_manifests/controller-job.yaml` | Controller Job manifest + PVC |
| `k8s_manifests/network-policy.yaml` | NetworkPolicy for blast radius containment |
| `k8s_manifests/namespace.yaml` | Namespace definition |
| `examples/k8s_controller_chain.yaml` | Demo chain for K8s execution |
| `tests/test_k8s_executor.py` | 12 tests with mocked K8s client |

## Test Coverage (12 tests)

- Job creation per step (3 tests: count, naming, labels)
- ownerReferences propagation
- Step output registration in context
- Step failure handling (early termination)
- Gate runs in-process (not as K8s Job)
- Parallel dispatch (concurrent child Jobs)
- Checkpoint persistence after each step
- Resume-from-failure (skip completed, re-execute from failure)
- Resume run linkage (resumed_from)
- Orphan Job cleanup
- Manifest builder (structure + PVC mount)

## Limitations (Intentional for PoC)

- MockK8sClient for testing; RealK8sClient requires cluster access
- Step Jobs use stub entrypoint (no real module import)
- Poll interval set to 0.1s for testing speed (production: 2-5s)
- SQLite checkpoint store (production: PostgreSQL)
- No Prometheus metrics (production: job duration, retry counts)
- No webhook-based gate approval (PoC auto-approves)

## Production Path

1. Replace MockK8sClient with RealK8sClient (in-cluster ServiceAccount auth)
2. Swap SQLite → PostgreSQL for checkpoint store
3. Add Prometheus metrics for observability
4. Implement webhook gate approval endpoint
5. Add Pod Security Standards (restricted profile)
6. Consider kubernetes-sigs/agent-sandbox for step isolation (gVisor/Kata)
