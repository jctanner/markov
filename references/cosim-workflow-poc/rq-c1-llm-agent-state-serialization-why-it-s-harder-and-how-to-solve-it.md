# RQ-C1: LLM Agent State Serialization Analysis
## Why Agent Checkpointing Is Harder Than Traditional Workflow Checkpointing

**Author:** Raj (Technical Researcher)  
**Date:** 2026-04-21  
**Assignment:** RQ-C1 (Workstream 2 — HIGH priority)  
**Related:** ADR-001 (RQ-B1), Follow-Up Technical Analysis (Topics A/B/C)

---

## Executive Summary

**Conclusion: LLM agent state serialization is harder than traditional workflow state for five specific reasons.** All five have emerging solutions, but none are fully production-solved. The best strategy for our architecture is **per-step checkpoint to PostgreSQL with explicit state registration** — avoiding the hard serialization problems entirely by treating each step's output as the serialization boundary.

---

## Why LLM Agent State Is Harder to Serialize

### 1. Non-Deterministic Outputs

Traditional workflows: step f(x) → y is deterministic. Replay from checkpoint produces identical results.

LLM agents: step f(x) → y₁ or y₂ or y₃. The same prompt to the same model can produce different outputs due to temperature, sampling, and model updates. This means:
- **Replay ≠ Resume.** You can't "replay" an LLM call — you must checkpoint the actual output.
- **Debugging is harder.** Time-travel debugging (LangGraph's approach) must store the actual LLM response, not just the inputs.
- **Branching history.** A resumed chain may diverge from the original execution path because a retried LLM call produces different output, which may trigger different conditional branches downstream.

**Implication for our design:** Every `llm_invoke` step MUST checkpoint its actual output. The `idempotent: false` marker in our YAML schema is essential.

### 2. Context Window as Hidden State

Traditional workflows: state is the explicit inputs/outputs of each step. There is no implicit accumulated state.

LLM agents: the **conversation history / context window** is a massive, implicit, growing state object. A chain that runs 10 steps may accumulate 50K+ tokens of context. This creates:
- **Large checkpoint payloads.** A single step checkpoint might be 100KB+ of conversation history, vs. a few KB for traditional workflow state.
- **Context compression dilemma.** If you compress context between steps (summarization, sliding window), the checkpoint of the compressed context is lossy — resume may not produce identical behavior.
- **Token budget coupling.** Checkpoint/resume must also restore the token budget state: how many tokens remain, what was the budget at each step, whether model fallback was triggered.

**Implication for our design:** The checkpoint store must handle JSONB payloads up to ~500KB per step (context window + output). PostgreSQL handles this natively. The `register:` pattern captures what matters — we checkpoint registered variables, not the raw conversation history.

### 3. Tool State and Side Effects

Traditional workflows: steps are typically self-contained — read input, produce output.

LLM agents: steps invoke external tools (APIs, databases, file systems, code execution sandboxes) that create real-world side effects. This creates:
- **Idempotency problems.** Retrying a step that sent an email or created a Jira ticket produces duplicate side effects.
- **Tool state is external.** A code execution sandbox may have in-memory state (variables, file handles) that isn't captured by checkpointing the step output.
- **Tool credential state.** OAuth tokens, session cookies, API rate limit state — all may expire between checkpoint and resume.

**Implication for our design:** Steps with side effects need an `idempotency_key` pattern: `f"{chain_run_id}:{step_name}:{attempt}"`. The YAML schema should support `side_effects: [email, jira_create]` annotations so the resume logic can warn before re-executing.

### 4. Multi-Agent Coordination State

Traditional workflows: single execution thread with clear step boundaries.

Multi-agent systems: multiple agents running concurrently, sharing memory, passing messages, maintaining individual and collective state. Serializing this requires:
- **Distributed snapshot consistency** — a Chandy-Lamport–style protocol to capture a consistent cut across agents (2026 research confirms this is still an open problem for LLM multi-agent systems)
- **Shared memory contention** — read/write ordering must be preserved across checkpoint/resume
- **Agent identity state** — each agent's role, persona, accumulated knowledge, and relationship to other agents

**Implication for our design:** For v1, our chains are single-threaded (even `parallel` blocks fan out to independent steps with no shared mutable state). Multi-agent checkpoint consistency is a v2 concern. Flag for future architecture review.

### 5. Python Runtime State

Many LLM agent frameworks are Python-based and maintain state in Python objects that resist serialization:
- **Closures and lambdas** — not pickleable
- **Open file handles, database connections** — not serializable
- **Generator/coroutine state** — partially supported by `dill` but fragile
- **Global interpreter state** — projects like CloudPicker and PyCheckpoint have tried and hit limitations

**Implication for our design:** Don't try to serialize Python runtime state. Our YAML-driven approach already avoids this — each step is a fresh process/container that receives explicit inputs via the context store. This is a design advantage over framework-embedded approaches like raw LangGraph.

---

## Checkpoint/Resume Patterns in Production Systems (2025-2026)

### Temporal (Durable Execution)
- **Mechanism:** Event-sourced history; replay workflow function from event log
- **Agent support:** OpenAI Agents SDK + Temporal integration (2025) provides automatic checkpoint recovery for AI agents
- **Strengths:** Invisible to developer; code "just resumes." Activity-level granularity.
- **Weakness for agents:** Requires deterministic workflow code. LLM calls must be wrapped as Activities (non-deterministic side effects), not inline workflow logic.
- **Resume:** Automatic on worker restart. Manual via `temporal workflow reset`.

### Argo Workflows (K8s-native)
- **Mechanism:** Workflow status stored as K8s resource; `argo retry` re-runs failed nodes
- **Strengths:** K8s-native, no external state store. Selective node retry.
- **Weakness for agents:** No checkpoint of step outputs beyond pod logs/artifacts. Context passing between steps requires explicit artifact config.
- **Resume:** `argo retry <wf-name>` — re-runs failed/errored nodes, preserves successful.

### Dapr Workflows (Sidecar-based)
- **Mechanism:** Event-sourced, log-append checkpointing to pluggable state store (Redis, PostgreSQL, CosmosDB)
- **Agent support:** **Dapr Agents 1.0** (announced KubeCon 2025) — built specifically for production AI agents on K8s. Uses continuous log-based checkpointing: "a continuous stream of log events is being written like fast, light checkpoints." On recovery: "It loads up all its previous context... it perfectly carries on where it executed last."
- **Strengths:** Pluggable state store; runs as sidecar (no separate server); Python SDK; built-in suspend/resume/rerun APIs
- **Weakness for agents:** Requires deterministic workflow code (same as Temporal). Replay-based recovery means workflow functions re-execute from start, fast-forwarding past checkpointed activities.
- **Resume:** `dapr workflow resume <instance-id>` or programmatic `resume_workflow()`. Also supports `dapr workflow rerun` from specific event ID.

### LangGraph (Graph-based)
- **Mechanism:** Checkpointer writes state after each graph node, keyed by `thread_id`
- **Strengths:** Agent-native; handles non-deterministic LLM outputs by checkpointing actual results. Time-travel debugging. Human-in-the-loop as native interrupt.
- **Weakness:** Tight coupling to LangGraph's graph model. Checkpoint backends limited to MemorySaver, SqliteSaver, PostgresSaver.
- **Resume:** Re-invoke with same `thread_id` and `None` input — resumes from last checkpoint.

### DBOS (Postgres-only)
- **Mechanism:** Each step result stored as Postgres row. On crash, steps 1..N-1 loaded from store.
- **Strengths:** Zero infrastructure beyond Postgres. `fork_workflow` API for restart from specific step. Production-proven.
- **Weakness:** Sequential steps only (no native DAG). Steps never retried individually — full task retry with checkpoint fast-forward.

---

## Recommended Serialization Strategy for Our Architecture

### Strategy: Explicit State Registration + Per-Step PostgreSQL Checkpoint

```
┌──────────────────────────────────────────────────┐
│                Chain Step Execution               │
│                                                    │
│  1. Load context (registered vars from prior steps)│
│  2. Execute step (LLM call, tool invocation, etc.) │
│  3. Register outputs (explicit `register:` vars)   │
│  4. Checkpoint to PostgreSQL:                      │
│     - chain_run_id                                 │
│     - step_name                                    │
│     - attempt_number                               │
│     - status (completed/failed)                    │
│     - registered_outputs (JSONB)                   │
│     - token_usage                                  │
│     - duration_ms                                  │
│     - error_context (if failed)                    │
│  5. TMS: propagate truth values to dependents      │
└──────────────────────────────────────────────────┘
```

### Why This Works for LLM Agents Specifically

1. **Avoids non-determinism problem.** We checkpoint actual outputs, not replay inputs. Resume loads the real results.
2. **Avoids context window serialization.** The `register:` pattern captures semantically meaningful state, not raw conversation history. Each step reconstructs its context from registered variables, not from a serialized context window.
3. **Avoids Python serialization.** Each step runs in a fresh container. State is explicit JSONB, not pickled Python objects.
4. **Handles side effects.** The checkpoint includes `attempt_number` which serves as idempotency key. Steps can check `is_retry` and skip side effects.
5. **Supports all four resume strategies** from the original analysis (auto-retry, manual, fork, selective re-run) because the checkpoint store is a queryable relational table, not an opaque event log.

### Checkpoint Schema (PostgreSQL)

```sql
CREATE TABLE chain_checkpoints (
    id              BIGSERIAL PRIMARY KEY,
    chain_run_id    UUID NOT NULL,
    step_name       VARCHAR(255) NOT NULL,
    attempt         INT NOT NULL DEFAULT 1,
    status          VARCHAR(20) NOT NULL,  -- pending, running, completed, failed, skipped
    registered_vars JSONB,                 -- explicit outputs from register: directives
    token_usage     JSONB,                 -- {prompt_tokens, completion_tokens, total, cost_usd}
    model_used      VARCHAR(100),          -- actual model (may differ from requested if fallback)
    duration_ms     INT,
    error_context   JSONB,                 -- {error_type, message, traceback, retryable}
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE(chain_run_id, step_name, attempt)
);

CREATE INDEX idx_checkpoints_run ON chain_checkpoints(chain_run_id);
CREATE INDEX idx_checkpoints_status ON chain_checkpoints(status);
```

### Resume Flow

```
resume(chain_run_id):
  1. SELECT * FROM chain_checkpoints WHERE chain_run_id = ? ORDER BY step_name, attempt
  2. Build completed_steps map: {step_name: latest_completed_output}
  3. Find first_failed_step (status = 'failed', max attempt)
  4. Reconstruct context: merge all registered_vars from completed_steps
  5. Resume DAG execution from first_failed_step with reconstructed context
  6. For each step:
     - If step in completed_steps → skip (load from checkpoint)
     - If step = failed_step → re-execute with attempt+1
     - If step is downstream → execute normally
```

---

## Gap Analysis: What's Not Solved

| Gap | Severity | Mitigation |
|-----|----------|------------|
| Multi-agent distributed checkpoint | Low (v2) | Single-threaded chains for v1; Chandy-Lamport for v2 |
| Context window drift on resume | Medium | Re-derive context from registered vars, not from serialized history |
| Tool credential expiry between checkpoint/resume | Medium | Credential refresh as pre-step hook; fail fast if expired |
| Partial failure in parallel blocks | Medium | Checkpoint each parallel branch independently; resume only failed branches |
| Checkpoint store as bottleneck at scale | Low | PostgreSQL handles 10K+ writes/sec; shard by chain_run_id if needed |

---

## Sources

- [AI Agent Workflow Checkpointing and Resumability (Zylos Research, March 2026)](https://zylos.ai/research/2026-03-04-ai-agent-workflow-checkpointing-resumability)
- [Dapr Agents Production AI Workflow Recovery (TFiR/KubeCon 2025)](https://tfir.io/dapr-agents-production-ai-workflow-recovery/)
- [Dapr Workflow Overview](https://docs.dapr.io/developing-applications/building-blocks/workflow/workflow-overview/)
- [Dapr Workflow Features & Concepts](https://docs.dapr.io/developing-applications/building-blocks/workflow/workflow-features-concepts/)
- [Checkpoint/Restore Systems: Evolution and Applications in AI Agents (Eunomia, May 2025)](https://eunomia.dev/blog/2025/05/11/checkpointrestore-systems-evolution-techniques-and-applications-in-ai-agents/)
- [Debugging Non-Deterministic LLM Agents with LangGraph Time Travel](https://dev.to/sreeni5018/debugging-non-deterministic-llm-agents-implementing-checkpoint-based-state-replay-with-langgraph-5171)
- [Memory in LLM-based Multi-agent Systems (TechRxiv)](https://www.techrxiv.org/users/1007269/articles/1367390)
- [Understanding LLM Checkpoint/Restore I/O Strategies (SCA/HPCAsia 2026)](https://arxiv.org/html/2512.24511v1)
- [DBOS: Why Workflows Should Be Postgres Rows](https://www.dbos.dev/blog/why-workflows-should-be-postgres-rows)
- [Temporal: Reliable Data Processing](https://temporal.io/blog/reliable-data-processing-queues-workflows)
- [LangGraph Durable Execution](https://docs.langchain.com/oss/python/langgraph/durable-execution)
