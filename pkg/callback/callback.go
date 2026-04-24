package callback

import "time"

type EventHeader struct {
	Timestamp time.Time `json:"timestamp"`
	RunID     string    `json:"run_id"`
	EventType string    `json:"event_type"`
}

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
	WorkflowName string  `json:"workflow_name"`
	Duration     float64 `json:"duration_seconds"`
}

type RunFailedEvent struct {
	EventHeader
	WorkflowName string  `json:"workflow_name"`
	Error        string  `json:"error"`
	Duration     float64 `json:"duration_seconds"`
}

type RunResumedEvent struct {
	EventHeader
	WorkflowName   string `json:"workflow_name"`
	CompletedSteps int    `json:"completed_steps"`
	RemainingSteps int    `json:"remaining_steps"`
}

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

type GateEvaluatedEvent struct {
	EventHeader
	WorkflowName string         `json:"workflow_name"`
	StepName     string         `json:"step_name"`
	Action       string         `json:"action"`
	FiredRules   []string       `json:"fired_rules"`
	Facts        map[string]any `json:"facts,omitempty"`
}

type SubRunStartedEvent struct {
	EventHeader
	ParentRunID  string `json:"parent_run_id"`
	ParentStep   string `json:"parent_step"`
	WorkflowName string `json:"workflow_name"`
	ForEachKey   string `json:"for_each_key,omitempty"`
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

type Callback interface {
	OnRunStarted(event RunStartedEvent) error
	OnRunCompleted(event RunCompletedEvent) error
	OnRunFailed(event RunFailedEvent) error
	OnRunResumed(event RunResumedEvent) error

	OnStepStarted(event StepStartedEvent) error
	OnStepCompleted(event StepCompletedEvent) error
	OnStepFailed(event StepFailedEvent) error
	OnStepSkipped(event StepSkippedEvent) error

	OnGateEvaluated(event GateEvaluatedEvent) error

	OnSubRunStarted(event SubRunStartedEvent) error
	OnSubRunCompleted(event SubRunCompletedEvent) error
	OnSubRunFailed(event SubRunFailedEvent) error

	Close() error
}
