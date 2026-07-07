package state

import (
	"context"
	"net/url"
	"strings"
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
	Close() error

	CreateRun(ctx context.Context, run *Run) error
	UpdateRun(ctx context.Context, run *Run) error
	GetRun(ctx context.Context, runID string) (*Run, error)
	ListRuns(ctx context.Context) ([]*Run, error)

	GetChildRuns(ctx context.Context, parentRunID string) ([]*Run, error)

	SaveStep(ctx context.Context, step *StepResult) error
	GetSteps(ctx context.Context, runID string) ([]*StepResult, error)
	GetStep(ctx context.Context, runID, workflowName, stepName string) (*StepResult, error)
}

func OpenStore(stateStore string) (Store, error) {
	if isPostgresDSN(stateStore) {
		return NewPostgresStore(stateStore)
	}
	return NewSQLiteStore(stateStore)
}

func isPostgresDSN(stateStore string) bool {
	return strings.HasPrefix(stateStore, "postgres://") || strings.HasPrefix(stateStore, "postgresql://")
}

func RedactStoreLocation(stateStore string) string {
	if !isPostgresDSN(stateStore) {
		return stateStore
	}
	u, err := url.Parse(stateStore)
	if err != nil {
		return "postgres://<redacted>"
	}
	redacted := u.Scheme + "://"
	if u.User != nil {
		redacted += "<redacted>@"
	}
	redacted += u.Host
	redacted += u.Path
	if u.RawQuery != "" {
		redacted += "?<redacted>"
	}
	return redacted
}
