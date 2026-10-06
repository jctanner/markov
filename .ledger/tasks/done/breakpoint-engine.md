# Task: Breakpoints, step mode and control channel

## Status

Done

## Summary

The in-memory debugger from [ADR-0005](../../decisions/ADR-0005-in-memory-debugger-and-control-protocol.md) and [plan 001](../../plans/001-step-through-debugging.md): structured breakpoints (workflow, step, section, iteration, phase, `when`, `skip`, `log`), step mode, `until`, `evaluate`, `terminate`, a stdin control channel, and `debug` callback events. Docs: [debugging reference](../../docs/reference/debugging.md).

## Files

- `pkg/engine/debug.go` (debugger, matching, pause, control channel), hook in `pkg/engine/engine.go` (`executeStepWithOptions` wrapper for the after phase, `debugPause` before execution once the `when` check passed; run ID noted in `Run` and `ResumeWithVars`)
- `pkg/callback/{callback,jsonl,http,grpc}.go` (`DebugEvent`, `OnDebug`), test mocks
- `cmd/markov/main.go` (`--control`, `--breakpoint`, `--breakpoints-file`, `--break`, `--step`, startup resolution, strict JSON)
- `docs/reference/{debugging,callbacks,cli}.md`, `docs/guides/debugging.md`, `docs/README.md`, `examples/debugging.yaml`

## Verification

- `pkg/engine/engine_debug_test.go`, with the race detector and `-count=30`: pause before with the variables set by an earlier step, after phase with output, step then until then continue, conditions, `skip`, logpoints (no pause), a failing condition pausing with its error, `for_each` iteration breakpoints and stepping into iterations, sub-workflow hits told apart by run ID (`skip` on the second call), rescue and always sections, `evaluate` while paused and not paused, `set_breakpoints` acknowledgement with every kind of rejection, `terminate` leaving a failed resumable run, control channel closing while paused (aborts) and while running (disables), one pause at a time under four concurrent iterations, `continue` dropping step pauses queued behind it, state-name splitting for awkward names, shorthand resolution (awkward names, ambiguity, no match).
- `make lint` and the full test suite pass.
- Real binary: a script drove `markov run --control stdin --callback jsonl://… --break main.greet --break 'main.each host'`: pause with variables, `evaluate` (`level * 6` = 42), `continue`, step into a `for_each` iteration with `host` set, `until` the last step, exit 0. Startup errors checked: breakpoints without `--control`, unknown `--break`, an unknown JSON field.
- Not verified: Kubernetes jobs, gRPC and HTTP callbacks carrying `debug` events end to end (they use the same send path as other events), `resume` with a debugger attached beyond the run-ID bookkeeping.

## Notes

- With concurrent `for_each` iterations the pause order depends on scheduling; use `concurrency: 1` for deterministic stepping.
