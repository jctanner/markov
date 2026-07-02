# Task: Implement Directory Loader and Merge Validation

## Goal

Load a directory workflow project and merge it into the existing in-memory `WorkflowFile` representation.

## Context

The engine should continue to execute a single resolved workflow model. Directory support belongs in loading/parsing, not in execution.

## Acceptance Criteria

- [x] Add a loader path for directory inputs.
- [x] Parse `meta.yaml`, `vars.yaml`, `rules.yaml`, `step_types.yaml`, and `workflows/*.yaml`.
- [x] Merge parsed content into one `WorkflowFile`.
- [x] Reuse existing validation after merge.
- [x] Return clear errors for missing files, malformed YAML, and duplicate names.
- [x] Preserve support for existing `rules: - file: ...` includes.
- [x] Add unit tests covering directory merge behavior and single-file compatibility.

## Files Likely Involved

- `pkg/parser/parser.go`
- `pkg/parser/types.go`
- `pkg/parser/*_test.go`

## Status

Done

## Notes

Prefer a small loader abstraction that accepts either a file path or directory path and returns the existing `WorkflowFile` type.
