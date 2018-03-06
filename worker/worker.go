package worker

import (
	"errors"
	"time"

	"github.com/sjtug/lug/config"
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
func NewWorker(cfg config.RepoConfig) (Worker, error) {
	if syncType, ok := cfg["type"]; ok {
		switch syncType {
		case "rsync":
			w, err := NewRsyncWorker(
				Status{
					Result:       true,
					LastFinished: time.Now(),
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
		case "shell_script":
			w, err := NewShellScriptWorker(
				Status{
					Result:       true,
					LastFinished: time.Now(),
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
