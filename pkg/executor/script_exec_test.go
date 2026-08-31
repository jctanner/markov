package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptExecRunsInlineContentWithArgsAndEnv(t *testing.T) {
	t.Setenv("MARKOV_TEST_VALUE", "old-value")
	exec := NewScriptExec()
	result, err := exec.Execute(context.Background(), map[string]any{
		"interpreter": "sh",
		"content":     "printf '%s:%s' \"$1\" \"$MARKOV_TEST_VALUE\"",
		"args":        []any{"argument"},
		"env":         map[string]any{"MARKOV_TEST_VALUE": "environment"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := result.Output["stdout"], "argument:environment"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got, want := result.Output["exit_code"], 0; got != want {
		t.Fatalf("exit_code = %v, want %d", got, want)
	}
}

func TestScriptExecRunsPathFromScriptsDirectory(t *testing.T) {
	scriptsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(scriptsDir, "report.sh"), []byte("printf 'from-file:%s' \"$1\""), 0600); err != nil {
		t.Fatal(err)
	}

	exec := NewScriptExec()
	result, err := exec.Execute(context.Background(), map[string]any{
		"interpreter": "sh",
		"path":        "report.sh",
		"args":        []any{"ok"},
		"_script_dir": scriptsDir,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := result.Output["stdout"], "from-file:ok"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestScriptExecRejectsInvalidScriptSources(t *testing.T) {
	exec := NewScriptExec()
	for name, params := range map[string]map[string]any{
		"neither content nor path": {
			"interpreter": "sh",
		},
		"both content and path": {
			"interpreter": "sh",
			"content":     "echo inline",
			"path":        "file.sh",
		},
		"path escapes scripts directory": {
			"interpreter": "sh",
			"path":        "../file.sh",
			"_script_dir": t.TempDir(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := exec.Execute(context.Background(), params)
			if err == nil {
				t.Fatal("Execute() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), "script_exec:") {
				t.Fatalf("error = %q, want script_exec prefix", err)
			}
		})
	}
}

func TestScriptExecReturnsOutputOnScriptFailure(t *testing.T) {
	exec := NewScriptExec()
	result, err := exec.Execute(context.Background(), map[string]any{
		"interpreter": "sh",
		"content":     "echo failed >&2; exit 7",
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want script failure")
	}
	if result == nil {
		t.Fatal("Execute() result = nil, want captured output")
	}
	if got, want := result.Output["exit_code"], 7; got != want {
		t.Fatalf("exit_code = %v, want %d", got, want)
	}
	if got := result.Output["stderr"]; got != "failed\n" {
		t.Fatalf("stderr = %q, want failed newline", got)
	}
}
