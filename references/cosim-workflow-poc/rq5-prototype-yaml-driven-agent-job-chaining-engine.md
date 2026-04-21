# RQ5 Prototype: YAML-Driven Agent Job Chaining Engine

**Repo:** `yaml-agent-chain-poc`
**Ticket:** TK-56EA99
**Status:** PoC delivered + v2 with TMS and layer separation

## What It Demonstrates

A from-scratch system that ingests Ansible-playbook-like YAML to define and execute agent job chains. The PoC validates that Ansible's execution model maps cleanly onto agent chaining workflows, that Drools-inspired rule conditions add value for dynamic routing, and that Truth Maintenance enables automatic downstream invalidation.

## Architecture: Two-Layer Separation

**Key principle** (from `kiegroup/drools-ansible-rulebook-integration`, 538 commits, Apache 2.0, production in AAP 2.4+):

> "Keep your orchestrator dumb, your decision-maker stateless."

| Layer | Module | Responsibility |
|-------|--------|---------------|
| **Workflow Engine** | `executor.py` | Step sequencing, parallel dispatch, state flow, error recovery, human gates |
| **Rule Engine** | `rule_engine.py` | Condition evaluation, fact matching, working memory, truth maintenance |

This separation mirrors how Event-Driven Ansible works: Drools evaluates rules, Ansible executes tasks. The anti-pattern is stuffing complex decision logic into workflow routing nodes.

## Truth Maintenance System (TMS)

**The highest-value Drools concept for agent chaining** (per Maya's RQ2 analysis).

When an upstream step output is retracted or updated, all downstream dependents are automatically marked stale and flagged for re-execution.

**Agent chaining use case:** Human reviewer rejects step 3's analysis → TMS flags steps 4-N for re-run because they consumed step 3's output.

**Concept mapping:**

| Drools Concept | Agent Chain Analog |
|---------------|-------------------|
| Working Memory | Shared execution context |
| Facts | Registered step outputs |
| Forward Chaining | Event-driven step triggering |
| Conflict Resolution | Rule priority/salience |
| Truth Maintenance | Automatic invalidation on upstream changes |

## From Ansible (what transfers well)
- **Playbook → Chain:** Top-level YAML structure with `chain:` key
- **Tasks → Steps:** Each step has `name`, `module`, `params`
- **`register:` + Jinja2 `{{ }}`:** Variable passing between steps
- **`when:` clauses:** Conditional execution
- **`block/rescue/always`:** Error handling
- **Modules:** Pluggable execution backends

## From Drools (what adds value)
- **Forward-chaining rules:** `rules:` blocks evaluate conditions against working memory
- **Priority/salience:** Rules sorted by priority; highest-matching fires first
- **Working memory:** Fact tracking with dependency graph
- **Truth Maintenance:** Automatic downstream invalidation on retraction/update

## New for agent chaining (not in Ansible or Drools alone)
- **`parallel:` blocks:** Fan-out/fan-in execution via ThreadPoolExecutor
- **`human_gate` module:** First-class pause-for-approval primitive
- **Idempotency opt-in:** `idempotent: true` flag for non-deterministic agent calls
- **TMS + human gates:** Rejection at gate triggers automatic re-execution tracking

## Files

| File | Purpose |
|------|---------|
| `rule_engine.py` | Rule engine layer: RuleEngine, WorkingMemory, TMS |
| `executor.py` | Workflow engine layer: ChainExecutor, ModuleRegistry |
| `chain_schema.py` | YAML parser, schema validator, Jinja2-style variable resolver |
| `runner.py` | CLI entry point with dry-run, verbose, JSON output |
| `examples/basic_chain.yaml` | Simple 3-step sequential chain |
| `examples/parallel_chain.yaml` | Fan-out/fan-in with 3 parallel analyses |
| `examples/conditional_chain.yaml` | Gates, when-clauses, Drools rules |
| `examples/full_demo.yaml` | All features combined |
| `examples/tms_demo.yaml` | Truth Maintenance System demonstration |
| `tests/test_chain.py` | 14 tests covering parser, resolver, executor |
| `tests/test_tms.py` | 10 tests covering TMS: propagation, diamond deps, staleness |

## Prior Art

- **kiegroup/drools-ansible-rulebook-integration** — Production Drools+Ansible integration (AAP 2.4+ EDA)
- **Orloj** — Agent infrastructure-as-code with K8s-style resource kinds
- **Kagent** (CNCF Sandbox) — Agents as YAML manifests, GitOps-compatible
- **QuantumBlack/McKinsey** — "Keep orchestration deterministic. Agents don't decide what phase they're in."

## Limitations

1. **Modules are stubbed** — no real LLM calls
2. **No persistence** — state is in-memory only; production needs durable state
3. **No distributed execution** — single-process; production needs workflow engine
4. **Rule engine is simplified** — no full Rete algorithm
5. **Human gates are auto-approved** — real implementation needs webhook infra
6. **TMS is in-memory** — production needs durable fact store with versioning

## Key Questions for Raj's Architecture (RQ4)

1. Should the execution engine wrap a workflow engine (Temporal/Argo) or be standalone?
2. Should rules be evaluated at parse time or runtime? (Current: runtime)
3. How should idempotency work for non-deterministic LLM calls?
4. Should parallel steps share context or have isolated contexts that merge?
5. Should TMS propagation trigger automatic re-execution or just flag steps? (Current: flag only)
