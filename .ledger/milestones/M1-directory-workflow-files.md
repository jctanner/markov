# M1: Directory Workflow Files

## Goal

Allow Markov to run and validate a directory of workflow files while preserving the current single-file workflow format.

## Context

Detailed feature plan: [Directory Workflow Files Plan](../plans/000-directory-workflow-files.md)

Decision record: [ADR-0001: Directory Workflow File Layout](../decisions/ADR-0001-directory-workflow-layout.md)

## Tasks

- [Define directory workflow schema](../tasks/done/directory-workflow-schema.md)
- [Implement directory loader and merge validation](../tasks/done/directory-loader-merge-validation.md)
- [Wire directory input into CLI commands](../tasks/done/directory-cli-integration.md)
- [Add directory workflow documentation and examples](../tasks/done/directory-docs-and-examples.md)

## Acceptance Criteria

- `markov validate <file.yaml>` behavior is unchanged.
- `markov validate <directory>` validates a split workflow project.
- `markov run <directory>` executes the merged workflow.
- Directory mode rejects missing required files with clear errors.
- Directory mode rejects duplicate workflow, rule, and step type names.
- Existing external rule file support still works.
- Docs and examples demonstrate the directory layout.
