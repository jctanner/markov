// Package jev implements the System One HTTP contract shared by Jev and Kev.
package jev

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type Connection struct {
	Type        string `yaml:"type" json:"type"`
	BaseURL     string `yaml:"base_url" json:"base_url"`
	Model       string `yaml:"model" json:"model"`
	APIKeyEnv   string `yaml:"api_key_env" json:"api_key_env,omitempty"`
	APIKeyFile  string `yaml:"api_key_file" json:"api_key_file,omitempty"`
	Timeout     int    `yaml:"timeout" json:"timeout,omitempty"`
	Concurrency int    `yaml:"concurrency" json:"concurrency,omitempty"`
}

type Question struct {
	Type         string `yaml:"type" json:"type"`
	Instructions any    `yaml:"instructions" json:"instructions,omitempty"`
	Criteria     any    `yaml:"criteria" json:"criteria,omitempty"`
}

type Decision struct {
	Connection   string `yaml:"connection" json:"connection"`
	Type         string `yaml:"type" json:"type"`
	Instructions any    `yaml:"instructions" json:"instructions,omitempty"`
	Criteria     any    `yaml:"criteria" json:"criteria,omitempty"`
	Version      string `yaml:"version" json:"version,omitempty"`
}

func (d Decision) Question() Question {
	return Question{Type: d.Type, Instructions: d.Instructions, Criteria: d.Criteria}
}

func (c Connection) Validate() error {
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("base_url must be an HTTP(S) URL without credentials, query, or fragment")
	}
	if c.Type != "jev" {
		return fmt.Errorf("connection type must be jev")
	}
	if c.APIKeyEnv != "" && c.APIKeyFile != "" {
		return fmt.Errorf("choose api_key_env or api_key_file")
	}
	if c.Timeout < 0 || c.Concurrency < 0 {
		return fmt.Errorf("timeout and concurrency cannot be negative")
	}
	return nil
}

func (q Question) Validate() error {
	switch q.Type {
	case "noul":
		if q.Criteria != nil {
			m, ok := q.Criteria.(map[string]any)
			if !ok || len(m) != 2 {
				return fmt.Errorf("noul criteria must contain true and false")
			}
			if _, ok := m["true"]; !ok {
				return fmt.Errorf("noul criteria missing true")
			}
			if _, ok := m["false"]; !ok {
				return fmt.Errorf("noul criteria missing false")
			}
		}
	case "choice":
		m, ok := q.Criteria.(map[string]any)
		if !ok || len(m) == 0 || len(m) > 255 {
			return fmt.Errorf("choice requires 1–255 criteria")
		}
		for k := range m {
			if strings.TrimSpace(k) == "" {
				return fmt.Errorf("choice criteria keys cannot be empty")
			}
		}
	case "score":
		a, ok := q.Criteria.([]any)
		if !ok || len(a) == 0 {
			return fmt.Errorf("score requires a nonempty criteria list")
		}
	default:
		return fmt.Errorf("unknown question type %q", q.Type)
	}
	return nil
}

func Questions(raw any) (map[string]Question, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var qs map[string]Question
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&qs); err != nil {
		return nil, fmt.Errorf("questions: %w", err)
	}
	if len(qs) == 0 {
		return nil, fmt.Errorf("questions cannot be empty")
	}
	for name, q := range qs {
		if name == "" {
			return nil, fmt.Errorf("question name cannot be empty")
		}
		if err := q.Validate(); err != nil {
			return nil, fmt.Errorf("question %q: %w", name, err)
		}
	}
	return qs, nil
}
