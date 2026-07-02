package engine

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jctanner/markov/pkg/parser"
	"github.com/jctanner/markov/pkg/state"
)

func TestRunStoresWorkflowSourcePath(t *testing.T) {
	store, err := state.NewSQLiteStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	wfFile := &parser.WorkflowFile{
		Entrypoint: "main",
		Workflows: []parser.Workflow{
			{
				Name: "main",
				Steps: []parser.Step{
					{Name: "run_child", Workflow: "child"},
				},
			},
			{
				Name: "child",
				Steps: []parser.Step{
					{Name: "mark_done", Type: "set_fact", Vars: map[string]any{"done": true}},
				},
			},
		},
	}

	eng := New(wfFile, store, nil)
	eng.SourcePath = "examples/dir-based-hello-world"
	runID, err := eng.Run(context.Background(), "main", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	run, err := store.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.WorkflowFile != eng.SourcePath {
		t.Fatalf("WorkflowFile = %q, want %q", run.WorkflowFile, eng.SourcePath)
	}

	children, err := store.GetChildRuns(context.Background(), runID)
	if err != nil {
		t.Fatalf("GetChildRuns: %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("len(children) = %d, want 1", len(children))
	}
	if children[0].WorkflowFile != eng.SourcePath {
		t.Fatalf("child WorkflowFile = %q, want %q", children[0].WorkflowFile, eng.SourcePath)
	}
}
