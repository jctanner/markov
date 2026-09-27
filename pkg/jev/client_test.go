package jev

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func response(answers map[string]any) map[string]any {
	return map[string]any{"model": "kev-test", "answers": answers, "usage": map[string]any{"input_tokens": 12, "output_tokens": 3}}
}
func TestClientContractAndCredentials(t *testing.T) {
	t.Setenv("MARKOV_JEV_TEST_KEY", "test-secret")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/systemone" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Errorf("invalid request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "kev-latest" {
			t.Errorf("model %v", body["model"])
		}
		if _, ok := body["state"].(map[string]any); !ok {
			t.Error("lost structured state")
		}
		w.Header().Set("x-typesafe-request-id", "request-123")
		json.NewEncoder(w).Encode(response(map[string]any{
			"risk":    map[string]any{"type": "noul", "noul": 0.8},
			"route":   map[string]any{"type": "choice", "choice": "review", "probabilities": map[string]any{"review": 0.9, "ship": 0.1}, "confidence": 0.8},
			"urgency": map[string]any{"type": "score", "score": 0.75, "legend": map[string]any{"0": "low", "1": "high"}, "probabilities": map[string]any{"0": 0.25, "1": 0.75}, "confidence": 0.5},
		}))
	}))
	defer server.Close()
	qs := map[string]Question{"risk": {Type: "noul"}, "route": {Type: "choice", Criteria: map[string]any{"review": nil, "ship": nil}}, "urgency": {Type: "score", Criteria: []any{"low", "high"}}}
	conn := Connection{Type: "jev", BaseURL: server.URL, Model: "kev-latest", APIKeyEnv: "MARKOV_JEV_TEST_KEY"}
	for _, fileKey := range []bool{false, true} {
		if fileKey {
			file := filepath.Join(t.TempDir(), "key")
			os.WriteFile(file, []byte("test-secret\n"), 0600)
			conn.APIKeyEnv = ""
			conn.APIKeyFile = file
		}
		out, err := New(map[string]Connection{"local": conn}).Execute(context.Background(), "local", map[string]any{"ticket": "refund"}, qs)
		if err != nil {
			t.Fatal(err)
		}
		if out["metadata"].(map[string]any)["request_id"] != "request-123" {
			t.Fatal("missing request ID")
		}
		b, _ := json.Marshal(out)
		if strings.Contains(string(b), "test-secret") {
			t.Fatal("secret leaked")
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("calls %d", calls.Load())
	}
}

func TestClientRejectsBadResponsesAndAuth(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"auth", "secret reflected by upstream", 401},
		{"malformed", "not JSON", 200},
		{"missing_answer", `{"model":"x","usage":{"input_tokens":1,"output_tokens":1},"answers":{}}`, 200},
		{"out_of_range", `{"model":"x","usage":{"input_tokens":1,"output_tokens":1},"answers":{"q":{"type":"noul","noul":1.1}}}`, 200},
		{"wrong_type", `{"model":"x","usage":{"input_tokens":1,"output_tokens":1},"answers":{"q":{"type":"choice","choice":"yes"}}}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer s.Close()
			_, err := New(map[string]Connection{"c": {Type: "jev", BaseURL: s.URL}}).Execute(context.Background(), "c", "test", map[string]Question{"q": {Type: "noul"}})
			if err == nil {
				t.Fatal("expected error")
			}
			if strings.Contains(err.Error(), "secret reflected") {
				t.Fatal("response body leaked")
			}
			if calls.Load() != 1 {
				t.Fatal("unexpected retry")
			}
		})
	}
}

func TestClientConcurrencyAndCancellation(t *testing.T) {
	var active, peak atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		json.NewEncoder(w).Encode(response(map[string]any{"q": map[string]any{"type": "noul", "noul": 0.5}}))
	}))
	defer s.Close()
	c := New(map[string]Connection{"c": {Type: "jev", BaseURL: s.URL}})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Execute(context.Background(), "c", "test", map[string]Question{"q": {Type: "noul"}}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if peak.Load() != 1 {
		t.Fatalf("peak %d", peak.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Execute(ctx, "c", "test", map[string]Question{"q": {Type: "noul"}}); err == nil {
		t.Fatal("canceled request succeeded")
	}
}

func TestClientTransientRetry(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		json.NewEncoder(w).Encode(response(map[string]any{"q": map[string]any{"type": "noul", "noul": 0.5}}))
	}))
	defer s.Close()
	out, err := New(map[string]Connection{"c": {Type: "jev", BaseURL: s.URL}}).Execute(context.Background(), "c", "test", map[string]Question{"q": {Type: "noul"}})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || out["metadata"].(map[string]any)["attempts"] != 2 {
		t.Fatal("retry not recorded")
	}
}
