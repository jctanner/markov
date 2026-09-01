package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTreeTracksAllFilesModesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "workflow.yaml"), "entrypoint: main\n", 0o644)
	mustWrite(t, filepath.Join(root, "scripts", "task.py"), "print('one')\n", 0o755)
	if err := os.Symlink("scripts/task.py", filepath.Join(root, "task")); err != nil {
		t.Fatal(err)
	}

	first, err := Tree(root, nil)
	if err != nil {
		t.Fatalf("Tree() error = %v", err)
	}
	if first.FileCount != 3 {
		t.Fatalf("FileCount = %d, want 3", first.FileCount)
	}

	mustWrite(t, filepath.Join(root, "scripts", "task.py"), "print('two')\n", 0o755)
	second, err := Tree(root, nil)
	if err != nil {
		t.Fatalf("Tree() error = %v", err)
	}
	if first.Digest == second.Digest {
		t.Fatal("Digest did not change after script content changed")
	}

	mustWrite(t, filepath.Join(root, "scripts", "task.py"), "print('two')\n", 0o644)
	third, err := Tree(root, nil)
	if err != nil {
		t.Fatalf("Tree() error = %v", err)
	}
	if second.Digest == third.Digest {
		t.Fatal("Digest did not change after executable mode changed")
	}
}

func TestTreeExcludesGitAndStateStore(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "workflow.yaml"), "entrypoint: main\n", 0o644)
	mustWrite(t, filepath.Join(root, ".git", "HEAD"), "first\n", 0o644)
	statePath := filepath.Join(root, "markov-state.db")
	mustWrite(t, statePath, "first\n", 0o644)

	first, err := Tree(root, []string{statePath})
	if err != nil {
		t.Fatalf("Tree() error = %v", err)
	}
	mustWrite(t, filepath.Join(root, ".git", "HEAD"), "second\n", 0o644)
	mustWrite(t, statePath, "second\n", 0o644)
	second, err := Tree(root, []string{statePath})
	if err != nil {
		t.Fatalf("Tree() error = %v", err)
	}
	if first.Digest != second.Digest {
		t.Fatal("Digest changed for excluded files")
	}
}

func TestTreeUsesContainingDirectoryForSingleWorkflowFile(t *testing.T) {
	root := t.TempDir()
	workflow := filepath.Join(root, "workflow.yaml")
	mustWrite(t, workflow, "entrypoint: main\n", 0o644)
	mustWrite(t, filepath.Join(root, "scripts", "task.py"), "print('one')\n", 0o644)

	first, err := Tree(workflow, nil)
	if err != nil {
		t.Fatalf("Tree() error = %v", err)
	}
	mustWrite(t, filepath.Join(root, "scripts", "task.py"), "print('two')\n", 0o644)
	second, err := Tree(workflow, nil)
	if err != nil {
		t.Fatalf("Tree() error = %v", err)
	}
	if first.Root != root || first.Digest == second.Digest {
		t.Fatalf("single-file tree = %#v then %#v, want containing-root digest change", first, second)
	}
}

func mustWrite(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}
