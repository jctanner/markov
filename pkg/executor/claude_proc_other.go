//go:build windows

package executor

import "os/exec"

func isolateProcessGroup(cmd *exec.Cmd) {}
