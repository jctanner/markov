package state

import (
	"context"
	"time"
)

type RunStatus string

const (
	RunRunning   RunStatus = "running"
	RunCompleted RunStatus = "completed"
	RunFailed    RunStatus = "failed"
)

type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
)

type Run struct {
	RunID        string
	WorkflowFile string
	Entrypoint   string
	Status       RunStatus
	VarsJSON     string
	ParentRunID  string
	ParentStep   string
	ForEachKey   string
	StartedAt    time.Time
	CompletedAt  *time.Time
}

type StepResult struct {
	RunID         string
	WorkflowName  string
	StepName      string
	Status        StepStatus
	OutputJSON    string
	ArtifactsJSON string
	Error         string
	StartedAt     *time.Time
	CompletedAt   *time.Time
}

type Store interface {
	CreateRun(ctx context.Context, run *Run) error
	UpdateRun(ctx context.Context, run *Run) error
	GetRun(ctx context.Context, runID string) (*Run, error)
	ListRuns(ctx context.Context) ([]*Run, error)

	GetChildRuns(ctx context.Context, parentRunID string) ([]*Run, error)

	SaveStep(ctx context.Context, step *StepResult) error
	GetSteps(ctx context.Context, runID string) ([]*StepResult, error)
	GetStep(ctx context.Context, runID, workflowName, stepName string) (*StepResult, error)
}
