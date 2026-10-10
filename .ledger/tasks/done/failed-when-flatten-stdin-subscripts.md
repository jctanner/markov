# Task: failed_when, flatten/pluck, script stdin, chained subscripts

## Status

Done (2026-10-10)

## Summary

The second round of gaps found while writing Breadboard's strat-workflow
benchmark. The first round is in
[workflow-language-flow-controls](workflow-language-flow-controls.md).

| Addition | Why |
| --- | --- |
| `failed_when` | Decide a step's failure from its output instead of the executor's exit, as in Ansible. It sees the output as `result` (and under the register name); an executor error is `result.error`. It works with `ignore_errors`. It isn't allowed on sub-workflow steps. |
| `flatten` and `pluck` filters | Results of nested `for_each` sub-workflows come back as nested lists of contexts. `pluck:"a.b"` takes one field from each item, and `flatten` joins the lists, so a summary step needs no nested `{% for %}`. |
| `script_exec` `stdin` | A single process argument is capped at about 128 KB on Linux. `stdin` has no cap; a map or list is sent as JSON. |
| Chained subscripts | pongo2 v6.0.0 stopped parsing a variable after `[subscript]`, so `tiers[tier].tests` failed. Markov now builds with a patched copy in `third_party/pongo2`: a one-line `continue` in `parseVariableOrLiteral`. Patches are listed in `third_party/pongo2/MARKOV_PATCHES.md`. The Dockerfile copies `third_party/` before `go mod download`. |

## Bug fixed on the way

`for_each` checked for an earlier failure before waiting for a free slot, so
with `concurrency: 1` the item after a failed one still started. It now
checks again after taking the slot, using an atomic flag set with the first
error. Found by Breadboard's `var/demos/markov-feature-check`. Test:
`TestForEachStopsAfterAFailureWithConcurrencyOne`.

## Found running it through markovd

- **`markov resume`** rejected `--verbose`, `--namespace`, `--kubeconfig`,
  `--forks` and `--debug`, and never set up the engine's Kubernetes client.
  It now takes the same execution flags as `run` (`3520707`; `cli.md`
  updated).
- **Numbers after a resume:** saved state was decoded with `json.Unmarshal`,
  so whole numbers came back as float64 and printed as `2.000000`. They are
  now restored as int (`88190a4`, `TestResumeKeepsWholeNumbersAsInts`).

## Docs

- `workflow-file.md`: the `failed_when` step field.
- `template-engine.md`: `flatten` and `pluck`, chained subscripts, and a
  section on Markov's pongo2: it is Django-style, has no inline `if`, and
  `or` returns a boolean.
- `step-types.md`: `script_exec` `stdin`.
- `for-each.md`: replaced the stale Go-template example
  (`range .items | toJson`) with filter examples, and corrected the
  description of the fallback.

## Verification

- `go build ./...`, `gofmt -l` clean, `go test ./...` pass.
- New tests:
  - `TestChainedSubscripts`;
  - `TestFlattenAndPluck`;
  - `TestFailedWhen`, five cases (success turned into failure, an executor
    error turned into success, the register name bound, the error text
    visible, combined with `ignore_errors`);
  - `TestScriptExecStdin`: 300 KB of input, and JSON for a list.
