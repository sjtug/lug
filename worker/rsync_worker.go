package worker

import (
	"errors"
	"os/exec"
	"time"

	"bytes"
	log "github.com/Sirupsen/logrus"
	"github.com/sjtug/lug/config"
)

// RsyncWorker implements Worker interface
type RsyncWorker struct {
	status    Status
	cfg       config.RepoConfig
	signal    chan int
	logger    *log.Entry
	utilities []utility
}

// NewRsyncWorker returns a rsync worker
// Error when necessary keys not founded in repo config
func NewRsyncWorker(status *Status,
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
		status:    *status,
		cfg:       cfg,
		signal:    signal,
		utilities: []utility{},
		logger:    log.WithField("worker", cfg["name"]),
	}
	w.utilities = append(w.utilities, newRlimit(w))
	return w, nil
}

// GetStatus returns a snapshot of current worker status
func (w *RsyncWorker) GetStatus() Status {
	return w.status
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
		w.status.Idle = true
		<-w.signal
		w.status.Idle = false
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
			w.status.Result = false
			w.status.Idle = true
			continue
		}
		err = cmd.Wait()
		if err != nil {
			w.logger.Error("rsync failed")
			w.status.Result = false
			w.status.Idle = true
			continue
		}
		w.logger.Info("succeed")
		w.logger.Infof("Stderr: %s", bufErr.String())
		w.status.Stderr = append(w.status.Stderr, bufErr.String())
		w.logger.Debugf("Stdout: %s", bufOut.String())
		w.status.Stdout = append(w.status.Stdout, bufOut.String())
		w.status.Result = true
		w.status.LastFinished = time.Now()
	}
}
