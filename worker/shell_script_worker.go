package worker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
)

// ShellScriptWorker has Worker interface
type ShellScriptWorker struct {
	status    Status
	cfg       config.RepoConfig
	signal    chan int
	logger    *logging.Logger
	utilities []utility
}

// NewShellScriptWorker returns a shell script worker
func NewShellScriptWorker(status *Status,
	cfg config.RepoConfig,
	signal chan int) (*ShellScriptWorker, error) {
	_, ok := cfg["name"]
	if !ok {
		return nil, errors.New("No name in config")
	}
	_, ok = cfg["script"]
	if !ok {
		return nil, errors.New("No script in config")
	}
	w := &ShellScriptWorker{
		status:    *status,
		cfg:       cfg,
		signal:    signal,
		logger:    logging.MustGetLogger(cfg["name"]),
		utilities: []utility{},
	}
	w.utilities = append(w.utilities, newRlimit(w))
	return w, nil

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
	w.signal <- 1
}

// RunSync launches the worker
func (w *ShellScriptWorker) RunSync() {
	for {
		w.logger.Debugf("Worker %s start waiting for signal", w.cfg["name"])
		w.status.Idle = true
		<-w.signal
		w.status.Idle = false
		w.logger.Debugf("Worker %s finished waiting for signal", w.cfg["name"])
		script, _ := w.cfg["script"]
		cmd := exec.Command(script)

		// Forwarding config items to shell script as environmental variables
		// Adds a LUG_ prefix to their key
		env := os.Environ()
		for k, v := range w.cfg {
			env = append(env, fmt.Sprintf("LUG_%s=%s", k, v))
		}
		cmd.Env = env

		w.logger.Infof("Worker %s start execution", w.cfg["name"])
		for _, utility := range w.utilities {
			w.logger.Debug("Executing prehook of ", utility)
			if err := utility.preHook(); err != nil {
				w.logger.Error("Failed to execute preHook:", err)
			}
		}

		err := cmd.Start()

		for _, utility := range w.utilities {
			w.logger.Debug("Executing postHook of ", utility)
			if err := utility.postHook(); err != nil {
				w.logger.Error("Failed to execute postHook:", err)
			}
		}
		if err != nil {
			w.logger.Errorf("Worker %s execution cannot start", w.cfg["name"])
			w.status.Result = false
			w.status.Idle = true
			continue
		}
		err = cmd.Wait()
		if err != nil {
			w.logger.Errorf("Worker %s execution failed", w.cfg["name"])
			w.status.Result = false
			w.status.Idle = true
			continue
		}
		w.logger.Infof("Worker %s succeed", w.cfg["name"])
		w.status.Result = true
		w.status.LastFinished = time.Now()
	}
}
