package worker

import (
	"errors"
	"time"

	"github.com/sjtug/lug/pkg/config"
)

// Worker declares interface for workers using diffenent ways of sync.
type Worker interface {
	// This call should be thread-safe
	GetStatus() Status
	// This should block forever
	RunSync()
	// This call should be thread-safe
	TriggerSync()

	GetConfig() config.RepoConfig
}

// Status shows sync result and last timestamp.
type Status struct {
	// Result is true if sync succeed, else false
	Result bool
	// LastFinished indicates last success time
	LastFinished time.Time
	// Idle stands for whether worker is idle, false if syncing
	Idle bool
	// Last stdout(s) for admin. Internal implementation may vary to provide it in Status()
	Stdout []string
	// Last stderr(s) for admin. Internal implementation may vary to provide it in Status()
	Stderr []string
}

// NewWorker generates a worker by config and log.
func NewWorker(cfg config.RepoConfig, lastFinished time.Time, Result bool) (Worker, error) {
	if syncType, ok := cfg["type"]; ok {
		switch syncType {
		case "rsync":
			return nil, errors.New("rsync worker has been removed since 0.10. " +
				"Use rsync.sh with shell_script worker at https://github.com/sjtug/mirror-docker instead")
		case "shell_script":
			w, err := NewExecutorInvokeWorker(
				newShellScriptExecutor(cfg),
				Status{
					Result:       Result,
					LastFinished: lastFinished,
					Idle:         true,
					Stdout:       make([]string, 0),
					Stderr:       make([]string, 0),
				},
				cfg,
				make(chan int))
			if err != nil {
				return nil, err
			}
			return w, nil
		case "external":
			w, err := NewExternalWorker(cfg)
			if err != nil {
				return nil, err
			}
			return w, nil
		}
	}
	return nil, errors.New("Fail to create a new worker")
}
