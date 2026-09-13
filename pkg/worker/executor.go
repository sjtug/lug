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

// executor is a layer beneath worker, called by executorInvokeWorker.
// ctx cancellation must terminate the whole job (including descendants).
type executor interface {
	// When called, the executor performs sync for one time
	RunOnce(ctx context.Context, logger *logrus.Entry) (execResult, error)
}
