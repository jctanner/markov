# Task: Add ansible and ansible_playbook step types

## Status

Done

## Summary

Add first-class `ansible_playbook` and `ansible` (ad-hoc) primitives that run
the Ansible CLIs directly, without a shell.

## Design

- Two step types, because their input shapes differ: `ansible_playbook`
  takes `playbook`, `tags`, `skip_tags`, `start_at_task`; `ansible` takes
  `pattern`, `module`, `module_args`, `poll`, `background`.
- Shared params: `chdir`, `inventory` (string | list | inline map written to a
  temp YAML file), `limit`, `extra_vars` (map written to a 0600 temp JSON file
  so values stay off the process list), `extra_vars_files`, `check`, `diff`,
  `become*`, `connection`, `private_key`, `vault_*`, `forks`, `timeout`,
  `verbosity`, `env`, `extra_args` (escape hatch), `binary`.
- Output: `stdout`, `stderr`, `exit_code`, `changed`; `ansible_playbook` also
  returns a parsed `recap` map from `PLAY RECAP`.
- Non-zero exit fails the step. `ANSIBLE_NOCOLOR=1` is set by default.
- Absolute paths are allowed (unlike `script_exec`) because the user supplies
  the playbook and `chdir` explicitly and k8s runners mount them anywhere.
- Structured per-task results (JSON stdout callback) are deliberately not
  included in v1.

## Files

- `pkg/executor/ansible.go`, `pkg/executor/ansible_test.go`
- `pkg/parser/parser.go` (primitives), `cmd/markov/main.go` (executors)
- `docs/reference/step-types.md`, `workflow-file.md`, `concepts.md`,
  `custom-step-types.md`; `examples/ansible.yaml`

## Verification

- `go vet ./...`, `gofmt -l`, and `go test` for executor, parser, engine and
  cmd pass. Executor tests use a fake binary via `binary` to assert argv,
  working directory, env, temp-file contents and cleanup, recap parsing, and
  failure output.
- `markov validate examples/ansible.yaml` passes.
- Not verified against a real Ansible install (none available in the
  development environment).
