# Task: Define Directory Workflow Schema

## Goal

Define the exact YAML shape and validation contract for directory-based workflow projects.

## Context

The feature should support splitting one workflow file into a predictable directory layout while preserving the current single-file workflow format.

## Acceptance Criteria

- [ ] Specify required files: `meta.yaml`, `vars.yaml`, `rules.yaml`, `step_types.yaml`, and `workflows/*.yaml`.
- [ ] Specify whether empty category files are allowed.
- [ ] Specify that each `workflows/*.yaml` file contains one workflow object directly.
- [ ] Specify duplicate-name validation for workflows, rules, and step types.
- [ ] Specify relative path behavior for external rule files.
- [ ] Add parser tests for valid and invalid directory schemas.

## Files Likely Involved

- `pkg/parser/types.go`
- `pkg/parser/parser.go`
- `docs/reference/workflow-file.md`
- `docs/decisions/ADR-0001-directory-workflow-layout.md`

## Status

Pending

## Notes

Runtime artifact paths should probably remain unchanged strings because they may point to paths inside containers or mounted volumes, not local project files.

