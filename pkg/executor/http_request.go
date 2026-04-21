package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type HTTPRequest struct {
	client *http.Client
}

func NewHTTPRequest() *HTTPRequest {
	return &HTTPRequest{client: &http.Client{}}
}

func (e *HTTPRequest) Execute(ctx context.Context, params map[string]any) (*Result, error) {
	method, _ := params["method"].(string)
	if method == "" {
		method = "GET"
	}

	url, ok := params["url"].(string)
	if !ok || url == "" {
		baseURL, _ := params["base_url"].(string)
		path, _ := params["path"].(string)
		if baseURL == "" {
			return nil, fmt.Errorf("http_request: url or base_url is required")
		}
		url = baseURL + path
	}

	var bodyReader io.Reader
	if body, ok := params["body"]; ok {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("http_request: encoding body: %w", err)
		}
		bodyReader = strings.NewReader(string(encoded))
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("http_request: creating request: %w", err)
	}

	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http_request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http_request: reading response: %w", err)
	}

	output := map[string]any{
		"status_code": resp.StatusCode,
		"body":        string(respBody),
	}

	var parsed any
	if err := json.Unmarshal(respBody, &parsed); err == nil {
		output["body"] = parsed
	}

	if resp.StatusCode >= 400 {
		return &Result{Output: output}, fmt.Errorf("http_request: status %d", resp.StatusCode)
	}

	return &Result{Output: output}, nil
}
