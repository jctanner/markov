# Task: Implement Source Integrity Modes for Resumable Runs

## Goal

Fingerprint workflow source trees and apply the accepted `warn`, `strict`, and
`off` source-integrity policies when resuming a run.

## Acceptance Criteria

- [x] Persist the original source-tree digest at run creation.
- [x] Compare the current source tree before resume.
- [x] Reject mismatches in `strict` mode.
- [x] Record and expose accepted drift in `warn` mode.
- [x] Preserve `off` mode for compatibility.
- [x] Cover directory workflows and `scripts/` files in source hashing.
- [x] Keep SQLite and Postgres state behavior aligned.
- [x] Document CLI and state-store behavior.
- [x] Add focused tests.

## Status

Done

## Verification

- `GOCACHE=/tmp/go-build go test ./pkg/source ./pkg/state ./pkg/engine ./cmd/markov`
- `GOCACHE=/tmp/go-build go vet ./...`
- `GOCACHE=/tmp/go-build make build`
- A failed `examples/rescue-always.yaml` run resumed successfully through an
  unchanged `--source-integrity strict` check; `markov status` displayed the
  original digest and latest check.
