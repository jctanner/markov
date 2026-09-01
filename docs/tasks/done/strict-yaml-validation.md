# Task: Add Strict YAML Validation

## Goal

Reject unsupported fields in Markov's structured YAML DSL instead of silently
ignoring them, while preserving free-form executor parameter maps.

## Acceptance Criteria

- [x] Reject unknown top-level, workflow, step, rule, and step-type fields.
- [x] Preserve free-form values under `params`, `vars`, `facts`, and artifacts.
- [x] Report useful source context in parser errors.
- [x] Add focused parser tests and update reference documentation.

## Status

Done

## Verification

- `GOCACHE=/tmp/go-build go test ./pkg/parser ./pkg/engine ./pkg/state ./pkg/source ./cmd/markov`
- `GOCACHE=/tmp/go-build go vet ./...`
- `GOCACHE=/tmp/go-build make build`
- `bin/markov validate examples/pause-resume.yaml`
