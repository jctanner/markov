package engine

import (
	"context"
	"testing"

	"github.com/jctanner/markov/pkg/callback"
	"github.com/jctanner/markov/pkg/executor"
	"github.com/jctanner/markov/pkg/parser"
)

func TestForEachKey_UsesItemField(t *testing.T) {
	wfFile := &parser.WorkflowFile{
		Entrypoint: "main",
		Vars: map[string]any{
			"epics": []any{
				map[string]any{"key": "EPIC-100", "title": "First"},
				map[string]any{"key": "EPIC-200", "title": "Second"},
			},
		},
		Workflows: []parser.Workflow{
			{
				Name: "main",
				Steps: []parser.Step{
					{
						Name:       "per_epic",
						ForEach:    "epics",
						ForEachKey: "key",
						As:         "epic",
						Workflow:   "process",
						Vars:       map[string]any{"epic_key": "{{ epic.key }}"},
					},
				},
			},
			{
				Name: "process",
				Steps: []parser.Step{
					{Name: "do_work", Type: "set_fact", Vars: map[string]any{"done": "yes"}},
				},
			},
		},
	}

	eng, cb := newTestEngine(t, wfFile, map[string]executor.Executor{})

	ctx := context.Background()
	_, err := eng.Run(ctx, "main", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	var keys []string
	for _, ev := range cb.all {
		if sr, ok := ev.(callback.SubRunStartedEvent); ok {
			keys = append(keys, sr.ForEachKey)
		}
	}

	if len(keys) != 2 {
		t.Fatalf("got %d sub_run_started events, want 2", len(keys))
	}

	hasEpic100 := false
	hasEpic200 := false
	for _, k := range keys {
		if k == "EPIC-100" {
			hasEpic100 = true
		}
		if k == "EPIC-200" {
			hasEpic200 = true
		}
	}
	if !hasEpic100 || !hasEpic200 {
		t.Errorf("keys = %v, want [EPIC-100, EPIC-200]", keys)
	}
}

func TestForEachKey_FallsBackToIndex(t *testing.T) {
	wfFile := &parser.WorkflowFile{
		Entrypoint: "main",
		Vars:       map[string]any{"items": []any{"a", "b", "c"}},
		Workflows: []parser.Workflow{
			{
				Name: "main",
				Steps: []parser.Step{
					{
						Name:     "loop",
						ForEach:  "items",
						As:       "item",
						Workflow: "process",
					},
				},
			},
			{
				Name: "process",
				Steps: []parser.Step{
					{Name: "noop", Type: "set_fact", Vars: map[string]any{"x": "1"}},
				},
			},
		},
	}

	eng, cb := newTestEngine(t, wfFile, map[string]executor.Executor{})

	ctx := context.Background()
	_, err := eng.Run(ctx, "main", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	var keys []string
	for _, ev := range cb.all {
		if sr, ok := ev.(callback.SubRunStartedEvent); ok {
			keys = append(keys, sr.ForEachKey)
		}
	}

	for _, want := range []string{"0", "1", "2"} {
		found := false
		for _, k := range keys {
			if k == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing index key %q in %v", want, keys)
		}
	}
}

func TestForEachSort_OrdersByField(t *testing.T) {
	wfFile := &parser.WorkflowFile{
		Entrypoint: "main",
		Vars: map[string]any{
			"items": []any{
				map[string]any{"name": "charlie", "rank": "3"},
				map[string]any{"name": "alpha", "rank": "1"},
				map[string]any{"name": "bravo", "rank": "2"},
			},
		},
		Workflows: []parser.Workflow{
			{
				Name: "main",
				Steps: []parser.Step{
					{
						Name:        "sorted_loop",
						ForEach:     "items",
						ForEachKey:  "name",
						ForEachSort: "name",
						As:          "item",
						Workflow:    "process",
						Vars:        map[string]any{"item_name": "{{ item.name }}"},
						Concurrency: 1,
					},
				},
			},
			{
				Name: "process",
				Steps: []parser.Step{
					{Name: "noop", Type: "set_fact", Vars: map[string]any{"x": "1"}},
				},
			},
		},
	}

	eng, cb := newTestEngine(t, wfFile, map[string]executor.Executor{})

	ctx := context.Background()
	_, err := eng.Run(ctx, "main", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	var keys []string
	for _, ev := range cb.all {
		if sr, ok := ev.(callback.SubRunStartedEvent); ok {
			keys = append(keys, sr.ForEachKey)
		}
	}

	if len(keys) != 3 {
		t.Fatalf("got %d sub_run_started events, want 3", len(keys))
	}

	expected := []string{"alpha", "bravo", "charlie"}
	for i, want := range expected {
		if keys[i] != want {
			t.Errorf("keys[%d] = %q, want %q (full: %v)", i, keys[i], want, keys)
		}
	}
}

func TestForEachKey_DuplicateKeyFails(t *testing.T) {
	wfFile := &parser.WorkflowFile{
		Entrypoint: "main",
		Vars: map[string]any{
			"items": []any{
				map[string]any{"id": "AAA", "val": "1"},
				map[string]any{"id": "BBB", "val": "2"},
				map[string]any{"id": "AAA", "val": "3"},
			},
		},
		Workflows: []parser.Workflow{
			{
				Name: "main",
				Steps: []parser.Step{
					{
						Name:       "loop",
						ForEach:    "items",
						ForEachKey: "id",
						As:         "item",
						Workflow:   "process",
					},
				},
			},
			{
				Name: "process",
				Steps: []parser.Step{
					{Name: "noop", Type: "set_fact", Vars: map[string]any{"x": "1"}},
				},
			},
		},
	}

	eng, _ := newTestEngine(t, wfFile, map[string]executor.Executor{})

	ctx := context.Background()
	_, err := eng.Run(ctx, "main", nil)
	if err == nil {
		t.Fatal("expected error for duplicate for_each_key")
	}

	if got := err.Error(); !contains(got, "duplicate for_each_key") {
		t.Errorf("error = %q, want it to contain 'duplicate for_each_key'", got)
	}
}

func TestForEachKey_MissingFieldFails(t *testing.T) {
	wfFile := &parser.WorkflowFile{
		Entrypoint: "main",
		Vars: map[string]any{
			"items": []any{
				map[string]any{"id": "AAA"},
				map[string]any{"name": "BBB"},
			},
		},
		Workflows: []parser.Workflow{
			{
				Name: "main",
				Steps: []parser.Step{
					{
						Name:       "loop",
						ForEach:    "items",
						ForEachKey: "id",
						As:         "item",
						Workflow:   "process",
					},
				},
			},
			{
				Name: "process",
				Steps: []parser.Step{
					{Name: "noop", Type: "set_fact", Vars: map[string]any{"x": "1"}},
				},
			},
		},
	}

	eng, _ := newTestEngine(t, wfFile, map[string]executor.Executor{})

	ctx := context.Background()
	_, err := eng.Run(ctx, "main", nil)
	if err == nil {
		t.Fatal("expected error for missing for_each_key field")
	}

	if got := err.Error(); !contains(got, "for_each_key") && !contains(got, "not found") {
		t.Errorf("error = %q, want it to mention missing for_each_key", got)
	}
}

func TestForEachSort_MissingFieldFails(t *testing.T) {
	wfFile := &parser.WorkflowFile{
		Entrypoint: "main",
		Vars: map[string]any{
			"items": []any{
				map[string]any{"name": "alpha"},
				map[string]any{"title": "bravo"},
			},
		},
		Workflows: []parser.Workflow{
			{
				Name: "main",
				Steps: []parser.Step{
					{
						Name:        "loop",
						ForEach:     "items",
						ForEachSort: "name",
						As:          "item",
						Workflow:    "process",
					},
				},
			},
			{
				Name: "process",
				Steps: []parser.Step{
					{Name: "noop", Type: "set_fact", Vars: map[string]any{"x": "1"}},
				},
			},
		},
	}

	eng, _ := newTestEngine(t, wfFile, map[string]executor.Executor{})

	ctx := context.Background()
	_, err := eng.Run(ctx, "main", nil)
	if err == nil {
		t.Fatal("expected error for missing for_each_sort field")
	}

	if got := err.Error(); !contains(got, "for_each_sort") {
		t.Errorf("error = %q, want it to mention for_each_sort", got)
	}
}

func TestForEachSort_WithoutKey_DeterministicOrder(t *testing.T) {
	wfFile := &parser.WorkflowFile{
		Entrypoint: "main",
		Vars: map[string]any{
			"items": []any{
				map[string]any{"name": "zeta"},
				map[string]any{"name": "alpha"},
				map[string]any{"name": "mu"},
			},
		},
		Workflows: []parser.Workflow{
			{
				Name: "main",
				Steps: []parser.Step{
					{
						Name:        "loop",
						ForEach:     "items",
						ForEachSort: "name",
						As:          "item",
						Workflow:    "process",
						Concurrency: 1,
					},
				},
			},
			{
				Name: "process",
				Steps: []parser.Step{
					{Name: "noop", Type: "set_fact", Vars: map[string]any{"x": "1"}},
				},
			},
		},
	}

	eng, cb := newTestEngine(t, wfFile, map[string]executor.Executor{})

	ctx := context.Background()
	_, err := eng.Run(ctx, "main", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	var keys []string
	for _, ev := range cb.all {
		if sr, ok := ev.(callback.SubRunStartedEvent); ok {
			keys = append(keys, sr.ForEachKey)
		}
	}

	// Without for_each_key, keys are indices — but they should be in sorted order
	expected := []string{"0", "1", "2"}
	if len(keys) != 3 {
		t.Fatalf("got %d keys, want 3: %v", len(keys), keys)
	}
	for i, want := range expected {
		if keys[i] != want {
			t.Errorf("keys[%d] = %q, want %q", i, keys[i], want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
