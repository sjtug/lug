package worker

import (
	"fmt"
	"time"

	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
)

// Worker declares interface for workers using diffenent ways of sync.
type Worker interface {
	IsIdle() bool
	GetStatus() *Status
	RunSync()
	TriggerSync()

	getConfig() *config.RepoConfig // For test. TODO: remove.
}

// Status shows sync result and last timestamp.
type Status struct {
	// Result is true if sync succeed, else false
	Result bool
	// Last success time
	LastFinished time.Time
}

// NewWorker generates a worker by config and log.
func NewWorker(cfg *config.RepoConfig, log *logging.Logger) Worker {
	var w Worker = &PhantomWorker{
		status: Status{Result: true, LastFinished: time.Now()},
		cfg:    cfg,
	}
	return w
}

func Foo() {
	fmt.Println("worker")
}
