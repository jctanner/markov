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
	RunPaused    RunStatus = "paused"
	RunCompleted RunStatus = "completed"
	RunFailed    RunStatus = "failed"
)

type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepPaused    StepStatus = "paused"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
)

type Run struct {
	RunID        string
	WorkflowFile string
	SourceDigest string
	Entrypoint   string
	Status       RunStatus
	VarsJSON     string
	ParentRunID  string
	ParentStep   string
	ForEachKey   string
	StartedAt    time.Time
	CompletedAt  *time.Time
}

// SourceCheck records the workflow source that was present for one resume
// attempt. SourceDigest on Run remains the original run digest.
type SourceCheck struct {
	RunID          string
	CheckedAt      time.Time
	Mode           string
	ExpectedDigest string
	ObservedDigest string
	SourceDrifted  bool
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
	SaveSourceCheck(ctx context.Context, check *SourceCheck) error
	GetSourceChecks(ctx context.Context, runID string) ([]*SourceCheck, error)

	SaveStep(ctx context.Context, step *StepResult) error
	GetSteps(ctx context.Context, runID string) ([]*StepResult, error)
	GetStep(ctx context.Context, runID, workflowName, stepName string) (*StepResult, error)
}

func OpenStore(stateStore string) (Store, error) {
	if IsPostgresDSN(stateStore) {
		return NewPostgresStore(stateStore)
	}
	return NewSQLiteStore(stateStore)
}

// IsPostgresDSN reports whether stateStore selects the Postgres backend.
func IsPostgresDSN(stateStore string) bool {
	return strings.HasPrefix(stateStore, "postgres://") || strings.HasPrefix(stateStore, "postgresql://")
}

func RedactStoreLocation(stateStore string) string {
	if !IsPostgresDSN(stateStore) {
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
