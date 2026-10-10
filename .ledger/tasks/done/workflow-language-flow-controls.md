# Task: Language additions for data-driven benchmark workflows

## Status

Done (2026-10-10)

## Summary

Breadboard's strat-workflow benchmark (`var/benchmarks/strat-workflow/` in
Breadboard) first had to hand most of its control flow to Python scripts:
planning the runs, submitting jobs, waiting, and tolerating a failed job.
These additions let it be written in Markov steps instead.

| Addition | Why |
| --- | --- |
| `ignore_errors` on steps | `k8s_job_wait` fails the step when the Job fails, and nothing could catch it (`rescue` doesn't recover). A benchmark records a failed test and goes on. The output keeps the step's own fields plus `failed: true` and `error`. It works on executor steps, sub-workflows and `for_each` (every item runs). |
| `for_each_when` | Per-item filter, with the item bound to `as`. `when` is evaluated once, before the loop, so it couldn't select items such as "only S1 and S2". |
| `description` on steps | Long-form notes. Steps had no such field, and `description` on a step was a strict-YAML error. It is excluded from the step definition hash, so editing it doesn't count as a change on resume. |
| Native exact expressions | `"{{ expr }}"` keeps the expression's type for any expression: filters, arithmetic, `map[key]`. Before, only plain paths and `from_json`/`to_json` stayed native; a filter result rendered to its printed form. Implemented by binding the expression with `{% with %}` and passing it to a capture filter. Undefined still renders as `""`. |
| `for_each` over expressions | List resolution evaluates the expression natively, so `repeats \| seq` and `variants \| csv \| default:own` work. |
| `csv` and `seq` filters | `"S1, S2" \| csv` gives `["S1", "S2"]` (and `""` an empty list, so `default` applies); `3 \| seq` gives `[1, 2, 3]`. pongo2's own `split` keeps spaces and empty items, so it isn't overridden. |
| Loop item inside sub-workflows | A `for_each` sub-workflow now sees the current item under its `as` name, without re-passing it through `vars`. |
| Autoescape off | pongo2 HTML-escaped every value (`"` became `&quot;`), corrupting JSON passed to scripts and shell commands. Markov never renders HTML. |
| `script_exec` scalar args and env | `--var repeats=2` arrives as an integer, and `script_exec` rejected a non-string argument. Numbers and booleans are now passed as their text. |
| `vars/*.yaml` in directory workflows | Large data (test lists, arms, variants) can live in its own files, merged after `vars.yaml`; a variable may be defined once. |

## Behaviour changes to note

- An exact expression that used to fall back to string rendering now returns
  a native value, for example `{{ n + 1 }}` gives `3`, not `"3"`.
- Output is no longer HTML-escaped. A workflow that relied on `&lt;` etc. would
  see raw characters; none of the examples did.

## Verification

- `go build ./...`, `go vet ./pkg/...`, `gofmt -l` clean, `go test ./...`
  pass.
- New tests:
  - `pkg/template/native_test.go`: no escaping, `csv`, `seq`, native exact
    expressions, `Eval`.
  - `pkg/engine/engine_flow_controls_test.go`: `ignore_errors` on a step
    (and the run failing without it), `ignore_errors` on `for_each` with a
    sub-workflow, the loop item visible in the sub-workflow,
    `for_each_when`, `for_each` over filter expressions, description not
    changing the definition hash.
  - `pkg/parser`: `vars/` merge and duplicate rejection; `for_each_when`
    without `for_each` rejected; `description` and `ignore_errors` accepted.
  - `pkg/executor`: scalar `args`/`env` for `script_exec`.
- Reference docs updated: `workflow-file.md` (step fields, `vars/`),
  `for-each.md`, `template-engine.md`, `step-types.md` (`script_exec`,
  `k8s_job_wait` with `ignore_errors`).
