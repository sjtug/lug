package worker

import (
	"github.com/sjtug/lug/config"
	"os/exec"
	"time"
)

type RsyncWorker struct {
	status Status
	cfg    *config.RepoConfig
	idle   bool

	signal chan int
}

func (w *RsyncWorker) GetStatus() Status {
	return w.status
}

// GetConfig is for test.
// TODO: remove this func.
func (w *RsyncWorker) GetConfig() *config.RepoConfig {
	return w.cfg
}

func (w *RsyncWorker) TriggerSync() {
	go func() {
		w.signal <- 1
	}()
}

func (w *RsyncWorker) RunSync() {
	w.status.Idle = true
	for {
		start := <-w.signal
		if start == 1 {
			w.status.Idle = false
			if src, ok := (*w.cfg)["source"]; ok {
				if dst, ok := (*w.cfg)["path"]; ok {
					cmd := exec.Command("rsync", "-aHvh", "--no-o", "--no-g", "--stats",
						"--delete", "--delete-delay", "--safe-links",
						"--timeout=120", "--contimeout=120", src, dst)
					err := cmd.Start()
					if err != nil {
						w.status.Result = false
						w.idle = true
						continue
					}
					err = cmd.Wait()
					if err != nil {
						w.status.Result = false
						w.idle = true
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
