package worker

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

type execResult struct {
	Stdout string
	Stderr string
	// Telemetry for this attempt. Zero values mean "unavailable"
	// (e.g. cgroup isolation disabled).
	Duration     time.Duration
	PeakMemBytes uint64
	OOMKilled    bool
}

// ActiveJobInfo describes the currently executing sync job. Exposed through
// worker.Status so operators can inspect live resource usage and attach to
// the job's cgroup for debugging.
type ActiveJobInfo struct {
	CgroupPath string    `json:"cgroup_path,omitempty"`
	MainPID    int       `json:"main_pid,omitempty"`
	StartedAt  time.Time `json:"started_at"`
}

// CgroupStats holds live resource counters read from a cgroup v2 directory.
type CgroupStats struct {
	MemoryCurrentBytes uint64 `json:"memory_current_bytes"`
	MemoryPeakBytes    uint64 `json:"memory_peak_bytes,omitempty"`
	MemoryLimitBytes   uint64 `json:"memory_limit_bytes,omitempty"`
	PIDs               int    `json:"pids"`
}

// executor is a layer beneath worker, called by executorInvokeWorker.
// ctx cancellation must terminate the whole job (including descendants).
type executor interface {
	// When called, the executor performs sync for one time
	RunOnce(ctx context.Context, logger *logrus.Entry) (execResult, error)
}

// jobInspector is optionally implemented by executors that expose live
// information about the currently executing job.
type jobInspector interface {
	ActiveJob() *ActiveJobInfo
}
