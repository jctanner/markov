<img width="1173" height="382" alt="markov-logo" src="https://github.com/user-attachments/assets/18feb654-2d5c-47fc-993c-cc95afa2cbf7" />

# Markov Documentation

Markov is a Go-based YAML workflow engine for Kubernetes. Define workflows declaratively, and Markov executes them as K8s Jobs, HTTP requests, or shell commands with built-in concurrency, conditionals, checkpoint/resume, and a rule engine.

## Quick Links

- [Getting Started](getting-started.md) — install, validate, run your first workflow
- [Core Concepts](concepts.md) — understand the execution model
- [Workflow File Reference](reference/workflow-file.md) — complete YAML schema
- [CLI Reference](reference/cli.md) — all commands and flags

## Getting Started

New to markov? Start here:

1. [Getting Started](getting-started.md) — from zero to a running workflow
2. [Core Concepts](concepts.md) — the mental model behind markov
3. [Writing Workflows](guides/writing-workflows.md) — build a real workflow step by step

## Language Reference

Complete, field-by-field documentation for the markov YAML DSL:

- [Workflow File Schema](reference/workflow-file.md) — top-level fields, workflow, step, rule, artifact, and step_type schemas
- [Step Types](reference/step-types.md) — all built-in types: shell_exec, k8s_job, http_request, gate, set_fact, load_artifact, assert
- [Custom Step Types](reference/custom-step-types.md) — defining reusable step types with base/job/defaults/params
- [Template Engine](reference/template-engine.md) — Pongo2 templates, filters, expressions, and type coercion
- [Rule Engine](reference/rule-engine.md) — rules, gates, condition syntax, FactStore, and Grule evaluation
- [Fan-Out (for_each)](reference/for-each.md) — for_each, for_each_key, for_each_sort, concurrency
- [Variables and Context](reference/variables-and-context.md) — vars, register, set_fact, context resolution, and data flow
- [Artifacts](reference/artifacts.md) — loading YAML, markdown, and table files into the context
- [Callbacks](reference/callbacks.md) — event system, HTTP/gRPC/JSONL backends, event types
- [CLI](reference/cli.md) — all commands and flags
- [State Store](reference/state-store.md) — checkpoint/resume, SQLite schema, resume flow

## Guides

Practical how-to guides with complete examples:

- [Writing Workflows](guides/writing-workflows.md) — anatomy of a workflow, variables, register, conditions, custom types
- [Fan-Out Patterns](guides/fan-out-patterns.md) — for_each concurrency, sub-workflow fan-outs, stable keys
- [Resuming Workflows](guides/resuming-workflows.md) — checkpoint/resume workflow operations, caveats, and troubleshooting
- [Looping with Gates](guides/looping-with-gates.md) — recursive workflows, retry loops, gate-controlled iteration
- [Conditional Logic](guides/conditional-logic.md) — when vs gates vs assert, decision routing patterns
- [Running in Kubernetes](guides/k8s-deployment.md) — in-cluster auth, RBAC, state persistence, callbacks
- [Debugging](guides/debugging.md) — verbose/debug output, status inspection, diagrams, common errors

## Design Documents

Internal design docs and research notes (preserved for reference):

- [MVP Design](design/mvp-design.md) — original architecture and design decisions
- [Research Summary](design/research-summary.md) — business case and market analysis
- [Project Comparisons](design/project-comparisons.md) — vs Ansible, Argo, Jenkins, Tekton, Kestra, CrewAI, LangGraph
- [Callbacks Design](design/callbacks-design.md) — callback system design and trade-offs
- [Agent Runner](design/agent-runner.md) — AI agent pipeline lifecycle
- [K8s Job Support](design/k8s-job-support.md) — K8s Job integration design
- [Callback Jobs Startup](design/callback-jobs-startup.md) — callback initialization patterns
- [fromjson Support](design/fromjson-support.md) — JSON parsing filter design
- [Key Sorting](design/key-sorting.md) — stable iteration key design

## How to Use These Docs

- **Writing your first workflow?** Start with [Getting Started](getting-started.md), then [Writing Workflows](guides/writing-workflows.md).
- **Looking up a specific field?** Use the [Workflow File Reference](reference/workflow-file.md) or [Step Types Reference](reference/step-types.md).
- **Understanding a pattern?** Read the relevant [Guide](guides/writing-workflows.md).
- **Debugging a problem?** See [Debugging](guides/debugging.md).
