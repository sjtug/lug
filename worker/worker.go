package worker

import (
	"errors"
	"time"

	"github.com/sjtug/lug/config"
)

// Worker declares interface for workers using diffenent ways of sync.
type Worker interface {
	GetStatus() Status
	RunSync()
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
	// Stdout records outputs to stdout of each command execution
	Stdout []string
	// Stderr records outputs to stderr of each command execution
	Stderr []string
}

// NewWorker generates a worker by config and log.
func NewWorker(cfg config.RepoConfig) (Worker, error) {
	if syncType, ok := cfg["type"]; ok {
		switch syncType {
		case "rsync":
			w, err := NewRsyncWorker(
				&Status{Result: true, LastFinished: time.Now(), Idle: true},
				cfg,
				make(chan int))
			if err != nil {
				return nil, err
			}
			return w, nil
		case "shell_script":
			w, err := NewShellScriptWorker(
				&Status{Result: true, LastFinished: time.Now(), Idle: true},
				cfg,
				make(chan int))
			if err != nil {
				return nil, err
			}
			return w, nil
		}
	}
	return nil, errors.New("Fail to create a new worker")
}
