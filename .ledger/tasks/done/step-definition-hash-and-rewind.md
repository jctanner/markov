# Task: Step definition hash and rewind on resume

## Status

Done

## Summary

A completed step is skipped on resume by name, so an edited step silently kept its old output (verified experimentally, see [plan 001](../../plans/001-step-through-debugging.md)). Each step row now stores a hash of the definition that produced it; resume reports completed steps whose definition changed, and `markov resume --rewind` / `--rewind-changed` re-run from a chosen or the earliest changed step.

## Files

- `pkg/state/{store,sqlite,postgres}.go`: `StepResult.DefinitionHash`, nullable `definition_hash` column (migrated in place), `DeleteSteps`.
- `pkg/engine/rewind.go`: `definitionHash` (the parsed step plus its resolved custom step type, SHA-256 truncated), `findStep` (state name back to its step; longest name wins), `hashingStore` (stamps every saved row), `changedSteps` (walks sub-runs), `prepareResume`, rewind of the entrypoint workflow from an index (rows and `for_each` iterations) plus every record of the sub-runs those steps started; `engine.go` hooks it into `New` and `ResumeWithVars` (a completed run may be rewound).
- `pkg/callback`: `run_resumed` gains `changed_steps` and `rewound`.
- `cmd/markov/main.go`: `resume --rewind '<json>'` (repeatable, strict JSON, earliest wins) and `--rewind-changed`; `resume` also gained the callback and debugger flags of `run` (so the IDE can watch and debug a resumed run).
- Docs: `docs/reference/{state-store,cli,callbacks}.md`, `docs/guides/resuming-workflows.md`.

## Verification

- Store test: hash round trip, update, `DeleteSteps` leaves other runs alone; the existing legacy-schema migration test still passes. Full suite and `make lint` pass.
- Engine tests (race detector): rows carry distinct hashes; resume reports an edited completed step and reuses it; `--rewind-changed` re-runs the edited step and everything after, and is a no-op when nothing changed; explicit rewind (earliest of several) on a failed and on a completed run; helpful errors for a sub-workflow target (names the calling steps), unknown step (lists the steps), rescue target; editing a step inside a sub-workflow rewinds to the calling step and clears the sub-run; `for_each` iterations cleared; rows without a hash are trusted.
- Real binary: the original experiment (edit completed step `a`, fix `b`, resume) now logs the warning and reuses `ORIGINAL-A`; with `--rewind-changed` it clears 2 records and the database holds `EDITED-A`. Bad `--rewind` input fails with the real step names or an unknown-field error.
- Not verified: Postgres (same SQL with `$n` placeholders, not run), Kubernetes runs.

## Limitations

- The hash covers the step and its custom step type, not workflow or global `vars`, templates' inputs, or files a step reads.
- Rewind targets are steps of the entrypoint workflow; inside sub-workflows, rewind to the calling step.
