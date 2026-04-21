package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

type ShellExec struct{}

func NewShellExec() *ShellExec {
	return &ShellExec{}
}

func (e *ShellExec) Execute(ctx context.Context, params map[string]any) (*Result, error) {
	command, ok := params["command"].(string)
	if !ok || command == "" {
		return nil, fmt.Errorf("shell_exec: command is required")
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := map[string]any{
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": cmd.ProcessState.ExitCode(),
	}

	if err != nil {
		return &Result{Output: output}, fmt.Errorf("shell_exec: %w\nstderr: %s", err, stderr.String())
	}

	return &Result{Output: output}, nil
}
