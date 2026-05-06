{% raw %}
# Artifacts

Artifacts load files from the local filesystem or Kubernetes pods into the workflow context as structured data.

## Two ways to load artifacts

### 1. Dedicated load_artifact step

A step with `type: load_artifact` exists solely to load files. It has no executor -- it just reads and parses files.

```yaml
steps:
  - name: load-config
    type: load_artifact
    artifacts:
      config:
        path: /etc/myapp/config.yaml
        format: yaml
```

Context result: `ctx["load-config"] = {"artifacts": {"config": {parsed data}}}`

### 2. Inline artifacts on any step

Any step that has an executor (shell_exec, k8s_job, http_request, etc.) can include an `artifacts` field. Artifacts are loaded **after** the executor completes successfully.

```yaml
steps:
  - name: generate-report
    type: shell_exec
    params:
      command: "./gen-report.sh --output /tmp/report.yaml"
    artifacts:
      report:
        path: /tmp/report.yaml
        format: yaml
```

Context result: `ctx["generate-report"] = {"artifacts": {"report": {parsed data}}, plus executor output fields}`

## Artifact fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | (required) | File path, template-rendered against current context |
| `format` | string | raw text | Parse format: `yaml`, `markdown`, `markdown_table`, or omit for plain text |
| `source` | string | auto-detect | `local` or `k8s`; auto-detects based on whether a K8s client and namespace are available |
| `optional` | bool | `false` | If true, return nil instead of failing when the file is missing |

## Formats

### yaml

Parsed via `yaml.Unmarshal` into `map[string]any`.

```yaml
# If /tmp/scores.yaml contains:
# title: "My Report"
# score: 85
# tags:
#   - alpha
#   - beta

artifacts:
  result:
    path: /tmp/scores.yaml
    format: yaml
```

Access in templates:

```
{{ load_scores.artifacts.result.title }}   ->  "My Report"
{{ load_scores.artifacts.result.score }}    ->  85
```

### markdown

Split into YAML frontmatter and body content. The parser looks for an opening `---` line, reads lines until a closing `---`, parses the block as YAML, and treats the rest as content.

```yaml
# If /tmp/strategy.md contains:
# ---
# status: approved
# owner: alice
# ---
# This strategy covers the migration plan...

artifacts:
  doc:
    path: /tmp/strategy.md
    format: markdown
```

Result structure:

```json
{
  "frontmatter": {"status": "approved", "owner": "alice"},
  "content": "This strategy covers the migration plan..."
}
```

Access in templates:

```
{{ load_doc.artifacts.doc.frontmatter.status }}  ->  "approved"
{{ load_doc.artifacts.doc.content }}              ->  "This strategy covers..."
```

If the file does not start with `---`, frontmatter is nil and the entire content is returned as the content field.

### markdown_table

Parsed into rows and column aggregates. Designed for piped Markdown tables.

```yaml
# If /tmp/results.md contains:
# | Component | Status  | Score |
# |-----------|---------|-------|
# | api       | passing | 95    |
# | worker    | failing | 42    |

artifacts:
  results:
    path: /tmp/results.md
    format: markdown_table
```

Result structure:

```json
{
  "rows": [
    {"component": "api", "status": "passing", "score": "95"},
    {"component": "worker", "status": "failing", "score": "42"}
  ],
  "component": "api,worker",
  "status": "passing,failing",
  "score": "95,42"
}
```

Parsing rules:

- Only lines starting with `|` are processed.
- The first matching line becomes the header row.
- Headers are converted to snake_case (spaces and hyphens become underscores, lowercased).
- Separator rows (cells that are all dashes/spaces) are skipped.
- Each column name also gets a top-level key with all values joined by commas.

### Default (no format or unrecognized)

The file content is stored as a plain string.

```yaml
artifacts:
  raw_log:
    path: /tmp/output.log
```

Access: `{{ step.artifacts.raw_log }}` returns the entire file as a string.

## Source resolution

| `source` value | Behavior |
|----------------|----------|
| `"local"` | Read from local filesystem via `os.ReadFile` |
| `"k8s"` | Read from running pods in the namespace via exec |
| not set | Use K8s if client is available and namespace is set; otherwise fall back to local |

### K8s artifact reading

When source is `k8s` (or auto-detected):

1. Lists up to 20 running pods in the namespace (`status.phase=Running`).
2. Iterates through each pod's container statuses, checking for readiness.
3. Executes `cat <path>` via SPDY exec in the first ready container.
4. Returns the output from the first successful exec.
5. If no pod has the file, fails with `artifact not found at <path> in any running pod in <namespace>`.

```yaml
# Read a config file from a running pod
artifacts:
  live_config:
    path: /app/config/settings.yaml
    format: yaml
    source: k8s
```

## Context placement

### load_artifact step

```
ctx[stepname] = {
  "artifacts": {
    "artifact_name_1": <parsed data>,
    "artifact_name_2": <parsed data>
  }
}
```

### Inline artifacts on executor steps

```
ctx[stepname] = {
  "artifacts": {
    "artifact_name": <parsed data>
  },
  // plus executor output fields (stdout, rc, etc.)
}
```

The executor output fields and artifact data are merged into a single map under the step name.

## Optional artifacts

When `optional: true`, missing files produce a nil value instead of failing the step. This applies to both local file-not-found errors and K8s "not found" errors.

```yaml
artifacts:
  maybe_config:
    path: /etc/myapp/override.yaml
    format: yaml
    optional: true
```

In templates, check for nil before accessing:

```yaml
when: "load_step.artifacts.maybe_config != nil"
```

## Examples

### YAML artifact with conditional check

```yaml
steps:
  - name: load-scores
    type: load_artifact
    artifacts:
      report:
        path: "/tmp/{{ run_id }}/scores.yaml"
        format: yaml

  - name: check-quality
    type: assert
    when: "load_scores.artifacts.report.score >= 80"
    that:
      - "load_scores.artifacts.report.failures == 0"
    msg: "Quality gate failed"
```

### Markdown artifact for strategy docs

```yaml
steps:
  - name: load-strategy
    type: load_artifact
    artifacts:
      strategy:
        path: "{{ workspace }}/strategy.md"
        format: markdown

  - name: check-approved
    type: assert
    that:
      - "load_strategy.artifacts.strategy.frontmatter.status == 'approved'"
    msg: "Strategy not yet approved"
```

### Optional artifact with fallback

```yaml
steps:
  - name: load-overrides
    type: load_artifact
    artifacts:
      overrides:
        path: /etc/myapp/overrides.yaml
        format: yaml
        optional: true

  - name: set-defaults
    type: set_fact
    when: "load_overrides.artifacts.overrides == nil"
    vars:
      replicas: 3
      timeout: 30
```
{% endraw %}
