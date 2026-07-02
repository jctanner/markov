# Task: Implement Directory Loader and Merge Validation

## Goal

Load a directory workflow project and merge it into the existing in-memory `WorkflowFile` representation.

## Context

The engine should continue to execute a single resolved workflow model. Directory support belongs in loading/parsing, not in execution.

## Acceptance Criteria

- [ ] Add a loader path for directory inputs.
- [ ] Parse `meta.yaml`, `vars.yaml`, `rules.yaml`, `step_types.yaml`, and `workflows/*.yaml`.
- [ ] Merge parsed content into one `WorkflowFile`.
- [ ] Reuse existing validation after merge.
- [ ] Return clear errors for missing files, malformed YAML, and duplicate names.
- [ ] Preserve support for existing `rules: - file: ...` includes.
- [ ] Add unit tests covering merge output equivalence with a single-file workflow.

## Files Likely Involved

- `pkg/parser/parser.go`
- `pkg/parser/types.go`
- `pkg/parser/*_test.go`

## Status

Pending

## Notes

Prefer a small loader abstraction that accepts either a file path or directory path and returns the existing `WorkflowFile` type.

