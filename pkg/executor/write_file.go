package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// WriteFile writes content to a file on the runner, creating parent directories. It is the
// file-writing counterpart of script_exec: the content is a rendered template or a value (a map
// or list is written as indented JSON), with no size limit and no shell quoting.
type WriteFile struct{}

func NewWriteFile() *WriteFile {
	return &WriteFile{}
}

func (e *WriteFile) Execute(ctx context.Context, params map[string]any) (*Result, error) {
	path, _ := params["path"].(string)
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("write_file: path is required")
	}
	raw, ok := params["content"]
	if !ok {
		return nil, fmt.Errorf("write_file: content is required")
	}
	content, err := fileContent(raw)
	if err != nil {
		return nil, err
	}
	fileMode, err := modeParam(params, "mode", 0o644)
	if err != nil {
		return nil, err
	}
	dirMode, err := modeParam(params, "dir_mode", 0o755)
	if err != nil {
		return nil, err
	}
	appendMode, _ := params["append"].(bool)

	if err := mkdirAllMode(filepath.Dir(path), dirMode); err != nil {
		return nil, fmt.Errorf("write_file: creating %s: %w", filepath.Dir(path), err)
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if appendMode {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}
	f, err := os.OpenFile(path, flags, fileMode)
	if err != nil {
		return nil, fmt.Errorf("write_file: %w", err)
	}
	n, err := f.WriteString(content)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, fmt.Errorf("write_file: writing %s: %w", path, err)
	}
	// The umask can strip bits from the requested mode; apply it exactly.
	if err := os.Chmod(path, fileMode); err != nil {
		return nil, fmt.Errorf("write_file: chmod %s: %w", path, err)
	}
	return &Result{Output: map[string]any{"path": path, "bytes": n}}, nil
}

func fileContent(raw any) (string, error) {
	switch v := raw.(type) {
	case string:
		return v, nil
	case nil:
		return "", nil
	case map[string]any, []any, []map[string]any:
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "", fmt.Errorf("write_file: content: %w", err)
		}
		return string(data) + "\n", nil
	default:
		return fmt.Sprint(v), nil
	}
}

// modeParam reads an octal file mode such as "0664" (a string, so YAML doesn't read it as a
// decimal number).
func modeParam(params map[string]any, name string, def os.FileMode) (os.FileMode, error) {
	raw, ok := params[name]
	if !ok || raw == nil || raw == "" {
		return def, nil
	}
	text := fmt.Sprint(raw)
	mode, err := strconv.ParseUint(text, 8, 32)
	if err != nil || mode > 0o7777 {
		return 0, fmt.Errorf("write_file: %s must be an octal mode such as \"0644\", got %q", name, text)
	}
	return os.FileMode(mode), nil
}

// mkdirAllMode creates missing directories with exactly mode (not reduced by the umask);
// existing directories are left as they are.
func mkdirAllMode(dir string, mode os.FileMode) error {
	if info, err := os.Stat(dir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", dir)
		}
		return nil
	}
	if err := mkdirAllMode(filepath.Dir(dir), mode); err != nil {
		return err
	}
	if err := os.Mkdir(dir, mode); err != nil && !os.IsExist(err) {
		return err
	}
	return os.Chmod(dir, mode|(dirSetgid(dir)))
}

// dirSetgid keeps a parent's setgid bit, so new directories stay in a shared volume's group.
func dirSetgid(dir string) os.FileMode {
	if info, err := os.Stat(filepath.Dir(dir)); err == nil && info.Mode()&os.ModeSetgid != 0 {
		return os.ModeSetgid
	}
	return 0
}
