package worker

import (
	"fmt"
	"time"

	"github.com/sjtug/lug/config"
)

// Worker declares interface for workers using diffenent ways of sync.
type Worker interface {
	IsIdle() bool
	GetStatus() Status
	RunSync()
	TriggerSync()

	GetConfig() *config.RepoConfig
}

// Status shows sync result and last timestamp.
type Status struct {
	// Result is true if sync succeed, else false
	Result bool
	// Last success time
	LastFinished time.Time
}

type errorString struct {
	s string
}

func (e *errorString) Error() string {
	return e.s
}

// NewWorker generates a worker by config and log.
func NewWorker(cfg *config.RepoConfig) (*Worker, error) {
	if syncType, ok := (*cfg)["type"]; ok {
		switch syncType {
		case "rsync":
			var w Worker = &PhantomWorker{
				status: Status{Result: true, LastFinished: time.Now()},
				cfg:    cfg,
				idle:   false,
				signal: make(chan int),
			}
			return &w, nil
		}
	}
	return nil, &errorString{"new worker fail"}
}

func Foo() {
	fmt.Println("worker")
}
