package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
	if err := applyHeaders(req, params["headers"]); err != nil {
		return nil, err
	}
	if err := applyBasicAuth(req, params["basic_auth"]); err != nil {
		return nil, err
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

	ignoreStatus, err := shouldIgnoreStatus(resp.StatusCode, params["ignore_status"])
	if err != nil {
		return &Result{Output: output}, err
	}
	if resp.StatusCode >= 400 && !ignoreStatus {
		return &Result{Output: output}, fmt.Errorf("http_request: status %d", resp.StatusCode)
	}

	return &Result{Output: output}, nil
}

func applyHeaders(req *http.Request, raw any) error {
	if raw == nil {
		return nil
	}
	for key, value := range stringMap(raw) {
		if key == "" {
			return fmt.Errorf("http_request: headers contains empty key")
		}
		req.Header.Set(key, value)
	}
	return nil
}

func applyBasicAuth(req *http.Request, raw any) error {
	if raw == nil {
		return nil
	}
	auth := stringMap(raw)
	username, ok := auth["username"]
	if !ok {
		return fmt.Errorf("http_request: basic_auth.username is required")
	}
	password, ok := auth["password"]
	if !ok {
		return fmt.Errorf("http_request: basic_auth.password is required")
	}
	req.SetBasicAuth(username, password)
	return nil
}

func shouldIgnoreStatus(statusCode int, raw any) (bool, error) {
	switch v := raw.(type) {
	case nil:
		return false, nil
	case bool:
		return v && statusCode >= 400, nil
	case []any:
		for _, item := range v {
			code, err := statusCodeValue(item)
			if err != nil {
				return false, err
			}
			if code == statusCode {
				return true, nil
			}
		}
		return false, nil
	case []int:
		for _, code := range v {
			if code == statusCode {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("http_request: ignore_status must be a bool or list of status codes")
	}
}

func statusCodeValue(raw any) (int, error) {
	switch v := raw.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		if v != float64(int(v)) {
			return 0, fmt.Errorf("http_request: ignore_status contains non-integer status code %v", v)
		}
		return int(v), nil
	case string:
		code, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("http_request: ignore_status contains invalid status code %q", v)
		}
		return code, nil
	default:
		return 0, fmt.Errorf("http_request: ignore_status contains invalid status code %v", raw)
	}
}

func stringMap(raw any) map[string]string {
	switch v := raw.(type) {
	case map[string]string:
		return v
	case map[string]any:
		out := make(map[string]string, len(v))
		for key, value := range v {
			out[key] = fmt.Sprint(value)
		}
		return out
	default:
		return map[string]string{}
	}
}
