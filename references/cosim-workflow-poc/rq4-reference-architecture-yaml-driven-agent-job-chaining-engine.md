# RQ4: Reference Architecture — YAML-Driven Agent Job Chaining Engine

**Ticket:** TK-CC24ED | **Priority:** CRITICAL | **Author:** Raj (Technical Researcher)
**Date:** 2026-04-20 | **Updated:** 2026-04-20 (rule engine recommendation updated per Maya's RQ2 assessment)
**Depends on:** RQ1 (Ansible Mapping), RQ2 (Drools/Rule Engine Patterns)

---

## Executive Summary

This document presents a complete reference architecture for a from-scratch system that ingests Ansible-playbook-like YAML to define and execute agent job chaining workflows with gating criteria. The design synthesizes three traditions:

1. **Ansible** — declarative YAML schema, role composition, variable passing, error handling, Jinja2 templating
2. **Drools** — forward-chaining rule evaluation, working memory, salience-based conflict resolution, fact-driven reactivity
3. **Modern durable execution** — Temporal-style workflow/activity separation, checkpoint/resume, bounded recovery

**Core design principle:** The Auton framework's "Cognitive Blueprint" separation — declarative specification (YAML) is strictly decoupled from runtime execution (engine). The same chain definition deploys across local dev, CI, and production K8s without modification.

---

## 1. System Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    Flask Dashboard                       │
│  (Chain catalog, trigger UI, status monitor, approvals)  │
└──────────────┬──────────────────────────┬───────────────┘
               │ REST API                 │ WebSocket/SSE
               ▼                          ▲
┌──────────────────────────────────────────────────────────────┐
│                   Chain Controller                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│  │  Parser  │→ │Validator │→ │ Planner  │→ │Scheduler │ │
│  │  (YAML)  │  │ (Schema) │  │  (DAG)   │  │(Dispatch)│ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────────────┐│
│  │  Rule    │  │ Context  │  │    State Manager         ││
│  │  Engine  │  │  Store   │  │ (checkpoint/resume/replay)││
│  └──────────┘  └──────────┘  └──────────────────────────┘│
└──────────────┬──────────────────────────┬───────────────┘
               │ gRPC / REST              │ Events
               ▼                          ▲
┌──────────────────────────────────────────────────────────────┐
│                   Step Executors                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│  │  Claude  │  │  Script  │  │  API     │  │  Human   │ │
│  │  Skill   │  │  Runner  │  │  Call    │  │  Gate    │ │
│  │ Executor │  │ Executor │  │ Executor │  │ Executor │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │
└─────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Role | Inspired By |
|---|---|---|
| **Parser** | Parse YAML chain definitions, resolve includes/roles, expand Jinja2 templates | Ansible PlaybookExecutor |
| **Validator** | Validate chain against schema, check step contracts, verify variable references | Ansible `ansible-lint` + JSON Schema |
| **Planner** | Build execution DAG from sequential/parallel/conditional declarations | Custom (Ansible lacks this) |
| **Scheduler** | Dispatch ready steps to executors, manage concurrency limits | Ansible TQM + Graph Harness |
| **Rule Engine** | Evaluate gating criteria, route decisions, conflict resolution | Drools forward chaining |
| **Context Store** | Working memory for chain execution — variables, outputs, facts, rule state | Drools Working Memory + Ansible facts |
| **State Manager** | Checkpoint/resume, failure recovery, execution history | Temporal workflow state |
| **Step Executors** | Module plugins that perform actual work (LLM calls, scripts, APIs, human gates) | Ansible module/action plugins |

---

## 2. YAML Schema Design

### 2.1 Chain Definition (Top-Level)

```yaml
# chain.yaml — Ansible-playbook-like top-level structure
apiVersion: agentchain/v1
kind: Chain

metadata:
  name: security-ticket-processor
  version: "1.2.0"
  description: "Analyze and remediate security tickets from Jira"
  tags: [security, jira, automated-fix]

# Global configuration (like Ansible play-level vars)
config:
  model: claude-sonnet-4        # Default LLM model
  max_cost_usd: 5.00             # Chain-level cost budget
  timeout: 30m                    # Chain-level timeout
  concurrency: 3                  # Max parallel steps
  retry_policy:
    max_retries: 2
    escalation: [retry, patch, human]  # 3-level recovery protocol

# Variable definitions (like Ansible vars)
vars:
  jira_project: "RHOAI"
  severity_threshold: "high"

# Context gathering (like Ansible gather_facts)
gather_context:
  - jira_ticket: "{{ trigger.ticket_key }}"
  - repo_state: "{{ trigger.repo_url }}"

# Chain steps
steps:
  - name: Analyze ticket
    skill: claude_analyze
    args:
      prompt: "Analyze this security ticket for severity and affected components"
      input: "{{ jira_ticket.description }}"
      output_schema:
        type: object
        required: [severity, components, analysis]
        properties:
          severity: { type: string, enum: [low, medium, high, critical] }
          components: { type: array, items: { type: string } }
          analysis: { type: string }
    register: analysis
    cost_budget: 0.50

  - name: Gate — severity check
    gate:
      rules:
        - condition: "analysis.severity in ['high', 'critical']"
          action: continue
          salience: 10
        - condition: "analysis.severity == 'medium'"
          action: continue_with_flag
          set_fact:
            needs_review: true
          salience: 5
        - condition: "analysis.severity == 'low'"
          action: skip_to
          target: log_and_close
          salience: 1

  - name: Generate fixes
    parallel:
      concurrency: 2
      steps:
        - name: Generate code fix
          skill: claude_code
          args:
            prompt: "Generate a fix for: {{ analysis.analysis }}"
            repo: "{{ repo_state.clone_path }}"
            affected: "{{ analysis.components }}"
          register: code_fix
          side_effects: [file_write, git_commit]
          idempotency_key: "fix-{{ trigger.ticket_key }}-{{ analysis.components | hash }}"

        - name: Generate test
          skill: claude_test
          args:
            prompt: "Write tests validating the fix for: {{ analysis.analysis }}"
            fix_context: "{{ code_fix.diff }}"  # waits for code_fix via dependency
          register: test_gen
          depends_on: [code_fix]

  - name: Run tests
    skill: script_runner
    args:
      command: "pytest {{ test_gen.test_files | join(' ') }}"
      working_dir: "{{ repo_state.clone_path }}"
    register: test_result
    block:
      rescue:
        - name: Retry with fix
          skill: claude_code
          args:
            prompt: "Tests failed with: {{ _rescue_error }}. Fix the code."
          register: code_fix_v2
          retry_budget: 1

  - name: Human approval gate
    gate:
      type: human_approval
      approvers: ["{{ ticket.assignee }}", "@security-team"]
      timeout: 4h
      context:
        diff: "{{ code_fix.diff }}"
        tests: "{{ test_result.summary }}"
        cost_so_far: "{{ _chain_cost_usd }}"
      on_timeout: escalate
    when: needs_review | default(false)

  - name: Create PR
    skill: github_pr
    args:
      repo: "{{ trigger.repo_url }}"
      branch: "fix/{{ trigger.ticket_key }}"
      title: "Security fix: {{ trigger.ticket_key }}"
      body: |
        ## Automated Security Fix
        {{ analysis.analysis }}
        
        **Confidence:** {{ code_fix.confidence }}
        **Tests:** {{ test_result.passed }}/{{ test_result.total }} passing
    register: pr_result
    side_effects: [git_push, github_api]
    idempotency_key: "pr-{{ trigger.ticket_key }}"

  - name: log_and_close
    skill: jira_update
    args:
      ticket: "{{ trigger.ticket_key }}"
      status: "{{ 'In Review' if pr_result is defined else 'Triaged' }}"
      comment: "{{ _chain_summary }}"

# Handlers (like Ansible handlers — run once at end if notified)
handlers:
  - name: notify_slack
    skill: slack_post
    args:
      channel: "#security-fixes"
      message: "{{ trigger.ticket_key }}: {{ _chain_outcome }}"
    listen: chain_complete

  - name: update_metrics
    skill: metrics_push
    args:
      metric: "agent_chain_completion"
      labels:
        chain: "security-ticket-processor"
        outcome: "{{ _chain_outcome }}"
        cost: "{{ _chain_cost_usd }}"
    listen: chain_complete
```

### 2.2 Skill Definition (Role Equivalent)

```yaml
# skills/claude_analyze/skill.yaml
apiVersion: agentchain/v1
kind: Skill

metadata:
  name: claude_analyze
  version: "2.1.0"
  description: "General-purpose analysis using Claude"
  
defaults:
  model: claude-sonnet-4
  temperature: 0.3
  max_tokens: 4096

input_schema:
  type: object
  required: [prompt]
  properties:
    prompt: { type: string }
    input: { type: string }
    output_schema: { type: object }  # Optional structured output contract

output_schema:
  type: object
  properties:
    raw_response: { type: string }
    structured: { type: object }  # If output_schema was provided
    token_usage: { type: object }
    cost_usd: { type: number }

side_effect_class: read_only  # This skill has no side effects by default

executor: claude_skill_executor
connection:
  type: anthropic_api
  credential_ref: "vault:anthropic_api_key"
```

### 2.3 Rule/Gate Definition (Drools-Inspired)

```yaml
# Inline gate rules (Drools-inspired)
gate:
  rules:
    # Rules evaluated in salience order (highest first)
    # Forward chaining: rule output can trigger subsequent rules
    - name: "critical_auto_approve"
      condition: "analysis.severity == 'critical' AND analysis.confidence > 0.95"
      action: continue
      set_fact:
        auto_approved: true
        approval_reason: "High confidence critical fix"
      salience: 100  # Drools-style priority

    - name: "needs_human_review"  
      condition: "analysis.severity in ['high', 'critical'] AND NOT auto_approved"
      action: pause
      set_fact:
        review_required: true
      salience: 50

    - name: "low_priority_defer"
      condition: "analysis.severity == 'low' AND backlog_count > 50"
      action: skip
      set_fact:
        deferred: true
        defer_reason: "Backlog overflow"
      salience: 10

  # Conflict resolution strategy (Drools-inspired)
  conflict_resolution: salience_then_recency
  # Options: salience_then_recency, salience_then_specificity, first_match
```

---

## 3. Execution Engine Architecture

### 3.1 Parser Pipeline

```
YAML File(s)
    │
    ▼
┌─────────────────┐
│  YAML Parser    │  PyYAML / ruamel.yaml (preserves comments)
│  + Include      │  Resolve !include, !role references
│  Resolution     │  Merge multiple chain files
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Jinja2 Engine  │  Resolve {{ var }} references (lazy — some at parse, some at runtime)
│  (Two-Phase)    │  Phase 1: Static vars (config, defaults)
│                 │  Phase 2: Runtime vars (register outputs, facts)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Schema         │  Validate against agentchain/v1 JSON Schema
│  Validator      │  Check: step names unique, register refs valid, 
│                 │  dependency graph acyclic, cost budgets sum ≤ chain budget
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  DAG Builder    │  Convert sequential + parallel + depends_on into execution DAG
│                 │  Detect implicit dependencies (register → usage)
│                 │  Identify critical path
└────────┬────────┘
         │
         ▼
  Execution Plan (immutable DAG + resolved config)
```

### 3.2 DAG Scheduler

The scheduler is the core execution loop, inspired by Graph Harness (arXiv:2604.11378):

**Algorithm:**
```
1. Initialize: mark all steps as PENDING
2. Compute ready set: steps whose dependencies are all COMPLETE
3. While ready set is non-empty:
   a. For each step in ready set (up to concurrency limit):
      - If step is a GATE: evaluate rules against working memory
      - If step is a SKILL: dispatch to appropriate executor
      - If step is a PARALLEL block: expand sub-steps, add to DAG
   b. Wait for any step to complete (or timeout)
   c. On completion:
      - Validate output against step's output_schema (contract validation)
      - Store output in context store (register)
      - Update working memory (for rule engine)
      - Checkpoint state
      - Recompute ready set
   d. On failure:
      - Execute bounded recovery protocol:
        Level 1: Mechanical retry (if retries remain)
        Level 2: Rescue block (if defined)
        Level 3: Escalate to human / abort
4. When all steps COMPLETE or chain ABORTED:
   - Fire handlers (on chain_complete / chain_failed)
   - Persist final state
   - Return chain result
```

**Key properties:**
- **Deterministic scheduling:** ready set computed from DAG topology, not LLM decisions
- **Immutable plan:** the DAG is fixed at parse time (no dynamic replanning by default)
- **Bounded recovery:** max retries + escalation levels prevent infinite loops
- **Checkpoint/resume:** state persisted after each step for crash recovery

### 3.3 Rule Engine — Grule Recommended (Updated per RQ2 Assessment)

The rule engine handles gating criteria. It uses a simplified forward-chaining model inspired by Drools.

**Architecture:**
```
┌─────────────────────────────────────────────┐
│              Rule Engine                     │
│                                             │
│  ┌─────────────┐    ┌───────────────────┐   │
│  │  Working     │    │  Rule Base        │   │
│  │  Memory      │◄──►│  (parsed from     │   │
│  │  (facts)     │    │   gate: blocks)   │   │
│  └──────┬──────┘    └───────────────────┘   │
│         │                                    │
│  ┌──────▼──────┐    ┌───────────────────┐   │
│  │  Pattern    │    │  Conflict         │   │
│  │  Matcher    │───►│  Resolution       │   │
│  │             │    │  (salience-based)  │   │
│  └─────────────┘    └────────┬──────────┘   │
│                              │               │
│                     ┌────────▼──────────┐   │
│                     │  Action           │   │
│                     │  Executor         │   │
│                     │  (continue/skip/  │   │
│                     │   pause/set_fact) │   │
│                     └───────────────────┘   │
└─────────────────────────────────────────────┘
```

**Working Memory contents (facts):**
- All `register`ed step outputs
- All `set_fact` values from prior rules
- Chain-level metadata (`_chain_cost_usd`, `_chain_step_index`, `_elapsed_time`)
- Gathered context (ticket state, repo state, environment)

**Why NOT full Rete/Phreak:** Full Rete is optimized for large rule bases (hundreds/thousands of rules) with incremental evaluation. Agent chain gates typically have 3-10 rules per gate. A simple linear evaluation with salience ordering is sufficient and far simpler to implement and debug. Reserve Rete-class algorithms for enterprise deployments with complex, cross-chain rule bases.

**Rule Engine Recommendation (Updated 2026-04-20):**

Based on Maya's capability assessment (RQ2 addendum), the recommendation is now definitive:

| Engine | Forward Chaining | Truth Maintenance | Performance | Verdict |
|---|---|---|---|---|
| **Grule** (Go) | **YES** — cyclic re-evaluation after each rule fires | **Partial** — manual `Changed()` required | ~0.01ms/100 rules, ~0.57ms/1000 rules | **RECOMMENDED** — default lightweight engine |
| **GoRules** (Rust/WASM) | **NO** — deterministic, stateless, single-pass | **NO** — stateless by design | Sub-ms startup | **Suitable only for simple condition→action gates** |
| **Drools** (JVM) | **YES** — full Rete/Phreak | **YES** — automatic TMS | Optimized for large rule bases | **Enterprise upgrade path** — when rule count exceeds ~100 cross-chain |
| **pyKnow** (Python) | YES | Partial | Slower | Viable if engine is Python-only |

**Why Grule is the right default:**
1. **Forward chaining is required** for this architecture. When a quality-check gate fires and downgrades a confidence score, dependent gates downstream need to re-evaluate. GoRules cannot do this — it's deterministic single-pass by design.
2. **Same ecosystem as K8s tooling** (Go), avoiding JVM dependency overhead of Drools.
3. **Performance is adequate:** ~0.57ms for 1000 rules is negligible vs. LLM call latency (typically 1-30 seconds). Typical gates (3-10 rules) execute in sub-0.1ms.
4. **Manual `Changed()` is mitigable:** Gate evaluations happen at discrete step boundaries, so the surface area for missed mutations is small. Wrap Grule in a thin adapter that auto-calls `Changed()` on all modified working memory fields after each rule action.

**Grule integration pattern:**
```go
// GruleGateAdapter wraps Grule with auto-Changed() notification
type GruleGateAdapter struct {
    engine   *engine.GruleEngine
    knowledgeLibrary *ast.KnowledgeLibrary
}

func (g *GruleGateAdapter) EvaluateGate(facts map[string]interface{}, rules []GateRule) (GateResult, error) {
    // 1. Load facts into Grule DataContext
    dataCtx := ast.NewDataContext()
    for k, v := range facts {
        dataCtx.Add(k, v)
    }
    
    // 2. Load rules (parsed from YAML gate: blocks)
    // Rules use salience for conflict resolution (same as Drools)
    
    // 3. Execute — Grule's cyclic engine re-evaluates all rules
    //    after each firing until no more match (or max cycles hit)
    
    // 4. Auto-Changed() wrapper: after each rule action that modifies
    //    working memory, call Changed() on all modified fields
    //    This compensates for Grule's manual truth maintenance
    
    // 5. Return: action (continue/skip/pause), modified facts, rules fired
}
```

**Upgrade path to Drools:** For enterprise deployments with 100+ cross-chain rules requiring automatic truth maintenance (fact retraction cascades), provide a `DroolsGateAdapter` implementing the same `GateEvaluator` interface. The YAML gate definitions remain identical — only the runtime engine changes.

- Source: [Grule Rule Engine (Go)](https://github.com/hyperjumptech/grule-rule-engine) | [Grule Docs](https://github.com/hyperjumptech/grule-rule-engine/blob/master/docs/en/RuleEngine_en.md)
- Source: [GoRules vs Drools](https://gorules.io/compare/gorules-vs-drools) | [GoRules Zen Engine](https://github.com/gorules/zen)

### 3.4 Context Store (Working Memory)

The context store merges Ansible's variable system with Drools' working memory concept:

```python
class ChainContext:
    """
    Unified context store for chain execution.
    Combines Ansible variable precedence with Drools working memory.
    """
    
    # Variable precedence (low → high):
    # 1. Chain defaults
    # 2. Skill defaults  
    # 3. Chain vars
    # 4. Gathered context (facts)
    # 5. Registered outputs
    # 6. Rule-set facts (set_fact)
    # 7. Runtime overrides (trigger vars, CLI args)
    
    layers = [
        "chain_defaults",
        "skill_defaults", 
        "chain_vars",
        "gathered_context",
        "registered_outputs",
        "rule_facts",
        "runtime_overrides"
    ]
    
    # Working memory for rule engine
    facts: Dict[str, Any]  # All resolved variables as facts
    
    # Chain metadata (auto-populated)
    _chain_cost_usd: float
    _chain_step_index: int
    _chain_elapsed_seconds: float
    _chain_outcome: str  # running / success / failed / aborted
    _rescue_error: Optional[str]  # Set in rescue blocks
```

### 3.5 Step Executor Plugin System (Module System)

Modeled on Ansible's module/action plugin architecture:

```python
class StepExecutor(ABC):
    """Base class for step executors (like Ansible ActionPlugin)."""
    
    @abstractmethod
    def execute(self, args: dict, context: ChainContext) -> StepResult:
        """Execute the step and return results."""
        pass
    
    @abstractmethod
    def validate_args(self, args: dict, input_schema: dict) -> bool:
        """Validate step arguments against schema."""
        pass
    
    @property
    def side_effect_class(self) -> SideEffectClass:
        """Classify step as READ_ONLY, IDEMPOTENT, or NON_IDEMPOTENT."""
        return SideEffectClass.NON_IDEMPOTENT

# Built-in executors:
class ClaudeSkillExecutor(StepExecutor):
    """Execute a Claude CLI/SDK skill."""
    side_effect_class = SideEffectClass.READ_ONLY  # Unless skill writes files

class ScriptRunnerExecutor(StepExecutor):
    """Run a shell script / CLI command."""
    side_effect_class = SideEffectClass.NON_IDEMPOTENT

class APICallExecutor(StepExecutor):
    """Make an HTTP API call (Jira, GitHub, Slack, etc.)."""
    # side_effect_class depends on HTTP method (GET=read_only, POST=non_idempotent)

class HumanGateExecutor(StepExecutor):
    """Pause for human approval."""
    side_effect_class = SideEffectClass.READ_ONLY

class RuleGateExecutor(StepExecutor):
    """Evaluate gate rules against working memory."""
    side_effect_class = SideEffectClass.READ_ONLY

class SubChainExecutor(StepExecutor):
    """Execute another chain as a step (role inclusion)."""
    # side_effect_class = depends on sub-chain
```

**Plugin discovery:** Follow Ansible's convention — executors are registered by name in a plugin registry. Custom executors can be loaded from a `plugins/` directory in the chain project, or installed as Python packages.

---

## 4. Handling Non-Deterministic LLM Calls

This is the hardest architectural problem. LLM calls break three assumptions that traditional workflow engines rely on:

### 4.1 Non-Idempotent Retry Strategy

Based on the Graph Harness framework (arXiv:2604.11378) and production patterns from Composio/Temporal:

```yaml
# Per-step retry configuration
retry_policy:
  max_retries: 2
  escalation:
    - level: mechanical_retry
      # Same inputs, hope for different (better) output
      # Only for transient failures (API timeout, rate limit)
      delay: exponential  # 1s, 2s, 4s
      
    - level: local_patch
      # Augment prompt with error context, retry with more guidance
      # For reasoning failures (bad output, schema violation)
      patch_prompt: "Previous attempt failed because: {{ _retry_error }}. Please fix."
      
    - level: escalate
      # Give up, route to human or alternative path
      action: rescue_block  # or: human_gate, abort, skip
```

**Critical insight from Graph Harness:** Separate *execution context* (task inputs) from *diagnostic context* (failure history). Don't feed failure information back into the main prompt — it can reinforce errors. Instead, append diagnostic context as a separate system message or use a patching prompt.

### 4.2 Output Contract Validation

Every LLM step should declare an `output_schema`. The engine validates LLM output against this schema BEFORE registering it:

```yaml
- name: Analyze ticket
  skill: claude_analyze
  args:
    output_schema:
      type: object
      required: [severity, components]
      properties:
        severity: { type: string, enum: [low, medium, high, critical] }
        components: { type: array, items: { type: string } }
  register: analysis
  # If output doesn't match schema → trigger retry_policy
```

This replaces Ansible's idempotency check ("is desired state already achieved?") with a contract check ("did the LLM produce valid structured output?").

### 4.3 Idempotency Keys for Side Effects

For steps that create external artifacts (PRs, Jira comments, Slack messages), use idempotency keys to prevent duplicates on retry:

```yaml
- name: Create PR
  skill: github_pr
  idempotency_key: "pr-{{ trigger.ticket_key }}-{{ analysis.hash }}"
  # Engine checks: does a PR with this key already exist?
  # If yes: skip creation, return existing PR
  # If no: create new PR, tag with key
```

---

## 5. Answering Sam's PoC Questions

### 5.1 Should the execution engine wrap Temporal, or stay standalone?

**Answer: Two-tier architecture.** 

- **Development / PoC / low-volume:** Standalone with in-process state (SQLite or PostgreSQL). Sam's current `executor.py` with `ThreadPoolExecutor` is correct for this tier. Add checkpoint persistence to a database and you have a viable lightweight engine.

- **Production / high-volume / multi-user:** Wrap Temporal as the durable execution substrate. The YAML parser and DAG planner remain the same. The scheduler delegates to Temporal workflows instead of local threads. Each chain step becomes a Temporal Activity. Temporal provides: durability, replay-based recovery, multi-worker distribution, and visibility tooling for free.

**Implementation approach:**
```python
class SchedulerBackend(ABC):
    """Pluggable scheduler backend."""
    async def dispatch_step(self, step, context) -> StepResult: ...
    async def checkpoint(self, state) -> None: ...
    async def resume(self, chain_id) -> ChainState: ...

class LocalScheduler(SchedulerBackend):
    """ThreadPoolExecutor + SQLite. For dev/PoC."""
    
class TemporalScheduler(SchedulerBackend):
    """Temporal workflow/activity. For production."""
```

This is exactly the Auton framework's "Runtime Engine" separation — the YAML spec doesn't know or care which backend runs it.

### 5.2 How should we handle idempotency for non-deterministic LLM calls?

**Answer: Don't try to make LLM calls idempotent. Instead, classify and compensate.**

Three mechanisms (detailed in Section 4 above):
1. **Output contract validation** — validate output structure before accepting
2. **Idempotency keys for side effects** — prevent duplicate external artifacts
3. **Bounded retry with escalation** — mechanical retry → prompt patching → human escalation

The key insight: idempotency is a *step-level* property, not a *chain-level* property. Some steps ARE idempotent (API reads, status checks). Some are NOT (LLM generation, PR creation). Tag each step with its `side_effect_class` and handle accordingly.

---

## 6. Cost Management Subsystem

Not present in Ansible; essential for agent chains.

```yaml
config:
  max_cost_usd: 10.00          # Hard chain-level budget
  cost_warning_threshold: 0.8   # Warn at 80% budget
  
steps:
  - name: Expensive analysis
    skill: claude_opus
    cost_budget: 3.00            # Per-step budget cap
    args:
      model: claude-opus-4      # Explicitly use expensive model
      
  - name: Cheap validation
    skill: claude_haiku
    cost_budget: 0.10
    args:
      model: claude-haiku       # Use cheap model for validation
```

**Engine behavior:**
- Track cumulative cost after each step
- If cumulative > `max_cost_usd`: abort chain, fire `chain_budget_exceeded` handler
- If cumulative > `cost_warning_threshold * max_cost_usd`: log warning, optionally notify
- In dry-run mode: estimate costs without executing (based on token estimates)

---

## 7. Observability & Audit

Modeled on ansible-runner's event system:

```
Events emitted (via WebSocket/SSE to Flask dashboard):
  chain_started        { chain_id, chain_name, trigger }
  step_started         { chain_id, step_name, step_type }
  step_completed       { chain_id, step_name, output_preview, cost_usd, duration }
  step_failed          { chain_id, step_name, error, retry_level }
  gate_evaluated       { chain_id, gate_name, rules_fired, action }
  human_gate_waiting   { chain_id, gate_name, approvers, context }
  human_gate_resolved  { chain_id, gate_name, decision, approver }
  chain_completed      { chain_id, outcome, total_cost, duration }
  chain_checkpoint     { chain_id, step_name, state_snapshot }
```

**Audit trail:** Every event persisted to a structured log (JSON lines). Enables:
- Chain replay (re-execute from checkpoint with modified vars)
- Cost analysis (which chains/steps are most expensive?)
- Performance profiling (where are the bottlenecks?)
- Compliance (who approved what, when?)

---

## 8. Deployment Architecture (K8s)

```
┌────────────────────────────────────────────────────────┐
│                  Kubernetes Cluster                  │
│                                                      │
│  ┌──────────────┐     ┌───────────────────────────┐ │
│  │ Flask        │     │ Chain Controller           │ │
│  │ Dashboard    │────►│ (Deployment, 2+ replicas)  │ │
│  │ (Deployment) │     │                            │ │
│  └──────────────┘     └──────────┬────────────────┘ │
│                                  │                   │
│                    ┌─────────────┼─────────────┐     │
│                    ▼             ▼             ▼     │
│              ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│              │ Claude   │ │ Script   │ │ API      │ │
│              │ Executor │ │ Executor │ │ Executor │ │
│              │ (Job)    │ │ (Job)    │ │ (Job)    │ │
│              └──────────┘ └──────────┘ └──────────┘ │
│                                                      │
│  ┌──────────────┐     ┌───────────────────────────┐ │
│  │ PostgreSQL   │     │ Redis (optional)           │ │
│  │ (state,      │     │ (event bus, caching)       │ │
│  │  audit log)  │     │                            │ │
│  └──────────────┘     └───────────────────────────┘ │
│                                                      │
│  ┌──────────────────────────────────────────────────┐│
│  │ Temporal Server (optional, for production)       ││
│  │ (durable execution, multi-worker)                ││
│  └──────────────────────────────────────────────────┘│
└────────────────────────────────────────────────────────┘
```

**Two deployment tiers:**

| Tier | State Backend | Executor Model | When to Use |
|---|---|---|---|
| **Lightweight** | PostgreSQL | In-process ThreadPool | PoC, dev, low-volume (<50 chains/day) |
| **Production** | Temporal + PostgreSQL | K8s Jobs per step | Production, high-volume, multi-user |

---

## 9. Schema Extensions Beyond Ansible

Features that Ansible doesn't have but agent chains need:

| Feature | YAML Keyword | Purpose |
|---|---|---|
| Parallel fan-out/fan-in | `parallel:` with `merge_strategy:` | Run steps concurrently, merge results |
| Cost budgets | `cost_budget:`, `max_cost_usd:` | Prevent runaway LLM spending |
| Output contracts | `output_schema:` | Validate LLM output structure |
| Side-effect classification | `side_effects:` | Enable safe retry logic |
| Idempotency keys | `idempotency_key:` | Prevent duplicate artifacts |
| Human approval gates | `gate: { type: human_approval }` | Pause for human decision |
| Rule-based gates | `gate: { rules: [...] }` | Drools-style conditional routing |
| Model selection | `model:` at chain/step level | Route to appropriate LLM |
| Dependency declaration | `depends_on:` | Explicit DAG edges (beyond ordering) |
| Sub-chain inclusion | `include_chain:` | Compose chains from reusable sub-chains |

---

## 10. Build vs. Extend Decision

| Approach | Pros | Cons | Verdict |
|---|---|---|---|
| **Extend Ansible** (custom modules + plugins) | Leverage existing ecosystem, familiar to ops | Fight host-centric model, no DAG, no durable execution, wrong parallelism axis | ❌ Not recommended |
| **Extend Temporal** (YAML layer on top) | Durable execution for free, production-proven | YAML parser is custom anyway, Temporal complexity for simple chains | ⚠️ Good for production backend, but not the primary interface |
| **Extend Argo Workflows** (YAML CRDs) | K8s-native, existing DAG engine | Pod-per-step overhead, no durable state, static DAG only | ⚠️ If already running Argo |
| **Build from scratch** (YAML engine + pluggable backend) | Full control, clean semantics, right abstractions | More upfront work, less battle-tested | ✅ **Recommended** |

**Recommended approach:** Build the YAML parser, DAG planner, rule engine, and context store from scratch. Make the scheduler backend pluggable (local for dev, Temporal for production). This gives you the right abstractions without fighting an existing framework's assumptions.

Sam's PoC (`yaml-agent-chain-poc`) is already on this path. The key additions needed:
1. **Schema validation** (JSON Schema for chain definitions)
2. **DAG builder** (currently linear + parallel; add `depends_on` for arbitrary DAGs)
3. **Pluggable scheduler backend** (add Temporal backend alongside ThreadPoolExecutor)
4. **Persistent state** (currently in-memory; add PostgreSQL/SQLite checkpoint)
5. **Output contract validation** (validate LLM outputs before registering)
6. **Cost tracking** (per-step and cumulative)
7. **Grule integration** (replace custom rule evaluator with GruleGateAdapter for production gating)

---

## 11. Comparison: This Architecture vs. Existing Approaches

| Dimension | This Design | Ansible | Temporal | Argo Workflows | Microsoft Agent Framework |
|---|---|---|---|---|---|
| **YAML-first** | ✅ | ✅ | ❌ (code-first) | ✅ (CRDs) | ✅ |
| **DAG execution** | ✅ | ❌ (linear) | ✅ | ✅ | ✅ |
| **Rule engine/gates** | ✅ (Drools-inspired, Grule runtime) | ❌ | ❌ | ❌ | ⚠️ (basic) |
| **Durable execution** | ✅ (via backend) | ❌ | ✅ | ❌ | ✅ |
| **LLM-aware** | ✅ (cost, contracts, retries) | ❌ | ❌ | ❌ | ✅ |
| **Ops familiarity** | ✅ (Ansible-like YAML) | ✅ | ❌ | ⚠️ | ❌ |
| **Open source** | ✅ (proposed) | ✅ | ✅ | ✅ | ✅ |
| **K8s-native** | ✅ (K8s Jobs) | ❌ | ⚠️ | ✅ | ❌ |
| **Human-in-the-loop** | ✅ (first-class gates) | ❌ | ✅ (signals) | ✅ (suspend) | ✅ |

---

## Sources

- [Auton Framework — Declarative Agent Architecture (arXiv:2602.23720)](https://arxiv.org/html/2602.23720v1)
- [Graph Harness — Scheduler-Theoretic Framework for LLM Agents (arXiv:2604.11378)](https://arxiv.org/html/2604.11378v1)
- [Declarative Language for LLM Agent Workflows (arXiv:2512.19769)](https://arxiv.org/abs/2512.19769)
- [Ansible Execution Architecture (DeepWiki)](https://deepwiki.com/ansible/ansible)
- [Drools Rule Engine Documentation](https://docs.drools.org/latest/drools-docs/drools/rule-engine/index.html)
- [Grule Rule Engine (Go)](https://github.com/hyperjumptech/grule-rule-engine)
- [Grule Rule Engine Documentation](https://github.com/hyperjumptech/grule-rule-engine/blob/master/docs/en/RuleEngine_en.md)
- [GoRules vs Drools Comparison](https://gorules.io/compare/gorules-vs-drools)
- [GoRules Zen Engine](https://github.com/gorules/zen)
- [Microsoft Agent Framework v1.0](https://devblogs.microsoft.com/agent-framework/microsoft-agent-framework-version-1-0/)
- [Kestra Event-Driven Orchestration](https://github.com/kestra-io/kestra)
- [Ansible Runner for Python Applications](https://oneuptime.com/blog/post/2026-02-21-ansible-runner-python-applications/view)



---

## 5.3 Should TMS Propagation Trigger Auto-Re-execution? (Added 2026-04-20)

**Answer: Flag-only is the correct default. Auto-re-execution should be opt-in, gated by `side_effect_class`.**

### Rationale

1. **Cost safety.** LLM calls are non-deterministic and expensive. If a human rejects step 3 and TMS auto-re-executes steps 4-N, you spend money re-running prompts that will likely produce similar output — because the *inputs* haven't changed, only the *approval status*. The rejection needs human guidance about *what to change*, not a blind retry.

2. **Drools precedent supports flag-only.** Drools TMS retracts derived facts but does NOT automatically re-derive them. It removes stale conclusions from working memory; re-derivation only happens if new facts enter the system and trigger rule re-evaluation on the next cycle. Sam's current "flag stale" behavior mirrors this correctly.

3. **The right answer depends on `side_effect_class`.** Deterministic steps (script runners, API reads) CAN safely auto-re-execute — the output is a pure function of inputs. Non-deterministic steps (LLM calls) SHOULD NOT — you get different output each time, potentially worse.

### TMS Policy Keyword

A `tms_policy` keyword at chain and step level controls propagation behavior:

```yaml
config:
  tms_policy: flag_only  # Default — safest

steps:
  - name: run_linter
    skill: script_runner
    tms_policy: auto_reexecute  # Safe — deterministic

  - name: analyze_code
    skill: claude_analyze
    tms_policy: flag_and_notify  # Flag + alert dashboard

  - name: generate_fix
    skill: claude_code
    tms_policy: flag_only  # Default — requires human decision
```

### Three Policies

| Policy | Behavior | Safe For |
|---|---|---|
| `flag_only` | Mark stale, stop. Human decides. | All steps (default) |
| `flag_and_notify` | Mark stale + push notification to chain owner/dashboard | LLM steps where human should decide quickly |
| `auto_reexecute` | Automatically re-run stale steps | Deterministic/idempotent steps ONLY |

### Engine Safety Guards

1. **Parse-time warning:** If `auto_reexecute` is set on a step with `side_effect_class: NON_IDEMPOTENT`, the engine emits a WARNING. Don't block — the chain author may know what they're doing — but flag the risk.

2. **`max_cascade_depth`:** Prevents infinite TMS propagation in diamond dependencies. In production with auto-reexecute, A→B→D + A→C→D could cause D to re-execute twice. Cap it: re-execute once per TMS wave, not per dependency edge.

```yaml
config:
  tms_policy: auto_reexecute
  max_cascade_depth: 3  # Max levels of transitive re-execution
```

### Schema Extension (Addition to Section 9)

| Feature | YAML Keyword | Purpose |
|---|---|---|
| TMS propagation policy | `tms_policy:` | Control behavior when upstream step output is retracted/updated (flag_only / flag_and_notify / auto_reexecute) |
| TMS cascade limit | `max_cascade_depth:` | Maximum levels of transitive re-execution to prevent unbounded cascades |
