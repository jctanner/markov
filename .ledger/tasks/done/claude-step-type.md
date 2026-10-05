# Task: Add claude step type

## Status

Done

## Summary

First-class `claude` primitive: run a prompt or skill through the Claude Code CLI in a chosen directory, stream its JSON output live, forward progress to callbacks, and enforce turn, token, and duration limits. See [ADR-0004](../../decisions/ADR-0004-claude-step-type.md).

## Files

- `pkg/executor/claude.go`, `claude_proc_unix.go`, `claude_proc_other.go`, `claude_test.go`
- `pkg/executor/executor.go` (`WithProgress`)
- `pkg/callback/*` (`StepProgressEvent`, `OnStepProgress`), `pkg/engine/engine.go`
- `pkg/parser/parser.go`, `cmd/markov/main.go`
- `docs/reference/{step-types,callbacks,workflow-file,custom-step-types}.md`, `docs/concepts.md`, `examples/claude.yaml`

## Verification

- `make lint test` pass. Executor tests use a fake streaming binary to assert argv, stdin prompt, cwd, env, no injected key, stream parsing, progress order, token accounting (dedupe by message id, cache reads excluded), max_turns/max_tokens/max_duration breaches including process-group kill, error results, missing result, and event capping. An engine test asserts `step_progress` callbacks.
- Real CLI (v2.1.289) with OAuth login and `ANTHROPIC_API_KEY` unset: a two-turn Bash task completed with live `step_progress` events in a jsonl callback; the same task with `max_turns: 1` emitted `limit_exceeded`, failed the step, and left no process behind.
- `markov validate examples/claude.yaml` passes.
- Not verified: Kubernetes execution, `bare` mode, `resume`, and parallel `for_each` against the real CLI.
