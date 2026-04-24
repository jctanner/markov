# Markov Callback System

## Overview

Markov needs a callback system that emits structured events during workflow execution. Callbacks allow external systems to observe, record, and react to workflow lifecycle events without coupling markov to any specific backend.

Inspired by Ansible's callback plugin architecture: the engine fires events at well-defined points, and one or more callback implementations handle them independently.

## Interface

Define a `Callback` interface in `pkg/callback/callback.go`:

```go
type Callback interface {
    // Run lifecycle
    OnRunStarted(event RunStartedEvent) error
    OnRunCompleted(event RunCompletedEvent) error
    OnRunFailed(event RunFailedEvent) error
    OnRunResumed(event RunResumedEvent) error

    // Step lifecycle
    OnStepStarted(event StepStartedEvent) error
    OnStepCompleted(event StepCompletedEvent) error
    OnStepFailed(event StepFailedEvent) error
    OnStepSkipped(event StepSkippedEvent) error

    // Gate lifecycle
    OnGateEvaluated(event GateEvaluatedEvent) error

    // Sub-workflow / fan-out
    OnSubRunStarted(event SubRunStartedEvent) error
    OnSubRunCompleted(event SubRunCompletedEvent) error
    OnSubRunFailed(event SubRunFailedEvent) error

    // Shutdown hook (flush buffers, close connections)
    Close() error
}
```

The engine should accept a slice of callbacks (`[]Callback`) so multiple implementations can run simultaneously (e.g., JSONL to disk + HTTP to markovd).

If a callback returns an error, the engine should log the error but **not** fail the workflow. Callback errors must never interrupt execution.

## Event Types

All events share a common header:

```go
type EventHeader struct {
    Timestamp   time.Time `json:"timestamp"`
    RunID       string    `json:"run_id"`
    EventType   string    `json:"event_type"`
}
```

### Run Events

```go
type RunStartedEvent struct {
    EventHeader
    WorkflowName string         `json:"workflow_name"`
    WorkflowFile string         `json:"workflow_file"`
    Vars         map[string]any `json:"vars"`
    Forks        int            `json:"forks"`
    Namespace    string         `json:"namespace"`
}

type RunCompletedEvent struct {
    EventHeader
    WorkflowName string    `json:"workflow_name"`
    Duration     float64   `json:"duration_seconds"`
}

type RunFailedEvent struct {
    EventHeader
    WorkflowName string `json:"workflow_name"`
    Error        string `json:"error"`
    Duration     float64 `json:"duration_seconds"`
}

type RunResumedEvent struct {
    EventHeader
    WorkflowName    string `json:"workflow_name"`
    CompletedSteps  int    `json:"completed_steps"`
    RemainingSteps  int    `json:"remaining_steps"`
}
```

### Step Events

```go
type StepStartedEvent struct {
    EventHeader
    WorkflowName string         `json:"workflow_name"`
    StepName     string         `json:"step_name"`
    StepType     string         `json:"step_type"`
    ResolvedType string         `json:"resolved_type"`
    Params       map[string]any `json:"params,omitempty"`
}

type StepCompletedEvent struct {
    EventHeader
    WorkflowName string         `json:"workflow_name"`
    StepName     string         `json:"step_name"`
    StepType     string         `json:"step_type"`
    Output       map[string]any `json:"output,omitempty"`
    Duration     float64        `json:"duration_seconds"`
}

type StepFailedEvent struct {
    EventHeader
    WorkflowName string  `json:"workflow_name"`
    StepName     string  `json:"step_name"`
    StepType     string  `json:"step_type"`
    Error        string  `json:"error"`
    Duration     float64 `json:"duration_seconds"`
}

type StepSkippedEvent struct {
    EventHeader
    WorkflowName string `json:"workflow_name"`
    StepName     string `json:"step_name"`
    Reason       string `json:"reason"`
}
```

### Gate Events

```go
type GateEvaluatedEvent struct {
    EventHeader
    WorkflowName string         `json:"workflow_name"`
    StepName     string         `json:"step_name"`
    Action       string         `json:"action"`
    FiredRules   []string       `json:"fired_rules"`
    Facts        map[string]any `json:"facts,omitempty"`
}
```

### Sub-Run Events

```go
type SubRunStartedEvent struct {
    EventHeader
    ParentRunID   string `json:"parent_run_id"`
    ParentStep    string `json:"parent_step"`
    WorkflowName  string `json:"workflow_name"`
    ForEachKey    string `json:"for_each_key,omitempty"`
}

type SubRunCompletedEvent struct {
    EventHeader
    ParentRunID  string  `json:"parent_run_id"`
    ParentStep   string  `json:"parent_step"`
    WorkflowName string  `json:"workflow_name"`
    ForEachKey   string  `json:"for_each_key,omitempty"`
    Duration     float64 `json:"duration_seconds"`
}

type SubRunFailedEvent struct {
    EventHeader
    ParentRunID  string  `json:"parent_run_id"`
    ParentStep   string  `json:"parent_step"`
    WorkflowName string  `json:"workflow_name"`
    ForEachKey   string  `json:"for_each_key,omitempty"`
    Error        string  `json:"error"`
    Duration     float64 `json:"duration_seconds"`
}
```

## Built-in Implementations

### 1. JSONL File Callback

Appends one JSON object per line to a file on disk.

```
--callback jsonl:/path/to/events.jsonl
```

Behavior:
- Opens file in append mode at startup
- Writes one JSON line per event (no pretty-printing)
- Flushes after each write
- `Close()` closes the file handle

### 2. HTTP Callback

POSTs each event as JSON to an HTTP endpoint.

```
--callback http://markovd:8080/api/v1/events
--callback https://example.com/webhooks/markov
```

Behavior:
- POSTs to the URL with `Content-Type: application/json`
- Includes configurable headers (e.g., auth tokens) via `--callback-header "Authorization=Bearer <token>"`
- Events are sent asynchronously via a buffered channel to avoid blocking the engine
- Retries transient failures (HTTP 5xx, connection errors) with exponential backoff, up to 3 attempts
- Drops events that fail after retries (logs a warning, never blocks the workflow)
- `Close()` drains the send buffer (with a timeout) and closes the HTTP client

### 3. gRPC Callback

Streams events to a gRPC server.

```
--callback grpc://markovd:9090
```

Behavior:
- Connects at startup, streams events via a unary or streaming RPC
- Reconnects on transient failures
- TLS support via `grpcs://` scheme or `--callback-tls-cert` / `--callback-tls-insecure`
- `Close()` sends a close message and disconnects

Define a minimal proto service:

```protobuf
service MarkovCallback {
    rpc SendEvent(CallbackEvent) returns (Ack);
    rpc StreamEvents(stream CallbackEvent) returns (Ack);
}

message CallbackEvent {
    string event_type = 1;
    string run_id = 2;
    google.protobuf.Timestamp timestamp = 3;
    bytes payload = 4;  // JSON-encoded event body
}

message Ack {}
```

## Engine Integration

### Wiring

The `Engine` struct gains a `callbacks` field:

```go
type Engine struct {
    // ... existing fields ...
    callbacks []callback.Callback
}
```

Add a setter or constructor option:

```go
func (e *Engine) SetCallbacks(cbs []callback.Callback)
```

### Firing Events

At each lifecycle point in `engine.go`, fire the corresponding event on all registered callbacks. Example for step completion:

```go
for _, cb := range e.callbacks {
    if err := cb.OnStepCompleted(event); err != nil {
        log.Printf("[run:%s] callback error: %v", runID, err)
    }
}
```

Extract this into a helper to reduce boilerplate:

```go
func (e *Engine) fireEvent(fn func(callback.Callback) error) {
    for _, cb := range e.callbacks {
        if err := fn(cb); err != nil {
            log.Printf("callback error: %v", err)
        }
    }
}
```

### Where to Fire Events

Map to existing code in `pkg/engine/engine.go`:

| Event | Location |
|-------|----------|
| `OnRunStarted` | `Run()`, after `CreateRun` |
| `OnRunCompleted` | `Run()`, when `err == nil` before `UpdateRun` |
| `OnRunFailed` | `Run()`, when `err != nil` before `UpdateRun` |
| `OnRunResumed` | `Resume()`, after loading completed steps |
| `OnStepStarted` | `executeStep()`, after `when` check passes, before executor dispatch |
| `OnStepCompleted` | `executeStep()`, after `SaveStep` with `StepCompleted` |
| `OnStepFailed` | `failStep()` |
| `OnStepSkipped` | `executeStep()`, in the `when` false branch |
| `OnGateEvaluated` | `executeStep()`, after `evaluateGate` returns |
| `OnSubRunStarted` | `executeSubWorkflow()` / `executeForEach()`, after `CreateRun` |
| `OnSubRunCompleted` | `executeSubWorkflow()` / `executeForEach()`, on success |
| `OnSubRunFailed` | `executeSubWorkflow()` / `executeForEach()`, on failure |

### CLI Flags

```
--callback <url>              Callback destination (repeatable)
                              Schemes: jsonl://, http://, https://, grpc://, grpcs://
--callback-header <key=val>   Extra HTTP headers for http callbacks (repeatable)
--callback-tls-insecure       Skip TLS verification for callback connections
--callback-tls-cert <path>    Client TLS certificate for callback connections
--callback-buffer-size <n>    Async send buffer size (default: 100)
```

## Implementation Priority

1. **Interface + event types + `fireEvent` helper** -- foundation, no backends yet
2. **JSONL callback** -- simplest, useful for debugging and local dev
3. **HTTP callback** -- required for markovd integration
4. **gRPC callback** -- future, when performance or streaming matters

## Design Constraints

- Callbacks must never block or fail the workflow. All errors are logged and swallowed.
- The engine should work fine with zero callbacks registered (current behavior, no regressions).
- Event payloads should be serializable as JSON. No pointers to engine internals.
- The `Params` field on `StepStartedEvent` should be the rendered (template-resolved) params, not raw YAML, so callbacks see actual values.
- Sensitive data (secrets, credentials in params) is a concern. Consider a `--callback-redact-params` flag or a param-level `sensitive: true` annotation in the workflow YAML to control what gets emitted.
