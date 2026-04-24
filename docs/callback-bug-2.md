# Bug: Callback events not emitted for sub-workflow and forked steps

## Summary

The HTTP callback system only emits `step_started` / `step_completed` events for steps in the top-level (entrypoint) workflow. Steps inside sub-workflows — including `for_each` fan-outs, recursive workflow invocations, and nested sub-workflow calls — produce no callback events at all. This makes the entire sub-workflow execution tree invisible to external consumers like markovd.

## Reproduction

Run the `deploy-pipeline.yaml` example via markovd:

```
POST /api/v1/runs
{"workflow_name": "deploy-pipeline", "vars": {}}
```

This workflow structure is:

```
main
  ├── preflight          (shell_exec)
  ├── build              (shell_exec)
  ├── test_gate          (gate)
  ├── deploy_all         (for_each: 3 environments → deploy_env)
  │     └── deploy_env
  │           ├── announce, apply_manifests, health_check → retry_loop (recursive)
  │           ├── promotion_gate, smoke_tests, mark_complete
  └── summary            (shell_exec)
```

### Observed behavior

- markovd receives **11 callback events** — all for the `main` workflow only:

  | event_type     | workflow | step      |
  |----------------|----------|-----------|
  | run_started    | main     |           |
  | step_started   | main     | preflight |
  | step_completed | main     | preflight |
  | step_started   | main     | build     |
  | step_completed | main     | build     |
  | step_started   | main     | test_gate |
  | gate_evaluated | main     | test_gate |
  | step_completed | main     | test_gate |
  | step_started   | main     | summary   |
  | step_completed | main     | summary   |
  | run_completed  | main     |           |

- **Zero events** for any sub-workflow step (deploy_env, retry_loop)
- markov's own logs confirm **55 steps** actually executed across **16 execution contexts**:

  ```
  2a9c9ece                                                  (main)
  2a9c9ece-deploy_all-0                                     (deploy_env: staging)
  2a9c9ece-deploy_all-0-health_check                        (retry_loop iter 1)
  2a9c9ece-deploy_all-0-health_check-next_retry             (retry_loop iter 2)
  2a9c9ece-deploy_all-0-health_check-next_retry-next_retry  (retry_loop iter 3)
  ...                                                       (same for -1/canary, -2/production)
  ```

- The `deploy_all` step itself has no `step_started`/`step_completed` events — it's a `for_each` orchestrator, not a typed step, so the callback emitter doesn't see it.

### Expected behavior

All 55 step executions across all sub-workflows should produce callback events. The events should include the fork/sub-workflow context (run ID suffix or parent workflow name) so consumers can reconstruct the execution tree.

## Root cause

The callback emitter is wired into the step execution loop of the entrypoint workflow only. When a `for_each` step fans out to sub-workflows, or a step invokes a nested workflow via `workflow:`, the sub-workflow executor runs in a separate goroutine/context that does not have the callback hooks attached.

The execution log prefixes (`[run:2a9c9ece-deploy_all-0]`) show that markov already generates hierarchical context IDs for forks — this information exists at runtime but is not passed to the callback system.

## Impact

- **Invisible sub-workflows**: markovd's step table and graph only show top-level steps. For `deploy-pipeline`, the UI shows 4 steps instead of 55.
- **No fan-out visibility**: `for_each` steps produce no events at all — consumers can't see that 3 parallel deployments ran, or which environments succeeded/failed.
- **No recursion tracking**: Recursive retry loops are completely invisible. Operators can't tell how many retries occurred or where a retry chain failed.
- **Misleading timeline**: The gap between `test_gate` completing and `summary` starting (where all sub-workflow work happened) appears as dead time with no events.

## Suggested fix

### Propagate callbacks to sub-workflow executors

When the main workflow executor spawns a sub-workflow (via `for_each` or `workflow:` step), pass the callback configuration into the sub-workflow's execution context. Each sub-workflow step should emit the same `step_started`/`step_completed`/`gate_evaluated` events as top-level steps.

### Event schema additions

Sub-workflow events need additional context fields so consumers can reconstruct the tree:

```json
{
  "event_type": "step_completed",
  "run_id": "2a9c9ece",
  "workflow_name": "deploy_env",
  "step_name": "announce",
  "fork_id": "2a9c9ece-deploy_all-0",
  "parent_step": "deploy_all",
  "fork_index": 0,
  "status": "completed"
}
```

Key fields:
- `fork_id` — the hierarchical execution context ID (already generated for logging)
- `parent_step` — the `for_each` or `workflow:` step that spawned this sub-workflow
- `fork_index` — the index within a `for_each` iteration (0, 1, 2 for staging/canary/production)

### New event types to consider

- `fork_started` / `fork_completed` — bracket a `for_each` fan-out (how many items, concurrency)
- `subworkflow_started` / `subworkflow_completed` — bracket a `workflow:` step invocation

## Workaround

For now, markovd operators can parse the markov process stdout/stderr logs (which contain full sub-workflow execution details) as a secondary data source. However, this requires markovd to capture and parse unstructured log output rather than receiving typed events.

## Related

- `callback-bug-1.md` — buffer overflow that drops events (fixed; this bug is independent)
- `deploy-pipeline.yaml` — example workflow that exercises fan-out, recursion, and gating
- `internal/callback/http.go` (presumed) — callback emitter, only wired to top-level executor
- `internal/workflow/executor.go` (presumed) — where sub-workflows are spawned without callback hooks
