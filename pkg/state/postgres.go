package state

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening postgres db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting postgres db: %w", err)
	}
	if err := migratePostgres(db); err != nil {
		db.Close()
		return nil, err
	}
	return &PostgresStore{db: db}, nil
}

func migratePostgres(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS runs (
			run_id          TEXT PRIMARY KEY,
			workflow_file   TEXT NOT NULL,
			entrypoint      TEXT NOT NULL,
			status          TEXT NOT NULL,
			vars_json       TEXT NOT NULL DEFAULT '{}',
			parent_run_id   TEXT,
			parent_step     TEXT,
			for_each_key    TEXT,
			started_at      TIMESTAMPTZ NOT NULL,
			completed_at    TIMESTAMPTZ
		);

		CREATE TABLE IF NOT EXISTS steps (
			run_id          TEXT NOT NULL,
			workflow_name   TEXT NOT NULL,
			step_name       TEXT NOT NULL,
			status          TEXT NOT NULL,
			output_json     TEXT,
			artifacts_json  TEXT,
			error           TEXT,
			started_at      TIMESTAMPTZ,
			completed_at    TIMESTAMPTZ,
			PRIMARY KEY (run_id, workflow_name, step_name)
		);
	`)
	if err != nil {
		return fmt.Errorf("migrating postgres schema: %w", err)
	}
	return nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) CreateRun(ctx context.Context, run *Run) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO runs (run_id, workflow_file, entrypoint, status, vars_json, parent_run_id, parent_step, for_each_key, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		run.RunID, run.WorkflowFile, run.Entrypoint, run.Status, run.VarsJSON,
		nullStr(run.ParentRunID), nullStr(run.ParentStep), nullStr(run.ForEachKey), run.StartedAt)
	if err != nil {
		return fmt.Errorf("creating run: %w", err)
	}
	return nil
}

func (s *PostgresStore) UpdateRun(ctx context.Context, run *Run) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE runs SET status = $1, completed_at = $2 WHERE run_id = $3`,
		run.Status, run.CompletedAt, run.RunID)
	if err != nil {
		return fmt.Errorf("updating run: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetRun(ctx context.Context, runID string) (*Run, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT run_id, workflow_file, entrypoint, status, vars_json,
		       parent_run_id, parent_step, for_each_key, started_at, completed_at
		FROM runs WHERE run_id = $1`, runID)

	var run Run
	var parentRunID, parentStep, forEachKey sql.NullString
	var completedAt sql.NullTime
	err := row.Scan(&run.RunID, &run.WorkflowFile, &run.Entrypoint, &run.Status, &run.VarsJSON,
		&parentRunID, &parentStep, &forEachKey, &run.StartedAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("run %q not found", runID)
	}
	if err != nil {
		return nil, fmt.Errorf("getting run: %w", err)
	}
	run.ParentRunID = parentRunID.String
	run.ParentStep = parentStep.String
	run.ForEachKey = forEachKey.String
	if completedAt.Valid {
		run.CompletedAt = &completedAt.Time
	}
	return &run, nil
}

func (s *PostgresStore) ListRuns(ctx context.Context) ([]*Run, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, workflow_file, entrypoint, status, vars_json,
		       parent_run_id, parent_step, for_each_key, started_at, completed_at
		FROM runs ORDER BY started_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing runs: %w", err)
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		var run Run
		var parentRunID, parentStep, forEachKey sql.NullString
		var completedAt sql.NullTime
		if err := rows.Scan(&run.RunID, &run.WorkflowFile, &run.Entrypoint, &run.Status, &run.VarsJSON,
			&parentRunID, &parentStep, &forEachKey, &run.StartedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scanning run: %w", err)
		}
		run.ParentRunID = parentRunID.String
		run.ParentStep = parentStep.String
		run.ForEachKey = forEachKey.String
		if completedAt.Valid {
			run.CompletedAt = &completedAt.Time
		}
		runs = append(runs, &run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing runs: %w", err)
	}
	return runs, nil
}

func (s *PostgresStore) GetChildRuns(ctx context.Context, parentRunID string) ([]*Run, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, workflow_file, entrypoint, status, vars_json,
		       parent_run_id, parent_step, for_each_key, started_at, completed_at
		FROM runs WHERE parent_run_id = $1 ORDER BY started_at`, parentRunID)
	if err != nil {
		return nil, fmt.Errorf("getting child runs: %w", err)
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		var run Run
		var parentRID, parentStep, forEachKey sql.NullString
		var completedAt sql.NullTime
		if err := rows.Scan(&run.RunID, &run.WorkflowFile, &run.Entrypoint, &run.Status, &run.VarsJSON,
			&parentRID, &parentStep, &forEachKey, &run.StartedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scanning child run: %w", err)
		}
		run.ParentRunID = parentRID.String
		run.ParentStep = parentStep.String
		run.ForEachKey = forEachKey.String
		if completedAt.Valid {
			run.CompletedAt = &completedAt.Time
		}
		runs = append(runs, &run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("getting child runs: %w", err)
	}
	return runs, nil
}

func (s *PostgresStore) SaveStep(ctx context.Context, step *StepResult) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO steps (run_id, workflow_name, step_name, status, output_json, artifacts_json, error, started_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (run_id, workflow_name, step_name) DO UPDATE SET
			status = excluded.status,
			output_json = excluded.output_json,
			artifacts_json = excluded.artifacts_json,
			error = excluded.error,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at`,
		step.RunID, step.WorkflowName, step.StepName, step.Status,
		nullStr(step.OutputJSON), nullStr(step.ArtifactsJSON), nullStr(step.Error),
		step.StartedAt, step.CompletedAt)
	if err != nil {
		return fmt.Errorf("saving step: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetSteps(ctx context.Context, runID string) ([]*StepResult, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, workflow_name, step_name, status, output_json, artifacts_json, error, started_at, completed_at
		FROM steps WHERE run_id = $1 ORDER BY started_at`, runID)
	if err != nil {
		return nil, fmt.Errorf("getting steps: %w", err)
	}
	defer rows.Close()

	var steps []*StepResult
	for rows.Next() {
		var s StepResult
		var outputJSON, artifactsJSON, stepErr sql.NullString
		var startedAt, completedAt sql.NullTime
		if err := rows.Scan(&s.RunID, &s.WorkflowName, &s.StepName, &s.Status,
			&outputJSON, &artifactsJSON, &stepErr, &startedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scanning step: %w", err)
		}
		s.OutputJSON = outputJSON.String
		s.ArtifactsJSON = artifactsJSON.String
		s.Error = stepErr.String
		if startedAt.Valid {
			s.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			s.CompletedAt = &completedAt.Time
		}
		steps = append(steps, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("getting steps: %w", err)
	}
	return steps, nil
}

func (s *PostgresStore) GetStep(ctx context.Context, runID, workflowName, stepName string) (*StepResult, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT run_id, workflow_name, step_name, status, output_json, artifacts_json, error, started_at, completed_at
		FROM steps WHERE run_id = $1 AND workflow_name = $2 AND step_name = $3`,
		runID, workflowName, stepName)

	var sr StepResult
	var outputJSON, artifactsJSON, stepErr sql.NullString
	var startedAt, completedAt sql.NullTime
	err := row.Scan(&sr.RunID, &sr.WorkflowName, &sr.StepName, &sr.Status,
		&outputJSON, &artifactsJSON, &stepErr, &startedAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting step: %w", err)
	}
	sr.OutputJSON = outputJSON.String
	sr.ArtifactsJSON = artifactsJSON.String
	sr.Error = stepErr.String
	if startedAt.Valid {
		sr.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		sr.CompletedAt = &completedAt.Time
	}
	return &sr, nil
}
