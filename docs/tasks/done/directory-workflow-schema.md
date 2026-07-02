# Task: Define Directory Workflow Schema

## Goal

Define the exact YAML shape and validation contract for directory-based workflow projects.

## Context

The feature should support splitting one workflow file into a predictable directory layout while preserving the current single-file workflow format.

## Acceptance Criteria

- [x] Specify required files: `meta.yaml`, `vars.yaml`, `rules.yaml`, `step_types.yaml`, and `workflows/*.yaml`.
- [x] Specify whether empty category files are allowed.
- [x] Specify that each `workflows/*.yaml` file contains one workflow object directly.
- [x] Specify duplicate-name validation for workflows, rules, and step types.
- [x] Specify relative path behavior for external rule files.
- [x] Add parser tests for valid and invalid directory schemas.

## Files Likely Involved

- `pkg/parser/types.go`
- `pkg/parser/parser.go`
- `docs/reference/workflow-file.md`
- `docs/decisions/ADR-0001-directory-workflow-layout.md`

## Status

Done

## Notes

Runtime artifact paths should probably remain unchanged strings because they may point to paths inside containers or mounted volumes, not local project files.
