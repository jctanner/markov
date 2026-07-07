# State Store

Markov stores checkpoint/resume data in a state backend. SQLite is the default for local use and simple deployments. Postgres is available for orchestrators and concurrent Kubernetes runs that need durable shared state.

For practical recovery steps and operational caveats, see [Resuming Workflows](../guides/resuming-workflows.md).

## Backend selection

The state store value selects the backend:

| Value | Backend |
|-------|---------|
| `./markov-state.db` | SQLite |
| `/path/to/markov-state.db` | SQLite |
| `postgres://user:pass@host:5432/db?sslmode=disable` | Postgres |
| `postgresql://user:pass@host:5432/db?sslmode=disable` | Postgres |

SQLite uses pure Go SQLite via `modernc.org/sqlite` and does not require CGO. Postgres uses a native Go driver and creates Markov-owned checkpoint tables in the selected database.

## Default location and precedence

Markov chooses the state store in this order:

| Priority | Source | Description |
|----------|--------|-------------|
| 1 | `--state-store <path-or-dsn>` | Explicit CLI flag on stateful commands. |
| 2 | `MARKOV_STATE_STORE` | Environment variable containing a SQLite path or Postgres DSN. |
| 3 | In-cluster default | `/tmp/markov-state.db` when a Kubernetes ServiceAccount token is mounted. |
| 4 | Local default | `./markov-state.db`. |

### Default SQLite location

| Environment | Path | Detection |
|-------------|------|-----------|
| Local | `./markov-state.db` | Default |
| In-cluster (K8s) | `/tmp/markov-state.db` | ServiceAccount token exists at `/var/run/secrets/kubernetes.io/serviceaccount/token` |
| Custom | Any path or DSN | `MARKOV_STATE_STORE` or `--state-store <path-or-dsn>` |

## Postgres usage

Use Postgres when multiple workflow pods need a shared durable checkpoint store:

```bash
export MARKOV_STATE_STORE='postgres://markov:...@postgres:5432/markov_state?sslmode=disable'

markov run /etc/markov/workflow \
  --run-id markov-run-abc123

markov resume markov-run-abc123
```

Recommended production practice:

- Pass the DSN through an environment variable sourced from a Kubernetes Secret.
- Use a dedicated database, schema, or database user for Markov checkpoint state.
- Keep Markov checkpoint tables separate from orchestrator application tables.
- Ensure the original workflow source path is mounted at the same path for resume jobs.
- Avoid putting Postgres credentials directly in shell history or logs.

## Schema

### runs table

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `run_id` | TEXT | PRIMARY KEY | Unique run identifier (UUID prefix or `{parentRunID}-{stepName}-{key}`) |
| `workflow_file` | TEXT | NOT NULL | Source YAML file path |
| `entrypoint` | TEXT | NOT NULL | Workflow name that was executed |
| `status` | TEXT | NOT NULL | `running`, `completed`, or `failed` |
| `vars_json` | TEXT | NOT NULL, DEFAULT '{}' | Serialized run context variables |
| `parent_run_id` | TEXT | NULL | Set for sub-workflow and for_each runs |
| `parent_step` | TEXT | NULL | Step name that spawned this sub-run |
| `for_each_key` | TEXT | NULL | Iteration key for for_each sub-runs |
| `started_at` | TIMESTAMP | NOT NULL | When the run began |
| `completed_at` | TIMESTAMP | NULL | When the run finished (NULL while running) |

### steps table

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `run_id` | TEXT | NOT NULL | Parent run identifier |
| `workflow_name` | TEXT | NOT NULL | Workflow containing this step |
| `step_name` | TEXT | NOT NULL | Step name |
| `status` | TEXT | NOT NULL | `pending`, `running`, `completed`, `failed`, or `skipped` |
| `output_json` | TEXT | NULL | Step output as serialized JSON |
| `artifacts_json` | TEXT | NULL | Artifact data as serialized JSON |
| `error` | TEXT | NULL | Error message if step failed |
| `started_at` | TIMESTAMP | NULL | When the step began |
| `completed_at` | TIMESTAMP | NULL | When the step finished |

Primary key: `(run_id, workflow_name, step_name)`

## Statuses

### Run statuses

```
running --> completed
        \-> failed
```

### Step statuses

```
pending --> running --> completed
                    \-> failed
                    \-> skipped
```

A step is `skipped` when its `when` condition evaluates to false.

## Resume flow

Resume picks up a failed run and re-executes it, skipping already-completed steps.

```
markov resume <run_id>
```

### Step-by-step resume process

1. **Load run record** from state store by `run_id`.
2. **Load all steps** for the run.
3. **Replay completed step outputs** into the context:

   | Step type | Replay behavior |
   |-----------|----------------|
   | `set_fact` | Merge facts directly into context (`ctx[key] = value`) |
   | `gate` | Extract `facts` from output and merge into context |
   | Steps with artifacts | Restore full step data to `ctx[stepname]` |
   | Steps with output | Restore output map to `ctx[stepname]` |

4. **Mark run as "running"** again.
5. **Fire `run_resumed` event** with completed/remaining step counts.
6. **Continue execution** -- `executeWorkflow` runs all steps in order; `executeStep` skips already-completed steps.

### Context replay example

If a workflow ran steps A (set_fact), B (shell_exec), and C (failed):

```
Resume:
  1. Replay A's facts into ctx      --> ctx["region"] = "us-east-1"
  2. Replay B's output into ctx     --> ctx["B"] = {"stdout": "...", "rc": 0}
  3. Execute step A -> skip (completed)
  4. Execute step B -> skip (completed)
  5. Execute step C -> run (was failed)
```

## Step deduplication

`executeStep` checks if a step is already completed before running it:

1. Query state store for `(run_id, workflow_name, step_name)`.
2. If status is `completed`:
   - Replay output into context (same rules as resume replay).
   - Log skip and return nil.
3. Otherwise, execute the step normally.

This makes `executeWorkflow` safe to call on a partially-completed workflow -- it simply fast-forwards through completed steps.

## for_each resume

Sub-runs created by `for_each` are matched by `for_each_key`:

1. Each sub-run is stored with `parent_run_id`, `parent_step`, and `for_each_key`.
2. Sub-run IDs follow the pattern: `{parentRunID}-{stepName}-{key}`.
3. On resume, completed sub-runs are skipped via the normal step deduplication logic.
4. Failed or missing sub-runs are retried.

This is why `for_each_key` is important for production workflows -- without it, keys default to array indices, which can change if the input list is modified between runs.

## Step save behavior

Steps are saved using `INSERT ... ON CONFLICT DO UPDATE` (upsert):

```sql
INSERT INTO steps (run_id, workflow_name, step_name, status, ...)
VALUES (?, ?, ?, ?, ...)
ON CONFLICT (run_id, workflow_name, step_name) DO UPDATE SET
    status = excluded.status,
    output_json = excluded.output_json,
    artifacts_json = excluded.artifacts_json,
    error = excluded.error,
    started_at = excluded.started_at,
    completed_at = excluded.completed_at
```

This means:

- A step can be re-saved safely on resume without manual cleanup.
- A failed step that is retried will have its record updated in place.

## Inspecting state

### Show run status

```bash
markov status <run_id>
```

### Show run status with step details

```bash
markov status <run_id> --steps
```

Output:

```
Run:        a1b2c3d4
Workflow:   deploy-pipeline
Status:     failed
Started:    2025-01-15 10:30:00

STEP                           STATUS       DURATION
----                           ------       --------
fetch-config                   completed    1.2s
validate-input                 completed    0.1s
deploy-service                 failed       45.3s
  error: container image pull failed
```

### List all runs

```bash
markov list
```

Output:

```
RUN ID     WORKFLOW                  STATUS       STARTED              DURATION
------     --------                  ------       -------              --------
a1b2c3d4   deploy-pipeline           failed       2025-01-15 10:30:00  46.6s
e5f6g7h8   health-check              completed    2025-01-15 10:25:00  3.2s
```

## Parent-child relationships

The state store tracks run hierarchy through `parent_run_id` and `parent_step`:

```
Top-level run (a1b2c3d4)
  |
  +-- Sub-run (a1b2c3d4-deploy-api)        [for_each_key: "api"]
  +-- Sub-run (a1b2c3d4-deploy-worker)     [for_each_key: "worker"]
  +-- Sub-run (a1b2c3d4-deploy-frontend)   [for_each_key: "frontend"]
```

Child runs can be queried with `GetChildRuns(parentRunID)`.

## Diagram generation

Generate a Mermaid diagram of a completed run and its sub-runs:

```bash
markov diagram <run_id>
```

This walks the run tree (parent and children) and produces a Mermaid-formatted output showing the execution flow.
