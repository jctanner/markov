//go:build !windows

package executor

import (
	"os/exec"
	"syscall"
	"time"
)

// isolateProcessGroup runs cmd in its own process group and terminates the
// whole group on cancellation, so tools spawned by claude do not outlive it.
func isolateProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second
}
