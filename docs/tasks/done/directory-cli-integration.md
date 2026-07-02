# Task: Wire Directory Input Into CLI Commands

## Goal

Allow CLI commands that accept workflow input paths to accept either a YAML file or a workflow directory.

## Context

The intended UX is path detection:

```bash
markov run ./pipeline
markov validate ./pipeline
```

No explicit `--project-dir` flag is planned for v1.

## Acceptance Criteria

- [x] `markov validate <directory>` validates a directory workflow.
- [x] `markov run <directory>` executes a directory workflow.
- [x] State store records enough source information for `resume` to reload the directory workflow.
- [x] Error messages distinguish missing directory files from YAML parse errors.
- [x] Existing `markov run <file.yaml>` and `markov validate <file.yaml>` behavior is unchanged.

## Files Likely Involved

- `cmd/markov/main.go`
- `pkg/parser/parser.go`
- `pkg/state/store.go`
- `pkg/state/sqlite.go`
- `cmd/markov/main_test.go`

## Status

Done

## Notes

Resume currently relies on the stored workflow file path. Directory inputs should store the directory path and reload through the same path-detecting loader.
