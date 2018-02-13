package worker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"bytes"
	log "github.com/Sirupsen/logrus"
	"github.com/cosiner/argv"
	"github.com/sjtug/lug/config"
	"github.com/sjtug/lug/exporter"
	"github.com/sjtug/lug/helper"
	"strings"
)

// ShellScriptWorker has Worker interface
type ShellScriptWorker struct {
	idle         bool
	result       bool
	lastFinished time.Time
	stdout       *helper.MaxLengthStringSliceAdaptor
	stderr       *helper.MaxLengthStringSliceAdaptor
	cfg          config.RepoConfig
	name         string
	signal       chan int
	logger       *log.Entry
	utilities    []utility
}

// NewShellScriptWorker returns a shell script worker
func NewShellScriptWorker(status Status,
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
		idle:         status.Idle,
		result:       status.Result,
		lastFinished: status.LastFinished,
		stdout:       helper.NewMaxLengthSlice(status.Stdout, 200),
		stderr:       helper.NewMaxLengthSlice(status.Stderr, 200),
		cfg:          cfg,
		signal:       signal,
		name:         cfg["name"],
		logger:       log.WithField("worker", cfg["name"]),
		utilities:    []utility{},
	}
	w.utilities = append(w.utilities, newRlimit(w))
	return w, nil

}

// GetStatus returns a snapshot of current status
func (w *ShellScriptWorker) GetStatus() Status {
	return Status{
		Idle:         w.idle,
		Result:       w.result,
		LastFinished: w.lastFinished,
		Stdout:       w.stdout.GetAll(),
		Stderr:       w.stderr.GetAll(),
	}
}

// GetConfig returns config of this repo.
func (w *ShellScriptWorker) GetConfig() config.RepoConfig {
	return w.cfg
}

// TriggerSync send start signal to channel
func (w *ShellScriptWorker) TriggerSync() {
	w.signal <- 1
}

func getOsEnvsAsMap() (result map[string]string) {
	envs := os.Environ()
	result = map[string]string{}
	for _, e := range envs {
		pair := strings.Split(e, "=")
		key := pair[0]
		val := pair[1]
		result[key] = val
	}
	return
}

// RunSync launches the worker
func (w *ShellScriptWorker) RunSync() {
	for {
		w.logger.WithField("event", "start_wait_signal").Debug("start waiting for signal")
		w.idle = true
		<-w.signal
		w.idle = false
		w.logger.WithField("event", "signal_received").Debug("finished waiting for signal")
		script, _ := w.cfg["script"]

		args, err := argv.Argv([]rune(script), getOsEnvsAsMap(), argv.Run)
		if err != nil {
			w.logger.Error("Failed to parse argument:", err)
			continue
		}
		if len(args) > 1 {
			w.logger.Error("pipe is not supported in shell_script_worker")
		}
		invokeArgs := args[0]
		w.logger.Debug("Invoking args:", invokeArgs)
		cmd := exec.Command(invokeArgs[0], invokeArgs[1:]...)

		// Forwarding config items to shell script as environmental variables
		// Adds a LUG_ prefix to their key
		env := os.Environ()
		for k, v := range w.cfg {
			env = append(env, fmt.Sprintf("LUG_%s=%s", k, v))
		}
		cmd.Env = env

		w.logger.WithField("event", "start_execution").Info("start execution")
		for _, utility := range w.utilities {
			w.logger.WithField("event", "exec_prehook").Debug("Executing prehook of ", utility)
			if err := utility.preHook(); err != nil {
				w.logger.Error("Failed to execute preHook:", err)
			}
		}

		var bufErr, bufOut bytes.Buffer
		cmd.Stdout = &bufOut
		cmd.Stderr = &bufErr

		err = cmd.Start()

		for _, utility := range w.utilities {
			w.logger.WithField("event", "exec_posthook").Debug("Executing postHook of ", utility)
			if err := utility.postHook(); err != nil {
				w.logger.Error("Failed to execute postHook:", err)
			}
		}
		if err != nil {
			w.logger.WithField("event", "execution_fail").Error("execution cannot start")
			w.result = false
			w.idle = true
			continue
		}
		err = cmd.Wait()
		if err != nil {
			exporter.GetInstance().SyncFail(w.name)
			w.logger.WithField("event", "execution_fail").Error("execution failed")
			w.result = false
			w.idle = true
			continue
		}
		exporter.GetInstance().SyncSuccess(w.name)
		w.logger.WithField("event", "execution_succeed").Info("succeed")
		w.logger.Infof("Stderr: %s", bufErr.String())
		w.stderr.Put(bufErr.String())
		w.logger.Debugf("Stdout: %s", bufOut.String())
		w.stdout.Put(bufOut.String())
		w.result = true
		w.lastFinished = time.Now()
	}
}
