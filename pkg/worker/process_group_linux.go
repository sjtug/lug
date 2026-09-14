//go:build linux

package worker

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// configureProcessGroup starts the command as a process-group leader so the
// fallback cancellation path can terminate wrappers and all descendants.
func configureProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	// Descendants retain the leader's process-group ID even if the wrapper has
	// already exited. Address the group directly instead of killing only the
	// process represented by cmd.Process.
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return nil
}
