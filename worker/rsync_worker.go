package worker

import (
	"errors"
	"os/exec"
	"time"

	"bytes"
	log "github.com/Sirupsen/logrus"
	"github.com/sjtug/lug/config"
	"github.com/sjtug/lug/helper"
	"github.com/sjtug/lug/exporter"
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
		w.logger.Debug("start waiting for signal")
		w.idle = true
		<-w.signal
		w.idle = false
		w.logger.Debug("finished waiting for signal")
		src, _ := w.cfg["source"]
		dst, _ := w.cfg["path"]
		cmd := exec.Command("rsync", "-aHvh", "--no-o", "--no-g", "--stats",
			"--delete", "--delete-delay", "--safe-links",
			"--timeout=120", "--contimeout=120", src, dst)
		var bufErr, bufOut bytes.Buffer
		cmd.Stdout = &bufOut
		cmd.Stderr = &bufErr
		w.logger.Info("start rsync command")

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
			w.logger.Error("rsync cannot start")
			w.result = false
			w.idle = true
			continue
		}
		err = cmd.Wait()
		if err != nil {
			exporter.GetInstance().SyncFail(w.name)
			w.logger.Error("rsync failed")
			w.result = false
			w.idle = true
			continue
		}
		exporter.GetInstance().SyncSuccess(w.name)
		w.logger.Info("succeed")
		w.logger.Infof("Stderr: %s", bufErr.String())
		w.stderr.Put(bufErr.String())
		w.logger.Debugf("Stdout: %s", bufOut.String())
		w.stdout.Put(bufOut.String())
		w.result = true
		w.lastFinished = time.Now()
	}
}
