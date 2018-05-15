package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/cosiner/argv"
	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"

	"github.com/sjtug/lug/pkg/config"
	"github.com/sjtug/lug/pkg/exporter"
	"github.com/sjtug/lug/pkg/helper"
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
	rwmutex      sync.RWMutex
}

func convertMapToEnvVars(m map[string]interface{}) (map[string]string, error) {
	result := map[string]string{}
	for k, v := range m {
		switch v.(type) {
		case nil:
			// skip
		case bool:
			if v.(bool) {
				result["LUG_"+k] = "1"
			}
		case int, uint, float32, float64, string:
			result["LUG_"+k] = fmt.Sprint(v)
		default:
			return nil, errors.New("invalid type:" + spew.Sdump(v))
		}
	}
	marshal, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	result["LUG_config_json"] = string(marshal)
	return result, nil
}

// NewShellScriptWorker returns a shell script worker
func NewShellScriptWorker(status Status,
	cfg config.RepoConfig,
	signal chan int) (*ShellScriptWorker, error) {
	name, ok := cfg["name"].(string)
	if !ok {
		return nil, errors.New("No name in config")
	}
	_, ok = cfg["script"].(string)
	if !ok {
		return nil, errors.New("No script in config")
	}
	w := &ShellScriptWorker{
		idle:         status.Idle,
		result:       status.Result,
		lastFinished: status.LastFinished,
		stdout:       helper.NewMaxLengthSlice(status.Stdout, 20),
		stderr:       helper.NewMaxLengthSlice(status.Stderr, 20),
		cfg:          cfg,
		signal:       signal,
		name:         name,
		logger:       log.WithField("worker", name),
		utilities:    []utility{},
	}
	w.utilities = append(w.utilities, newRlimit(w))
	return w, nil

}

// GetStatus returns a snapshot of current status
func (w *ShellScriptWorker) GetStatus() Status {
	w.rwmutex.RLock()
	defer w.rwmutex.RUnlock()
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
		func() {
			w.rwmutex.Lock()
			defer w.rwmutex.Unlock()
			w.idle = true
		}()
		<-w.signal
		func() {
			w.rwmutex.Lock()
			defer w.rwmutex.Unlock()
			w.idle = false
		}()
		w.logger.WithField("event", "signal_received").Debug("finished waiting for signal")
		script, _ := w.cfg["script"]

		args, err := argv.Argv([]rune(script.(string)), getOsEnvsAsMap(), argv.Run)
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
		envvars, err := convertMapToEnvVars(w.cfg)
		if err != nil {
			w.logger.WithField("event", "execution_fail").Error("cannot convert w.cfg to env vars")
			func() {
				w.rwmutex.Lock()
				defer w.rwmutex.Unlock()
				w.result = false
				w.idle = true
			}()
			continue
		}
		for k, v := range envvars {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
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
			func() {
				w.rwmutex.Lock()
				defer w.rwmutex.Unlock()
				w.result = false
				w.idle = true
			}()
			continue
		}
		err = cmd.Wait()
		if err != nil {
			exporter.GetInstance().SyncFail(w.name)
			w.logger.WithField("event", "execution_fail").Error("execution failed")
			func() {
				w.rwmutex.Lock()
				defer w.rwmutex.Unlock()
				w.result = false
				w.idle = true
				w.logger.Infof("Stderr: %s", bufErr.String())
				w.stderr.Put(bufErr.String())
				w.logger.Debugf("Stdout: %s", bufOut.String())
				w.stdout.Put(bufOut.String())
			}()
			continue
		}
		exporter.GetInstance().SyncSuccess(w.name)
		w.logger.WithField("event", "execution_succeed").Info("succeed")
		w.logger.Infof("Stderr: %s", bufErr.String())
		func() {
			w.rwmutex.Lock()
			defer w.rwmutex.Unlock()
			w.stderr.Put(bufErr.String())
			w.logger.Debugf("Stdout: %s", bufOut.String())
			w.stdout.Put(bufOut.String())
			w.result = true
			w.lastFinished = time.Now()
		}()
	}
}
