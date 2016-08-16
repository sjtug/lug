package worker

import (
	"errors"
	"os/exec"
	"time"

	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
	"github.com/dustin/go-humanize"
	"syscall"
)

// RsyncWorker implements Worker interface
type RsyncWorker struct {
	status Status
	cfg    config.RepoConfig
	signal chan int
	logger *logging.Logger
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
	return &RsyncWorker{*status, cfg, signal, logging.MustGetLogger(cfg["name"])}, nil
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
		w.logger.Infof("Worker %s start rsync command", w.cfg["name"])

		var rlimit_as syscall.Rlimit

		if rlimitMem, ok := w.cfg["rlimit_mem"]; ok {
			if err := syscall.Getrlimit(syscall.RLIMIT_AS, &rlimit_as); err != nil {
				w.logger.Error("Failed to getrlimit:", err)
			}
			if bytes, err := humanize.ParseBytes(rlimitMem); err == nil {
				w.logger.Infof("Setting rlimit_mem... Original %d, set to %d", rlimit_as.Cur, bytes)
				var rlimit_newas syscall.Rlimit
				rlimit_newas = rlimit_as
				rlimit_newas.Cur = bytes
				err := syscall.Setrlimit(syscall.RLIMIT_AS, &rlimit_newas)
				if err != nil {
					w.logger.Error("Failed to setrlimit:", err)
				}
			} else {
				w.logger.Error("Invalid rlimit_mem: must be size:", err)
			}
		}
		err := cmd.Start()
		if _, ok := w.cfg["rlimit_mem"]; ok {
			w.logger.Info("Restoring previous rlimit")
			err := syscall.Setrlimit(syscall.RLIMIT_AS, &rlimit_as)
			if err != nil {
				w.logger.Error("Failed to restore rlimit:", err)
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
		w.status.Result = true
		w.status.LastFinished = time.Now()
	}
}
