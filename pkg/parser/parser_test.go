package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFileSingleFileStillWorks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.yaml")
	writeFile(t, path, `
entrypoint: main
vars:
  greeting: hello
workflows:
  - name: main
    steps:
      - name: say_hello
        type: shell_exec
        params:
          command: "echo {{ greeting }}"
`)

	wf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if wf.Entrypoint != "main" {
		t.Fatalf("Entrypoint = %q, want main", wf.Entrypoint)
	}
	if len(wf.Workflows) != 1 {
		t.Fatalf("len(Workflows) = %d, want 1", len(wf.Workflows))
	}
}

func TestParseDirMergesDirectoryWorkflow(t *testing.T) {
	dir := makeDirectoryWorkflow(t)

	wf, err := ParseFile(dir)
	if err != nil {
		t.Fatalf("ParseFile(directory) error = %v", err)
	}

	if wf.Entrypoint != "main" {
		t.Fatalf("Entrypoint = %q, want main", wf.Entrypoint)
	}
	if wf.Namespace != "markov-test" {
		t.Fatalf("Namespace = %q, want markov-test", wf.Namespace)
	}
	if wf.Forks != 2 {
		t.Fatalf("Forks = %d, want 2", wf.Forks)
	}
	if wf.Vars["greeting"] != "hello from directory" {
		t.Fatalf("Vars[greeting] = %v, want hello from directory", wf.Vars["greeting"])
	}
	if _, ok := wf.StepTypes["echo_local"]; !ok {
		t.Fatalf("step type echo_local missing")
	}
	if len(wf.Rules) != 1 || wf.Rules[0].Name != "always_continue" {
		t.Fatalf("Rules = %#v, want always_continue", wf.Rules)
	}
	if wf.GetWorkflow("main") == nil || wf.GetWorkflow("child") == nil {
		t.Fatalf("expected main and child workflows, got %#v", wf.Workflows)
	}
}

func TestParseDirRequiresCategoryFiles(t *testing.T) {
	dir := makeDirectoryWorkflow(t)
	if err := os.Remove(filepath.Join(dir, "vars.yaml")); err != nil {
		t.Fatal(err)
	}

	_, err := ParseFile(dir)
	if err == nil {
		t.Fatalf("ParseFile(directory) error = nil, want missing vars.yaml error")
	}
	if !strings.Contains(err.Error(), "vars.yaml") {
		t.Fatalf("error = %q, want vars.yaml", err)
	}
}

func TestParseDirRejectsDuplicateRules(t *testing.T) {
	dir := makeDirectoryWorkflow(t)
	writeFile(t, filepath.Join(dir, "rules.yaml"), `
- name: duplicate
  when: "true"
  action: continue
- name: duplicate
  when: "false"
  action: skip
`)

	_, err := ParseFile(dir)
	if err == nil {
		t.Fatalf("ParseFile(directory) error = nil, want duplicate rule error")
	}
	if !strings.Contains(err.Error(), `duplicate rule name "duplicate"`) {
		t.Fatalf("error = %q, want duplicate rule error", err)
	}
}

func TestParseDirRejectsDuplicateWorkflows(t *testing.T) {
	dir := makeDirectoryWorkflow(t)
	writeFile(t, filepath.Join(dir, "workflows", "duplicate.yaml"), `
name: main
steps:
  - name: other
    type: shell_exec
    params:
      command: "echo duplicate"
`)

	_, err := ParseFile(dir)
	if err == nil {
		t.Fatalf("ParseFile(directory) error = nil, want duplicate workflow error")
	}
	if !strings.Contains(err.Error(), `duplicate workflow name "main"`) {
		t.Fatalf("error = %q, want duplicate workflow error", err)
	}
}

func TestParseDirRejectsDuplicateStepTypeKeys(t *testing.T) {
	dir := makeDirectoryWorkflow(t)
	writeFile(t, filepath.Join(dir, "step_types.yaml"), `
echo_local:
  base: shell_exec
echo_local:
  base: http_request
`)

	_, err := ParseFile(dir)
	if err == nil {
		t.Fatalf("ParseFile(directory) error = nil, want duplicate step type key error")
	}
	if !strings.Contains(err.Error(), "step_types.yaml") {
		t.Fatalf("error = %q, want step_types.yaml parse error", err)
	}
}

func TestParseDirLoadsStepTypesDirectory(t *testing.T) {
	dir := makeDirectoryWorkflow(t)
	if err := os.Remove(filepath.Join(dir, "step_types.yaml")); err != nil {
		t.Fatal(err)
	}
	stepTypesDir := filepath.Join(dir, "step_types")
	if err := os.MkdirAll(stepTypesDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(stepTypesDir, "shell.yaml"), `
echo_local:
  base: shell_exec
`)
	writeFile(t, filepath.Join(stepTypesDir, "http.yaml"), `
get_json:
  base: http_request
  defaults:
    method: GET
`)

	wf, err := ParseFile(dir)
	if err != nil {
		t.Fatalf("ParseFile(directory) error = %v", err)
	}
	if _, ok := wf.StepTypes["echo_local"]; !ok {
		t.Fatalf("step type echo_local missing")
	}
	if _, ok := wf.StepTypes["get_json"]; !ok {
		t.Fatalf("step type get_json missing")
	}
}

func TestParseDirRejectsDuplicateStepTypesAcrossDirectoryFiles(t *testing.T) {
	dir := makeDirectoryWorkflow(t)
	if err := os.Remove(filepath.Join(dir, "step_types.yaml")); err != nil {
		t.Fatal(err)
	}
	stepTypesDir := filepath.Join(dir, "step_types")
	if err := os.MkdirAll(stepTypesDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(stepTypesDir, "a.yaml"), `
echo_local:
  base: shell_exec
`)
	writeFile(t, filepath.Join(stepTypesDir, "b.yaml"), `
echo_local:
  base: http_request
`)

	_, err := ParseFile(dir)
	if err == nil {
		t.Fatalf("ParseFile(directory) error = nil, want duplicate step type error")
	}
	if !strings.Contains(err.Error(), `duplicate step type "echo_local"`) {
		t.Fatalf("error = %q, want duplicate step type error", err)
	}
	if !strings.Contains(err.Error(), "step_types") {
		t.Fatalf("error = %q, want step_types path", err)
	}
}

func TestParseDirAllowsEmptyCategoryFiles(t *testing.T) {
	dir := makeDirectoryWorkflow(t)
	writeFile(t, filepath.Join(dir, "vars.yaml"), "")
	writeFile(t, filepath.Join(dir, "rules.yaml"), "")
	writeFile(t, filepath.Join(dir, "step_types.yaml"), "")
	writeFile(t, filepath.Join(dir, "workflows", "main.yaml"), `
name: main
steps:
  - name: say_hello
    type: shell_exec
    params:
      command: "echo hello"
`)

	wf, err := ParseFile(dir)
	if err != nil {
		t.Fatalf("ParseFile(directory) error = %v", err)
	}
	if len(wf.Vars) != 0 {
		t.Fatalf("Vars = %#v, want empty map", wf.Vars)
	}
	if len(wf.Rules) != 0 {
		t.Fatalf("Rules = %#v, want empty list", wf.Rules)
	}
	if len(wf.StepTypes) != 0 {
		t.Fatalf("StepTypes = %#v, want empty map", wf.StepTypes)
	}
}

func TestParseDirLoadsRuleFilesRelativeToDirectoryRoot(t *testing.T) {
	dir := makeDirectoryWorkflow(t)
	rulesDir := filepath.Join(dir, "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "rules.yaml"), `
- file: rules/common.yaml
`)
	writeFile(t, filepath.Join(rulesDir, "common.yaml"), `
rules:
  - name: from_external_file
    when: "true"
    action: continue
`)
	writeFile(t, filepath.Join(dir, "workflows", "main.yaml"), `
name: main
steps:
  - name: check_gate
    type: gate
    rules:
      - from_external_file
`)

	wf, err := ParseFile(dir)
	if err != nil {
		t.Fatalf("ParseFile(directory) error = %v", err)
	}
	if len(wf.Rules) != 1 || wf.Rules[0].Name != "from_external_file" {
		t.Fatalf("Rules = %#v, want external rule", wf.Rules)
	}
}

func TestResolveStepTypeMergesHeaders(t *testing.T) {
	wf := &WorkflowFile{
		StepTypes: map[string]StepType{
			"github_api": {
				Base: "http_request",
				Defaults: map[string]any{
					"headers": map[string]any{
						"User-Agent": "markov",
					},
				},
				Params: map[string]any{
					"base_url": "https://github.example/api/v3",
					"basic_auth": map[string]any{
						"username": "api-user",
						"password": "api-token",
					},
					"headers": map[string]any{
						"Accept":        "application/vnd.github+json",
						"Authorization": "token default",
					},
				},
			},
		},
	}
	step := &Step{
		Type: "github_api",
		Params: map[string]any{
			"path": "/repos/example/repo",
			"headers": map[string]any{
				"Authorization": "token step",
				"X-Request-ID":  "abc123",
			},
		},
	}

	base, params := wf.ResolveStepType(step)
	if base != "http_request" {
		t.Fatalf("base = %q, want http_request", base)
	}
	headers, ok := params["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers = %#v, want map[string]any", params["headers"])
	}
	if headers["Accept"] != "application/vnd.github+json" {
		t.Fatalf("Accept = %v, want default header", headers["Accept"])
	}
	if headers["User-Agent"] != "markov" {
		t.Fatalf("User-Agent = %v, want default header", headers["User-Agent"])
	}
	if headers["Authorization"] != "token step" {
		t.Fatalf("Authorization = %v, want step override", headers["Authorization"])
	}
	if headers["X-Request-ID"] != "abc123" {
		t.Fatalf("X-Request-ID = %v, want step header", headers["X-Request-ID"])
	}
	basicAuth, ok := params["basic_auth"].(map[string]any)
	if !ok {
		t.Fatalf("basic_auth = %#v, want map[string]any", params["basic_auth"])
	}
	if basicAuth["username"] != "api-user" || basicAuth["password"] != "api-token" {
		t.Fatalf("basic_auth = %#v, want step type values", basicAuth)
	}
}

func makeDirectoryWorkflow(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "workflows"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "meta.yaml"), `
entrypoint: main
namespace: markov-test
forks: 2
`)
	writeFile(t, filepath.Join(dir, "vars.yaml"), `
greeting: hello from directory
`)
	writeFile(t, filepath.Join(dir, "rules.yaml"), `
- name: always_continue
  when: "true"
  action: continue
`)
	writeFile(t, filepath.Join(dir, "step_types.yaml"), `
echo_local:
  base: shell_exec
`)
	writeFile(t, filepath.Join(dir, "workflows", "main.yaml"), `
name: main
steps:
  - name: say_hello
    type: echo_local
    params:
      command: "echo {{ greeting }}"
  - name: run_child
    workflow: child
`)
	writeFile(t, filepath.Join(dir, "workflows", "child.yaml"), `
name: child
steps:
  - name: child_step
    type: shell_exec
    params:
      command: "echo child"
`)
	return dir
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimPrefix(content, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
}
