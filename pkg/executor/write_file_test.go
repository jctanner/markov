package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileCreatesDirectoriesWithModes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "result.json")
	result, err := NewWriteFile().Execute(context.Background(), map[string]any{
		"path": path, "content": map[string]any{"pass": true}, "mode": "0664", "dir_mode": "0775",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "{\n  \"pass\": true\n}\n" {
		t.Fatalf("content = %q", data)
	}
	if result.Output["bytes"] != len(data) {
		t.Fatalf("bytes = %v", result.Output["bytes"])
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o664 {
		t.Fatalf("file mode = %v", info.Mode().Perm())
	}
	if info, _ := os.Stat(filepath.Join(dir, "a")); info.Mode().Perm() != 0o775 {
		t.Fatalf("dir mode = %v", info.Mode().Perm())
	}
}

func TestWriteFileAppendAndErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.txt")
	for _, line := range []string{"one\n", "two\n"} {
		if _, err := NewWriteFile().Execute(context.Background(), map[string]any{"path": path, "content": line, "append": true}); err != nil {
			t.Fatal(err)
		}
	}
	if data, _ := os.ReadFile(path); string(data) != "one\ntwo\n" {
		t.Fatalf("appended = %q", data)
	}
	for _, params := range []map[string]any{
		{"content": "x"},
		{"path": path},
		{"path": path, "content": "x", "mode": "rw"},
	} {
		if _, err := NewWriteFile().Execute(context.Background(), params); err == nil || !strings.HasPrefix(err.Error(), "write_file:") {
			t.Fatalf("params %v: err = %v", params, err)
		}
	}
}

func TestWriteFileSetgidDirMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shared", "x.txt")
	if _, err := NewWriteFile().Execute(context.Background(), map[string]any{
		"path": path, "content": "x", "dir_mode": "2775",
	}); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(dir, "shared"))
	if info.Mode()&os.ModeSetgid == 0 || info.Mode().Perm() != 0o775 {
		t.Fatalf("dir mode = %v, want setgid and 0775", info.Mode())
	}
}
