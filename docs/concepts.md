# Core Concepts

This page explains the mental model behind markov. For field-by-field details, see the [Reference](reference/workflow-file.md) section.

## Workflow Files

A markov workflow file is a single YAML document that defines one or more workflows. One workflow is designated as the `entrypoint` — it runs first when you execute the file.

## Steps

Each workflow contains an ordered list of steps. Steps execute sequentially — each step waits for the previous one to complete before starting. There is no parallel execution between steps within the same workflow; parallelism comes from `for_each`.

## Step Types

Every step has a `type` that determines what it does. Markov provides these primitives:

| Type | Purpose |
|------|---------|
| `shell_exec` | Run a shell command |
| `k8s_job` | Create and run a Kubernetes Job |
| `http_request` | Make an HTTP request |
| `gate` | Evaluate rules and control flow |
| `set_fact` | Compute and store variables |
| `load_artifact` | Load files into the context |
| `assert` | Validate conditions or fail |

You can also define custom step types (`step_types`) that wrap a primitive with preset defaults.

## Context

The context is a map of variables that flows through execution. Every workflow runs with a context built from:

1. **Global vars** — defined at the top of the file
2. **Workflow vars** — defined on the workflow (non-nil values override globals)
3. **CLI vars** — passed via `--var key=value` (highest priority)

Steps can add to the context via `register` (stores step output), `set_fact` (computes values), or gate `set_fact` (sets values when rules fire).

## Template Rendering

All string values in `params`, `vars`, `when`, `facts`, artifact `path`, and `msg` fields are rendered through a Jinja2-compatible template engine (Pongo2). Use `{{ variable }}` to interpolate values and `{{ value | filter }}` to apply filters.

## Conditional Execution

The `when` field on any step takes a boolean expression. If it evaluates to false, the step is skipped — not failed. Skipped steps don't block downstream execution.

## Fan-Out and Concurrency

`for_each` iterates over a list, running a step or sub-workflow per item with concurrency control. The `forks` setting (default 5) limits how many items execute simultaneously. Each iteration gets its own copy of the context plus the current item.

## Sub-Workflows

A step can invoke another workflow by name using the `workflow` field instead of `type`. Sub-workflows get a copy of the parent context plus any `vars` declared on the step. Changes inside a sub-workflow don't propagate back to the parent unless captured with `register`.

## Recursive Workflows

Workflows can call themselves. Combined with gate steps, this enables looping: a gate evaluates rules against the current state and decides whether to continue, skip, or pause. This follows the Markov chain principle — each iteration's next move depends only on the current state.

## Rule Engine and Gates

Rules are defined at the top of the file and evaluated by gate steps. Each rule has a condition (`when`), a priority (`salience`), an action (`continue`, `skip`, or `pause`), and optional side effects (`set_fact`). The Grule rule engine evaluates rules with forward chaining — when a rule fires and sets facts, remaining rules re-evaluate against the updated state.

## Checkpoint and Resume

Every step result is persisted to a SQLite database. If a workflow fails, `markov resume <run-id>` replays the context from completed steps and continues from the first incomplete step. For `for_each`, only failed or missing items are retried.

## Callbacks

Markov fires events at key lifecycle points (run started, step completed, gate evaluated, etc.) to external systems via HTTP, gRPC, or JSONL file backends. Callback errors are logged but never fail the workflow.
