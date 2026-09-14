//go:build !linux

package worker

import "os/exec"

// Non-Linux stub: cgroup job isolation is Linux-only; executors fall back to
// plain subprocess execution with no memory enforcement or peak telemetry.

type jobCgroup struct{}

func newJobCgroup(name string, memMax uint64) (*jobCgroup, error) { return nil, nil }

func (j *jobCgroup) Path() string                        { return "" }
func (j *jobCgroup) Kill() error                         { return nil }
func (j *jobCgroup) Collect() (peakMem uint64, oom bool) { return 0, false }
func (j *jobCgroup) Close()                              {}
func attachToCgroup(cmd *exec.Cmd, cg *jobCgroup)        {}

// ReadCgroupStats is a no-op on non-Linux platforms.
func ReadCgroupStats(dir string) CgroupStats { return CgroupStats{} }
