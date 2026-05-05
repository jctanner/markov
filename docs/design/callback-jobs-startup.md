# Callback: k8s_job Startup Event

## Problem

When a `k8s_job` step runs, the `job_name` and `namespace` only appear in the callback payload at step completion (or failure). The `StepStartedEvent` fires *before* the executor creates the Job, so it has no k8s-specific information.

This means markovd (and any other callback consumer) cannot stream live pod logs during execution. It must wait until the step finishes to learn which Job existed, at which point the pod may already be terminated and its logs ephemeral.

## Requirements

### 1. New callback event: `OnJobCreated`

Add a new event to the `Callback` interface that fires immediately after the k8s_job executor successfully creates a Job via the K8s API:

```go
OnJobCreated(event JobCreatedEvent) error
```

```go
type JobCreatedEvent struct {
    EventHeader
    WorkflowName string `json:"workflow_name"`
    StepName     string `json:"step_name"`
    StepType     string `json:"step_type"`
    JobName      string `json:"job_name"`
    Namespace    string `json:"namespace"`
    PodSelector  string `json:"pod_selector"`
}
```

Fields:
- `job_name` — the K8s Job name as created by the executor
- `namespace` — the namespace the Job was created in
- `pod_selector` — the label selector to find the Job's pods (e.g. `job-name=<jobName>`), so consumers don't have to construct it themselves

### 2. Firing location

**File**: `pkg/executor/k8s_job.go`

Fire `OnJobCreated` immediately after `client.BatchV1().Jobs(ns).Create()` returns successfully, before `waitForCompletion()`. This is the earliest point where the Job name is known and the Job exists in the cluster.

The executor currently does not have access to the callback interface. The engine should pass the `fireEvent` function (or the callbacks slice) to the executor, or the executor should return a channel/struct with the job metadata so the engine can fire the event.

Recommended approach — return job metadata from the executor to the engine:

```go
type K8sJobInfo struct {
    JobName   string
    Namespace string
}
```

The executor's `Execute()` method should accept an optional channel or callback function:

```go
func (e *K8sJobExecutor) Execute(ctx context.Context, params map[string]any, onJobCreated func(K8sJobInfo)) (map[string]any, error)
```

The engine calls `onJobCreated` from within `executeStep()`, which has access to `e.fireEvent`. This keeps the executor decoupled from the callback system.

### 3. Timing guarantee

The event MUST fire after the K8s API confirms Job creation (the `Create()` call returns without error). It MUST fire before `waitForCompletion()` blocks. This ensures:

- The Job exists in the cluster when the consumer receives the event
- The consumer can immediately start watching for pods and streaming logs
- The event is not sent for Jobs that fail to create

### 4. HTTP callback payload

When sent via the HTTP callback, the event should look like:

```json
{
  "timestamp": "2026-04-25T14:30:00Z",
  "run_id": "markov-run-abc123",
  "event_type": "job_created",
  "workflow_name": "deploy-pipeline",
  "step_name": "run_tests",
  "step_type": "k8s_job",
  "job_name": "markov-job-run_tests-7f3a",
  "namespace": "ai-pipeline",
  "pod_selector": "job-name=markov-job-run_tests-7f3a"
}
```

### 5. Include fork context

For steps running inside a `for_each` fork, the event must include the fork ID so markovd can associate the job with the correct step instance:

```go
type JobCreatedEvent struct {
    EventHeader
    WorkflowName string `json:"workflow_name"`
    StepName     string `json:"step_name"`
    StepType     string `json:"step_type"`
    ForkID       string `json:"fork_id,omitempty"`
    JobName      string `json:"job_name"`
    Namespace    string `json:"namespace"`
    PodSelector  string `json:"pod_selector"`
}
```

### 6. Non-blocking

Like all callbacks, `OnJobCreated` errors must be logged and swallowed. A callback failure must never prevent the Job from proceeding to `waitForCompletion()`.

## What markovd Does With This

When markovd receives a `job_created` event:

1. Store the `job_name` and `namespace` on the step record (update `output_json` or add dedicated columns)
2. The frontend can then call `GET /api/v1/jobs/{name}/logs` immediately to begin streaming live pod logs, rather than waiting for step completion
3. The frontend can show the job name in the graph/gantt/table views while the step is still running, not just after it finishes

## Files to Modify

| File | Change |
|------|--------|
| `pkg/callback/callback.go` | Add `OnJobCreated(JobCreatedEvent)` to interface, define `JobCreatedEvent` |
| `pkg/callback/http.go` | Implement `OnJobCreated` (POST event) |
| `pkg/callback/jsonl.go` | Implement `OnJobCreated` (write line) |
| `pkg/callback/grpc.go` | Implement `OnJobCreated` (send event) |
| `pkg/executor/k8s_job.go` | Accept `onJobCreated` callback, call it after `Jobs().Create()` |
| `pkg/engine/engine.go` | Wire `onJobCreated` in `executeStep()` to fire `OnJobCreated` via `fireEvent` |

## Downstream markovd Changes

| File | Change |
|------|--------|
| `internal/api/events.go` | Handle `job_created` event type, update step's `output_json` with job metadata |
| `internal/db/steps.go` | Ensure `UpsertStep` can merge partial `output_json` (job metadata at start, logs at completion) |
