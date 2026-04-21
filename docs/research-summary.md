# Research Summary: `references/cosim-workflow-poc/`

This directory contains a comprehensive research and prototyping effort for building a **YAML-driven AI agent job chaining engine** — essentially an "Ansible for LLM agents" that can orchestrate hundreds/thousands of agent tasks reliably on Kubernetes.

## The Problem

LLM agents can't reliably self-orchestrate at scale. Context windows degrade after ~500-1000 tickets, 30% of autonomous runs hit exceptions, and there's no existing tool that combines declarative YAML workflows + rule-engine gating + durable agent execution.

## Research (6 major documents)

- **RQ1** — Agent self-orchestration breaks down at scale; external harnesses are needed for >1000 tickets
- **RQ3** — Market landscape analysis finds a 12-18 month competitive window; no incumbent covers all pillars (YAML + rules + K8s + agent-aware)
- **RQ4** — Reference architecture: 6-layer design (YAML parser -> schema validator -> DAG planner -> pluggable scheduler -> rule engine -> step executors)
- **RQ5/RQ6** — Synthesis validates the approach against production precedents (Drools+Ansible integration, ZenML, DBOS, Temporal)
- **RQ-B1/C1** — Deep dives on K8s orchestration (chose bare K8s client over Argo), security threat model (5-layer defense-in-depth), and checkpoint/resume (step-level is solved; mid-step agent state restoration remains aspirational)

## Prototypes (4 working implementations)

1. **yaml-agent-chain-poc** — Core engine: YAML parsing, DAG execution, rule-engine gating, Truth Maintenance System (TMS) with policies, checkpoint/resume, and K8s controller-as-Job pattern. 32+ tests.
2. **claude-skill-chain-poc** — End-to-end Flask -> Argo/Temporal -> Claude CLI jobs, with cost tracking, human gates, and quality-score gating. Includes Argo vs Temporal comparison (Argo: 25-50s overhead; Temporal: <250ms).
3. **claude-k8s-workflow-poc** — Claude running headless in K8s Jobs with structured output contracts.
4. **chain-dag-visualizer** — Flask + React Flow real-time DAG visualization with WebSocket status streaming and inline human-gate approvals.

## Key Architecture Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| Orchestration | Bare K8s client (not Argo) | Full control over LLM-specific logic, tighter RBAC, checkpoint store handles resume |
| Rule engine | Grule (Go) | Supports forward chaining; GoRules does not |
| State persistence | PostgreSQL + `register:` pattern | Step-boundary checkpoints; avoids hard serialization problems |
| Security | Namespace-scoped RBAC, zero Secrets access at controller, NetworkPolicy default-deny | Blast radius: MEDIUM-HIGH (containable) vs Argo cluster controller: CRITICAL |

## Bottom Line

The research concludes the initiative is ready for v1 implementation — architecturally sound, market-validated, with a genuine competitive white space. The novel contribution is collapsing workflow orchestration, LLM observability, and agent state management into a single plane with TMS-aware gating.
