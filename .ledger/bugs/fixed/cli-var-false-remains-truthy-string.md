# Bug: CLI `--var false` remains a truthy string

## Summary

`markov run ... --var run_pipeline=false` stores the override as the string
`"false"`. A condition such as `when: "run_pipeline"` therefore evaluates as
true and executes the guarded step.

## Status

Fixed. CLI overrides now use the engine's canonical string coercion before
being merged into the workflow context.

## Reproduction

Use a workflow with a boolean default and a guarded step:

```yaml
vars:
  run_pipeline: true

workflows:
  - name: main
    steps:
      - name: guarded
        when: "run_pipeline"
        type: shell_exec
        params:
          command: echo should-not-run
```

Run:

```bash
markov run workflow.yaml --var run_pipeline=false
```

The guarded step runs.

The issue was also observed through Markovd run
`markov-run-d47e6650`. Markovd recorded
`{"run_pipeline":"false","seed_rfe":"true"}`, while both
`main.run_pipeline` and `run-pipeline.rfe` entered `running`. The run was
cancelled to stop the unintended agent pipeline.

## Expected

CLI variable values use the same coercion already documented and implemented
for rendered workflow variables: `true`/`false` become booleans, numeric values
become numbers, and JSON arrays/objects become native structured values.

## Actual

`cmd/markov/main.go:parseVarFlags()` stores `parts[1]` directly in a
`map[string]any`, so every CLI override remains a string. The engine's existing
`coerceString()` helper is not used at this boundary.

## Impact

High. Boolean safety switches passed from Markovd or the CLI can invert their
intended behavior. In the observed case, `run_pipeline=false` launched an
expensive end-to-end agent workflow instead of performing a reset only.

## Acceptance Criteria

- [x] CLI overrides use the engine's canonical string coercion.
- [x] Regression tests cover booleans, integers, floats, JSON arrays/objects,
      ordinary strings, empty strings, and values containing `=`.
- [x] Variable reference documentation states the CLI coercion behavior.
- [x] `--var run_pipeline=false` skips a step guarded by
      `when: "run_pipeline"` in a live Markovd run.

## Resolution

- Exported `engine.CoerceString()` as the canonical conversion boundary.
- Updated `parseVarFlags()` to coerce each value while retaining `SplitN(...,
  2)` behavior for values containing `=`.
- Added a table-driven value-shape regression in `cmd/markov/main_test.go`.
- Documented CLI coercion in `docs/reference/variables-and-context.md`.

## Verification

- `GOCACHE=/tmp/go-build make test`: passed all packages.
- `GOCACHE=/tmp/go-build make lint`: `go vet ./...` passed; the formatting
  phase still reports five unrelated pre-existing files. Neither modified Go
  file was reported.
- `GOCACHE=/tmp/go-build make build`: built `bin/markov`.
- Platform `make host-rebuild-markov`: built and imported
  `markov:latest` into k3s.
- Live Markovd run `markov-run-622d5874` completed using image ID
  `sha256:df23af175162136d0445ac85e73d94e32fc7474de01c8920c0abd79ea2f23aa9`.
  Markovd still recorded its request as `"run_pipeline":"false"`, proving the
  conversion occurred inside Markov. `main.seed_rfe` completed,
  `main.run_pipeline` was skipped with `when condition "run_pipeline"
  evaluated to false`, and no `run-pipeline` child steps were created.
