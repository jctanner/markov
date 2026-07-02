# Task: Add Directory Workflow Documentation and Examples

## Goal

Document directory workflow projects and add an example that validates and runs.

## Context

Users need to understand when to choose single-file versus directory mode and how the merge contract works.

## Acceptance Criteria

- [ ] Add a directory workflow section to the workflow file reference.
- [ ] Add guide coverage for organizing large workflows.
- [ ] Add an executable example under `examples/`.
- [ ] Include commands for `markov validate <directory>` and `markov run <directory>`.
- [ ] Document duplicate-name behavior and relative path behavior.

## Files Likely Involved

- `docs/reference/workflow-file.md`
- `docs/guides/writing-workflows.md`
- `docs/getting-started.md`
- `examples/`

## Status

Pending

## Notes

The example should be equivalent to a compact single-file workflow so tests can compare parser output across both forms.

