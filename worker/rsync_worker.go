package worker

import (
	"errors"
	"os/exec"
	"time"

	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
	"bytes"
)

// RsyncWorker implements Worker interface
type RsyncWorker struct {
	status    Status
	cfg       config.RepoConfig
	signal    chan int
	logger    *logging.Logger
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
		logger:    logging.MustGetLogger(cfg["name"]),
		utilities: []utility{},
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
		w.logger.Debugf("Worker %s start waiting for signal", w.cfg["name"])
		w.status.Idle = true
		<-w.signal
		w.status.Idle = false
		w.logger.Debugf("Worker %s finished waiting for signal", w.cfg["name"])
		src, _ := w.cfg["source"]
		dst, _ := w.cfg["path"]
		cmd := exec.Command("rsync", "-aHvh", "--no-o", "--no-g", "--stats",
			"--delete", "--delete-delay", "--safe-links",
			"--timeout=120", "--contimeout=120", src, dst)
		var bufErr, bufOut bytes.Buffer
		cmd.Stdout = &bufOut
		cmd.Stderr = &bufErr
		w.logger.Infof("Worker %s start rsync command", w.cfg["name"])

		for _, utility := range w.utilities {
			w.logger.Debug("Executing prehook of ", utility)
			if err := utility.preHook(); err != nil {
				w.logger.Error("Failed to execute preHook:", err)
			}
		}

		err := cmd.Start()

		for _,utility := range w.utilities {
			w.logger.Debug("Executing postHook of ", utility)
			if err := utility.postHook(); err != nil {
				w.logger.Error("Failed to execute postHook:", err)
			}
		}

		if err != nil {
			w.logger.Errorf("Worker %s rsync cannot start", w.cfg["name"])
			w.status.Result = false
			w.status.Idle = true
			continue
		}
		err = cmd.Wait()
		if err != nil {
			w.logger.Errorf("Worker %s rsync failed", w.cfg["name"])
			w.status.Result = false
			w.status.Idle = true
			continue
		}
		w.logger.Infof("Worker %s succeed", w.cfg["name"])
		w.logger.Infof("Stderr of worker %s: %s", w.cfg["name"], bufErr.String())
		w.logger.Debugf("Stdout of worker %s: %s", w.cfg["name"], bufOut.String())
		w.status.Result = true
		w.status.LastFinished = time.Now()
	}
}
