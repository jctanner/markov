package callback

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventHeaderJSON(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	h := EventHeader{
		Timestamp: ts,
		RunID:     "abc12345",
		EventType: "run_started",
	}
	data, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if got["run_id"] != "abc12345" {
		t.Errorf("run_id = %v, want abc12345", got["run_id"])
	}
	if got["event_type"] != "run_started" {
		t.Errorf("event_type = %v, want run_started", got["event_type"])
	}
}

func TestRunStartedEventJSON(t *testing.T) {
	event := RunStartedEvent{
		EventHeader: EventHeader{
			Timestamp: time.Now(),
			RunID:     "run1",
			EventType: "run_started",
		},
		WorkflowName: "deploy",
		WorkflowFile: "deploy.yaml",
		Vars:         map[string]any{"env": "prod"},
		Forks:        5,
		Namespace:    "default",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got RunStartedEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.WorkflowName != "deploy" {
		t.Errorf("WorkflowName = %q, want deploy", got.WorkflowName)
	}
	if got.Forks != 5 {
		t.Errorf("Forks = %d, want 5", got.Forks)
	}
	if got.Vars["env"] != "prod" {
		t.Errorf("Vars[env] = %v, want prod", got.Vars["env"])
	}
}

func TestStepCompletedEventOmitsEmptyOutput(t *testing.T) {
	event := StepCompletedEvent{
		EventHeader: EventHeader{
			Timestamp: time.Now(),
			RunID:     "run1",
			EventType: "step_completed",
		},
		WorkflowName: "deploy",
		StepName:     "step1",
		StepType:     "shell_exec",
		Duration:     1.5,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if _, ok := got["output"]; ok {
		t.Error("expected output to be omitted when nil")
	}
}

func TestJobCreatedEventJSON(t *testing.T) {
	event := JobCreatedEvent{
		EventHeader: EventHeader{
			Timestamp: time.Now(),
			RunID:     "run1",
			EventType: "job_created",
		},
		WorkflowName: "deploy",
		StepName:     "run_tests",
		StepType:     "k8s_job",
		JobName:      "markov-job-run_tests-7f3a",
		Namespace:    "ai-pipeline",
		PodSelector:  "job-name=markov-job-run_tests-7f3a",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if got["job_name"] != "markov-job-run_tests-7f3a" {
		t.Errorf("job_name = %v, want markov-job-run_tests-7f3a", got["job_name"])
	}
	if got["pod_selector"] != "job-name=markov-job-run_tests-7f3a" {
		t.Errorf("pod_selector = %v, want job-name=markov-job-run_tests-7f3a", got["pod_selector"])
	}
	if got["namespace"] != "ai-pipeline" {
		t.Errorf("namespace = %v, want ai-pipeline", got["namespace"])
	}
	if _, ok := got["fork_id"]; ok {
		t.Error("expected fork_id to be omitted when empty")
	}
}

func TestJobCreatedEventWithForkID(t *testing.T) {
	event := JobCreatedEvent{
		EventHeader: EventHeader{
			Timestamp: time.Now(),
			RunID:     "run1",
			EventType: "job_created",
		},
		WorkflowName: "deploy",
		StepName:     "run_tests",
		StepType:     "k8s_job",
		ForkID:       "3",
		JobName:      "markov-job-run_tests-7f3a",
		Namespace:    "ai-pipeline",
		PodSelector:  "job-name=markov-job-run_tests-7f3a",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if got["fork_id"] != "3" {
		t.Errorf("fork_id = %v, want 3", got["fork_id"])
	}
}

func TestSubRunEventForEachKey(t *testing.T) {
	event := SubRunStartedEvent{
		EventHeader: EventHeader{
			Timestamp: time.Now(),
			RunID:     "run1-loop-0",
			EventType: "sub_run_started",
		},
		ParentRunID:  "run1",
		ParentStep:   "loop",
		WorkflowName: "process_item",
		ForEachKey:   "0",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if got["for_each_key"] != "0" {
		t.Errorf("for_each_key = %v, want 0", got["for_each_key"])
	}
	if got["parent_run_id"] != "run1" {
		t.Errorf("parent_run_id = %v, want run1", got["parent_run_id"])
	}
}

func TestSubRunEventOmitsEmptyForEachKey(t *testing.T) {
	event := SubRunStartedEvent{
		EventHeader: EventHeader{
			Timestamp: time.Now(),
			RunID:     "run1-sub",
			EventType: "sub_run_started",
		},
		ParentRunID:  "run1",
		ParentStep:   "deploy_child",
		WorkflowName: "child_workflow",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if _, ok := got["for_each_key"]; ok {
		t.Error("expected for_each_key to be omitted when empty")
	}
}
