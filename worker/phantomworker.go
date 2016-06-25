package worker

import (
	"github.com/sjtug/lug/config"
)

type PhantomWorker struct {
	status Status
	cfg    *config.RepoConfig
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
}

func (w *PhantomWorker) RunSync() {
}
