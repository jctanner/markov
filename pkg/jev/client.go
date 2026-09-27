package jev

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	connections map[string]Connection
	limits      map[string]chan struct{}
	http        *http.Client
}

func New(connections map[string]Connection) *Client {
	c := &Client{connections: map[string]Connection{}, limits: map[string]chan struct{}{}, http: &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
	for name, conn := range connections {
		c.connections[name] = conn
		n := conn.Concurrency
		if n <= 0 {
			n = 1
		}
		c.limits[name] = make(chan struct{}, n)
	}
	return c
}

func (c *Client) Execute(ctx context.Context, name string, state any, qs map[string]Question) (map[string]any, error) {
	conn, ok := c.connections[name]
	if !ok {
		return nil, fmt.Errorf("unknown Jev connection %q", name)
	}
	if err := conn.Validate(); err != nil {
		return nil, err
	}
	if len(qs) == 0 {
		return nil, fmt.Errorf("questions cannot be empty")
	}
	for key, q := range qs {
		if err := q.Validate(); err != nil {
			return nil, fmt.Errorf("question %q: %w", key, err)
		}
	}
	seconds := conn.Timeout
	if seconds == 0 {
		seconds = 120
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()
	select {
	case c.limits[name] <- struct{}{}:
		defer func() { <-c.limits[name] }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	key := ""
	if conn.APIKeyEnv != "" {
		key = os.Getenv(conn.APIKeyEnv)
		if key == "" {
			return nil, fmt.Errorf("Jev credential environment variable %q is empty", conn.APIKeyEnv)
		}
	}
	if conn.APIKeyFile != "" {
		b, err := os.ReadFile(conn.APIKeyFile)
		if err != nil {
			return nil, fmt.Errorf("reading Jev credential file: %w", err)
		}
		key = strings.TrimSpace(string(b))
		if key == "" {
			return nil, fmt.Errorf("Jev credential file is empty")
		}
	}
	model := conn.Model
	if model == "" {
		model = "jev-latest"
	}
	body, err := json.Marshal(map[string]any{"model": model, "state": state, "questions": qs})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(append([]byte(conn.BaseURL+"\x00"), body...))
	started := time.Now()
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(conn.BaseURL, "/")+"/v1/systemone", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			// Transport errors may include a URL; never include request headers or the secret.
			return nil, fmt.Errorf("Jev transport failed: %w", err)
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024+1))
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading Jev response: %w", readErr)
		}
		if len(data) > 8*1024*1024 {
			return nil, fmt.Errorf("Jev response exceeds 8 MiB")
		}
		if (resp.StatusCode == 429 || resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504) && attempt < 2 {
			delay := time.Duration(attempt+1) * time.Second
			if n, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && n > 0 {
				delay = time.Duration(n) * time.Second
				if delay > 30*time.Second {
					delay = 30 * time.Second
				}
			}
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("Jev HTTP status %d", resp.StatusCode)
		}
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, fmt.Errorf("Jev returned invalid JSON")
		}
		if err := validateResponse(out, qs); err != nil {
			return nil, err
		}
		out["metadata"] = map[string]any{"connection": name, "requested_model": model, "request_id": resp.Header.Get("x-typesafe-request-id"), "elapsed_seconds": time.Since(started).Seconds(), "attempts": attempt + 1, "request_fingerprint": hex.EncodeToString(digest[:])}
		return out, nil
	}
	return nil, fmt.Errorf("Jev retries exhausted")
}

func number(v any, lo, hi float64) bool {
	n, ok := v.(float64)
	return ok && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= lo && n <= hi
}

func validateResponse(out map[string]any, qs map[string]Question) error {
	if model, ok := out["model"].(string); !ok || model == "" {
		return fmt.Errorf("Jev response missing model")
	}
	usage, ok := out["usage"].(map[string]any)
	if !ok || !number(usage["input_tokens"], 0, math.MaxFloat64) || !number(usage["output_tokens"], 0, math.MaxFloat64) {
		return fmt.Errorf("Jev response missing valid usage")
	}
	answers, ok := out["answers"].(map[string]any)
	if !ok || len(answers) != len(qs) {
		return fmt.Errorf("Jev response answer count mismatch")
	}
	for name, q := range qs {
		a, ok := answers[name].(map[string]any)
		if !ok || a["type"] != q.Type {
			return fmt.Errorf("Jev answer %q type mismatch", name)
		}
		if q.Type == "noul" {
			if !number(a["noul"], 0, 1) {
				return fmt.Errorf("Jev answer %q has invalid noul", name)
			}
			continue
		}
		expected := map[string]bool{}
		if q.Type == "choice" {
			for key := range q.Criteria.(map[string]any) {
				expected[key] = true
			}
			choice, ok := a["choice"].(string)
			if !ok || !expected[choice] {
				return fmt.Errorf("Jev answer %q has invalid choice", name)
			}
		} else {
			for i := range q.Criteria.([]any) {
				expected[strconv.Itoa(i)] = true
			}
			if !number(a["score"], 0, float64(len(expected)-1)) {
				return fmt.Errorf("Jev answer %q has invalid score", name)
			}
			legend, ok := a["legend"].(map[string]any)
			if !ok || len(legend) != len(expected) {
				return fmt.Errorf("Jev answer %q has invalid legend", name)
			}
			for key := range expected {
				if _, ok := legend[key]; !ok {
					return fmt.Errorf("Jev answer %q has incomplete legend", name)
				}
			}
		}
		probs, ok := a["probabilities"].(map[string]any)
		if !ok || len(probs) != len(expected) {
			return fmt.Errorf("Jev answer %q has invalid probabilities", name)
		}
		sum := 0.0
		for key := range expected {
			if !number(probs[key], 0, 1) {
				return fmt.Errorf("Jev answer %q has invalid probability", name)
			}
			sum += probs[key].(float64)
		}
		if math.Abs(sum-1) > 0.03 {
			return fmt.Errorf("Jev answer %q probabilities do not sum to one", name)
		}
		if !number(a["confidence"], 0, 1) {
			return fmt.Errorf("Jev answer %q has invalid confidence", name)
		}
	}
	return nil
}
