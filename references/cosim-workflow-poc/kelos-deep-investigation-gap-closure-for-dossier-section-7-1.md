# Kelos Deep Investigation
## Closing MEDIUM-Severity Gap — Dossier Section 7.1
**Researcher:** Maya (OSINT) | **Date:** 2026-04-20 | **Priority:** MEDIUM

---

## Executive Summary

Kelos is the most architecturally similar open-source project to our proposed YAML-driven agent job chaining engine. It shares our "workflow as YAML on Kubernetes" thesis but is **narrowly scoped to coding agents** and **lacks conditional gating** — our core differentiator. Bottom line: **complement, not competitor.** Low community adoption limits near-term collaboration value, but its execution model validates our architectural direction.

---

## 1. Conditional Gating Between Workflow Steps

**Finding: Kelos does NOT implement conditional gating.**

What it has:
- **`dependsOn` sequencing** — Tasks wait until all dependencies succeed before executing. Binary pass/fail only.
- **Template variable interpolation** — Downstream tasks access upstream results via `{{.Deps}}` syntax (branch names, PR URLs, commit SHAs, token usage).
- **TaskSpawner filtering** — Event-driven task creation uses filter expressions (e.g., `labels: [bug]`, `state: open`) to determine which GitHub events trigger spawning.

What it lacks:
- No if/else conditional branching between steps
- No gate/rule evaluation logic
- No deterministic vs. agentic routing decisions
- No GREEN/AMBER/RED zone classification
- No conditional fan-out based on intermediate results

**Critical quote from the creator on HN:** Regarding `dependsOn` chaining, the creator noted there's **"no real usage now"** and questioned **"whether simpler numbered pipeline prompts would suffice."** This suggests conditional workflow logic is not on their near-term radar.

**Implication for us:** Our gate/rule evaluation and deterministic/agentic hybrid classification remain **unique differentiators** with no overlap from Kelos. The comparison matrix in the dossier (Section 2.6) correctly identifies this gap.

---

## 2. Community Momentum

| Metric | Value | Assessment |
|--------|-------|------------|
| GitHub Stars | ~111 | Low — early-stage project |
| Forks | 18 | Minimal community contribution |
| Total Releases | 26 | High velocity for project age |
| Latest Release | v0.30.0 (April 16, 2026) | Very active — 3 releases in ~2 weeks |
| Release Cadence | ~every 3–5 days | Exceptionally fast iteration |
| Activity Score | 9.7/10 (libhunt) | Top-tier activity for its category |
| Language | Go (97.3%) | K8s-native ecosystem |
| License | Apache 2.0 | Permissive, collaboration-friendly |
| HN Engagement | 5 comments (Show HN #1), 1 comment (Show HN #2) | Very low community traction |
| Contributors | Appears primarily single-developer (gjkim) | Bus factor risk |
| CNCF Status | None | Not a sandbox/incubating project (contrast: kagent is CNCF sandbox) |

**Assessment:** Kelos is a fast-iterating solo project with high development velocity but low community adoption. The creator is actively dogfooding (Kelos develops Kelos via 24/7 TaskSpawners), which drives the rapid release cadence. However, the project has not yet achieved meaningful community traction.

---

## 3. Public Roadmap & Expansion Beyond Coding Agents

**Finding: No public roadmap. No indication of expansion beyond coding agents.**

- All documented use cases center on software development: issue triage, PR review, bug fixing, fleet-wide refactoring, CI/CD integration, code scanning
- The creator's HN comments focus exclusively on coding workflows
- No mention of general-purpose agent orchestration, business process automation, or non-coding domains
- The project's tagline is explicitly "orchestrating autonomous AI **coding** agents"

**Recent feature trajectory (v0.28–v0.30):**
- Helm chart customization for `kelos install`
- Pod label overrides
- File pattern filtering for PR/webhook sources
- Generic webhook source type (JSONPath mapping)
- Agent image updates (Claude Code, Cursor, Gemini, OpenCode)

All features reinforce the coding agent focus. No signals of broadening scope.

---

## 4. Architecture Deep Dive

### Core Primitives
1. **Tasks** — Ephemeral units of work wrapping an AI agent run. Each runs in an isolated Pod with a fresh git clone. No host access.
2. **Workspaces** — Persistent or ephemeral git repo environments. GitHub serves as the persistent state layer (branches, PRs).
3. **AgentConfigs** — Reusable bundles of agent instructions (CLAUDE.md, AGENTS.md), plugins, and MCP server configurations.
4. **TaskSpawners** — Orchestration engines reacting to external triggers (GitHub Issues, PRs, Webhooks, Jira, Cron) to create Tasks automatically.

### Step Execution
- Each Task = one Kubernetes Job = one ephemeral Pod
- Fresh git clone per task (isolation)
- Agent-agnostic via standardized container interface
- Supports: Claude Code, OpenAI Codex, Google Gemini, OpenCode, Cursor, custom images

### State Persistence
- **GitHub is the persistent layer** — branches, PRs, commits survive across tasks
- Without Workspace config, files are lost when pod terminates
- Task status captures deterministic outputs: branch name, commit SHA, PR URL, token usage

### Error Recovery
- **Limited.** No documented retry logic or error recovery mechanisms in the framework itself
- Creator mentions the self-dev pipeline "retries on CI failures" and labels issues "needs-input" when stuck, but this appears to be prompt-driven behavior, not framework-level
- Resource guardrails: `maxConcurrency`, `maxTotalTasks`, `podOverrides.activeDeadlineSeconds`

### What's Missing vs. Our Proposed Engine
| Capability | Kelos | Our Engine |
|-----------|-------|------------|
| YAML-defined workflows | ✅ | ✅ |
| K8s-native execution | ✅ | ✅ |
| Conditional gating/rules | ❌ | ✅ (core differentiator) |
| Deterministic/agentic hybrid | ❌ | ✅ (GREEN/AMBER/RED zones) |
| Multi-domain (not just coding) | ❌ | ✅ |
| Error recovery/retry framework | ❌ | ✅ (planned) |
| State persistence beyond git | ❌ (GitHub only) | ✅ |
| Community/ecosystem | Low (111 stars) | N/A (greenfield) |

---

## 5. Competitive Landscape Context

Kelos sits alongside two other K8s-native agent projects:

| | Kelos | kagent | Agent Sandbox |
|---|---|---|---|
| **Focus** | Coding agent orchestration | DevOps/SRE agent management | Secure execution primitives |
| **Layer** | Workflow/pipeline | Agent runtime & tools | Infrastructure |
| **CNCF** | No | Sandbox project | SIG Apps project |
| **Backed by** | Solo developer | Solo.io | Google (SIG Apps) |
| **Stars** | ~111 | Higher (CNCF backing) | K8s-sigs official |

These three are **complementary layers**, not competitors to each other. Agent Sandbox provides isolation, kagent provides runtime, Kelos provides workflow orchestration. None of them implement conditional gating or deterministic/agentic hybrid classification.

---

## 6. Bottom Line Assessment

### Classification: **COMPLEMENT, not competitor**

**Why not a competitor:**
- Kelos is narrowly scoped to coding agents; our engine targets general-purpose agent job chaining
- Kelos lacks conditional gating — the entire premise of our GREEN/AMBER/RED zone architecture
- Kelos has no deterministic/agentic hybrid routing
- Low community adoption means it's not setting market expectations

**Why a complement:**
- Validates the "YAML + K8s + agent workflows" thesis independently
- Its execution model (ephemeral pods, git-as-state, agent-agnostic containers) is compatible with our proposed architecture
- Apache 2.0 license permits integration or reference
- Could theoretically be a downstream consumer of our gating engine — we provide intelligent routing, Kelos provides sandboxed execution

**Why NOT a collaboration target (yet):**
- Single-developer project — bus factor risk
- Low community traction — limited ecosystem leverage
- No interest in conditional gating from the creator
- Narrowly scoped to coding — not aligned with our multi-domain vision

**Recommendation:** Monitor Kelos for architectural inspiration (especially the TaskSpawner pattern and agent-agnostic container interface), but do not pursue active collaboration at this stage. Revisit if community adoption exceeds ~500 stars or if they add conditional workflow logic.

---

## Sources

- [Kelos GitHub Repository](https://github.com/kelos-dev/kelos)
- [Kelos DEV Community Article (Mar 2026)](https://dev.to/gjkim/kelos-orchestrating-autonomous-ai-coding-agents-on-kubernetes-apl)
- [Show HN: Kelos — Define AI coding agent workflow as YAML on K8s](https://news.ycombinator.com/item?id=47218469)
- [Show HN: Kelos — Run Claude on Kubernetes](https://news.ycombinator.com/item?id=47247803)
- [Kelos Alternatives — libhunt](https://www.libhunt.com/r/kelos)
- [Kelos Releases — GitHub](https://github.com/kelos-dev/kelos/releases)
- [kagent — Cloud Native Agentic AI](https://github.com/kagent-dev/kagent)
- [Agent Sandbox — K8s SIG Apps](https://kubernetes.io/blog/2026/03/20/running-agents-on-kubernetes-with-agent-sandbox/)
- [Cloud Native Now — Gas Town](https://cloudnativenow.com/features/gas-town-what-kubernetes-for-ai-coding-agents-actually-looks-like/)
