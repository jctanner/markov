# Bug: Direct for_each Iterations Collide in Step State

## Status

Fixed. Direct typed `for_each` iterations now use distinct internal step state names derived from the iteration key, while preserving the original step name for execution metadata.

## Summary

When `for_each` is used directly on a typed step instead of a sub-workflow, iterations share the same `(run_id, workflow_name, step_name)` state key. After one iteration saves the step as completed, later iterations can be treated as already completed and skipped.

## Reproduction

Run a workflow with a direct typed `for_each` step:

```yaml
entrypoint: main
vars:
  items: [alpha, bravo, charlie]
workflows:
  - name: main
    steps:
      - name: process_items
        for_each: items
        as: item
        type: shell_exec
        params:
          command: "echo {{ item }}"
```

## Expected

All three items execute independently.

## Actual

At least one later iteration may log `skipping completed step "process_items"` because a previous iteration already saved that step name as completed for the parent run.

## Impact

Direct typed `for_each` is unreliable for multi-item execution and resume. Sub-workflow fan-out is not affected because each item has a distinct sub-run ID.

## Fix Evidence

- Added `TestDirectForEachTypedStepUsesDistinctState`.
- Added `TestDirectForEachTypedStepLargeFanOutUsesDistinctState` covering 200 direct typed iterations with concurrency 20.
- Serialized SQLite state-store writes through one open connection after the large fan-out test exposed missing rows under concurrent writes.
- Verified `GOCACHE=/tmp/go-build go test ./pkg/engine -run TestDirectForEachTypedStepLargeFanOutUsesDistinctState -count=1 -v`.
- Verified `GOCACHE=/tmp/go-build go test ./pkg/engine`.
- Verified `GOCACHE=/tmp/go-build go test ./cmd/markov ./pkg/engine ./pkg/executor ./pkg/parser ./pkg/state ./pkg/template`.
- Verified `GOCACHE=/tmp/go-build make vet`.
- Full `GOCACHE=/tmp/go-build make test` is blocked in this sandbox by callback tests that open local TCP listeners.

## Related Tasks

- `docs/tasks/pending/directory-docs-and-examples.md`
