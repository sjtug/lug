package worker

import "github.com/sirupsen/logrus"

type execResult struct {
	Stdout string
	Stderr string
}

// executor is a layer beneath worker, called by executorInvokeWorker
type executor interface {
	// When called, the executor performs sync for one time
	RunOnce(logger *logrus.Entry, utilities []utility) (execResult, error)
}
