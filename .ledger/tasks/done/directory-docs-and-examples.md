# Task: Add Directory Workflow Documentation and Examples

## Goal

Document directory workflow projects and add an example that validates and runs.

## Context

Users need to understand when to choose single-file versus directory mode and how the merge contract works.

## Acceptance Criteria

- [x] Add a directory workflow section to the workflow file reference.
- [x] Add guide coverage for organizing large workflows.
- [x] Add an executable example under `examples/dir-based-hello-world/`.
- [x] Include commands for `markov validate <directory>` and `markov run <directory>`.
- [x] Document duplicate-name behavior and relative path behavior.

## Files Likely Involved

- `docs/reference/workflow-file.md`
- `docs/guides/writing-workflows.md`
- `docs/getting-started.md`
- `examples/dir-based-hello-world/`

## Status

Done

## Notes

The example should be equivalent to a compact single-file workflow so tests can compare parser output across both forms.
