# Directory Workflow Files Plan

## Goal

Allow Markov to run and validate a directory of workflow files while preserving the current single-file workflow format.

## Context

Markov workflow files are currently self-contained YAML documents. This is simple and should remain supported, but larger workflows need a maintainable layout where metadata, variables, rules, step types, and workflows can live in separate files.

## Proposed User Experience

Single-file mode continues to work:

```bash
markov run pipeline.yaml
markov validate pipeline.yaml
```

Directory mode works by path detection:

```bash
markov run ./pipeline
markov validate ./pipeline
```

Recommended layout:

```text
pipeline/
  meta.yaml
  vars.yaml
  rules.yaml
  step_types.yaml
  workflows/
    main.yaml
    deploy.yaml
    cleanup.yaml
```

## Directory File Contract

- `meta.yaml` contains top-level metadata: `entrypoint`, `namespace`, `forks`.
- `vars.yaml` contains the top-level `vars` map.
- `rules.yaml` contains the top-level `rules` list.
- `step_types.yaml` contains the top-level `step_types` map.
- `workflows/*.yaml` files each contain one workflow object directly, without a wrapping `workflows:` key.

Example workflow file:

```yaml
name: main
steps:
  - name: hello
    type: shell_exec
    params:
      command: "echo hello"
```

## Scope

- Keep the existing single-file parser path.
- Add directory loading that resolves to the same in-memory `WorkflowFile` structure used today.
- Run existing validation after merge.
- Preserve external `rules: - file: ...` support.
- Detect duplicate workflow, rule, and step type names as validation errors.

## Non-Goals

- Recursive discovery of arbitrary YAML files.
- Multiple files per category, except for one workflow per file under `workflows/`.
- Engine behavior changes.
- New workflow execution semantics.

## Open Detail: Relative Path Resolution

Relative path resolution controls how paths inside split files are interpreted.

Simple example:

```text
pipeline/
  rules.yaml
  workflows/
    main.yaml
  artifacts/
    report.yaml
```

If `workflows/main.yaml` contains:

```yaml
steps:
  - name: load_report
    type: load_artifact
    artifacts:
      report:
        path: artifacts/report.yaml
        format: yaml
```

There are two possible meanings:

- Directory-root-relative: `pipeline/artifacts/report.yaml`
- File-relative: `pipeline/workflows/artifacts/report.yaml`

Recommendation for v1: paths owned by the workflow language, such as external rule includes, should resolve from the workflow directory root for consistency. Runtime artifact paths should remain runtime paths exactly as written, because they may refer to container paths such as `/app/artifacts/result.yaml`.

## Acceptance Criteria

- `markov validate <file.yaml>` behavior is unchanged.
- `markov validate <directory>` validates a split workflow project.
- `markov run <directory>` executes the merged workflow.
- Directory mode rejects missing required files with clear errors.
- Directory mode rejects duplicate workflow, rule, and step type names.
- Existing external rule file support still works.
- Docs and examples demonstrate the directory layout.
- `examples/dir-based-hello-world/` provides a minimal runnable directory workflow.
