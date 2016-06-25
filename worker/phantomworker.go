package worker

import (
	"github.com/sjtug/lug/config"
)

type PhantomWorker struct {
	status Status
	cfg    *config.RepoConfig
	idle   bool

	signal chan int
}

func (w *PhantomWorker) IsIdle() bool {
	return true
}

func (w *PhantomWorker) GetStatus() *Status {
	return &w.status
}

// GetConfig is for test.
// TODO: remove this func.
func (w *PhantomWorker) getConfig() *config.RepoConfig {
	return w.cfg
}

func (w *PhantomWorker) TriggerSync() {
	w.signal <- 1
}

func (w *PhantomWorker) RunSync() {
	w.idle = true
	for {
		start := <-w.signal
		if start == 1 {
			w.idle = false
			if _, ok := (*w.cfg)["source"]; ok {
			}
		}
	}
}
