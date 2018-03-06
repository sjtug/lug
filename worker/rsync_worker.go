package worker

import (
	"errors"
	"os/exec"
	"time"

	"bytes"
	log "github.com/sirupsen/logrus"
	"github.com/sjtug/lug/config"
	"github.com/sjtug/lug/exporter"
	"github.com/sjtug/lug/helper"
	"sync"
)

// RsyncWorker implements Worker interface
type RsyncWorker struct {
	cfg          config.RepoConfig
	name         string
	signal       chan int
	logger       *log.Entry
	utilities    []utility
	idle         bool
	result       bool
	lastFinished time.Time
	stdout       *helper.MaxLengthStringSliceAdaptor
	stderr       *helper.MaxLengthStringSliceAdaptor
	rwmutex      sync.RWMutex
}

// NewRsyncWorker returns a rsync worker
// Error when necessary keys not founded in repo config
func NewRsyncWorker(status Status,
	cfg config.RepoConfig,
	signal chan int) (*RsyncWorker, error) {
	_, ok := cfg["name"]
	if !ok {
		return nil, errors.New("No name in config")
	}
	_, ok = cfg["source"]
	if !ok {
		return nil, errors.New("No source in config")
	}
	_, ok = cfg["path"]
	if !ok {
		return nil, errors.New("No path in config")
	}
	w := &RsyncWorker{
		idle:         status.Idle,
		result:       status.Result,
		lastFinished: status.LastFinished,
		stdout:       helper.NewMaxLengthSlice(status.Stdout, 200),
		stderr:       helper.NewMaxLengthSlice(status.Stderr, 200),
		cfg:          cfg,
		signal:       signal,
		utilities:    []utility{},
		name:         cfg["name"],
		logger:       log.WithField("worker", cfg["name"]),
	}
	w.utilities = append(w.utilities, newRlimit(w))
	return w, nil
}

// GetStatus returns a snapshot of current worker status
func (w *RsyncWorker) GetStatus() Status {
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

// GetConfig returns config of this repo
func (w *RsyncWorker) GetConfig() config.RepoConfig {
	return w.cfg
}

// TriggerSync sends start signal to channel
func (w *RsyncWorker) TriggerSync() {
	w.signal <- 1
}

// RunSync launches the worker and waits signal from channel
func (w *RsyncWorker) RunSync() {
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
		src, _ := w.cfg["source"]
		dst, _ := w.cfg["path"]
		cmd := exec.Command("rsync", "-aHvh", "--no-o", "--no-g", "--stats",
			"--delete", "--delete-delay", "--safe-links",
			"--timeout=120", "--contimeout=120", src, dst)
		var bufErr, bufOut bytes.Buffer
		cmd.Stdout = &bufOut
		cmd.Stderr = &bufErr
		w.logger.WithField("event", "start_execution").Info("start rsync command")

		for _, utility := range w.utilities {
			w.logger.WithField("event", "exec_prehook").Debug("Executing prehook of ", utility)
			if err := utility.preHook(); err != nil {
				w.logger.Error("Failed to execute preHook:", err)
			}
		}

		err := cmd.Start()

		for _, utility := range w.utilities {
			w.logger.WithField("event", "exec_posthook").Debug("Executing postHook of ", utility)
			if err := utility.postHook(); err != nil {
				w.logger.Error("Failed to execute postHook:", err)
			}
		}

		if err != nil {
			w.logger.WithField("event", "execution_fail").Error("rsync cannot start")
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
			exporter.GetInstance().UpdateDiskUsage(w.name, w.cfg["path"])
			w.logger.WithField("event", "execution_fail").Error("rsync failed")
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
		exporter.GetInstance().UpdateDiskUsage(w.name, w.cfg["path"])
		func() {
			w.rwmutex.Lock()
			defer w.rwmutex.Unlock()
			w.logger.WithField("event", "execution_succeed").Info("succeed")
			w.logger.Infof("Stderr: %s", bufErr.String())
			w.stderr.Put(bufErr.String())
			w.logger.Debugf("Stdout: %s", bufOut.String())
			w.stdout.Put(bufOut.String())
			w.result = true
			w.lastFinished = time.Now()
		}()
	}
}
