package worker

import (
	"fmt"
	"github.com/sjtug/lug/config"
	"time"
)

// Worker declares interface for workers using diffenent ways of sync.
type Worker interface {
	IsIdle() bool
	GetStatus() *Status
	StartSync() // Wrap the channel.

	getConfig() *config.Config // For test. TODO: remove.
}

// Status shows sync result and last timestamp.
type Status struct {
	Result       bool
	LastFinished time.Time
}

// NewWorker generates a worker by config and log.
func NewWorker(cfg *config.Config, log *Log) Worker {
	var w Worker = &phantomWorker{
		status: Status{Result: true, LastFinished: time.Now()},
		cfg:    cfg,
	}
	return w
}

// Log is for passing compile.
// It will be imported from manager or somewhere else.
type Log struct{}

type phantomWorker struct {
	status Status
	cfg    *config.Config
}

func (w *phantomWorker) IsIdle() bool {
	return true
}

func (w *phantomWorker) GetStatus() *Status {
	return &w.status
}

// GetConfig is for test.
// TODO: remove this func.
func (w *phantomWorker) getConfig() *config.Config {
	return w.cfg
}

func (w *phantomWorker) StartSync() {
}

func Foo() {
	fmt.Println("worker")
}
