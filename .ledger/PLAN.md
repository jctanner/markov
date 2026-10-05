# Project Plan

## Current Milestone

- [M1 Directory Workflow Files](milestones/M1-directory-workflow-files.md)

## Plans

- [Directory Workflow Files Plan](plans/000-directory-workflow-files.md)

## Active Tasks

- None

## Pending Tasks

- None

## Completed Tasks

- [First-class Jev support](tasks/done/jev-support.md)
- [Make all current examples validate](tasks/done/validate-all-examples.md)
- [Add strict YAML validation](tasks/done/strict-yaml-validation.md)
- [Implement source integrity modes](tasks/done/source-integrity-modes.md)
- [Add k8s_job_wait step type](tasks/done/k8s-job-wait-step-type.md)
- [Add Postgres state store support](tasks/done/postgres-state-store.md)
- [Add ansible and ansible_playbook step types](tasks/done/ansible-step-types.md)
- [Define directory workflow schema](tasks/done/directory-workflow-schema.md)
- [Implement directory loader and merge validation](tasks/done/directory-loader-merge-validation.md)
- [Wire directory input into CLI commands](tasks/done/directory-cli-integration.md)
- [Add directory workflow documentation and examples](tasks/done/directory-docs-and-examples.md)

## Open Bugs

- [Direct for_each iterations collide in step state](bugs/fixed/direct-foreach-step-dedup.md)

## Fixed Bugs

- [Template map values render as Go strings in HTTP bodies](bugs/fixed/template-map-values-render-as-go-strings.md)
- [CLI `--var false` remains a truthy string](bugs/fixed/cli-var-false-remains-truthy-string.md)

## Decisions

- [ADR-0001: Directory Workflow File Layout](decisions/ADR-0001-directory-workflow-layout.md)
- [ADR-0002: Source Integrity Modes for Resumable Workflow Runs](decisions/ADR-0002-source-integrity-modes.md)

- [ADR-0003: Durable Jev decisions](decisions/ADR-0003-durable-jev-decisions.md)
