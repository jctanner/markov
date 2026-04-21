# RQ-C1: LLM Agent Checkpoint/Resume Analysis

**Author:** Raj (Technical Researcher)  
**Date:** 2026-04-21  
**Ticket:** TK-075E12  
**Status:** COMPLETE  

---

## TL;DR

**LLM agent state serialization is fundamentally harder than traditional workflow state** because of three unique properties: (1) context windows are ephemeral projections not persistent stores, (2) reasoning traces are non-deterministic and non-reproducible, and (3) tool call history accumulates serialization overhead that compounds with chain length.

**Recommended approach:** Temporal's event sourcing model for workflow-level durability + application-level context management with tiered state (ephemeral context projection + persistent substrate + checkpoint layer).

**Confidence:** HIGH for architectural recommendations, MEDIUM for specific implementation patterns (limited production case studies for Claude-specific agent chains).

---

## 1. Checkpoint/Resume Mechanisms by Platform

### 1.1 Temporal — Event Sourcing (Replay-Based)

**Mechanism:** Every Activity (unit of work) result is recorded in an append-only Event History. On worker crash, a replacement worker replays the full event history, skipping completed Activities by reading cached results instead of re-executing them.

**How it works for LLM agents:**
```
Workflow start → Activity: call Claude (skill 1) → RECORD result
                → Activity: evaluate gate → RECORD result  
                → Activity: call Claude (skill 2) → CRASH
                
Recovery: replay events 1-2 (cached), resume at event 3
```

**Key properties:**
- **Determinism requirement:** Workflow code must be deterministic — no timestamps, random numbers, or direct API calls in workflow logic. LLM calls (inherently non-deterministic) are wrapped as Activities, which is the correct abstraction.
- **State management:** Workflow variables hold state in-memory, automatically replayed from event history. Full control over what data to store and how it's assembled for LLM input.
- **Continue-As-New:** For very long chains, atomically completes the current workflow run and starts fresh with carried-forward essential state. Prevents unbounded event history growth.
- **Crash recovery:** "Your app crashes when it's just about done with a long-running task? Restart it, and Temporal will see to it that it picks up where it left off." (temporal.io)
- **Maturity:** Production-proven at scale. OpenAI Agents SDK + Temporal Python SDK integration reached GA on March 23, 2026. Temporal Nexus GA, multi-region replication GA with 99.99% SLA.

**Trade-offs:**
- Infrastructure footprint: Requires Temporal server cluster (Cassandra/MySQL + Elasticsearch). Heavier than Argo.
- Learning curve: Event sourcing mental model requires understanding replay semantics, determinism constraints, and Activity boundaries.
- Not K8s-native: Runs on any infrastructure, which is both a strength (portability) and weakness (another system to operate alongside K8s).

### 1.2 Argo Workflows — Retry/Restart from Node

**Mechanism:** Container-level retry and restart. No built-in agent-level checkpointing. State between workflow steps must be externalized via artifacts, parameters, or shared volumes.

**How it works for LLM agents:**
```
Workflow step A (Pod A) → writes output to artifact/parameter
Workflow step B (Pod B) → reads A's output, runs Claude skill
                        → CRASH (Pod B fails)
                        
Recovery: Argo retries step B (creates new Pod B')
          Reads A's output again from stored artifact
          Re-runs Claude skill from scratch
```

**Key properties:**
- **Retry strategy:** `retryStrategy` with configurable `limit`, `retryPolicy` (Always, OnFailure, OnError, OnTransientError), `backoff` (duration, factor, maxDuration).
- **Restart from node:** `argo retry --restart-successful` can rerun specific failed nodes. `argo resubmit` creates a new workflow from an existing one.
- **State passing:** Parameters (small text values, <256KB) or Artifacts (files via S3/GCS/MinIO). Each step is an independent container — no shared memory.
- **No replay:** Failed steps re-execute entirely. For expensive LLM calls, this means re-spending tokens on retry.

**Trade-offs:**
- No durable execution model — orchestration state lives in the Workflow CR status, not in an event history.
- Context serialization between steps is entirely the developer's responsibility.
- Retry = full re-execution of the failed step. No "resume from where the LLM was in its reasoning."
- For short, independent skill steps (our primary use case), this is acceptable — each step is a self-contained Claude invocation.

### 1.3 Dapr Workflows — Checkpoint-Based (Event Sourcing with Actor Model)

**Mechanism:** Append-only history log with replay, similar to Temporal. Built on Dapr Actors. Checkpoints persisted via configurable state stores (Redis, PostgreSQL, CosmosDB).

**How it works for LLM agents:**
```
Workflow function executes → schedules Activity A → awaits result
  → Checkpoint written to state store
  → Workflow unloads from memory
  → Activity A completes → triggers replay
  → Workflow replays from beginning, reads cached Activity A result
  → Schedules Activity B → awaits result
  → Checkpoint written...
```

**Key properties:**
- **Sidecar architecture:** Dapr runs alongside application pods — no separate server cluster. Lower operational overhead than Temporal.
- **CNCF graduated:** Community governance, not single-vendor controlled.
- **Dapr Agents integration:** Foundation for Dapr Agents agentic framework. Broader AI framework support than Temporal (which is limited to OpenAI and PydanticAI SDKs).
- **Checkpoint size:** Determined by concurrent actions scheduled before idle. Sequential workflows = smaller checkpoints.
- **Production scale:** Derivco and Tempestive execute hundreds of millions of transactions/day.

**Trade-offs:**
- Younger workflow engine compared to Temporal. Less mature tooling (no equivalent of Temporal's web UI).
- Workflow latency: "may not be appropriate for latency-sensitive workloads" due to frequent state store writes.
- Less granular replay than Temporal — checkpoint boundaries are at Activity completion, not within Activities.
- K8s integration: Good (sidecar model), but workflow definitions are code-based, not CRD-based like Argo.

---

## 2. Why LLM Agent State Serialization is Harder Than Traditional Workflows

This is the core technical insight. Traditional workflow state (database transaction IDs, file paths, counters, flags) serializes trivially to JSON/protobuf. LLM agent state is qualitatively different:

### 2.1 Context Window = Ephemeral Projection, Not Storage

**Key 2026 insight:** "The context window is not storage; it is a projection — a temporary, purpose-built view assembled from [persistent] substrate on demand for each inference step." (Zylos Research, March 2026)

The context window contains:
- **System prompt** (static, reproducible)
- **Conversation history** (serializable, but grows unboundedly)
- **Tool call/response pairs** (serializable, but large)
- **Ephemeral reasoning state** (NOT serializable — this is the problem)

When a Claude agent is mid-reasoning within a single invocation — considering multiple options, weighing evidence, building toward a conclusion — that intermediate cognitive state exists only inside the model's forward pass. It cannot be captured, serialized, or restored. If the process dies during an LLM call, the entire invocation must restart from scratch.

**Implication for checkpointing:** The granularity floor for LLM agent checkpointing is **per-LLM-invocation**, not per-token or per-reasoning-step. You can checkpoint between LLM calls, but never within one.

### 2.2 Tool Call History Serialization Overhead

Traditional workflow: Step A produces a database record ID (4 bytes). Step B reads it.

LLM agent: Step A produces a full Claude response (potentially 4,000-100,000 tokens of text). To give Step B context about what Step A did, you must either:

1. **Pass the full output** — serialization explosion. A 10-step chain with 4K token outputs accumulates 40K tokens of history just for inter-step context.
2. **Summarize** — lossy compression. Agent loses detail that may be relevant later.
3. **Store externally and reference** — requires a retrieval layer and careful context assembly.

**CaveAgent research (2026)** demonstrates the magnitude: CaveAgent achieved 100% accuracy with 123K tokens by storing query results in runtime variables (bypassing prompt accumulation), while Function Calling style required 295K tokens for the same task — a 2.4x serialization overhead.

### 2.3 Non-Determinism and Replay Impossibility

Traditional workflow: Replay event history → same state (deterministic).

LLM agent: Replay same prompt → different output (non-deterministic). Even with temperature=0, model updates, load balancing across GPU instances, and floating-point non-determinism mean replayed LLM calls produce different results.

**Consequence:** Temporal's replay model works for the workflow *structure* (which Activities completed, what their outputs were) but NOT for reasoning *content*. If you replay a workflow, the cached Activity results contain the original LLM output — which is correct behavior. But if you need to "resume reasoning" (e.g., an LLM was mid-chain-of-thought when the process died), you cannot replay that reasoning. You must re-invoke the LLM with reconstructed context.

### 2.4 Context Rot and "Lost in the Middle"

As checkpointed conversation history grows, LLM performance degrades:
- **Effective context gap:** Effective context often falls far below advertised maximum — up to 99% degradation on complex tasks.
- **Context rot:** Accuracy drops 30%+ when relevant information sits in middle positions of long contexts.
- **Working memory bottleneck:** Frontier models manage only "a handful of variables" before reasoning breaks down.

**Implication:** Naive checkpoint-and-restore (dump full conversation history, restore it into context window) degrades agent quality. Production systems need **context engineering** — selective restoration of only relevant history.

---

## 3. Serialization Strategies for Agent Job Chains

### 3.1 Strategy: Tiered State Architecture (RECOMMENDED)

```
┌─────────────────────────────────────┐
│  Layer 1: Ephemeral Context Window  │  ← Per-invocation, assembled on demand
│  (system prompt + relevant history  │     NOT persisted between steps
│   + current task context)           │
├─────────────────────────────────────┤
│  Layer 2: Persistent Substrate      │  ← Cross-step, durable
│  (completed step outputs, artifacts,│     Stored in DB/object storage
│   decisions, tool call summaries)   │
├─────────────────────────────────────┤
│  Layer 3: Checkpoint Layer          │  ← Workflow-level durability
│  (Temporal event history / Argo     │     Enables retry/resume
│   workflow status / Dapr state)     │
└─────────────────────────────────────┘
```

**How this maps to ChainExecutor:**

- **Layer 1:** Each Claude skill invocation gets a purpose-built context window: system prompt + skill-specific instructions + relevant outputs from prior steps (not ALL outputs, only relevant ones).
- **Layer 2:** After each skill completes, its output is stored as an Argo artifact or Temporal Activity result. A summary/extraction step can distill key outputs for downstream context assembly.
- **Layer 3:** The workflow engine (Argo or Temporal) tracks which steps completed successfully. On failure, resume from the last successful step.

### 3.2 Strategy: Checkpoint Granularity Selection

| Granularity | Implementation | When to Use |
|---|---|---|
| **Per-skill (coarse)** | Checkpoint after each Claude skill completes. On failure, retry the entire failed skill. | Default for ChainExecutor. Each skill is 5-30 min. Acceptable re-execution cost. |
| **Per-LLM-call (fine)** | Within a single skill, checkpoint after each Claude API call. Requires skill_runner to manage multi-turn state. | Only if skills involve multi-turn conversations (e.g., iterative code review with back-and-forth). |
| **Per-gate (medium)** | Checkpoint after gating evaluation. Gate results are small and serializable. | Always — gates are cheap to checkpoint and critical for resume correctness. |

**For our use case:** Per-skill granularity is sufficient. Each Claude skill job is a self-contained invocation. If it fails, re-running it from scratch (with the same input context) costs 1-10 minutes and $0.10-$2.00 in API tokens. The workflow engine ensures we don't re-run already-completed skills.

### 3.3 Strategy: Context Assembly for Resumed Chains

When a chain resumes after failure, downstream skills need context about what upstream skills produced. Two patterns:

**Pattern A: Artifact Pass-Through**
```
Skill 1 → writes output.json to shared volume/S3
Gate → reads output.json, evaluates, writes gate_result.json
Skill 2 → reads output.json + gate_result.json as input context
```
- Simple, deterministic
- Works with Argo artifacts natively
- Risk: large outputs bloat context windows

**Pattern B: Summary Extraction**
```
Skill 1 → writes full output to artifact store
Summary step → Claude extracts key findings into summary.json (structured)
Skill 2 → reads summary.json (compact) as input context
```
- Prevents context window bloat
- Adds one LLM call per step (~$0.02-$0.10)
- Better for long chains (>5 steps) where accumulated context would exceed effective window

**Recommendation:** Start with Pattern A for chains ≤5 steps. Add Pattern B for longer chains where context quality matters.

### 3.4 Strategy: Idempotency for Safe Replay

LLM agent tool calls that produce side effects (creating PRs, sending messages, writing files) must be idempotent for safe replay:

```python
# BAD: replay creates duplicate PR
create_pr(title="Fix bug", branch="fix/123")

# GOOD: idempotency key prevents duplicates
create_pr(
    title="Fix bug", 
    branch="fix/123",
    idempotency_key=f"{workflow_id}:{step_name}"
)
```

Temporal handles this automatically for Activities (cached results prevent re-execution). Argo does NOT — if a step is retried, all side effects re-execute. **For Argo-based chains, skill containers must implement their own idempotency checks.**

---

## 4. Platform Recommendation Matrix

| Requirement | Temporal | Argo | Dapr |
|---|---|---|---|
| **Workflow-level durability** | ⭐⭐⭐ Event sourcing | ⭐⭐ Workflow CR status | ⭐⭐⭐ Event sourcing |
| **Crash recovery** | ⭐⭐⭐ Automatic replay | ⭐⭐ Retry failed nodes | ⭐⭐⭐ Automatic replay |
| **LLM context management** | ⭐⭐ DIY (workflow variables) | ⭐ DIY (artifacts/params) | ⭐⭐ DIY (workflow variables) |
| **K8s-native** | ⭐ No (separate infra) | ⭐⭐⭐ Yes (CRDs) | ⭐⭐ Sidecar model |
| **Infrastructure overhead** | ⭐ Heavy (Cassandra/MySQL+ES) | ⭐⭐⭐ Moderate (controller pod) | ⭐⭐ Moderate (sidecar + state store) |
| **Long-running chains (>1hr)** | ⭐⭐⭐ Native support | ⭐⭐ Possible with TTL tuning | ⭐⭐⭐ Native support |
| **Human-in-the-loop gates** | ⭐⭐⭐ Signals + wait_condition | ⭐⭐ Suspend + resume API | ⭐⭐ Manual (activity-based) |
| **Debugging/observability** | ⭐⭐⭐ Web UI + event history | ⭐⭐ Argo UI + logs | ⭐⭐ Zipkin/Dashboard |
| **AI agent ecosystem** | ⭐⭐ OpenAI SDK, PydanticAI | ⭐ Generic containers | ⭐⭐⭐ Dapr Agents |

### For ChainExecutor specifically:

**If using Argo Workflows (per RQ-B1 ADR recommendation):**
- Per-skill checkpointing via Argo's built-in DAG step tracking
- Artifact-based context passing between steps
- Idempotency implemented in skill_runner.py
- Acceptable — Argo's retry model is sufficient for chains of 3-10 self-contained Claude skills
- **Gap:** No within-skill checkpointing. If a 30-minute skill fails at minute 29, the entire skill re-runs.

**If long-running chains become the norm (>10 skills, >2 hours):**
- Evaluate Temporal as the orchestration layer
- Event sourcing provides finer-grained recovery
- Continue-As-New prevents event history explosion
- Higher infrastructure cost justified by saved API token costs from avoided re-execution

---

## 5. Critical Gaps and Open Questions

### Gap 1: No Standard for LLM Agent State Serialization

There is no equivalent of Protocol Buffers or Apache Avro for LLM agent state. Letta's `.af` (Agent File) format is the closest attempt — "an open file format for serializing stateful agents with persistent memory and behavior" — but adoption is minimal. Each framework implements its own serialization.

**Impact:** Moving agent chains between orchestration platforms requires re-implementing state serialization.

### Gap 2: Context Quality Degradation on Resume

No framework currently addresses the fact that resumed agents may produce lower-quality outputs than fresh agents, because the reconstructed context window is a lossy representation of the original reasoning flow. This is an inherent limitation of the "context window as projection" architecture.

**Mitigation:** For ChainExecutor, each skill step starts fresh with a purpose-built context window, so this primarily affects multi-turn skills, not inter-skill chains.

### Gap 3: Cost of Re-execution vs. Cost of Checkpointing

No published benchmarks compare the amortized cost of:
- Frequent checkpointing (storage + serialization overhead) vs.
- Occasional re-execution (API token cost on retry)

For Claude-based chains: if a skill costs $1.00 in API tokens and fails 5% of the time, the expected re-execution cost is $0.05/skill. Checkpointing overhead must be less than this to be economically justified at per-skill granularity.

### Gap 4: Agent-Specific Retry Semantics

Should a retried LLM agent skill:
a) Receive the exact same prompt (deterministic retry) → may hit the same failure
b) Receive modified prompt with failure context → better chance of success, but different execution path
c) Receive summarized context from the failed attempt → hybrid approach

No framework provides guidance here. This is a decision for the ChainExecutor implementation.

---

## 6. Recommendations

1. **Start with per-skill checkpointing via Argo** — matches our current architecture (RQ-B1 ADR). Each skill is a self-contained Claude invocation. Argo tracks which skills completed.

2. **Implement tiered state** — persistent substrate (artifact store) + ephemeral context assembly (per-skill context builder in skill_runner.py).

3. **Build idempotency into skill_runner.py** — every tool call with side effects must include workflow_id + step_name as idempotency key.

4. **Monitor re-execution costs** — if skills routinely fail late in execution (>50% complete), evaluate upgrading to Temporal for finer-grained checkpointing.

5. **Flag for Sam:** Prototype the artifact pass-through pattern (Pattern A) first. Measure context window utilization at each step. If >60% of context is prior step outputs, implement Pattern B (summary extraction).

---

## Sources

- [AI Agent Workflow Checkpointing and Resumability — Zylos Research (March 2026)](https://zylos.ai/research/2026-03-04-ai-agent-workflow-checkpointing-resumability)
- [Dynamic Context Assembly and Projection Patterns — Zylos Research](https://zylos.ai/research/2026-03-17-dynamic-context-assembly-projection-llm-agent-runtimes)
- [Temporal for AI — Official](https://temporal.io/solutions/ai)
- [Build Resilient Agentic AI with Temporal](https://temporal.io/blog/build-resilient-agentic-ai-with-temporal)
- [Temporal + OpenAI Agents SDK GA](https://temporal.io/blog/announcing-openai-agents-sdk-integration)
- [Durable Multi-Agentic AI with Temporal](https://temporal.io/blog/using-multi-agent-architectures-with-temporal)
- [Of Course You Can Build Dynamic AI Agents with Temporal](https://temporal.io/blog/of-course-you-can-build-dynamic-ai-agents-with-temporal)
- [Dapr vs Temporal — Workflow Orchestration Comparison (March 2026)](https://oneuptime.com/blog/post/2026-03-31-dapr-vs-temporal-workflow-comparison/view)
- [Dapr vs Temporal — Diagrid](https://www.diagrid.io/compare-temporal)
- [Dapr Workflow Architecture](https://docs.dapr.io/developing-applications/building-blocks/workflow/workflow-architecture/)
- [Checkpoint/Restore Systems for AI Agents — eunomia (May 2025)](https://eunomia.dev/blog/2025/05/11/checkpointrestore-systems-evolution-techniques-and-applications-in-ai-agents/)
- [Stateful Agents: The Missing Link — Letta](https://www.letta.com/blog/stateful-agents)
- [CaveAgent: Stateful Runtime Operators — arXiv](https://arxiv.org/html/2601.01569v1)
- [LLM Context Window Limitations 2026 — Atlan](https://atlan.com/know/llm-context-window-limitations/)
- [Context Engineering for AI Agents — Weaviate](https://weaviate.io/blog/context-engineering)
- [Debugging Non-Deterministic LLM Agents — LangGraph Time Travel](https://dev.to/sreeni5018/debugging-non-deterministic-llm-agents-implementing-checkpoint-based-state-replay-with-langgraph-5171)
- [The 2026 Guide to Agentic Workflow Architectures — StackAI](https://www.stackai.com/blog/the-2026-guide-to-agentic-workflow-architectures)
- [Agentic Workflows in 2026 — Vellum AI](https://www.vellum.ai/blog/agentic-workflows-emerging-architectures-and-design-patterns)
- [LLM Workflows: Patterns & Production Architecture 2026 — Morph](https://www.morphllm.com/llm-workflows)
- [Survey: From Static Templates to Dynamic Runtime Graphs — arXiv](https://arxiv.org/html/2603.22386v1)
- [Argo Workflows Retry Documentation](https://argo-workflows.readthedocs.io/en/latest/retries/)
