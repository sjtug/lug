package worker

import (
	"context"

	"github.com/sirupsen/logrus"
)

type execResult struct {
	Stdout string
	Stderr string
}

// executor is a layer beneath worker, called by executorInvokeWorker
type executor interface {
	// RunOnce performs a single sync attempt. The context is honoured for
	// graceful shutdown: an in-flight command started via exec.CommandContext
	// is killed when ctx is cancelled (12-factor IX — no orphaned children).
	RunOnce(ctx context.Context, logger *logrus.Entry, utilities []utility) (execResult, error)
}
