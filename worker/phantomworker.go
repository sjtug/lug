package worker

import (
	"os/exec"
	"time"

	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
)

// RsyncWorker has Worker interface
type RsyncWorker struct {
	status Status
	cfg    config.RepoConfig
	signal chan int
	logger *logging.Logger
}

// NewRsyncWorker returns a rsync worker
func NewRsyncWorker(status *Status,
	cfg config.RepoConfig,
	signal chan int) *RsyncWorker {
	return &RsyncWorker{*status, cfg, signal, logging.MustGetLogger(cfg["name"])}
}

// GetStatus returns a snapshot of current status
func (w *RsyncWorker) GetStatus() Status {
	return w.status
}

// GetConfig returns config of this repo.
func (w *RsyncWorker) GetConfig() config.RepoConfig {
	return w.cfg
}

// TriggerSync send start signal to channel
func (w *RsyncWorker) TriggerSync() {
	go func() {
		w.signal <- 1
	}()
}

// RunSync launches the worker
func (w *RsyncWorker) RunSync() {
	w.status.Idle = true
	for {
		w.logger.Debugf("Worker %s start waiting for signal", w.cfg["name"])
		start := <-w.signal
		w.logger.Debugf("Worker %s finished waiting for signal", w.cfg["name"])
		if start == 1 {
			w.status.Idle = false
			if src, ok := w.cfg["source"]; ok {
				if dst, ok := w.cfg["path"]; ok {
					cmd := exec.Command("rsync", "-aHvh", "--no-o", "--no-g", "--stats",
						"--delete", "--delete-delay", "--safe-links",
						"--timeout=120", "--contimeout=120", src, dst)
					err := cmd.Start()
					if err != nil {
						w.status.Result = false
						w.status.Idle = true
						continue
					}
					err = cmd.Wait()
					if err != nil {
						w.status.Result = false
						w.status.Idle = true
						continue
					}
					w.status.Result = true
					w.status.LastFinished = time.Now()
					w.status.Idle = true
				}
			}
		}
	}
}
