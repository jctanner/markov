# Task: markov schema

## Status

Done

## Summary

`markov schema` prints the workflow format as JSON so editors and agents stop keeping hand-written copies (a hand-written list is how the IDE once showed `set_fact` as taking `params`). See [plan 001](../../plans/001-step-through-debugging.md).

## Files

- `pkg/schema/schema.go`: `Build()` returns `{version, file, workflow, step, step_types}`. File, workflow and step fields are reflected from the parser structs (`yaml` tags, coarse types). Each built-in step type has its inputs location (`params`, or `step` with `step_fields` for `set_fact`, `gate`, `load_artifact`, `assert`), a summary, and its parameters (names, required flag, short doc).
- `pkg/parser/parser.go`: `PrimitiveNames()` exposes the built-in type list.
- `cmd/markov/main.go`: the `schema` command. Docs: `docs/reference/cli.md`.

## Verification

- `pkg/schema` tests: the schema's step types equal the parser's primitives (each once, with a summary and consistent inputs), fields are reflected with the right types (and `ScriptDir` is excluded), every parameter appears in its type's section of `docs/reference/step-types.md`, and every `step_fields` entry is a real field of `Step`. They passed against the current docs on the first run, so the table and docs agree today and will fail together if they drift. `make lint` and the full suite pass.
- Real binary: the output has the 15 built-in types with the expected parameter counts (claude 22, ansible_playbook 25, ...).

## Limitations

- The parameter table is hand-written (the executors have no central registry); the docs test is what keeps it honest. `jev` and `llm_invoke` list no parameters. Custom `step_types` from a workflow are not included.
