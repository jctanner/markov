package executor

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPRequestAppliesHeadersAndBasicAuth(t *testing.T) {
	transport := &captureTransport{
		statusCode: http.StatusOK,
		body:       `{"ok":true}`,
	}
	exec := &HTTPRequest{client: &http.Client{Transport: transport}}

	result, err := exec.Execute(context.Background(), map[string]any{
		"url":    "https://example.test/api",
		"method": "POST",
		"body": map[string]any{
			"name": "markov",
		},
		"headers": map[string]any{
			"Accept":        "application/json",
			"Authorization": "Bearer ignored",
			"Content-Type":  "application/vnd.example+json",
			"X-Trace":       12345,
		},
		"basic_auth": map[string]any{
			"username": "admin",
			"password": "secret",
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output["status_code"] != http.StatusOK {
		t.Fatalf("status_code = %v, want %d", result.Output["status_code"], http.StatusOK)
	}

	req := transport.request
	if req == nil {
		t.Fatal("transport did not receive request")
	}
	if got := req.Header.Get("Accept"); got != "application/json" {
		t.Fatalf("Accept = %q, want application/json", got)
	}
	if got := req.Header.Get("Content-Type"); got != "application/vnd.example+json" {
		t.Fatalf("Content-Type = %q, want custom override", got)
	}
	if got := req.Header.Get("X-Trace"); got != "12345" {
		t.Fatalf("X-Trace = %q, want 12345", got)
	}
	username, password, ok := req.BasicAuth()
	if !ok || username != "admin" || password != "secret" {
		t.Fatalf("BasicAuth = (%q, %q, %v), want admin/secret", username, password, ok)
	}
}

func TestHTTPRequestIgnoreStatusAllowsConfiguredErrorStatus(t *testing.T) {
	transport := &captureTransport{
		statusCode: http.StatusConflict,
		body:       `{"error":"already exists"}`,
	}
	exec := &HTTPRequest{client: &http.Client{Transport: transport}}

	result, err := exec.Execute(context.Background(), map[string]any{
		"url":           "https://example.test/api/resource",
		"ignore_status": []any{http.StatusNotFound, http.StatusConflict},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output["status_code"] != http.StatusConflict {
		t.Fatalf("status_code = %v, want %d", result.Output["status_code"], http.StatusConflict)
	}
}

func TestHTTPRequestIgnoreStatusTrueAllowsAnyErrorStatus(t *testing.T) {
	transport := &captureTransport{
		statusCode: http.StatusInternalServerError,
		body:       "server error",
	}
	exec := &HTTPRequest{client: &http.Client{Transport: transport}}

	result, err := exec.Execute(context.Background(), map[string]any{
		"url":           "https://example.test/api/resource",
		"ignore_status": true,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output["status_code"] != http.StatusInternalServerError {
		t.Fatalf("status_code = %v, want %d", result.Output["status_code"], http.StatusInternalServerError)
	}
}

func TestHTTPRequestReturnsErrorForUnhandledErrorStatus(t *testing.T) {
	transport := &captureTransport{
		statusCode: http.StatusNotFound,
		body:       "missing",
	}
	exec := &HTTPRequest{client: &http.Client{Transport: transport}}

	result, err := exec.Execute(context.Background(), map[string]any{
		"url":           "https://example.test/api/resource",
		"ignore_status": []any{http.StatusConflict},
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want status error")
	}
	if result == nil || result.Output["status_code"] != http.StatusNotFound {
		t.Fatalf("Result = %#v, want populated output with 404", result)
	}
}

type captureTransport struct {
	request    *http.Request
	statusCode int
	body       string
}

func (t *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.request = req
	return &http.Response{
		StatusCode: t.statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(t.body)),
		Request:    req,
	}, nil
}
