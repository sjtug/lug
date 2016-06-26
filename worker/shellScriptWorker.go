package worker

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
)

// ShellScriptWorker has Worker interface
type ShellScriptWorker struct {
	status Status
	cfg    config.RepoConfig
	signal chan int
	logger *logging.Logger
}

// NewRsyncWorker returns a rsync worker
func NewShellScriptWorker(status *Status,
	cfg config.RepoConfig,
	signal chan int) *ShellScriptWorker {
	return &ShellScriptWorker{*status, cfg, signal, logging.MustGetLogger(cfg["name"])}
}

// GetStatus returns a snapshot of current status
func (w *ShellScriptWorker) GetStatus() Status {
	return w.status
}

// GetConfig returns config of this repo.
func (w *ShellScriptWorker) GetConfig() config.RepoConfig {
	return w.cfg
}

// TriggerSync send start signal to channel
func (w *ShellScriptWorker) TriggerSync() {
	go func() {
		w.signal <- 1
	}()
}

// RunSync launches the worker
func (w *ShellScriptWorker) RunSync() {
	w.status.Idle = true
	for {
		w.logger.Debugf("Worker %s start waiting for signal", w.cfg["name"])
		start := <-w.signal
		w.logger.Debugf("Worker %s finished waiting for signal", w.cfg["name"])
		if start == 1 {
			w.status.Idle = false
			if script, ok := w.cfg["script"]; ok {
				cmd := exec.Command(script)

				// Forwarding config items to shell script as environmental variables
				// Adds a LUG_ prefix to their key
				env := os.Environ()
				for k, v := range w.cfg {
					env = append(env, fmt.Sprintf("LUG_%s=%s", k, v))
				}
				cmd.Env = env

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
