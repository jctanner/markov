# Markov

A Go-based YAML workflow engine for Kubernetes. Define workflows declaratively, and Markov executes them as K8s Jobs, HTTP requests, or shell commands with built-in concurrency, conditionals, and checkpoint/resume.

## Features

- **Declarative YAML workflows** — multiple workflows per file with an entrypoint
- **User-defined step types** — compose reusable types from engine primitives (`k8s_job`, `http_request`, `shell_exec`)
- **Fan-out / fan-in** — `for_each` with sliding-window concurrency control (`forks`)
- **Sub-workflows** — invoke named workflows inline, with or without `for_each`
- **Conditionals** — `when:` expressions to skip or run steps
- **Template rendering** — Jinja2-compatible (pongo2) for params and expressions
- **Checkpoint/resume** — SQLite state store; resume failed runs from the last successful step
- **K8s native** — creates `batch/v1` Jobs directly (no Argo dependency)

## Quick Start

```bash
go build -o markov ./cmd/markov

# Validate a workflow file
markov validate examples/k8s-job-test.yaml

# Run a workflow
markov run examples/k8s-job-test.yaml --namespace markov-test --verbose

# Check status
markov status <run-id> --steps

# Resume a failed run
markov resume <run-id>
```

## Example

```yaml
entrypoint: main
namespace: markov-test
forks: 2

vars:
  greeting: "hello from markov"
  items: ["alpha", "bravo", "charlie"]

step_types:
  echo_job:
    base: k8s_job
    job:
      image: alpine:3.19
      command: ["/bin/sh", "-c"]

workflows:
  - name: main
    steps:
      - name: hello
        type: echo_job
        params:
          args: ["echo '{{ greeting }}'"]

      - name: fan-out
        for_each: "items"
        as: item
        workflow: per-item
        vars:
          value: "{{ item }}"

  - name: per-item
    vars:
      value: null
    steps:
      - name: process
        type: echo_job
        params:
          args: ["echo 'processing: {{ value }}'"]
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `--var key=value` | Override workflow vars (repeatable) |
| `--workflow name` | Run a specific workflow instead of the entrypoint |
| `--forks N` | Override concurrency limit |
| `--namespace ns` | Override K8s namespace |
| `--kubeconfig path` | Path to kubeconfig |
| `--state-store path` | SQLite state file (default: `./markov-state.db`) |
| `--verbose` | Show detailed execution output |
| `--steps` | Show per-step status (with `status` command) |

## Project Structure

```
cmd/markov/          CLI entrypoint
pkg/engine/          Workflow execution engine
pkg/parser/          YAML parsing and validation
pkg/executor/        Step executors (k8s_job, shell_exec, http_request)
pkg/state/           Checkpoint store (SQLite)
pkg/template/        Pongo2 template rendering
examples/            Example workflow files
docs/                Design documents and research
```

## License

TBD
