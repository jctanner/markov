# Step-Through Debugging and Interactive Workflow Building

## Goal

Let a person (or an agent) build a workflow interactively and run it a step at a time: set breakpoints on steps, pause, inspect the variables the next step will see, edit, and continue. The Markov IDE (`../markov-ide`) is the main client; the engine does the pausing.

## Findings that shape the design

Verified against the code and the real binary on 2026-10-05:

- **Steps are identified by name only.** The parser decodes YAML straight into Go structs (`pkg/parser/types.go`, `yaml.v3`, `KnownFields(true)`), so no line numbers survive. Events and the state store key steps by `(workflow, step name)`; the `steps` table primary key is `(run_id, workflow_name, step_name)`.
- **Names are unique only within a workflow** (steps, rescue and always together). The same name in two workflows is valid. Workflow and step names are otherwise unrestricted: `my flow.v1:x` and `step one.a:b [c]` validate. So no delimiter-based `workflow.step` string can be split reliably.
- **State-key variants.** `for_each` iterations are stored as `name[<key>]`, rescue and always steps as `rescue/name` and `always/name`. Sub-workflow runs are separate runs with ID `<parent run id>-<call step name>`.
- **Resume is a replay.** `ResumeWithVars` reloads the workflow file, rebuilds variables from `vars_json` plus every completed step's stored output, walks the workflow from the top, and skips any step whose row is `completed`. There is no hash of a step's definition, so editing an already-completed step and resuming silently reuses the old output (experiment: `a` edited to `echo EDITED-A` still skipped, database kept `ORIGINAL-A`). Only the whole-source digest is compared, and by default drift only warns.
- **`pause` today** (gates) exits the process and is resumed from the store. That suits approvals, but not stopping inside a running `for_each`.

## Design

1. **Breakpoints are structured, never parsed from a string.** Object fields: `workflow`, `step` (required); `section` (`steps` default, `rescue`, `always`); `iteration` (a `for_each` key); `phase` (`before` default, `after`); `when` (an expression in the engine's own condition syntax, evaluated against the variables); `skip` (ignore the first N hits); `log` (a template: a logpoint that prints and continues).
2. **CLI.**
   - `--breakpoint '<json object>'` (repeatable) and `--breakpoints-file <file>` (JSON array).
   - `--break <workflow.step>`: human shorthand. The engine tries each real workflow name as a prefix and takes the remainder as the step name; no match or an ambiguous match is an error that lists the real names and points at the JSON form.
   - `--step`: pause before every step.
   - `--control stdin`: read JSON commands, one per line, from standard input (`--debug` already means logging, so it is not reused).
   - Breakpoints that do not resolve (unknown workflow, step or section) fail at startup with the real names.
3. **Pausing is in memory.** At a hit the engine emits a `debug` callback event and blocks that step until a command arrives; fan-out and loop state stay intact. At most one pause is active; others queue. If the control channel closes while paused, the run is aborted (state is saved as failed and stays resumable) so nothing is left blocked forever; if it closes while running, debugging is simply switched off.
4. **Control commands** (stdin, one JSON object per line): `set_breakpoints` (replaces the list; acknowledged with resolved and unresolved entries), `continue`, `step` (pause before the next step that is about to run, in any thread), `until {workflow, step, section?, iteration?}` (one-shot breakpoint then continue), `evaluate {id, expression}` (render a template against the paused variables), `terminate`.
5. **Events.** One new callback method, `OnDebug(DebugEvent)`, with `kind` plus `data` (the same shape as `step_progress`), so there is one interface change. Kinds: `breakpoints_set`, `paused`, `resumed`, `logpoint`, `evaluated`. A `paused` event carries `reason` (`breakpoint`, `step`, `until`), `workflow`, `step`, `section`, `iteration`, `phase`, the run ID (a sub-workflow run is named `<parent run id>-<call step name>`, so it embeds the chain), the variables (JSON-safe, with failures reported rather than fatal), and for `after` the step's output.
6. **Hook point.** In `executeStepWithOptions`, after a completed-step skip and after the `when` check passes, before execution (so a step that will not run never pauses). The `after` phase wraps the call and checks the store to see the step actually completed during this call. Section and iteration come from the state name by stripping a known section prefix and the known step name, which is unambiguous because the step name is known.
7. **Per-step definition hash and rewind.** Store a hash of each step's parsed definition in a new nullable `definition_hash` column (old rows stay unknown and are never flagged). On resume, report completed steps whose definition changed. `markov resume <id> --rewind '{"workflow":..,"step":..}'` marks that step and every later step of that workflow (and its terminal steps' dedup) as not completed before resuming; `--rewind-changed` rewinds to the earliest changed step.
8. **Schema.** `markov schema` prints the step types, their parameters and the parser's top-level and step fields as JSON, generated from the structs and the executors, so clients stop hand-maintaining lists.
9. **Source positions.** Decode through `yaml.Node` and record each step's file and line; add them to events and to the duplicate-name error. Not needed for stepping.

## IDE side (see the markov-ide ledger)

Identify steps by position (ordinal) so duplicate names stay distinct; sync files changed on disk (on focus, save and run); breakpoints in the gutter and on graph nodes with a right-click menu; a debugger toolbar (Continue, Step, Run to here, Stop), a paused-step highlight, a variables panel with watch expressions; then an agent interface so Claude can drive the same session.

## Out of scope for the first version

Editing variables while paused, hot-patching a step's definition inside a running process (edit, then rewind-and-resume instead), breakpoints inside a running `claude` step, pausing in sub-workflows launched by `for_each` across hosts (they pause like any other step, on the node running them).

## Task order

1. `breakpoint-engine`: core, CLI, events, tests, docs (done).
2. `step-definition-hash-and-rewind` (done).
3. `markov-schema` (done).
4. `step-source-positions`.

Decision record: [ADR-0005](../decisions/ADR-0005-in-memory-debugger-and-control-protocol.md).
