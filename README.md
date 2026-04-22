<img width="1173" height="382" alt="markov-logo" src="https://github.com/user-attachments/assets/18feb654-2d5c-47fc-993c-cc95afa2cbf7" />


# Markov

A Go-based YAML workflow engine for Kubernetes. Define workflows declaratively, and Markov executes them as K8s Jobs, HTTP requests, or shell commands with built-in concurrency, conditionals, and checkpoint/resume.

## Why "Markov"?

A [Markov chain](https://en.wikipedia.org/wiki/Markov_chain) is a system that transitions between states based on its current state. Markov workflows work the same way: gate steps evaluate rules against the current state, decide the next transition (continue, skip, or pause), and workflows can recursively invoke themselves — looping until the rules say to stop. The result is a declarative state machine where each iteration's next move depends only on the facts right now, not the history of how it got there.

## Features

- **Declarative YAML workflows** — multiple workflows per file with an entrypoint
- **User-defined step types** — compose reusable types from engine primitives (`k8s_job`, `http_request`, `shell_exec`, `load_artifact`)
- **Fan-out / fan-in** — `for_each` with sliding-window concurrency control (`forks`)
- **Sub-workflows & recursion** — invoke named workflows inline; workflows can call themselves to loop, controlled by gate rules
- **Conditionals** — `when:` expressions to skip or run steps
- **Template rendering** — Jinja2-compatible (pongo2) for params and expressions
- **Artifact loading** — load YAML, markdown, and markdown table files from local or K8s volumes; use parsed data in conditions
- **`set_fact`** — compute and store variables from expressions or table lookups for use in downstream steps
- **`assert`** — validate conditions and fail the workflow with a message if any are false
- **Rule engine / gates** — define named rules with salience-based priority; gate steps evaluate rules via [Grule](https://github.com/hyperjumptech/grule-rule-engine) with forward chaining, set facts, and control flow (continue/skip/pause)
- **Checkpoint/resume** — SQLite state store; resume failed runs from the last successful step
- **K8s native** — creates `batch/v1` Jobs directly (no Argo dependency)

## How is this different from X?

See [docs/project-comparisons.md](docs/project-comparisons.md) for detailed comparisons with Ansible, Argo Workflows, Jenkins, Tekton, Kestra, CrewAI, and LangGraph.

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

# Generate a Mermaid diagram of a completed run
markov diagram <run-id>
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
pkg/engine/          Workflow execution, gate evaluation, artifact loading, facts
pkg/parser/          YAML parsing, validation, rule loading
pkg/executor/        Step executors (k8s_job, shell_exec, http_request)
pkg/state/           Checkpoint store (SQLite)
pkg/template/        Pongo2 template rendering
examples/            Example workflow files
docs/                Design documents and research
```

## License

TBD
