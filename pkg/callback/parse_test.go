package callback

import (
	"path/filepath"
	"testing"
)

func TestParseCallbackURLJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	cb, err := ParseCallbackURL("jsonl://"+path, nil, 100, false, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	defer cb.Close()

	if _, ok := cb.(*JSONLCallback); !ok {
		t.Errorf("type = %T, want *JSONLCallback", cb)
	}
}

func TestParseCallbackURLHTTP(t *testing.T) {
	cb, err := ParseCallbackURL("http://localhost:8080/events", nil, 50, false, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	defer cb.Close()

	if _, ok := cb.(*HTTPCallback); !ok {
		t.Errorf("type = %T, want *HTTPCallback", cb)
	}
}

func TestParseCallbackURLHTTPS(t *testing.T) {
	cb, err := ParseCallbackURL("https://example.com/events", nil, 50, false, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	defer cb.Close()

	if _, ok := cb.(*HTTPCallback); !ok {
		t.Errorf("type = %T, want *HTTPCallback", cb)
	}
}

func TestParseCallbackURLGRPC(t *testing.T) {
	cb, err := ParseCallbackURL("grpc://localhost:9090", nil, 100, false, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	defer cb.Close()

	if _, ok := cb.(*GRPCCallback); !ok {
		t.Errorf("type = %T, want *GRPCCallback", cb)
	}
}

func TestParseCallbackURLGRPCS(t *testing.T) {
	cb, err := ParseCallbackURL("grpcs://localhost:9090", nil, 100, true, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	defer cb.Close()

	if _, ok := cb.(*GRPCCallback); !ok {
		t.Errorf("type = %T, want *GRPCCallback", cb)
	}
}

func TestParseCallbackURLUnsupportedScheme(t *testing.T) {
	_, err := ParseCallbackURL("ftp://localhost/events", nil, 100, false, "")
	if err == nil {
		t.Error("expected error for unsupported scheme")
	}
}

func TestParseCallbackURLJSONLEmptyPath(t *testing.T) {
	_, err := ParseCallbackURL("jsonl://", nil, 100, false, "")
	if err == nil {
		t.Error("expected error for empty JSONL path")
	}
}

func TestParseCallbackURLGRPCEmptyAddr(t *testing.T) {
	_, err := ParseCallbackURL("grpc://", nil, 100, false, "")
	if err == nil {
		t.Error("expected error for empty gRPC address")
	}
}

func TestParseCallbackURLHTTPWithHeaders(t *testing.T) {
	headers := map[string]string{
		"Authorization": "Bearer token",
		"X-Custom":      "value",
	}
	cb, err := ParseCallbackURL("http://localhost:8080/events", headers, 50, false, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	defer cb.Close()

	httpCB, ok := cb.(*HTTPCallback)
	if !ok {
		t.Fatalf("type = %T, want *HTTPCallback", cb)
	}
	if httpCB.headers["Authorization"] != "Bearer token" {
		t.Errorf("Authorization header = %q, want Bearer token", httpCB.headers["Authorization"])
	}
}
