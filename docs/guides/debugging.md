{% raw %}
# Debugging Workflows

This guide covers tools and techniques for investigating workflow problems.

## Verbose Output

The `--verbose` flag shows parameter values, template resolutions, and gate evaluations:

```bash
markov run pipeline.yaml --verbose
```

Output includes:
- Step parameters after template rendering
- Register variable values
- Gate rule compilation and evaluation
- Artifact loading details
- K8s job names and images

## Debug Logging

The `--debug` flag enables deeper diagnostic logging (and implies `--verbose`):

```bash
markov run pipeline.yaml --debug
```

Debug output includes:
- All CLI flag values as parsed
- Callback URL parsing and creation
- K8s client initialization (in-cluster vs kubeconfig)
- Registered executor types
- Namespace resolution

## Inspecting Run Status

After a run completes or fails, inspect it with `markov status`:

```bash
# Basic run status
markov status <run-id>

# With per-step breakdown
markov status <run-id> --steps
```

The `--steps` flag shows a table of every step with status and duration:

```
STEP                           STATUS       DURATION
----                           ------       --------
say-hello                      completed    100ms
process-items                  completed    3.2s
quality_gate                   completed    10ms
deploy                         failed       1.5s
  error: shell_exec: exit status 1
```

After fixing the underlying problem, resume the failed run:

```bash
markov resume <run-id>
```

For detailed resume behavior and caveats, see [Resuming Workflows](resuming-workflows.md).

## Listing All Runs

See all runs in the state store:

```bash
markov list
```

Output shows run ID, workflow name, status, start time, and duration.

## Generating Diagrams

Visualize a completed run as a Mermaid flowchart:

```bash
markov diagram <run-id>
```

Paste the output into any Mermaid renderer to see the execution graph. This is useful for understanding complex for_each fan-outs and sub-workflow hierarchies.

## JSONL Callbacks for Replay

Write events to a file for offline inspection:

```bash
markov run pipeline.yaml --callback jsonl:///tmp/events.jsonl
```

Each line is a JSON object with an `event_type` field. Use `jq` to filter:

```bash
# Show only failed steps
jq 'select(.event_type == "step_failed")' /tmp/events.jsonl

# Show gate evaluations
jq 'select(.event_type == "gate_evaluated")' /tmp/events.jsonl

# Show step durations
jq 'select(.event_type == "step_completed") | {step: .step_name, duration: .duration_seconds}' /tmp/events.jsonl
```

## Common Errors

### "no executor for type"

```
step "my-step": no executor for type "k8s_job"
```

The K8s client is not available. Either you're running locally without a kubeconfig, or the in-cluster ServiceAccount doesn't have permissions. Check with `--debug` to see K8s client initialization details.

### Template errors

```
step "my-step": rendering params: parsing template: ...
```

A `{{ }}` expression references a variable that doesn't exist in the current context. Check that the variable is defined in `vars`, set by a previous `set_fact`, or registered by a previous step.

### Gate evaluation errors

```
step "my-gate": gate evaluation: building GRL rules: ...
```

A rule's `when` expression uses syntax not supported by the condition translator. Supported: identifiers, quoted strings, numbers, comparison operators (`==`, `!=`, `<`, `>`, `<=`, `>=`), logical operators (`and`, `or`, `not`), `None`, `true`/`True`, `false`/`False`, and parentheses.

### "for_each expression did not resolve to a list"

```
step "fan-out": resolving for_each: for_each expression "items" did not resolve to a list
```

The `for_each` value doesn't point to a list in the context. Verify that the variable exists and contains a list (not a string, map, or nil).

### "for_each requires as"

```
workflow "main", step "process": for_each requires as
```

Every `for_each` step must declare an `as` field naming the iteration variable.

### "duplicate for_each_key"

```
duplicate for_each_key "PROJ-100" at indices 0 and 3
```

When using `for_each_key`, every item must have a unique value for that field. Check for duplicates in your input data.

## Inspecting the State Store Directly

The state store is a SQLite database. You can query it directly:

```bash
# List all runs
sqlite3 markov-state.db "SELECT run_id, entrypoint, status FROM runs ORDER BY started_at DESC"

# Show failed steps for a run
sqlite3 markov-state.db "SELECT step_name, error FROM steps WHERE run_id='abc123' AND status='failed'"

# Show sub-runs for a for_each
sqlite3 markov-state.db "SELECT run_id, for_each_key, status FROM runs WHERE parent_run_id='abc123'"
```

## What's Next

- [CLI Reference](../reference/cli.md) — all commands and flags
- [State Store Reference](../reference/state-store.md) — database schema and resume behavior
- [Resuming Workflows](resuming-workflows.md) — practical checkpoint/resume recovery guide
- [Callbacks Reference](../reference/callbacks.md) — event types for monitoring
{% endraw %}
