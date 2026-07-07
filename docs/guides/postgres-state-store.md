# Using Postgres for Markov State

Markov can store workflow checkpoints in Postgres instead of a local SQLite file. Use Postgres when runs are started by Kubernetes Jobs, orchestrators, or multiple hosts that need a shared durable state store for `status`, `list`, `diagram`, and `resume`.

SQLite remains the simplest option for local development and single-process runs. Postgres is the better default for production-style Kubernetes execution because the state survives pod deletion and can be read by separate resume or inspection commands.

## What Markov Stores

The state store contains only Markov checkpoint data:

- Run identity, source path, entrypoint, variables, status, and timestamps.
- Step status, output JSON, artifact JSON, errors, and timestamps.
- Parent/child run relationships for sub-workflows and `for_each`.

It does not store workflow source files. Resume still requires the original workflow file or directory to exist at the same path recorded when the run started.

## Selecting Postgres

Markov selects the backend from the state store value:

| Value | Backend |
|-------|---------|
| `./markov-state.db` | SQLite |
| `/data/markov-state.db` | SQLite |
| `postgres://user:pass@host:5432/db?sslmode=disable` | Postgres |
| `postgresql://user:pass@host:5432/db?sslmode=disable` | Postgres |

State store precedence is:

1. `--state-store <path-or-dsn>`
2. `MARKOV_STATE_STORE`
3. In-cluster default: `/tmp/markov-state.db`
4. Local default: `./markov-state.db`

For automation, prefer `MARKOV_STATE_STORE` so every Markov command uses the same backend without repeating the DSN:

```bash
export MARKOV_STATE_STORE='postgres://markov:markov@localhost:5432/markov_state?sslmode=disable'

markov run workflows/deploy.yaml --run-id deploy-001
markov status deploy-001 --steps
markov resume deploy-001
```

Use `--state-store` when a command needs to override the environment:

```bash
markov list --state-store 'postgres://markov:markov@db.example.com:5432/markov_state?sslmode=require'
```

## Local Postgres Quickstart

Start a disposable Postgres container:

```bash
docker run --rm --name markov-postgres \
  -e POSTGRES_USER=markov \
  -e POSTGRES_PASSWORD=markov \
  -e POSTGRES_DB=markov_state \
  -p 5432:5432 \
  postgres:16
```

In another shell:

```bash
export MARKOV_STATE_STORE='postgres://markov:markov@localhost:5432/markov_state?sslmode=disable'

markov run examples/simple-test.yaml --run-id pg-demo-001
markov status pg-demo-001 --steps
markov list
```

Markov creates its checkpoint tables automatically on first connection.

To inspect the tables directly:

```bash
docker exec -it markov-postgres \
  psql -U markov -d markov_state
```

Useful SQL:

```sql
select run_id, entrypoint, status, started_at, completed_at
from runs
order by started_at desc;

select workflow_name, step_name, status, error
from steps
where run_id = 'pg-demo-001'
order by started_at;
```

## Kubernetes Setup

For Kubernetes, pass the DSN through a Secret and set `MARKOV_STATE_STORE` in the Markov runner container.

Create the Secret:

```bash
kubectl create secret generic markov-state \
  --from-literal=dsn='postgres://markov:REDACTED@postgres.example.svc.cluster.local:5432/markov_state?sslmode=require'
```

Use it in the Job or controller that starts Markov:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: markov-run-example
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: markov
          image: markov:latest
          command:
            - markov
            - run
            - /etc/markov/workflow
            - --run-id
            - markov-run-example
          env:
            - name: MARKOV_STATE_STORE
              valueFrom:
                secretKeyRef:
                  name: markov-state
                  key: dsn
          volumeMounts:
            - name: workflow
              mountPath: /etc/markov/workflow
              readOnly: true
      volumes:
        - name: workflow
          configMap:
            name: markov-workflow
```

Now a separate inspection or resume pod can use the same Secret:

```bash
markov status markov-run-example --steps
markov resume markov-run-example
```

Those commands must also have the workflow source mounted at the same path when resuming.

## DSN Guidance

Common DSN forms:

```text
postgres://markov:password@postgres:5432/markov_state?sslmode=disable
postgres://markov:password@postgres:5432/markov_state?sslmode=require
postgresql://markov:password@postgres:5432/markov_state?sslmode=verify-full
```

Use `sslmode=require` or stronger for networked production databases. Use `sslmode=disable` only for local development or trusted in-cluster networks where transport security is handled separately.

If the password contains special characters, URL-encode it. For example, `p@ss/word` becomes `p%40ss%2Fword`.

## Permissions

Markov currently creates and updates its own tables. The database user needs permission to:

- Connect to the database.
- Create tables and indexes during initialization.
- Select, insert, update, and delete rows in Markov-owned tables.

For a dedicated database:

```sql
create database markov_state;
create user markov with password 'REDACTED';
grant all privileges on database markov_state to markov;
```

For stricter installations, create a dedicated schema and grant Markov ownership of that schema. Keep Markov checkpoint tables separate from dashboard, application, or analytics tables.

## Resume Requirements

Postgres solves shared durable checkpoint storage, but resume still depends on workflow stability:

- Use the same `run_id`.
- Keep workflow and step names stable.
- Keep `for_each_key` stable for production fan-outs.
- Mount the original workflow file or directory at the same path recorded by the first run.
- Preserve custom step types, `vars.yaml`, `rules.yaml`, and workflow files when using directory workflows.

Example:

```bash
export MARKOV_STATE_STORE='postgres://markov:markov@postgres:5432/markov_state?sslmode=require'

markov run /etc/markov/workflow --run-id release-2026-07-07

# After fixing an external issue or workflow bug:
markov resume release-2026-07-07
```

## Operational Checks

Check recent runs:

```bash
markov list
```

Check one run and its steps:

```bash
markov status release-2026-07-07 --steps
```

Generate a run diagram:

```bash
markov diagram release-2026-07-07
```

Inspect directly in SQL:

```sql
select run_id, status, started_at, completed_at
from runs
order by started_at desc
limit 20;

select run_id, workflow_name, step_name, status, left(error, 200) as error
from steps
where status = 'failed'
order by completed_at desc
limit 20;
```

## Troubleshooting

### Markov still writes `/tmp/markov-state.db`

The command did not receive `MARKOV_STATE_STORE` and did not pass `--state-store`. Check the pod environment:

```bash
kubectl exec <pod> -- env | grep MARKOV_STATE_STORE
```

Also check command arguments. An explicit `--state-store` always wins over the environment variable.

### `run_id` not found

The inspection command is pointed at a different state store than the original run, or the run used local SQLite. Make sure `MARKOV_STATE_STORE` is identical for `run`, `status`, and `resume`.

### Resume fails because the workflow file is missing

The state store has the checkpoint, but the workflow source path recorded in the run is not available. Mount the workflow at the same path or run resume from an environment where that path exists.

### Connection or authentication errors

Validate the DSN from the same network location as the Markov process:

```bash
psql 'postgres://markov:REDACTED@postgres:5432/markov_state?sslmode=require' -c 'select 1'
```

In Kubernetes, check DNS, NetworkPolicy, Secret values, and whether the database requires TLS.

### Help output hides the password

When `MARKOV_STATE_STORE` is set to a Postgres DSN, Markov redacts credentials in debug/help-style output. This does not change the runtime value used to connect.

## Backup and Retention

Treat the state database as operational recovery data:

- Back it up if resume history matters.
- Retain it at least as long as failed runs may need to be resumed.
- Prune or archive old completed runs according to local policy.
- Avoid mixing ephemeral test runs and production runs in the same database unless retention rules account for both.

There is no built-in pruning command yet, so retention is currently handled outside Markov.

## When to Use SQLite Instead

SQLite is still appropriate when:

- You are developing locally.
- A single process owns the run lifecycle.
- You do not need resume after pod deletion.
- You want the fewest external dependencies.

Use Postgres when:

- Runs start in short-lived Kubernetes pods.
- Status, resume, or diagram commands run from a different pod or host.
- Multiple concurrent runs need shared durable state.
- You need backups, centralized inspection, or operational audit history.

## Related Docs

- [State Store Reference](../reference/state-store.md)
- [Resuming Workflows](resuming-workflows.md)
- [Running in Kubernetes](k8s-deployment.md)
