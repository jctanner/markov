# AGENTS.md

Repository-specific guidance for AI agents working on Markov.

## Project Overview

Markov is a Go-based YAML workflow engine for Kubernetes-oriented pipelines. It executes declarative workflows through built-in primitives such as `shell_exec`, `k8s_job`, `http_request`, `gate`, `set_fact`, `load_artifact`, and `assert`.

The engine intentionally borrows familiar workflow semantics from Ansible: `vars`, `when`, `register`, `for_each`, templating, and reusable step types. It adds state-backed resume, rule-engine gates, callbacks, artifact loading, and direct Kubernetes Job execution.

## Important Paths

- `cmd/markov/` - CLI entrypoint, command wiring, diagram support.
- `pkg/engine/` - workflow execution, gates, facts, fan-out, artifact handling.
- `pkg/parser/` - YAML parsing, schema types, validation.
- `pkg/executor/` - primitive executors for shell, HTTP, and Kubernetes Jobs.
- `pkg/state/` - SQLite checkpoint/resume store.
- `pkg/template/` - Pongo2 rendering and custom filters such as `fromjson`.
- `pkg/callback/` - JSONL, HTTP, and gRPC callback implementations.
- `examples/` - executable workflow examples and diagram fixtures.
- `docs/` - user docs, reference docs, design notes, and project process notes.

Start with `README.md`, `docs/README.md`, and `docs/concepts.md` before making behavioral changes.

## Build And Test

Use the Makefile unless there is a specific reason not to:

- `make build` - builds `bin/markov`.
- `make test` - runs `go test ./...`.
- `make vet` - runs `go vet ./...`.
- `make fmt` - fails if any Go files need formatting.
- `make lint` - runs `vet` and `fmt`.
- `make docker-build` - builds the container image.

In restricted sandboxes, Go's default build cache may be read-only. Use:

```bash
GOCACHE=/tmp/go-build make test
```

Some callback tests open local TCP listeners via `httptest` or gRPC. If sockets are blocked by the environment, `pkg/callback` tests may fail with `socket: operation not permitted`; treat that as an environment limitation unless the code change directly touches callback networking.

## Coding Conventions

- Keep changes scoped to the requested behavior.
- Prefer existing package boundaries and patterns over new abstractions.
- Use `gofmt` on modified Go files.
- Add or update focused tests when changing parser, engine, executor, state, template, or callback behavior.
- Do not change workflow semantics casually; update reference docs and examples when behavior changes.
- Avoid introducing dependencies unless they clearly match the existing architecture.

## Workflow Semantics To Preserve

- Steps run sequentially unless `for_each` introduces concurrency.
- Sub-workflows receive isolated context; parent workflows only receive child data through `register`.
- `for_each_key` should be used for resumable production fan-outs where list order may change.
- `set_fact` has special direct handling for exact `{{ path | fromjson }}` expressions to preserve structured JSON.
- Gate rules use salience and forward chaining; `pause` is currently documented as not fully implemented.
- Callback failures must not fail workflow execution.

## Documentation Expectations

When changing user-visible behavior, update the relevant docs:

- YAML schema or validation: `docs/reference/workflow-file.md`.
- Step behavior: `docs/reference/step-types.md`.
- Fan-out behavior: `docs/reference/for-each.md` and related guides.
- Variables/context behavior: `docs/reference/variables-and-context.md`.
- Template behavior: `docs/reference/template-engine.md`.
- Callback behavior: `docs/reference/callbacks.md` and design notes if needed.
- Kubernetes behavior: `docs/guides/k8s-deployment.md` and `docs/design/k8s-job-support.md`.

Keep examples consistent with the documented DSL.

## Agent Work Ledger

`docs/agentic_work_ledger.md` describes the preferred long-running project-management pattern:

- `PLAN.md` is an index, not the full plan.
- Meaningful tasks are individual files.
- Status is represented by directory placement, such as `pending/`, `current/`, `blocked/`, and `done/`.
- Bugs and ADRs should be first-class markdown artifacts.
- Agents should record discoveries and evidence instead of relying on chat history.

If a task creates or depends on a durable work item, follow that ledger style rather than burying state in conversation.

## Git And Safety

- The worktree may contain user changes. Do not revert unrelated modifications.
- Check `git status --short` before and after edits.
- Do not use destructive git commands unless explicitly requested.
- If generated state files or local artifacts are present, avoid touching them unless they are part of the task.

