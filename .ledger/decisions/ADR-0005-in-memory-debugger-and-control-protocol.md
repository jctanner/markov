# ADR-0005: In-memory debugger with a stdin control protocol

Status: accepted, 2026-10-05.

## Context

Interactive building and stepping need pauses before and after steps, with the variable context visible and commands accepted while paused. Gate `pause` exits the process and resumes from the state store, which loses in-flight `for_each` state and cannot stop at an arbitrary step. Steps are identified only by name, names can contain any character, and the engine keeps no source positions.

## Decision

- Pause **in memory**: the engine blocks the step at the breakpoint and waits for a command. Gate `pause` and `resume` are unchanged.
- Breakpoints are **structured objects** `{workflow, step, section, iteration, phase, when, skip, log}`. The CLI takes them as JSON (`--breakpoint`, `--breakpoints-file`); `--break workflow.step` is a convenience resolved against the real workflow and step names, never split on a delimiter, and rejected when ambiguous.
- Commands arrive as **JSON lines on stdin** (`--control stdin`); events leave on the existing callback stream through one new `OnDebug` method with `kind` and `data`.
- Conditions use the engine's existing expression evaluator and must be side-effect free. A condition that errors pauses and reports the error rather than silently never firing.
- A closed control channel while paused aborts the run (resumable); while running it disables debugging.

## Consequences

- `callback.Callback` gains `OnDebug`; jsonl, http, grpc and test mocks implement it. Consumers that ignore unknown event types (markovd) need no change.
- One pause at a time, so `for_each` iterations that hit breakpoints queue; `step` means "the next step to start, in any thread".
- Breakpoints cannot be bound to duplicated step names (the engine rejects those workflows anyway).
- Editing a completed step and resuming still reuses stale output until the definition hash and `--rewind` land (task `step-definition-hash-and-rewind`).
