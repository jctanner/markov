# Task: Templated sub-workflow names, write_file, markov_run_id

## Status

Done (2026-10-10)

## Summary

| Addition | Why |
| --- | --- |
| Templated `workflow:` | One step can call different workflows, for example `submit-{{ test.arm }}` under a fan-out, instead of one guarded step per case. |
| `write_file` step type | Save results to a mounted volume without an inline script. Content is a template or a value (a map or list becomes indented JSON); no size limit; `mode`/`dir_mode` applied exactly; new directories keep a setgid parent's bit; `append`. |
| `markov_run_id` built-in var | Unique output paths per run. Saved with the run's vars, so a resume sees the same ID. |

## Templated names and state

Step state is keyed by run ID, workflow name and step name; sub-run IDs are
`<run>-<step>` or `<run>-<step>-<key>`. A templated name is therefore safe
while it resolves the same way.

If a resume makes it resolve to a different workflow, the other workflow's
finished steps would be ignored, and its work redone or mixed in. So:

- the resolved name is the sub-run's recorded entrypoint (it already was
  for static names);
- before a sub-run starts, `checkSubRunWorkflow` compares it with the
  stored sub-run and fails with an explicit error: start a new run, or
  rewind to before the step;
- validation skips the existence check for templated names, and an unknown
  resolved name fails the step, naming both the template and the result;
- `type:` stays static, so its params can still be validated.

## Verification

- `go build ./...`, `gofmt -l` clean, `go test ./...` pass.
- New tests:
  - `TestTemplatedSubWorkflowPerItem`;
  - `TestTemplatedSubWorkflowUnknownName`;
  - `TestResumeRefusesSubRunInADifferentWorkflow` (the resume is refused,
    and resuming with the same resolution works);
  - `TestMarkovRunIDVar`;
  - `TestWriteFileCreatesDirectoriesWithModes`;
  - `TestWriteFileAppendAndErrors`.
- Docs:
  - `step-types.md`: a `write_file` section, a common-fields note on
    templated `workflow`, and fifteen primitives;
  - `workflow-file.md`: the step field and the primitive list;
  - `variables-and-context.md`: built-in variables, and a caution that
    all-digit `--var` values become numbers.
- The schema lists `write_file` and `script_exec` `stdin`.

## Follow-up: workflow_names

A templated name can't be drawn or checked until it runs.
`workflow_names: [submit-workflow, submit-bash]` lists the workflows it may
resolve to:

- each must exist when the definition loads;
- a run that resolves to another name fails the step, naming the expected
  set;
- the list is only allowed with a templated `workflow`;
- markovd's diagram draws the listed workflows as the step's alternatives.

Tests: `TestWorkflowNamesLimitTemplatedTargets` and
`TestParseValidatesWorkflowNames`. Documented in `workflow-file.md` and in
the common fields of `step-types.md`.
