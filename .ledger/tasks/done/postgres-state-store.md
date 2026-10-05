# Task: Add Postgres state store support

## Status

Done

## Summary

Add a native Postgres implementation of Markov's checkpoint/resume state store
so orchestrators such as markovd can run Markov jobs with durable shared state
without relying on SQLite files mounted from Kubernetes PVCs.

## Motivation

Markov currently persists checkpoint and resume data through
`pkg/state.SQLiteStore`, selected by `--state-store <path>`. This works for
local development and simple single-writer deployments, but it is awkward for
markovd-managed Kubernetes runs:

- SQLite on a PVC depends on storage-class locking behavior.
- A shared SQLite state file is a poor fit for many concurrent workflow pods.
- One SQLite database per run complicates cleanup and resume orchestration.
- markovd already has access to Postgres and can pass database credentials to
  spawned Markov pods through Kubernetes Secrets.

The desired production path is for Markov to support Postgres as a first-class
state backend while preserving the existing `state.Store` interface and resume
semantics.

## Expected Use Case

markovd starts a workflow in Kubernetes and passes a Postgres-backed state store
to the Markov container:

```bash
markov run /etc/markov/workflow \
  --run-id markov-run-abc123 \
  --state-store postgres://markov:...@postgres:5432/markov_state?sslmode=disable
```

If the run fails, markovd can later start a resume job using the same state
backend:

```bash
markov resume markov-run-abc123 \
  --state-store postgres://markov:...@postgres:5432/markov_state?sslmode=disable
```

The resumed Markov process reloads the original workflow source path recorded
in the Postgres state store, replays completed steps, skips completed work, and
continues from failed, pending, running, or missing steps.

## Requirements

- Add a Postgres-backed implementation of `pkg/state.Store`.
- Preserve the existing SQLite backend and its current CLI behavior.
- Select the backend from CLI input without breaking existing users:
  - `--state-store ./markov-state.db` continues to mean SQLite.
  - `--state-store /path/to/markov-state.db` continues to mean SQLite.
  - `--state-store postgres://...` selects Postgres.
  - Consider accepting `postgresql://...` as an alias.
- Keep checkpoint/resume semantics identical across SQLite and Postgres:
  - run records are keyed by `run_id`
  - step records are keyed by `(run_id, workflow_name, step_name)`
  - completed steps are replayed and skipped on resume
  - child runs are queryable by `parent_run_id`
  - `for_each_key` remains stable for resumable fan-outs
- Make schema creation or migration deterministic and idempotent.
- Avoid coupling Postgres state tables to markovd application tables.
  Markov should own its checkpoint schema even when sharing the same Postgres
  server.
- Keep credentials out of logs and debug output.
- Document production deployment expectations for orchestrators:
  - pass the DSN through an environment variable or Kubernetes Secret
  - use a dedicated database, schema, or user for Markov state
  - ensure the original workflow source path is available to resume jobs

## Design Notes

The existing `state.Store` interface is small enough to support a second
backend directly:

- `CreateRun`
- `UpdateRun`
- `GetRun`
- `ListRuns`
- `GetChildRuns`
- `SaveStep`
- `GetSteps`
- `GetStep`

The current SQLite schema in `pkg/state/sqlite.go` can be translated directly
to Postgres. The main SQL differences are placeholder syntax, timestamp types,
and upsert syntax. Postgres can use `ON CONFLICT` for `SaveStep`, matching the
current SQLite behavior.

Prefer adding a backend constructor such as `state.OpenStore(stateStore string)`
or `state.NewStore(stateStore string)` so CLI commands do not need to know
which concrete store they are opening.

## Non-Goals

- Do not remove SQLite support.
- Do not require markovd to manage Markov's internal checkpoint tables.
- Do not implement a psql protocol proxy or passthrough layer.
- Do not change workflow resume matching rules.
- Do not solve workflow source snapshotting in this task. Markov should still
  record and reload `workflow_file` as it does today; orchestrators must ensure
  the source path is available for resume.

## Implementation Plan

1. Add a Postgres driver dependency that fits the existing Go module.
2. Introduce a store factory that chooses SQLite or Postgres from the
   `--state-store` value.
3. Implement `PostgresStore` in `pkg/state/`.
4. Add Postgres schema creation for the existing `runs` and `steps` tables.
5. Update CLI commands to use the store factory for `run`, `resume`, `status`,
   `list`, and `diagram`.
6. Add backend-neutral state store tests that can run against SQLite and
   Postgres.
7. Add Postgres integration tests gated by an environment variable, for
   example `MARKOV_TEST_POSTGRES_DSN`.
8. Update `docs/reference/state-store.md`,
   `docs/reference/cli.md`, and `docs/guides/resuming-workflows.md`.
9. Update Kubernetes deployment guidance with the Postgres DSN Secret pattern.

## Acceptance Criteria

- `markov run` and `markov resume` continue to work with the default SQLite
  state store.
- `markov run --state-store postgres://...` creates run and step checkpoint
  rows in Postgres.
- `markov resume <run-id> --state-store postgres://...` resumes a failed run
  using the Postgres checkpoint records.
- `markov status`, `markov list`, and `markov diagram` work against both
  SQLite and Postgres stores.
- Tests cover CRUD behavior for runs, steps, child runs, and idempotent
  `SaveStep` upserts.
- Docs explain when to use SQLite versus Postgres and how orchestrators should
  pass Postgres credentials safely.

## Verification

- `GOCACHE=/tmp/go-build go test ./pkg/state` passed with SQLite and skipped
  Postgres when `MARKOV_TEST_POSTGRES_DSN` was unset.
- `MARKOV_TEST_POSTGRES_DSN='postgres://markov:markov@127.0.0.1:55432/markov_state?sslmode=disable' GOCACHE=/tmp/go-build go test ./pkg/state`
  passed against a temporary `postgres:16` container.
- `GOCACHE=/tmp/go-build go test ./pkg/state ./cmd/markov ./pkg/engine ./pkg/parser`
  passed.
- `GOCACHE=/tmp/go-build make build` passed.
- Verified `markov run`, `markov status --steps`, `markov list`, and
  `markov diagram` against the temporary Postgres store.
- Verified `markov resume` against the temporary Postgres store by running a
  failing workflow, editing the stored source path, and resuming the same run
  ID. The completed first step was skipped and the failed/later steps ran.
- Verified SQLite `markov run`, `markov resume`, `markov status --steps`,
  `markov list`, and `markov diagram` using an explicit temporary SQLite state
  store path.
