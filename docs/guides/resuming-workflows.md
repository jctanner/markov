{% raw %}
# Resuming Workflows

Markov checkpoints every run and step in a state store. SQLite is the default; Postgres can be selected with a `postgres://` or `postgresql://` DSN for durable shared state. When a run fails or pauses, `markov resume` reloads the workflow source, rebuilds context from completed steps, skips completed work, and continues from the first incomplete, failed, or paused step.

Use resume for failures that are safe to retry after an external fix: a missing secret, an unavailable API, a transient Kubernetes error, or a bad variable value. Source edits are governed by the source-integrity policy described below.

## Basic Flow

Run with a durable state store:

```bash
markov run pipeline.yaml --state-store /data/markov-state.db
```

If the run fails, inspect it:

```bash
markov status <run-id> --steps --state-store /data/markov-state.db
```

Fix the underlying problem, then resume:

```bash
markov resume <run-id> --state-store /data/markov-state.db
```

For repeatable demos or automation, choose the run ID up front:

```bash
markov run pipeline.yaml \
  --run-id demo-reset-001 \
  --state-store /data/markov-state.db

markov resume demo-reset-001 --state-store /data/markov-state.db
```

## Resuming a paused gate

A gate with `action: pause` stops the workflow safely instead of falling through. The run and gate step are both recorded as `paused`; the gate step's output is an audit receipt containing its action, fired rules, and facts.

Inspect the receipt, then resume with an explicit decision input. A paused run without `--var` input is rejected.

```bash
markov status <run-id> --steps --state-store /data/markov-state.db
markov resume <run-id> --var approved=true --state-store /data/markov-state.db
```

The variable name is defined by the workflow's rules. For example, a rule can pause while `approved != true`; setting `approved=true` lets the gate re-evaluate to `continue`. The paused gate is intentionally re-executed, while preceding completed steps remain skipped.

Run [the pause/resume example](../../examples/pause-resume.yaml) to see the complete workflow definition.

For orchestrated Kubernetes runs, use Postgres through an environment variable or Secret-backed value:

```bash
export MARKOV_STATE_STORE='postgres://markov:...@postgres:5432/markov_state?sslmode=disable'

markov run /etc/markov/workflow \
  --run-id markov-run-abc123

markov resume markov-run-abc123
```

## What Resume Needs

Resume depends on three pieces of state:

| Requirement | Why it matters |
|-------------|----------------|
| Same state store | The run and step checkpoints are looked up by `run_id`. |
| Original source path still exists | The run record stores the workflow file or directory path and reloads it on resume. |
| Stable workflow and step names | Completed steps are matched by `(run_id, workflow_name, step_name)`. |

For directory workflows, the stored source path is the workflow directory. On resume, Markov reloads that directory, including `meta.yaml`, `vars.yaml`, `rules.yaml`, `step_types.yaml` or `step_types/`, and all `workflows/*.yaml` files.

## Source integrity modes

At the initial run, Markov records a SHA-256 digest of the complete workflow
source tree. For a directory workflow that is the supplied directory; for a
single YAML file it is the containing directory. The digest covers every regular
file by relative path, contents, and executable mode, as well as symlink targets.
This includes `scripts/` used by `script_exec`. Git metadata and the configured
local state-store files are excluded.

Before resuming, Markov calculates the tree digest again. Choose the policy that
fits the run:

| Mode | Command | Behavior on source change |
|------|---------|---------------------------|
| `warn` | `markov resume <run-id>` | Default. Continue using the changed source, log the drift, store it in run history, and include it in the `run_resumed` callback. Best for interactive iteration. |
| `strict` | `markov resume <run-id> --source-integrity strict` | Reject the resume with both digests. Restore the original source tree or start a new run. Use in CI and production automation. |
| `off` | `markov resume <run-id> --source-integrity off` | Do not calculate or compare a digest. Use only for compatibility situations. |

`markov status <run-id>` shows the original digest and the latest resume check.
Markov keeps the original digest immutable and records every resume check; a
drifted resume never silently rewrites the run's original source identity.

## What Gets Skipped or Re-run

On resume:

- Steps saved as `completed` are skipped.
- Completed `set_fact` steps replay their facts into the context.
- Completed `gate` steps replay their output facts into the context.
- Completed steps with `register` output restore that output for downstream templates.
- Paused, failed, running, missing, or pending steps run again.
- `when` conditions for non-completed steps are evaluated again against the replayed context.

Resume is step-boundary recovery. If a step started an external side effect and then failed before Markov saved it as completed, that step will run again. Design steps that may be retried to be idempotent where possible.

## Editing a workflow before resume

Use `strict` for runs whose definition must remain reproducible. For local
iteration, the default `warn` mode deliberately permits quick source changes and
keeps completed checkpoints, but marks that resume attempt as drifted. Be
especially careful when renaming completed workflows or steps, removing a
completed `register`, `set_fact`, or artifact-producing step, or reordering a
list-based fan-out without stable keys: source-integrity tracking makes the
change visible, but it cannot make an incompatible checkpoint safe.

## Fan-Out Resume

For production fan-outs, prefer sub-workflow fan-out with `for_each_key`:

```yaml
steps:
  - name: process_repos
    for_each: "{{ repos }}"
    for_each_key: "name"
    for_each_sort: "name"
    as: repo
    workflow: import-repo
    vars:
      repo_name: "{{ repo.name }}"
      upstream: "{{ repo.upstream }}"
```

Each item gets a child run keyed by `for_each_key`. On resume, completed child runs are skipped and failed or missing child runs are retried. Stable keys keep resume correct even if the list order changes.

Without `for_each_key`, Markov falls back to array indices. If the input list changes order between failure and resume, index-based keys may point at different items.

Direct typed `for_each` steps are useful for simple loops, but sub-workflow fan-out is the safer production pattern for resumable multi-item work.

## Inspecting Resume State

Use the CLI first:

```bash
markov list --state-store /data/markov-state.db
markov status <run-id> --steps --state-store /data/markov-state.db
```

For deeper inspection, query SQLite directly:

```bash
sqlite3 /data/markov-state.db \
  "SELECT run_id, entrypoint, status FROM runs ORDER BY started_at DESC"

sqlite3 /data/markov-state.db \
  "SELECT workflow_name, step_name, status, error FROM steps WHERE run_id='<run-id>'"

sqlite3 /data/markov-state.db \
  "SELECT run_id, for_each_key, status FROM runs WHERE parent_run_id='<run-id>'"
```

## Kubernetes Notes

When Markov runs in-cluster, the default SQLite state store path is `/tmp/markov-state.db`. That path is writable, but it is usually ephemeral. To resume across pod restarts, use Postgres or mount persistent storage and pass an explicit state store path.

For a complete Postgres setup guide, see [Using Postgres for Markov State](postgres-state-store.md).

Postgres DSNs should be passed through Kubernetes Secrets:

```bash
markov run /etc/markov/pipeline
```

For SQLite on a PersistentVolumeClaim:

```bash
markov run /etc/markov/pipeline \
  --state-store /data/markov-state.db

markov resume <run-id> \
  --state-store /data/markov-state.db
```

The workflow source path stored in the run must also be available when resuming. For directory workflows mounted into a container, mount the same directory path for both the original run and the resume command.

## Troubleshooting

### `run not found`

The `run_id` is not present in the selected state store. Check the run ID and the `--state-store` path.

### `workflow "<name>" not found`

The workflow source changed or the stored path now points at a different file or directory. Restore the workflow with the same `name:` that was used by the original run.

### A completed step ran again

Completed steps are matched by workflow name and step name. Check whether either name changed, whether a different state store was used, or whether the previous step record was not saved as `completed`.

### A fan-out item resumed against the wrong input

Use `for_each_key` with a stable unique item field and avoid relying on array indices for resumable production fan-outs.

## Related References

- [State Store Reference](../reference/state-store.md) — schema and detailed replay behavior
- [CLI Reference](../reference/cli.md) — `run`, `resume`, `status`, and `list`
- [Fan-Out Patterns](fan-out-patterns.md) — stable keys and sub-workflow fan-out
{% endraw %}
