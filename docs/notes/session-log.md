# Session Log

## 2026-07-01

Agent: codex

Completed:

- Created ledger plan structure for directory workflow file support.
- Added `PLAN.md`, milestone, ADR, and pending task files.
- Moved detailed feature planning into `docs/plans/000-directory-workflow-files.md`.

Discovered:

- Repository did not yet have the ledger directories described in `docs/agentic_work_ledger.md`.

Next:

- Directory workflow support is implemented and verified with focused tests.
- Full `go test ./...` still cannot complete in this sandbox because callback tests require local TCP listeners.

## 2026-07-13

Agent: codex

Fixed:

- CLI `--var` values now use canonical Markov type coercion, preventing the
  string `"false"` from evaluating truthy in `when` conditions.

Verified:

- Repository `make test` passed with callback listener access.
- Platform `make host-rebuild-markov` imported the rebuilt image into k3s.
- Live run `markov-run-622d5874` skipped `main.run_pipeline` for
  `--var run_pipeline=false` and created no pipeline child steps.
