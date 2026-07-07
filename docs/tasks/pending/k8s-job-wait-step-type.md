# Task: Add k8s_job_wait step type for watching existing jobs

## Status

Pending

## Summary

Add a `k8s_job_wait` step type that watches an existing Kubernetes Job by name until it completes or fails, streaming pod logs into the workflow output. This complements the existing `k8s_job` step type which only works when Markov creates the job itself.

## Motivation

In the end-to-end demo workflow, skill jobs are submitted via the dashboard API (`POST /api/jobs/submit`) because the dashboard orchestrator handles the complex job manifest construction (env vars from secrets, volume mounts, init containers, entrypoint scripts, etc.). The Markov workflow then needs to wait for that externally-created job to finish.

Currently this is done with a `shell_exec` polling loop that curls the dashboard API every 15 seconds — but this loses the pod log streaming that the native `k8s_job` executor provides, and the polling is coarse-grained compared to the K8s watch API. The markovd service account already has RBAC for `get`, `list`, `watch` on jobs and pods, and `get` on pods/log.

## Proposed Interface

```yaml
- name: wait_for_skill
  type: k8s_job_wait
  params:
    job_name: "{{ submitted_job.body.job_name }}"
    namespace: ai-pipeline     # optional, defaults to workflow namespace
    timeout: 3600              # optional, seconds, default 3600
    tail_logs: true            # optional, stream pod logs to workflow output
    log_bytes: 131072          # optional, max log bytes to capture (default 64KB)
```

## Behavior

1. Look up the named Job in the specified namespace.
2. If the job doesn't exist yet, poll until it appears (with timeout).
3. Watch the job using the K8s watch API (or poll every 5s like `k8s_job` does).
4. On completion or failure, collect pod logs (same as `k8s_job.getPodLogs`).
5. Return a result with `job_name`, `namespace`, `status` ("completed" or "failed"), and `logs`.
6. If the job fails, return an error (same as `k8s_job`).

## Result Schema

```yaml
# Available via register:
result:
  job_name: "strategy-create-rhairfe-1-opus-0707-162313"
  namespace: "ai-pipeline"
  status: "completed"    # or "failed"
  logs: "..."            # tail of pod logs (up to log_bytes)
```

## Implementation Notes

- Most of the logic already exists in `pkg/executor/k8s_job.go`: `waitForCompletion` and `getPodLogs` can be extracted and reused.
- The new executor only needs `Get`/`Watch` on jobs and `Get` on pods/log — no `Create` permission required, so it works with read-only service accounts.
- Consider adding a `k8s_job_wait` base type to the step type system so custom types can build on it (e.g., an `agent_job_wait` that pre-fills namespace).

## Use Case

```yaml
# Submit via dashboard API
- name: submit
  type: dashboard_api
  params:
    path: /api/jobs/submit
    method: POST
    body:
      fqn: "github.local/opendatahub-io/rfe-creator@main:rfe.speedrun"
      args:
        issue: RHAIRFE-1
        model: opus
  register: submitted_job

# Watch the K8s job natively instead of polling the dashboard API
- name: wait_for_completion
  type: k8s_job_wait
  params:
    job_name: "{{ submitted_job.body.job_name }}"
    timeout: 3600
    tail_logs: true
```

This replaces the current 20-line `shell_exec` curl polling loop with a single declarative step that also captures pod logs.
