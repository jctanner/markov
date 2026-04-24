package callback

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

type mockGRPCService interface{}

type mockGRPCServer struct {
	mu       sync.Mutex
	received []callbackEvent
}

func (s *mockGRPCServer) events() []callbackEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]callbackEvent, len(s.received))
	copy(result, s.received)
	return result
}

func startMockGRPCServer(t *testing.T) (string, *mockGRPCServer) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	mock := &mockGRPCServer{}
	s := grpc.NewServer()
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "markov.MarkovCallback",
		HandlerType: (*mockGRPCService)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "SendEvent",
				Handler: func(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
					var event callbackEvent
					if err := dec(&event); err != nil {
						return nil, err
					}
					mock.mu.Lock()
					mock.received = append(mock.received, event)
					mock.mu.Unlock()
					return &ack{}, nil
				},
			},
		},
	}, mock)

	go s.Serve(lis)
	t.Cleanup(func() { s.Stop() })

	return lis.Addr().String(), mock
}

func TestGRPCCallbackSendsEvents(t *testing.T) {
	addr, mock := startMockGRPCServer(t)

	cb, err := NewGRPCCallback(addr, true, "")
	if err != nil {
		t.Fatalf("NewGRPCCallback: %v", err)
	}

	ts := time.Now()
	if err := cb.OnRunStarted(RunStartedEvent{
		EventHeader:  EventHeader{Timestamp: ts, RunID: "run1", EventType: "run_started"},
		WorkflowName: "deploy",
		Forks:        5,
	}); err != nil {
		t.Fatalf("OnRunStarted: %v", err)
	}

	if err := cb.OnStepCompleted(StepCompletedEvent{
		EventHeader:  EventHeader{Timestamp: ts, RunID: "run1", EventType: "step_completed"},
		WorkflowName: "deploy",
		StepName:     "build",
		StepType:     "shell_exec",
		Duration:     1.5,
	}); err != nil {
		t.Fatalf("OnStepCompleted: %v", err)
	}

	cb.Close()

	events := mock.events()
	if len(events) != 2 {
		t.Fatalf("received %d events, want 2", len(events))
	}

	if events[0].EventType != "run_started" {
		t.Errorf("event[0].EventType = %q, want run_started", events[0].EventType)
	}
	if events[0].RunID != "run1" {
		t.Errorf("event[0].RunID = %q, want run1", events[0].RunID)
	}

	var payload map[string]any
	json.Unmarshal([]byte(events[0].Payload), &payload)
	if payload["workflow_name"] != "deploy" {
		t.Errorf("payload.workflow_name = %v, want deploy", payload["workflow_name"])
	}

	if events[1].EventType != "step_completed" {
		t.Errorf("event[1].EventType = %q, want step_completed", events[1].EventType)
	}
}

func TestGRPCCallbackConnectionFailure(t *testing.T) {
	cb, err := NewGRPCCallback("127.0.0.1:1", true, "")
	if err != nil {
		t.Fatalf("NewGRPCCallback: %v", err)
	}
	defer cb.Close()

	err = cb.OnRunStarted(RunStartedEvent{
		EventHeader:  EventHeader{Timestamp: time.Now(), RunID: "run1", EventType: "run_started"},
		WorkflowName: "test",
	})
	if err == nil {
		t.Error("expected error when connecting to invalid address")
	}
}
