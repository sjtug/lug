//go:build !linux

package worker

import (
	"os"
	"os/exec"
)

func configureProcessGroup(cmd *exec.Cmd) {}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	return cmd.Process.Kill()
}
