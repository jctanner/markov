{% raw %}
# Built-in Step Types

Markov ships with eight primitive step types. Every step in a workflow must resolve to one of these primitives, either directly or through a [custom step type](custom-step-types.md).

All step types support these common fields:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string, required | Unique step name within the workflow |
| `type` | string | The step type (primitive or custom) |
| `when` | string | Pongo2 boolean expression; step is skipped if false |
| `register` | string | Store the step output in the workflow context under this key |
| `timeout` | int | Maximum execution time in seconds |
| `for_each` | string | Context path to a list; runs the step once per item |
| `for_each_key` | string | Field on each item to use as iteration key (must be unique) |
| `for_each_sort` | string | Field on each item to sort by before iterating |
| `as` | string | Variable name for the current item (required when `for_each` is set) |
| `concurrency` | int | Max parallel iterations for `for_each` (defaults to global `forks`) |
| `workflow` | string | Name of a sub-workflow to invoke instead of running a type |

---

## shell_exec

Runs a shell command via `bash -c`. The command executes on the Markov runner host, not inside Kubernetes.

### Parameters

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | yes | Shell command to execute |

### Output Variables

| Variable | Type | Description |
|----------|------|-------------|
| `stdout` | string | Standard output from the command |
| `stderr` | string | Standard error from the command |
| `exit_code` | int | Process exit code |

### Failure Conditions

- The `command` param is empty or missing.
- The command exits with a non-zero exit code. The error message includes stderr.

### Examples

Simple echo:

```yaml
- name: greet
  type: shell_exec
  params:
    command: "echo 'hello from markov'"
```

Piped command with registered output:

```yaml
- name: count_pods
  type: shell_exec
  params:
    command: "kubectl get pods -n default --no-headers | wc -l"
  register: pod_count

- name: report
  type: shell_exec
  params:
    command: "echo 'Found {{ pod_count.stdout | trim }} pods'"
```

---

## k8s_job

Creates a Kubernetes `batch/v1` Job and polls for completion every 5 seconds. The pod's `RestartPolicy` is always `Never`.

### Parameters

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `image` | string, required | -- | Container image |
| `command` | string[] or string | -- | Container command (entrypoint) |
| `args` | string[] or string | -- | Container arguments |
| `env` | map[string]any | -- | Environment variables as key-value pairs |
| `secrets` | string[] | -- | Kubernetes Secret names to inject via `envFrom` / `secretRef` |
| `volumes` | list | -- | Volume specifications (see below) |
| `init_containers` | list | -- | Init container specifications (see below) |
| `resources` | map | -- | Resource requests/limits using standard Kubernetes quantities |
| `affinity` | map | -- | Pod affinity specification (see below) |
| `service_account` | string | -- | Kubernetes ServiceAccount name for the pod |
| `backoff_limit` | int | `0` | Kubernetes Job `.spec.backoffLimit` |
| `ttl_seconds` | int | `86400` | TTL in seconds after Job completion (`.spec.ttlSecondsAfterFinished`) |
| `image_pull_policy` | string | `IfNotPresent` | Image pull policy (`Always`, `IfNotPresent`, `Never`) |
| `namespace` | string | -- | Override the global namespace for this step |
| `name_prefix` | string | `markov` | Prefix for the auto-generated Job name |

#### Volume Specification

Each entry in the `volumes` list is a map with:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string, required | Volume name |
| `pvc` | string | PersistentVolumeClaim name (mutually exclusive with `config_map` and `secret`) |
| `config_map` | string | ConfigMap name |
| `secret` | string | Secret name |
| `mount` | string | Mount path inside the container |
| `read_only` | bool | Mount as read-only |

If none of `pvc`, `config_map`, or `secret` is specified, the volume is created as an `emptyDir`.

#### Init Container Specification

Each entry in the `init_containers` list is a map with:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string, required | Container name |
| `image` | string, required | Container image |
| `command` | string[] or string | Container command |
| `args` | string[] or string | Container arguments |
| `volume_mounts` | list | Each entry has `name` (string) and `mount_path` (string) |
| `image_pull_policy` | string | Defaults to `IfNotPresent` |

#### Affinity Specification

```yaml
affinity:
  pod_affinity:
    required:
      topology_key: kubernetes.io/hostname
      match_labels:
        app: my-app
```

This maps to a `requiredDuringSchedulingIgnoredDuringExecution` pod affinity term.

#### Resources Specification

```yaml
resources:
  requests:
    memory: 2Gi
    cpu: 500m
  limits:
    memory: 8Gi
    cpu: "2000m"
```

Values are parsed using Kubernetes standard quantity parsing (e.g., `500m`, `2Gi`, `"2000m"`).

### Auto-Injected Fields

These fields are set automatically by the engine and should not be specified in workflow YAML:

| Field | Description |
|-------|-------------|
| `_job_name` | Sanitized Kubernetes name, max 63 characters. Format: `{name_prefix}-{run_id}-{step_name}`. If the raw name exceeds 63 characters, it is truncated and a SHA-256 hash suffix is appended. |
| `_labels` | Labels applied to the Job and Pod: `app=markov`, `markov/run-id`, `markov/workflow`, `markov/step` |

### Output Variables

| Variable | Type | Description |
|----------|------|-------------|
| `job_name` | string | Name of the created Kubernetes Job |
| `namespace` | string | Namespace the Job was created in |
| `logs` | string | Pod logs, up to 64 KB |

### Failure Conditions

- The `image` param is empty or missing.
- The Kubernetes API rejects the Job creation request.
- The Job's status condition becomes `Failed`.
- The step `timeout` expires or the parent context is cancelled.

### Example

Complete k8s_job with volumes and secrets:

```yaml
- name: run_analysis
  type: k8s_job
  timeout: 600
  params:
    image: pipeline-agent:latest
    command: ["/bin/bash", "/app/scripts/run_skill.sh"]
    args: ["--issue", "{{ issue }}", "--model", "opus"]
    namespace: markov-pipelines
    service_account: pipeline-runner
    backoff_limit: 0
    ttl_seconds: 3600
    image_pull_policy: Always
    env:
      ISSUE_KEY: "{{ issue }}"
      LOG_LEVEL: debug
    secrets:
      - pipeline-secrets
      - gcp-credentials
    volumes:
      - name: workspace
        pvc: pipeline-workspace-pvc
        mount: /app/workspace
      - name: config
        config_map: pipeline-config
        mount: /app/config
        read_only: true
      - name: creds
        secret: api-credentials
        mount: /app/secrets
        read_only: true
      - name: scratch
        mount: /tmp/scratch
    init_containers:
      - name: fetch-source
        image: alpine/git:latest
        command: ["/bin/sh", "-c"]
        args: ["git clone https://github.com/org/repo.git /workspace/src"]
        volume_mounts:
          - name: workspace
            mount_path: /workspace
    resources:
      requests:
        memory: 2Gi
        cpu: 500m
      limits:
        memory: 8Gi
        cpu: "2000m"
    affinity:
      pod_affinity:
        required:
          topology_key: kubernetes.io/hostname
          match_labels:
            app: pipeline-dashboard
  register: job_result
```

---

## http_request

Makes HTTP requests with automatic JSON body encoding and response parsing.

### Parameters

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `method` | string | `GET` | HTTP method (GET, POST, PUT, DELETE, etc.) |
| `url` | string | -- | Full request URL. Required if `base_url` is not set. |
| `base_url` | string | -- | Base URL, concatenated with `path` to form the full URL |
| `path` | string | -- | Path appended to `base_url` |
| `body` | any | -- | Request body, JSON-encoded automatically |
| `headers` | map[string]string | `{}` | Custom HTTP headers to add to the request |
| `basic_auth` | map | -- | HTTP Basic Auth credentials with `username` and `password` fields |
| `ignore_status` | bool or list[int] | -- | Treat matching HTTP error status codes as success. `true` ignores all `>= 400` responses. |
| `tls_insecure` | bool | `false` | Skip TLS certificate verification for HTTPS requests. Intended for local development or trusted test environments. |
| `tls_ca_cert` | string | -- | Path to a PEM-encoded CA certificate bundle to trust for this request. Useful for self-signed or private CA certificates. |

When a `body` is provided, the `Content-Type` header is set to `application/json`. A custom `Content-Type` value in `headers` overrides that default. If both `basic_auth` and a custom `Authorization` header are provided, `basic_auth` takes precedence.

For HTTPS endpoints with self-signed certificates, prefer `tls_ca_cert` when you have the CA certificate. Use `tls_insecure: true` only when certificate verification must be disabled.

Either `url` or `base_url` must be specified. When both `base_url` and `path` are given, they are concatenated directly (no slash is inserted).

### Output Variables

| Variable | Type | Description |
|----------|------|-------------|
| `status_code` | int | HTTP response status code |
| `body` | any | Response body. Auto-parsed as JSON if the response is valid JSON; otherwise returned as a string. |

### Failure Conditions

- Neither `url` nor `base_url` is provided.
- The HTTP request fails at the transport level (DNS, connection refused, etc.).
- The response status code is >= 400 and is not allowed by `ignore_status`. The step fails, but output variables (including the response body) are still populated.

### Examples

GET request with registered output:

```yaml
- name: fetch_issues
  type: http_request
  params:
    base_url: "https://issues.redhat.com/rest/api/2"
    path: "/search?jql=project=MYPROJECT&maxResults=50"
    method: GET
  register: issue_list

- name: report_count
  type: shell_exec
  params:
    command: "echo 'Found {{ issue_list.body.total }} issues'"
```

POST request with JSON body:

```yaml
- name: create_webhook
  type: http_request
  params:
    url: "https://api.example.com/webhooks"
    method: POST
    body:
      name: "pipeline-notify"
      url: "https://hooks.example.com/markov"
      events: ["job.completed", "job.failed"]
  register: webhook
```

Authenticated request with custom headers:

```yaml
- name: create_issue
  type: http_request
  params:
    base_url: "https://issues.example.com/rest/api/2"
    path: "/issue"
    method: POST
    basic_auth:
      username: "{{ jira_user }}"
      password: "{{ jira_password }}"
    headers:
      Accept: "application/json"
    body:
      fields:
        project:
          key: PIPE
        summary: "Pipeline-created issue"
        issuetype:
          name: Task
  register: issue
```

Token auth and tolerated status codes:

```yaml
- name: ensure_repo
  type: http_request
  params:
    base_url: "https://github.example.com/api/v3"
    path: "/orgs/{{ org }}/repos"
    method: POST
    headers:
      Authorization: "token {{ github_token }}"
      Accept: "application/vnd.github+json"
    ignore_status: [422]
    body:
      name: "{{ repo_name }}"
  register: repo_create
```

HTTPS request to an endpoint signed by a private CA:

```yaml
- name: fetch_internal_status
  type: http_request
  params:
    url: "https://internal.example.test/status"
    headers:
      Accept: "application/json"
    tls_ca_cert: "/etc/markov/certs/internal-ca.pem"
  register: internal_status
```

For disposable local environments, certificate verification can be disabled:

```yaml
- name: fetch_local_status
  type: http_request
  params:
    url: "https://github.local/api/v3"
    tls_insecure: true
  register: github_status
```

---

## gate

Evaluates named rules against the workflow context using the Grule rule engine. Gates control workflow progression by producing an action (`continue`, `skip`, or `pause`) and optionally setting facts in the context.

### Step Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `rules` | string[] | yes | Names of rules to evaluate (defined in the top-level `rules:` block) |
| `facts` | map[string]any | no | Additional context values to make available to rule conditions. Template expressions are rendered against the current context. |

### Rule Definition (top-level `rules:` block)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string, required | -- | Rule identifier, referenced by gate steps |
| `description` | string | -- | Human-readable description |
| `salience` | int | `0` | Priority (higher salience fires first) |
| `when` | string, required | -- | Condition expression (Pongo2 syntax) |
| `action` | string | `continue` | Action when the rule fires: `continue`, `skip`, or `pause` |
| `set_fact` | map[string]any | -- | Variables to set in the workflow context when the rule fires |

### Output Variables

| Variable | Type | Description |
|----------|------|-------------|
| `action` | string | The winning action: `continue`, `skip`, or `pause` |
| `fired_rules` | string[] | Names of all rules that fired |
| `facts` | map[string]any | Values set by `set_fact` across all fired rules |

### Evaluation Rules

1. All rules named in the `rules` list are compiled from Pongo2 conditions into GRL (Grule Rule Language) and evaluated together.
2. Rules fire in salience order. When a rule fires and sets facts, remaining rules are re-evaluated against the updated context.
3. The highest-salience fired rule determines the gate action.
4. `set_fact` values from all fired rules are merged back into the workflow context, making them available to downstream steps.
5. `pause` is logged but not yet implemented -- execution continues as if `continue` was returned.

### Scoping

Rules see global vars, workflow vars, CLI `--var` overrides, and `set_fact` values. They do **not** see step results, register outputs, or artifact data directly. To pass step data to a rule, map it through the gate's `facts` block:

```yaml
- name: quality_gate
  type: gate
  facts:
    score: "{{ analysis.artifacts.result.confidence }}"
  rules:
    - auto_approve
    - needs_review
```

This keeps rules portable across workflows.

### Failure Conditions

- A referenced rule name is not found in the top-level `rules:` block.
- A rule condition fails to compile to GRL.
- The Grule engine returns an execution error.

### Example

```yaml
rules:
  - name: tests_pass
    description: "Allow deployment when coverage meets threshold"
    salience: 100
    when: "test_coverage >= min_coverage"
    action: continue
    set_fact:
      tests_approved: true

  - name: tests_fail
    description: "Block deployment on insufficient coverage"
    salience: 200
    when: "test_coverage < min_coverage"
    action: skip
    set_fact:
      tests_approved: false
      block_reason: "Coverage below threshold"

workflows:
  - name: main
    steps:
      - name: test_gate
        type: gate
        rules:
          - tests_pass
          - tests_fail

      - name: deploy
        type: shell_exec
        when: "tests_approved"
        params:
          command: "echo 'Deploying...'"
```

---

## set_fact

Computes and stores variables in the workflow context. Unlike other step types, `set_fact` does not use `params` -- it uses the `vars` field directly on the step.

### Step Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `vars` | map[string]any | yes | Key-value pairs to evaluate and store in the context |

### Evaluation Rules

Each value in the `vars` map is evaluated according to its type:

| Value Type | Behavior |
|-----------|----------|
| String containing `{{` or `{%` | Rendered as a Pongo2 template, then coerced: `"true"` becomes `true` (bool), integer strings are parsed to `int`, JSON arrays/objects are parsed into native types |
| String with `{{ path \| fromjson }}` | The context path is resolved and its string value is parsed as JSON, preserving structure (maps, arrays, nested types) |
| Plain string (no template syntax) | Evaluated as a boolean expression via `{% if expr %}true{% endif %}` |
| Map with a `from` key | Table lookup (see below) |
| Any other type | Stored directly (int, bool, list, map) |

#### Table Lookup

When a value is a map containing a `from` key, it performs a table lookup:

| Field | Type | Description |
|-------|------|-------------|
| `from` | string, required | Context path to a list of maps |
| `match` | map[string]any, required | Filter criteria -- each key-value pair must match |
| `field` | string | Extract a single field from the matched row. If omitted, the entire row is returned. |
| `default` | any | Fallback value if no match is found or the source list is nil |

Match values can contain template expressions (e.g., `"{{ issue }}"`).

### Output

`set_fact` does not produce output in the `register` sense. The computed values are merged directly into the workflow context and are available to all subsequent steps.

### Failure Conditions

- The `vars` map is empty or not defined.
- A template expression fails to render.
- A boolean expression fails to evaluate.
- A table lookup `from` path resolves to a non-list type.
- A table lookup is missing the `match` field.

### Examples

Template rendering and arithmetic:

```yaml
- name: compute_vars
  type: set_fact
  vars:
    stage: "{{ stage + 1 }}"
    label: "{{ environment }}-{{ build_version }}"
    is_production: "environment == 'production'"
    health_status: "healthy"
```

The `stage` value rendered from `"{{ stage + 1 }}"` produces an integer string like `"3"`, which is coerced to the integer `3`.

The `is_production` value is a plain string without template delimiters, so it is evaluated as a boolean expression.

Table lookup:

```yaml
- name: find_owner
  type: set_fact
  vars:
    owner:
      from: team_roster.artifacts.members.rows
      match:
        component: "{{ component }}"
        role: lead
      field: email
      default: "unassigned@example.com"
```

JSON parsing with `fromjson`:

```yaml
- name: parse_result
  type: set_fact
  vars:
    parsed_data: "{{ job_output.stdout | fromjson }}"
```

---

## load_artifact

Loads files from the filesystem or from Kubernetes pods into the workflow context as structured data.

### Step Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `artifacts` | map[string]Artifact | yes | Named artifacts to load |

#### Artifact Specification

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string, required | -- | File path (template expressions are rendered against the context) |
| `format` | string | -- | How to parse the file: `yaml`, `markdown`, `markdown_table`, or omit for raw string |
| `source` | string | auto | Where to read: `local` (filesystem), `k8s` (exec `cat` in a running pod), or omit for auto-detection |
| `optional` | bool | `false` | If `true`, missing files produce `nil` instead of failing |

### Format Details

| Format | Parsed Result |
|--------|---------------|
| `yaml` | `map[string]any` -- parsed YAML document |
| `markdown` | `map[string]any` with two keys: `frontmatter` (parsed YAML between `---` delimiters) and `content` (body text after the second `---`) |
| `markdown_table` | `map[string]any` with `rows` (list of maps with snake_case headers) plus one key per column header containing a comma-joined string of all values in that column |
| (omitted) | `string` -- raw file contents |

### Context Placement

Loaded artifacts are placed in the context at `{step_name}.artifacts.{artifact_name}`. For example, a step named `load_config` with an artifact named `settings` is accessible as `load_config.artifacts.settings`.

### Source Auto-Detection

When `source` is omitted:
- If a Kubernetes client is available and the workflow has a `namespace` set, the engine attempts to read from running pods in that namespace using `kubectl exec cat`.
- Otherwise, it reads from the local filesystem.

For `k8s` source, the engine lists running pods in the workflow namespace (up to 20), iterates through containers with a `Ready` status, and tries `cat {path}` in each until one succeeds.

### Failure Conditions

- The `artifacts` map is empty.
- A file is missing and `optional` is `false`.
- A YAML file fails to parse.

### Example

```yaml
- name: load_config
  type: load_artifact
  artifacts:
    settings:
      path: "/app/config/pipeline.yaml"
      format: yaml
    readme:
      path: "/app/docs/README.md"
      format: markdown
      optional: true
    test_results:
      path: "/app/reports/coverage.md"
      format: markdown_table

- name: check_config
  type: assert
  that:
    - "load_config.artifacts.settings.log_level == 'debug'"
    - "load_config.artifacts.readme != None"
  msg: "Configuration validation failed"
```

Artifacts can also be loaded alongside executor steps. When a step with a type like `k8s_job` includes an `artifacts` field, the engine loads those artifacts after the job completes and merges them into the step's context entry alongside the executor output:

```yaml
- name: run_analysis
  type: k8s_job
  params:
    image: pipeline-agent:latest
    command: ["/bin/sh", "-c"]
    args: ["python /app/analyze.py --issue {{ issue }}"]
  artifacts:
    result:
      path: "/app/artifacts/{{ issue }}/result.yaml"
      format: yaml

# Access both executor output and artifacts:
# run_analysis.job_name, run_analysis.logs
# run_analysis.artifacts.result.confidence
```

---

## assert

Validates conditions and fails the workflow immediately if any condition evaluates to false. Use for precondition checks and invariant validation.

### Step Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `that` | string[] | yes | List of Pongo2 boolean expressions to evaluate |
| `msg` | string | no | Custom failure message. If omitted, the default message is `"assertion failed: {expression}"`. |

### Evaluation

Each expression in the `that` list is evaluated using `EvalBool`, which wraps the expression in `{% if expr %}true{% endif %}` and checks whether the result is `"true"`. Expressions are evaluated in order; the step fails on the first false expression.

### Failure Conditions

- The `that` list is empty.
- Any expression in `that` fails to parse or evaluate.
- Any expression in `that` evaluates to false.

### Example

```yaml
- name: preflight_checks
  type: assert
  that:
    - "build_version is defined"
    - "environments | length > 0"
    - "test_coverage >= min_coverage"
  msg: "Preflight checks failed -- verify build_version, environments, and test_coverage"

- name: verify_triage
  type: assert
  that:
    - "needs_review"
    - "not auto_approved"
    - "not rejected"
    - "not deferred"
  msg: "Triage gate did not produce expected results for severity={{ severity }}"
```

---

## llm_invoke

Reserved primitive for future LLM integration. Steps using this type will fail with `"no executor for type"`.

This type is recognized by the parser as a valid primitive, so workflow YAML referencing it will pass validation. However, no executor is registered for it at runtime.

```yaml
# This will parse successfully but fail at execution time.
- name: classify
  type: llm_invoke
  params:
    model: sonnet
    prompt: "Classify this issue..."
  register: classification
```
{% endraw %}
