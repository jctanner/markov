# Project Comparisons

How Markov relates to existing tools in workflow orchestration, infrastructure automation, and AI agent frameworks.

## Quick Reference

| | Markov | Ansible | Argo Workflows | Jenkins | Tekton | Kestra | CrewAI | LangGraph |
|---|---|---|---|---|---|---|---|---|
| **YAML-native workflows** | ✅ | ✅ | ✅ | ❌ (Groovy/JCasC) | ✅ | ✅ | ✅ | ❌ (Python) |
| **K8s Jobs as execution** | ✅ | ❌ (SSH) | ✅ (Pods) | ❌ | ✅ (Pods) | ⚠️ (deployable) | ❌ | ❌ |
| **Rule engine / gates** | ✅ (Grule) | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **Recursive workflows** | ✅ (gate-controlled) | ❌ | ❌ | ❌ | ❌ | ⚠️ (subflows) | ❌ | ✅ (cycles) |
| **Checkpoint/resume** | ✅ (SQLite) | ❌ | ✅ (retry node) | ❌ | ❌ | ✅ | ❌ | ✅ |
| **Fan-out concurrency** | ✅ (`for_each` + forks) | ✅ (host forks) | ✅ (DAG) | ✅ (parallel) | ✅ (matrix) | ✅ | ✅ (crews) | ✅ |
| **Artifact loading** | ✅ (YAML/md/table) | ❌ | ✅ (S3/GCS) | ✅ (stash) | ✅ (results) | ✅ | ❌ | ❌ |
| **No CRDs required** | ✅ | ✅ | ❌ | ✅ | ❌ | ✅ | n/a | n/a |
| **AI/LLM-aware** | ✅ | ❌ | ❌ | ❌ | ❌ | ⚠️ | ✅ | ✅ |

## Ansible

The most direct comparison. Markov borrows Ansible's ergonomic patterns — `when:`, `register:`, `for_each`, `set_fact`, template rendering — because they are well-understood by operations teams.

| | Ansible | Markov |
|---|---|---|
| **Target** | Remote hosts via SSH | K8s Jobs in a namespace |
| **Execution** | Runs Python modules on hosts | Creates K8s Jobs, polls for completion |
| **State** | Stateless between runs | SQLite checkpoint/resume |
| **Concurrency** | Forks across hosts | Forks across sub-workflows (fan-out) |
| **Data flow** | Register + facts | Register + artifact loading + set_fact + rule engine |
| **Decision logic** | `when:` conditionals only | `when:` + gate steps with salience-ordered rules (Grule) |
| **Scope** | General-purpose IT automation | Long-running AI/ML pipeline orchestration |
| **Inventory** | Hosts and groups | Namespaces and K8s resources |

**Why not Ansible?** Ansible SSHes into machines and runs Python modules. Markov creates K8s Jobs that run containers (e.g., agent pods with LLM skills) and waits for them to finish. Checkpoint/resume and artifact loading exist because these jobs run for minutes to hours, not seconds. Ansible has no rule engine — complex conditional logic requires deeply nested `when:` chains or custom plugins. Ansible also has no concept of recursive workflow invocation — playbooks execute linearly, and loops (`loop:` / `until:`) are limited to retrying individual tasks, not re-entering an entire workflow with updated state.

## Argo Workflows

Argo is the most popular K8s-native workflow engine. The research evaluated it directly (see `references/cosim-workflow-poc/adr-bare-k8s-client-vs-argo-workflows-for-child-job-orchestration-rq-b1.md`).

| | Argo Workflows | Markov |
|---|---|---|
| **K8s model** | Custom CRDs (`Workflow`, `WorkflowTemplate`) | Plain `batch/v1` Jobs — no CRDs |
| **Infrastructure** | Always-on controller + server + CRDs | Single binary, SQLite state file |
| **RBAC surface** | Broad (controller needs pods, PVCs, configmaps, secrets) | Narrow (Jobs + pods/log only) |
| **Retry** | Built-in `retryStrategy` per step | Checkpoint/resume from last success |
| **DAG** | First-class `dag` template type (acyclic only) | Sequential, fan-out, and recursive workflows |
| **Artifacts** | S3/GCS/HTTP native | YAML/markdown from local or K8s volumes |
| **Human gates** | `suspend` node (no approval UI) | `gate` step with pause action (extensible) |
| **Rule engine** | ❌ | ✅ Grule forward-chaining |
| **Resource overhead** | Controller: 500m–2Gi; sidecar per pod | No overhead beyond the workflow runner |

**Why not Argo?** Argo is a general-purpose container orchestration engine. Its DAGs are acyclic by definition — there is no way to express "loop until a rule says stop." Argo's `retryStrategy` retries a failed step, but it can't re-enter a workflow with mutated state. Adding rule-engine logic requires building outside Argo's model. The CRD installation and RBAC surface area are also significant in enterprise multi-tenant environments. Markov uses bare `batch/v1` Jobs — no operator, no CRDs, no sidecar containers.

## Jenkins

Jenkins is a general-purpose CI/CD automation server, not a workflow engine, but it's widely used for pipeline orchestration.

| | Jenkins | Markov |
|---|---|---|
| **Definition** | Groovy `Jenkinsfile` or JCasC YAML | Declarative YAML |
| **Execution** | Agent nodes (SSH/JNLP) | K8s Jobs |
| **State** | Build history, no resume | SQLite checkpoint/resume |
| **Plugins** | 1800+ plugins (strength and weakness) | Built-in primitives, user-defined step types |
| **Rule engine** | ❌ | ✅ Grule |
| **Concurrency** | `parallel` block | `for_each` + forks |

**Why not Jenkins?** Jenkins is imperative (Groovy scripts), not declarative. Its plugin ecosystem is powerful but adds operational complexity. It has no concept of rule-engine gating, artifact-driven conditionals, or checkpoint/resume for long-running jobs.

## Tekton

Tekton is a K8s-native CI/CD framework that runs pipelines as pods.

| | Tekton | Markov |
|---|---|---|
| **K8s model** | CRDs (`Task`, `Pipeline`, `PipelineRun`) | Plain `batch/v1` Jobs |
| **Definition** | YAML CRD manifests | YAML workflow files |
| **Scope** | CI/CD pipelines | AI/ML workflow orchestration |
| **Results** | Step results via termination messages | Artifact loading + set_fact |
| **Rule engine** | ❌ | ✅ Grule |
| **Resume** | ❌ (re-run entire pipeline) | ✅ From last successful step |

**Why not Tekton?** Tekton is purpose-built for CI/CD (build, test, deploy). It doesn't have rule-engine gating, checkpoint/resume, or artifact-driven decision logic. Like Argo, it requires CRD installation and an operator.

## Kestra

Kestra is a YAML-first orchestration platform with recent AI capabilities ($25M Series A, March 2026).

| | Kestra | Markov |
|---|---|---|
| **Definition** | YAML flows | YAML workflows |
| **Execution** | Kestra workers (Docker/K8s) | K8s Jobs directly |
| **Event-driven** | ✅ Triggers, schedules, webhooks | ❌ (run/resume CLI) |
| **Rule engine** | ❌ | ✅ Grule |
| **State** | Internal DB (Postgres/MySQL) | SQLite |
| **Infrastructure** | Kestra server + workers + DB | Single binary |

**Why not Kestra?** Kestra is a full platform with a server, workers, and a database. Markov is a lightweight engine that creates K8s Jobs directly. Kestra has no rule-engine gating or Ansible-like execution semantics.

## CrewAI / LangGraph

AI agent frameworks that orchestrate LLM interactions.

| | CrewAI | LangGraph | Markov |
|---|---|---|---|
| **Definition** | YAML (agents/tasks) | Python (graphs) | YAML (workflows) |
| **Agent model** | Role-based crews | Graph nodes | K8s Jobs running agent containers |
| **K8s native** | ❌ | ❌ | ✅ |
| **Rule engine** | ❌ | ❌ | ✅ Grule |
| **Checkpoint** | ❌ | ✅ (thread state) | ✅ (step-level SQLite) |
| **Infrastructure awareness** | ❌ | ❌ | ✅ (namespaces, volumes, secrets) |

**Why not CrewAI/LangGraph?** These frameworks define agent behavior and LLM interactions. Markov orchestrates the infrastructure that runs agents — creating K8s Jobs, managing artifacts, gating on rule-engine evaluations. They operate at different layers: CrewAI/LangGraph define *what* agents do; Markov defines *when, where, and whether* agent jobs run.

## The Whitespace

No existing tool combines all four:

1. **Ansible-like YAML semantics** — `when:`, `register:`, `for_each`, template rendering
2. **Rule-engine gating** — salience-ordered forward-chaining rules (Grule)
3. **Recursive state-machine workflows** — gate-controlled loops where the next state depends on the current facts, not a fixed DAG
4. **K8s-native agent orchestration** — long-running Jobs with checkpoint/resume

Most workflow engines enforce acyclic execution — steps run in order or as a DAG, but can never revisit earlier states. Markov workflows can recursively invoke themselves or each other, with gate rules deciding on each cycle whether to continue, skip, or pause. This makes Markov a true state machine: evaluate rules, transition, repeat — the same pattern that defines a Markov chain.

The market research (`references/cosim-workflow-poc/rq3-market-landscape-yaml-declarative-ai-agent-chaining-tools.md`) confirms this is a genuine gap. Infrastructure automation tools (Ansible) don't have rule engines or recursive workflows. Workflow engines (Argo, Tekton) enforce acyclic execution and have no declarative decision logic. AI agent frameworks (CrewAI, LangGraph) don't manage infrastructure. Markov sits at their intersection.
