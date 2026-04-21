# Agent Runner: How Jobs Execute

This document describes the full lifecycle of a pipeline job — from the
dashboard submit button to the Claude agent running inside a Kubernetes pod.

## Overview

```
Dashboard UI ──POST──► Flask API ──► K8s Orchestrator ──► K8s Job
                                         (Python)           │
                                                            ├── Init: update-ca-trust (alpine:3.19)
                                                            └── Agent: pipeline-agent:latest
                                                                  └── run_skill.sh or run_skill_sdk.sh
                                                                        └── claude --print "/skill.name ..."
```

## 1. Job Submission

The user submits a job from the dashboard's Jobs page by selecting a skill,
optionally entering an issue key, choosing a model and runner, and clicking
Submit. The Re-run button on existing jobs does the same thing with the
original job's parameters.

**Frontend → API call:**

```javascript
POST /api/jobs/submit
{
  "command": "strategy-refine-fork",   // skill key from pipeline-skills.yaml
  "args": {
    "issue": "RHAISTRAT-2",           // optional — omit for batch skills
    "model": "opus",                   // opus | sonnet | haiku
    "runner": "cli"                    // cli | sdk
  }
}
```

**API handler** (`lib/webapp.py`): Passes the request to
`PipelineOrchestrator.submit_phase_job()`.

## 2. Job Manifest Generation

`PipelineOrchestrator._create_job_manifest()` in `lib/k8s_orchestrator.py`
builds a `batch/v1` Job object programmatically (not from a YAML template).

### Job naming

```
{skill_key}-{issue_key}-{model}-{MMdd-HHmmss}
```

Example: `strategy-refine-fork-rhaistrat-2-opus-0421-154853`

All characters lowercased, underscores replaced with hyphens.
When no issue key is provided, `all` is used instead.

### Labels

```yaml
labels:
  app: pipeline-agent
  phase: strategy-refine-fork    # skill key
  issue: rhaistrat-2             # lowercase, or "all"
  model: opus
  runner: cli
```

### Entrypoint

The runner type determines which script is invoked:

| Runner | Entrypoint |
|--------|------------|
| `cli`  | `/bin/bash /app/scripts/run_skill.sh --skill <key> [--issue <key>] --model <model>` |
| `sdk`  | `/bin/bash /app/scripts/run_skill_sdk.sh --skill <key> [--issue <key>] --model <model>` |

### Job spec

```yaml
spec:
  ttlSecondsAfterFinished: 86400   # auto-delete after 24 hours
  backoffLimit: 0                   # no retries
  template:
    spec:
      restartPolicy: Never
```

### Pod affinity

Jobs are scheduled on the same node as the dashboard pod. This is required
because the PVCs use `ReadWriteOnce` with `local-path` storage — they can
only be mounted on one node at a time.

```yaml
affinity:
  podAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchLabels:
          app: pipeline-dashboard
      topologyKey: kubernetes.io/hostname
```

## 3. Container Images

### Init container: `update-ca-trust`

- **Image:** `alpine:3.19`
- **Purpose:** Builds a CA certificate bundle that includes any internal CA
  from the `internal-ca-cert` ConfigMap. The resulting
  `ca-certificates.crt` is written to an `emptyDir` volume (`shared-ca`)
  that the agent container reads.
- **Why:** The agent makes HTTPS calls to Vertex AI and Jira. In
  environments with TLS-intercepting proxies or internal CAs, the default
  trust store won't work.

### Agent container: `pipeline-agent:latest`

- **Image:** Built from `deploy/pipeline-agent/Dockerfile`, imported into
  k3s with `ctr images import` (never pulled from a registry).
- **Base:** `python:3.13-slim`
- **System packages:** `git`, `curl`, `nodejs`, `npm`, `ca-certificates`
- **Global npm:** `@anthropic-ai/claude-code` (the Claude CLI)
- **Python deps:** Installed via `uv sync` from `pyproject.toml`/`uv.lock`
- **User:** `pipelineagent` (non-root, UID auto-assigned)

**Baked-in application files:**

| Path in image | Source |
|----------------|--------|
| `/app/main.py` | `main.py` |
| `/app/lib/` | `lib/` (skill_config, agent_runner, etc.) |
| `/app/scripts/` | `scripts/` (run_skill.sh, run_skill_sdk.sh, stream-claude.py) |
| `/app/.claude/` | `.claude/` (local skill definitions) |
| `/app/pipeline-skills.yaml` | Skill-to-source mapping config |
| `/app/skills-registry/` | Local staging marketplace (registry.yaml + marketplace.json) |

**Resource limits:**

```yaml
requests: { memory: 2Gi, cpu: 500m }
limits:   { memory: 8Gi, cpu: 2000m }
```

## 4. Secrets

All secrets come from a single K8s Secret named `pipeline-secrets` in the
`ai-pipeline` namespace.

| Env var | Secret key | Purpose |
|---------|-----------|---------|
| `CLAUDE_CODE_USE_VERTEX` | `CLAUDE_CODE_USE_VERTEX` | Enable Vertex AI provider (`1`) |
| `CLOUD_ML_REGION` | `CLOUD_ML_REGION` | GCP region (e.g., `us-east5`) |
| `ANTHROPIC_VERTEX_PROJECT_ID` | `ANTHROPIC_VERTEX_PROJECT_ID` | GCP project ID |
| `JIRA_SERVER` | `JIRA_SERVER` | Jira base URL |
| `JIRA_USER` | `JIRA_USER` | Jira username/email |
| `JIRA_TOKEN` | `JIRA_TOKEN` | Jira API token |
| `ATLASSIAN_MCP_URL` | `ATLASSIAN_MCP_URL` | MCP server SSE URL (optional) |

Additionally, GCP service account credentials are mounted from a separate
secret:

| Secret | Mount path | Purpose |
|--------|-----------|---------|
| `gcp-credentials` | `/home/pipelineagent/.config/gcloud/` | Service account JSON for Vertex AI auth |

The `GOOGLE_APPLICATION_CREDENTIALS` env var is set to
`/home/pipelineagent/.config/gcloud/credentials.json`.

## 5. ConfigMaps

| ConfigMap | Mount path | Purpose |
|-----------|-----------|---------|
| `internal-ca-cert` | `/tmp/ca-cert/` (init container) | Optional internal CA certificate (`ca.crt`). Used by init container to build the trust bundle. |

## 6. Volumes

### Persistent Volume Claims

All PVCs use `local-path` storage class (k3s default) with
`ReadWriteOnce` access mode.

| PVC name | Mount path | Size | Contents |
|----------|-----------|------|----------|
| `pipeline-issues` | `/app/issues` | 5Gi | Raw Jira issue JSON |
| `pipeline-workspace` | `/app/workspace` | 50Gi | Cloned repos and phase outputs per issue |
| `pipeline-logs` | `/app/logs` | 10Gi | Activity logs and phase logs |
| `pipeline-artifacts` | `/app/artifacts` | 5Gi | RFE/strategy artifacts (tasks, reviews, originals) |
| `pipeline-context` | `/app/.context` | 10Gi | Architecture context repos (git sparse-checkouts) |

These PVCs are shared between the dashboard deployment and all agent job
pods. The pod affinity rule ensures they run on the same node.

### Ephemeral volumes

| Volume | Type | Mount path | Purpose |
|--------|------|-----------|---------|
| `shared-ca` | `emptyDir` | `/shared` (read-only in agent) | CA certificate bundle produced by init container |
| `ca-cert` | ConfigMap (`internal-ca-cert`) | `/tmp/ca-cert` (init only) | Source CA cert for init container |

## 7. RBAC

The dashboard needs permissions to create and manage jobs on behalf of the
user. Defined in `deploy/k8s/16-pipeline-rbac.yaml`:

- **ServiceAccount:** `pipeline-dashboard`
- **Role:** `pipeline-orchestrator` — grants `get`, `list`, `watch`,
  `create`, `delete` on `batch/jobs`, and `get`, `list`, `watch` on
  `pods` and `pods/log`.
- **RoleBinding:** Binds the role to the service account, scoped to the
  `ai-pipeline` namespace.

## 8. Runner Script Execution

Once the agent container starts, the runner script
(`run_skill.sh` or `run_skill_sdk.sh`) executes the following steps:

### Phase 1: Environment setup

1. Configure SSL — if `/shared/ca-certificates.crt` exists (from init
   container), set `SSL_CERT_FILE` and `REQUESTS_CA_BUNDLE`.
2. Configure git — set `url."https://github.com/".insteadOf "git@github.com:"`
   so marketplace plugin clones work without SSH keys.
3. **(CLI runner only)** Configure Claude CLI for Vertex AI — write
   `~/.claude/settings.json` with `apiProvider: vertex`, project ID, and
   region. Also configure the Atlassian MCP server if `ATLASSIAN_MCP_URL`
   is set.
4. **(CLI runner only)** Configure MLflow tracing — run
   `mlflow autolog claude` to set up hooks that trace Claude conversations
   to the MLflow server.

### Phase 2: Plugin installation

1. Register marketplaces:
   ```bash
   claude plugin marketplace add opendatahub-io/skills-registry    # official
   claude plugin marketplace add /app/skills-registry               # local staging
   ```
2. Discover registries from `pipeline-skills.yaml` — iterate over
   `skill_repos`, read the `registry` field from each, and install:
   ```bash
   claude plugin install rfe-creator@opendatahub-skills
   claude plugin install strat-creator@pipeline-staging
   claude plugin install strat-creator-fork@pipeline-staging
   ```
3. Set up symlinks — for each installed plugin version directory, create
   symlinks so skills can access shared artifact directories:
   ```
   <plugin-version-dir>/artifacts → /app/artifacts
   <plugin-version-dir>/tmp → /app/tmp
   <plugin-version-dir>/.context → /app/.context
   ```

### Phase 3: Hotpatching

Strip `context: fork` from all installed SKILL.md files. Forked skill
contexts suppress all intermediate streaming output, making job logs
empty. This is a temporary workaround until upstream repos adopt
`${CLAUDE_SKILL_DIR}` paths and remove unnecessary fork annotations.

```bash
find ~/.claude/plugins/cache -name "SKILL.md" -exec sed -i '/^context: *fork/d' {} +
```

### Phase 4: Skill resolution

Resolve the skill key (e.g., `strategy-refine-fork`) to a skill name
(e.g., `strategy-refine`) using `pipeline-skills.yaml`. Also resolve the
plugin source directory for external skills.

### Phase 5: Execution

#### CLI runner (`run_skill.sh`)

Creates a FIFO, runs Claude CLI in the background writing stream-json to
the FIFO, and parses the stream with `stream-claude.py`:

```bash
mkfifo /tmp/claude-stream.fifo

claude --model opus --print --dangerously-skip-permissions \
  --output-format stream-json --include-partial-messages \
  --include-hook-events --verbose \
  "/strategy-refine --headless RHAISTRAT-2" \
  2>/tmp/claude-stderr.log > /tmp/claude-stream.fifo &

python3 -u scripts/stream-claude.py --claude-pid $! < /tmp/claude-stream.fifo
```

The stream parser (`scripts/stream-claude.py`) renders thinking blocks,
tool use summaries, text output, token counts, and hook events to stdout,
which becomes the job's pod logs.

Key CLI flags:
- `--print` — non-interactive mode, exit after completion
- `--dangerously-skip-permissions` — auto-approve all tool use (safe in
  sandboxed container)
- `--output-format stream-json` — emit structured events for parsing
- `--include-partial-messages` — stream as work happens
- `--include-hook-events` — include MLflow hook output
- `--verbose` — required for stream-json with --print
- `--headless` — passed to the skill, suppresses interactive prompts

#### SDK runner (`run_skill_sdk.sh`)

Runs the skill via the Claude Agent SDK (Python) with MLflow integration:

```python
from claude_agent_sdk import ClaudeSDKClient, ClaudeAgentOptions

options = ClaudeAgentOptions(
    model=get_model_id(model),
    allowed_tools=get_allowed_tools(skill_key),
    mcp_servers=get_mcp_servers(skill_key),
    cwd=cwd,
    permission_mode="auto",
)

async with ClaudeSDKClient(options=options) as client:
    await client.query(prompt)
    async for msg in client.receive_response():
        # Print text blocks, tool use, and result metrics
```

The SDK runner also logs tool usage, token counts, duration, and cost to
MLflow when `MLFLOW_TRACKING_URI` is set.

## 9. Skill Display Names

Skills are shown in the dashboard with fully-qualified names:

```
owner/repo@branch:skill.name
```

Examples:
- `ederign/strat-creator@main:strategy.create`
- `jctanner-opendatahub-io/eder-strat-creator@reorganize-scripts:strategy-refine`
- `local:bug-completeness`

This format is built by `lib/skill_config.list_skills()` from the
`pipeline-skills.yaml` config. The `github`, `ref`, and `skill` fields
from `skill_repos` and `skills` sections are combined.

## 10. Job Lifecycle

```
Submit ──► Pending ──► Running ──► Completed/Failed ──► (24hr TTL) ──► Deleted
                                        │
                                   Logs available via
                                   dashboard or kubectl
```

- **Pending:** Job created, waiting for pod scheduling.
- **Running:** Agent container is executing the skill.
- **Completed:** Exit code 0 — skill ran successfully.
- **Failed:** Non-zero exit code — check logs for errors.
- **TTL cleanup:** K8s automatically deletes the job and its pods 24 hours
  after completion (`ttlSecondsAfterFinished: 86400`).
- **Manual controls:** Stop (kills running jobs), Delete (immediate
  removal), Re-run (submit new job with same parameters).

## 11. Key Files

| File | Purpose |
|------|---------|
| `lib/k8s_orchestrator.py` | Job manifest generation, submission, status, logs, stop/delete |
| `lib/webapp.py` | Dashboard UI and REST API endpoints |
| `lib/skill_config.py` | Reads pipeline-skills.yaml, resolves skill names and display metadata |
| `scripts/run_skill.sh` | CLI runner — marketplace setup, plugin install, Claude CLI invocation |
| `scripts/run_skill_sdk.sh` | SDK runner — same setup, Python SDK invocation with MLflow |
| `scripts/stream-claude.py` | Parses stream-json events into human-readable log output |
| `pipeline-skills.yaml` | Maps skill keys to names, sources, repos, tools, MCP servers |
| `skills-registry/` | Local staging marketplace for pre-release plugins |
| `deploy/pipeline-agent/Dockerfile` | Agent container image definition |
| `deploy/k8s/14-pipeline-storage.yaml` | PVC definitions |
| `deploy/k8s/16-pipeline-rbac.yaml` | ServiceAccount, Role, RoleBinding |
| `deploy/k8s/20-pipeline-dashboard.yaml` | Dashboard Deployment and Service |
