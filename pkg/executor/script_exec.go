package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ScriptExec runs either inline script content or a script from the workflow's
// scripts directory through an explicitly selected interpreter. It deliberately
// does not invoke a shell to assemble the command.
type ScriptExec struct{}

func NewScriptExec() *ScriptExec {
	return &ScriptExec{}
}

func (e *ScriptExec) Execute(ctx context.Context, params map[string]any) (*Result, error) {
	interpreter, ok := params["interpreter"].(string)
	if !ok || interpreter == "" {
		return nil, fmt.Errorf("script_exec: interpreter is required")
	}

	content, hasContent := params["content"]
	path, hasPath := params["path"]
	if hasContent == hasPath {
		return nil, fmt.Errorf("script_exec: exactly one of content or path is required")
	}

	var scriptPath string
	var cleanup func()
	if hasContent {
		body, ok := content.(string)
		if !ok {
			return nil, fmt.Errorf("script_exec: content must be a string")
		}
		file, err := os.CreateTemp("", "markov-script-*")
		if err != nil {
			return nil, fmt.Errorf("script_exec: creating temporary script: %w", err)
		}
		scriptPath = file.Name()
		cleanup = func() { _ = os.Remove(scriptPath) }
		if _, err := file.WriteString(body); err != nil {
			_ = file.Close()
			cleanup()
			return nil, fmt.Errorf("script_exec: writing temporary script: %w", err)
		}
		if err := file.Close(); err != nil {
			cleanup()
			return nil, fmt.Errorf("script_exec: closing temporary script: %w", err)
		}
	} else {
		relPath, ok := path.(string)
		if !ok || relPath == "" {
			return nil, fmt.Errorf("script_exec: path must be a non-empty string")
		}
		var err error
		scriptPath, err = resolveScriptPath(relPath, params)
		if err != nil {
			return nil, err
		}
	}
	if cleanup != nil {
		defer cleanup()
	}

	args, err := stringSliceParam(params, "args")
	if err != nil {
		return nil, err
	}
	env, err := scriptEnv(params)
	if err != nil {
		return nil, err
	}

	commandArgs := append([]string{scriptPath}, args...)
	cmd := exec.CommandContext(ctx, interpreter, commandArgs...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	output := map[string]any{
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": exitCode,
	}
	if err != nil {
		return &Result{Output: output}, fmt.Errorf("script_exec: %w\nstderr: %s", err, stderr.String())
	}
	return &Result{Output: output}, nil
}

func resolveScriptPath(path string, params map[string]any) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("script_exec: path must be relative to the workflow scripts directory")
	}
	baseDir, ok := params["_script_dir"].(string)
	if !ok || baseDir == "" {
		return "", fmt.Errorf("script_exec: workflow scripts directory is not configured")
	}
	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("script_exec: resolving scripts directory: %w", err)
	}
	baseDir, err = filepath.EvalSymlinks(baseDir)
	if err != nil {
		return "", fmt.Errorf("script_exec: resolving scripts directory: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(baseDir, path))
	if err != nil {
		return "", fmt.Errorf("script_exec: resolving script path %q: %w", path, err)
	}
	rel, err := filepath.Rel(baseDir, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("script_exec: path %q escapes the workflow scripts directory", path)
	}
	return resolved, nil
}

func stringSliceParam(params map[string]any, name string) ([]string, error) {
	raw, exists := params[name]
	if !exists {
		return nil, nil
	}
	switch values := raw.(type) {
	case []string:
		return values, nil
	case []any:
		result := make([]string, len(values))
		for i, value := range values {
			stringValue, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("script_exec: args[%d] must be a string", i)
			}
			result[i] = stringValue
		}
		return result, nil
	default:
		return nil, fmt.Errorf("script_exec: args must be a list of strings")
	}
}

func scriptEnv(params map[string]any) ([]string, error) {
	env := os.Environ()
	raw, exists := params["env"]
	if !exists {
		return env, nil
	}
	values, ok := stringMapParam(raw)
	if !ok {
		return nil, fmt.Errorf("script_exec: env must be a map of strings")
	}
	indices := make(map[string]int, len(env))
	for i, item := range env {
		key, _, _ := strings.Cut(item, "=")
		indices[key] = i
	}
	for key, value := range values {
		if key == "" || strings.Contains(key, "=") {
			return nil, fmt.Errorf("script_exec: env key %q is invalid", key)
		}
		item := key + "=" + value
		if i, exists := indices[key]; exists {
			env[i] = item
		} else {
			indices[key] = len(env)
			env = append(env, item)
		}
	}
	return env, nil
}

func stringMapParam(raw any) (map[string]string, bool) {
	switch values := raw.(type) {
	case map[string]string:
		return values, true
	case map[string]any:
		result := make(map[string]string, len(values))
		for key, value := range values {
			stringValue, ok := value.(string)
			if !ok {
				return nil, false
			}
			result[key] = stringValue
		}
		return result, true
	default:
		return nil, false
	}
}
