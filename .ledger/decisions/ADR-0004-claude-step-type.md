# ADR-0004: Native claude step with Markov-enforced limits

Status: accepted, 2026-10-05.

## Context

Workflows need to run Claude Code for a prompt or skill in a chosen directory and to bound what it spends. `shell_exec` can launch the CLI but sees only buffered output after the process ends, so Markov cannot stop a runaway session or report progress while it runs.

## Decision

Add a `claude` primitive that runs `claude -p --output-format stream-json --verbose` directly (no shell), writes the prompt to stdin, and reads the event stream line by line as it arrives.

- **Auth:** never require or set an API key; inherit the runner environment so an existing OAuth login is used. `bare` is opt-in because it needs an API key.
- **Permissions:** unset by default (the CLI denies prompting actions non-interactively). `bypassPermissions` is explicit per step; `allowed_tools` is the preferred path.
- **Limits:** `max_turns`, `max_tokens`, `max_duration` are enforced by Markov from the stream. `max_tokens` counts input + output + cache-creation and excludes cache reads. `max_budget_usd` is passed to the CLI as a backstop. On breach Markov signals the process group, ignores later buffered events so counts reflect the breach, and fails the step while still returning output.
- **Progress:** executors receive a progress sink through the context (`executor.WithProgress`), not stored on the executor, so concurrent `for_each` iterations stay independent. The engine forwards it as a new `step_progress` callback event (jsonl, http, grpc). Progress is not persisted in the state store.
- **Output:** final result, session, turns, tokens, usage, cost, and a capped list of compact events.

## Consequences

- Adding `OnStepProgress` extends the `callback.Callback` interface; all implementations were updated.
- A resumed run restarts an interrupted `claude` step rather than resuming its session.
- Limits are checked per stream event, so a single very long turn can overshoot until its message arrives.
- Parallel `for_each` over `claude` steps shares one login's rate limits.
- Pods have no interactive login; credentials must be supplied through `env` or mounts.
