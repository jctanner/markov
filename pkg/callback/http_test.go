package callback

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPCallbackPostsEvents(t *testing.T) {
	var mu sync.Mutex
	var received []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		var event map[string]any
		json.NewDecoder(r.Body).Decode(&event)
		mu.Lock()
		received = append(received, event)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := NewHTTPCallback(server.URL, nil, 10)

	ts := time.Now()
	cb.OnRunStarted(RunStartedEvent{
		EventHeader:  EventHeader{Timestamp: ts, RunID: "run1", EventType: "run_started"},
		WorkflowName: "deploy",
		Forks:        3,
	})
	cb.OnStepStarted(StepStartedEvent{
		EventHeader:  EventHeader{Timestamp: ts, RunID: "run1", EventType: "step_started"},
		WorkflowName: "deploy",
		StepName:     "build",
		StepType:     "shell_exec",
	})
	cb.OnRunCompleted(RunCompletedEvent{
		EventHeader:  EventHeader{Timestamp: ts, RunID: "run1", EventType: "run_completed"},
		WorkflowName: "deploy",
		Duration:     2.0,
	})

	cb.Close()

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 3 {
		t.Fatalf("received %d events, want 3", len(received))
	}

	if received[0]["event_type"] != "run_started" {
		t.Errorf("first event type = %v, want run_started", received[0]["event_type"])
	}
	if received[2]["event_type"] != "run_completed" {
		t.Errorf("last event type = %v, want run_completed", received[2]["event_type"])
	}
}

func TestHTTPCallbackCustomHeaders(t *testing.T) {
	var gotAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	headers := map[string]string{
		"Authorization": "Bearer test-token",
	}
	cb := NewHTTPCallback(server.URL, headers, 10)

	cb.OnRunStarted(RunStartedEvent{
		EventHeader:  EventHeader{Timestamp: time.Now(), RunID: "run1", EventType: "run_started"},
		WorkflowName: "test",
	})
	cb.Close()

	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want Bearer test-token", gotAuth)
	}
}

func TestHTTPCallbackRetries5xx(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := NewHTTPCallback(server.URL, nil, 10)

	cb.OnRunStarted(RunStartedEvent{
		EventHeader:  EventHeader{Timestamp: time.Now(), RunID: "run1", EventType: "run_started"},
		WorkflowName: "test",
	})
	cb.Close()

	if got := attempts.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestHTTPCallbackNoRetryOn4xx(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	cb := NewHTTPCallback(server.URL, nil, 10)

	cb.OnRunStarted(RunStartedEvent{
		EventHeader:  EventHeader{Timestamp: time.Now(), RunID: "run1", EventType: "run_started"},
		WorkflowName: "test",
	})
	cb.Close()

	if got := attempts.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on 4xx)", got)
	}
}

func TestHTTPCallbackBackPressure(t *testing.T) {
	var count atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		count.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := NewHTTPCallback(server.URL, nil, 2)

	for i := 0; i < 10; i++ {
		cb.OnRunStarted(RunStartedEvent{
			EventHeader:  EventHeader{Timestamp: time.Now(), RunID: "run1", EventType: "run_started"},
			WorkflowName: "test",
		})
	}

	cb.Close()

	if got := count.Load(); got != 10 {
		t.Errorf("received %d events, want 10 (back-pressure, no drops)", got)
	}
}

func TestHTTPCallbackTerminalEventFlushes(t *testing.T) {
	var mu sync.Mutex
	var received []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		var event map[string]any
		json.NewDecoder(r.Body).Decode(&event)
		mu.Lock()
		received = append(received, event["event_type"].(string))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := NewHTTPCallback(server.URL, nil, 100)

	ts := time.Now()
	for i := 0; i < 20; i++ {
		cb.OnStepCompleted(StepCompletedEvent{
			EventHeader:  EventHeader{Timestamp: ts, RunID: "run1", EventType: "step_completed"},
			WorkflowName: "deploy",
			StepName:     "step",
		})
	}

	cb.OnRunCompleted(RunCompletedEvent{
		EventHeader:  EventHeader{Timestamp: ts, RunID: "run1", EventType: "run_completed"},
		WorkflowName: "deploy",
		Duration:     1.0,
	})

	mu.Lock()
	n := len(received)
	lastEvent := received[n-1]
	mu.Unlock()

	if n != 21 {
		t.Errorf("received %d events, want 21", n)
	}
	if lastEvent != "run_completed" {
		t.Errorf("last event = %q, want run_completed", lastEvent)
	}
}

func TestHTTPCallbackRunFailedFlushes(t *testing.T) {
	var mu sync.Mutex
	var received []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event map[string]any
		json.NewDecoder(r.Body).Decode(&event)
		mu.Lock()
		received = append(received, event["event_type"].(string))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := NewHTTPCallback(server.URL, nil, 100)

	ts := time.Now()
	for i := 0; i < 5; i++ {
		cb.OnStepStarted(StepStartedEvent{
			EventHeader: EventHeader{Timestamp: ts, RunID: "run1", EventType: "step_started"},
			StepName:    "s",
		})
	}

	cb.OnRunFailed(RunFailedEvent{
		EventHeader:  EventHeader{Timestamp: ts, RunID: "run1", EventType: "run_failed"},
		WorkflowName: "deploy",
		Error:        "boom",
	})

	mu.Lock()
	n := len(received)
	lastEvent := received[n-1]
	mu.Unlock()

	if n != 6 {
		t.Errorf("received %d events, want 6", n)
	}
	if lastEvent != "run_failed" {
		t.Errorf("last event = %q, want run_failed", lastEvent)
	}

	cb.Close()
}

func TestHTTPCallbackCloseIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := NewHTTPCallback(server.URL, nil, 10)
	cb.Close()
	cb.Close()
}
