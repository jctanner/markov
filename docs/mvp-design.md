# Markov — MVP Design

## Overview

Markov is a Go-based workflow engine that reads declarative YAML workflow
files and executes steps as Kubernetes Jobs, HTTP requests, or shell
commands. The engine runs as a controller (K8s Job or Deployment) that
creates child K8s Jobs for heavy steps and executes lightweight steps
in-process.

## MVP Scope

### In scope

- YAML parsing with multiple workflows per file
- User-defined step types composed from engine primitives
- Engine primitives: `k8s_job`, `http_request`, `shell_exec`, `load_artifact`
- `for_each` fan-out with concurrency control (`forks`)
- Sub-workflow invocation (`workflow:` on a step)
- Artifact loading from files (YAML, markdown frontmatter)
- `when:` conditional execution
- `register:` variable capture and template interpolation
- Checkpoint/resume via state store (SQLite for dev, PostgreSQL for prod)
- Error handling: fail-fast with checkpointed state
- CLI: `run`, `resume`, `status`, `list`, `validate`
- Pongo2 for template rendering and expression evaluation

### Post-MVP

- Human gates (Jira ticket as approval mechanism, block until resolved)
- Rules engine (Grule integration for Drools-style routing) — MVP uses
  `when:` expressions on steps for conditional routing, which covers
  simple switch/case patterns. A dedicated rules engine only becomes
  necessary if TMS (forward chaining, automatic downstream invalidation)
  is needed.
- block/rescue/always error handling
- Retry strategies with backoff
- Dashboard / REST API
- Triggers (cron, webhook, event-driven)
- Argo Workflow export adapter
- TMS (Truth Maintenance System) propagation
- Multi-tenancy
- `llm_invoke` primitive (direct LLM API calls from the engine process)
- Dry-run with mocked step outputs

## Architecture

```
                        ┌─────────────────────────┐
                        │     markov      │
                        │      (Go binary)         │
                        │                          │
 YAML file ──parse──►   │  ┌───────────────────┐   │
                        │  │   Workflow Runner  │   │
                        │  │                    │   │
                        │  │  for each step:    │   │
                        │  │   evaluate when:   │   │
                        │  │   resolve type     │   │
                        │  │   render templates │   │
                        │  │   dispatch         │──────► K8s API (batch/v1 Jobs)
                        │  │   load artifacts   │   │
                        │  │   checkpoint       │   │
                        │  └───────────────────┘   │
                        │                          │
                        │  ┌───────────────────┐   │
                        │  │    State Store     │   │
                        │  │  SQLite / Postgres │   │
                        │  └───────────────────┘   │
                        └─────────────────────────┘
```

### Execution model

The engine is a single Go process. It walks the workflow steps
sequentially. Each step is dispatched to an executor based on its
resolved type:

| Primitive | Execution | Blocks on |
|-----------|-----------|-----------|
| `k8s_job` | Creates a `batch/v1 Job` via K8s client, polls status | Job completion |
| `http_request` | Makes HTTP call in-process | Response |
| `llm_invoke` | Calls LLM API in-process | Response |
| `shell_exec` | Runs command in-process via `os/exec` | Exit code |

For `for_each` steps, the engine evaluates the list expression,
creates a semaphore sized to `concurrency` (or the global `forks`
default), and dispatches items as goroutines. Each goroutine runs
either a single step or a full sub-workflow.

### K8s Job lifecycle

When the engine dispatches a `k8s_job` step:

1. Resolve the step type definition (image, command, args, volumes, etc.)
2. Render all templates against the current context
3. Build a `batch/v1 Job` manifest programmatically
4. Apply labels: `workflow=<name>`, `run=<run_id>`, `step=<step_name>`
5. Create the Job via K8s API
6. Poll Job status until succeeded, failed, or timeout
7. On success: load declared artifacts, store in context
8. On failure: mark step failed, halt workflow
9. On engine crash/restart: reconcile by checking existing Jobs by label

## YAML Schema

### Top-level fields

```yaml
entrypoint: <workflow-name>       # which workflow to start
namespace: <k8s-namespace>        # K8s namespace for all jobs
forks: 5                          # default concurrency for for_each steps
vars:                             # global variables
  key: value
step_types:                       # user-defined step types
  <name>:
    base: <primitive>
    defaults: {}
    job: {}                       # k8s_job config
    params: {}                    # http_request/other config
workflows:                        # list of workflow definitions
  - name: <name>
    vars: {}
    steps: []
```

### Step fields

```yaml
- name: <unique-name>             # required
  type: <primitive-or-step-type>  # required (unless for_each+workflow)
  params: {}                      # type-specific parameters
  when: "<expression>"            # skip step if false
  register: <var-name>            # store output in context
  timeout: 300                    # seconds, optional
  artifacts:                      # files to load after completion
    <key>:
      path: "<path>"
      format: yaml | markdown
      optional: false
  for_each: "<list-expression>"   # fan-out over a list
  as: <item-var>                  # variable name for current item
  concurrency: 10                 # override forks for this step
  workflow: <workflow-name>       # spawn sub-workflow per item
  vars: {}                       # vars to pass to sub-workflow
```

### Step type definition

```yaml
step_types:
  agent_skill:
    base: k8s_job
    description: "Run a Claude skill in a pipeline-agent container"
    defaults:
      model: "{{ model }}"
      runner: "{{ runner }}"
    job:
      image: pipeline-agent:latest
      command: ["/bin/bash", "/app/scripts/run_skill.sh"]
      args: ["--skill", "{{ skill }}", ...]
      namespace: ai-pipeline
      service_account: pipeline-dashboard
      resources: { ... }
      secrets: [...]
      volumes: [...]
      affinity: { ... }
```

Steps using `type: agent_skill` only need to pass params that differ
from the defaults. The engine merges: step type defaults → step params
(step params win).

## Error Handling (MVP)

### Step failure

A step fails when:
- `k8s_job`: Job status is Failed, or timeout exceeded
- `http_request`: non-2xx response (configurable) or timeout
- `llm_invoke`: API error or timeout
- `shell_exec`: non-zero exit code or timeout

On failure: the step is marked `failed` in the state store. The
workflow halts. No subsequent steps execute. The user can inspect
the failure and `resume` after fixing the problem.

### for_each failure

When an item in a `for_each` loop fails:
- In-flight items are allowed to complete (not cancelled)
- No new items are started
- The step is marked `failed` with partial results
- `register:` contains results for completed items plus error records
  for failed items
- On `resume`: only failed items are retried, completed items are skipped

### Timeout

Optional per-step `timeout:` in seconds.

- `k8s_job`: engine stops polling and deletes the Job
- `http_request`: context deadline on the HTTP client
- `shell_exec`: process killed via context cancellation
- `llm_invoke`: context deadline on the API call

Default: no timeout (engine waits indefinitely). For `k8s_job` steps,
the Job's own `activeDeadlineSeconds` can also enforce a timeout at
the K8s level.

### when: failure

If a `when:` expression evaluates to false, the step is marked
`skipped`, not `failed`. The workflow continues to the next step.

## State Store

### Interface

```go
type StateStore interface {
    CreateRun(ctx context.Context, run *Run) error
    UpdateRun(ctx context.Context, run *Run) error
    GetRun(ctx context.Context, runID string) (*Run, error)
    ListRuns(ctx context.Context) ([]*Run, error)

    SaveStep(ctx context.Context, runID string, step *StepResult) error
    GetSteps(ctx context.Context, runID string) ([]*StepResult, error)
}
```

### Schema

```sql
CREATE TABLE runs (
    run_id          TEXT PRIMARY KEY,
    workflow_file   TEXT NOT NULL,
    entrypoint      TEXT NOT NULL,
    status          TEXT NOT NULL,  -- running, completed, failed, halted
    vars_json       TEXT NOT NULL,
    parent_run_id   TEXT,           -- null for top-level, set for sub-workflows
    parent_step     TEXT,           -- which for_each step spawned this
    for_each_key    TEXT,           -- the item key for this sub-workflow
    started_at      TIMESTAMP NOT NULL,
    completed_at    TIMESTAMP
);

CREATE TABLE steps (
    run_id          TEXT NOT NULL,
    workflow_name   TEXT NOT NULL,
    step_name       TEXT NOT NULL,
    status          TEXT NOT NULL,  -- pending, running, completed, failed, skipped
    output_json     TEXT,           -- registered output
    artifacts_json  TEXT,           -- loaded artifacts
    error           TEXT,
    started_at      TIMESTAMP,
    completed_at    TIMESTAMP,
    PRIMARY KEY (run_id, workflow_name, step_name)
);
```

### Resume flow

```
markov resume <run_id>
```

1. Load the run record and all step records from the state store
2. Rebuild the context by replaying all completed steps' outputs
3. Find the first non-completed step (failed or pending)
4. Continue execution from that step
5. For `for_each` steps: load completed items, retry only failed items

## CLI

```
markov run <file.yaml> [flags]
    --var key=value          Override top-level vars (repeatable)
    --workflow <name>        Run a specific workflow instead of entrypoint
    --forks <n>              Override global forks
    --state-store <path>     SQLite path (default: ./workflow-state.db)
    --namespace <name>       Override top-level namespace
    --kubeconfig <path>      K8s config (default: in-cluster or ~/.kube/config)
    --dry-run                Validate and print execution plan, don't execute

markov resume <run_id> [flags]
    --state-store <path>

markov status <run_id> [flags]
    --state-store <path>
    --steps                  Show individual step statuses

markov validate <file.yaml>
    Parse, resolve step types, check workflow references,
    validate expressions. Exit 0 if valid, non-zero with errors.
```

## Go Project Structure

```
markov/
├── cmd/
│   └── markov/
│       └── main.go              # CLI entrypoint (cobra)
├── pkg/
│   ├── parser/
│   │   └── parser.go           # YAML parsing, step type resolution
│   ├── engine/
│   │   ├── engine.go           # Workflow runner, step dispatch loop
│   │   ├── context.go          # Variable context, template rendering
│   │   └── foreach.go          # for_each fan-out with semaphore
│   ├── executor/
│   │   ├── executor.go         # Executor interface
│   │   ├── k8s_job.go          # K8s Job executor
│   │   ├── http_request.go     # HTTP request executor
│   │   ├── llm_invoke.go       # LLM API executor
│   │   └── shell_exec.go       # Shell command executor
│   ├── state/
│   │   ├── store.go            # StateStore interface
│   │   ├── sqlite.go           # SQLite implementation
│   │   └── postgres.go         # PostgreSQL implementation (post-MVP)
│   └── template/
│       └── pongo2.go           # Pongo2 template + expression evaluation
├── examples/
│   ├── batch-rfe-pipeline.yaml
│   └── artifact-conditions.yaml
├── docs/
│   ├── mvp-design.md
│   └── research-summary.md
├── go.mod
└── go.sum
```

## Key Dependencies

| Dependency | Purpose |
|-----------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/flosch/pongo2` | Jinja2-compatible template engine |
| `k8s.io/client-go` | K8s API client |
| `gopkg.in/yaml.v3` | YAML parsing |
| `modernc.org/sqlite` | SQLite driver (pure Go, no CGO) |
| `github.com/google/uuid` | Run ID generation |
