package worker

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/dustin/go-humanize"
)

type rlimit struct {
	w Worker
}

func newRlimit(w Worker) *rlimit {
	return &rlimit{
		w: w,
	}
}

type rlimitError string

func (re rlimitError) Error() string {
	return string(re)
}

func (r *rlimit) apply(cmd *exec.Cmd) error {
	cfg := r.w.GetConfig()
	rlimitMem, ok := cfg["rlimit_mem"]
	if !ok {
		return nil
	}
	rlimitMemStr, ok := rlimitMem.(string)
	if !ok || rlimitMemStr == "" {
		return rlimitError("Invalid rlimit_mem: must be a non-empty size string")
	}
	if _, err := humanize.ParseBytes(rlimitMemStr); err != nil {
		return rlimitError(fmt.Sprint("Invalid rlimit_mem: must be size:", err))
	}
	helper, err := rlimitHelperPath()
	if err != nil {
		return rlimitError(fmt.Sprint("Failed to locate rlimit helper:", err))
	}

	// Wrap the command with Lug's hidden rlimit helper. The helper sets
	// RLIMIT_AS on itself and then syscall.Exec's the real command, so the
	// memory limit applies to the sync process (and its descendants) rather
	// than to the long-running Lug daemon.
	originalArgs := append([]string(nil), cmd.Args...)
	cmd.Path = helper
	cmd.Args = append([]string{helper, "__run-with-rlimit", rlimitMemStr, "--"}, originalArgs...)
	return nil
}

func rlimitHelperPath() (string, error) {
	if helper := os.Getenv("LUG_RLIMIT_HELPER"); helper != "" {
		return helper, nil
	}
	return os.Executable()
}
