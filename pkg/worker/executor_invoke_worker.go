package worker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"github.com/sjtug/lug/pkg/config"
	"github.com/sjtug/lug/pkg/exporter"
	"github.com/sjtug/lug/pkg/helper"
)

type executorInvokeWorker struct {
	executor       executor
	idle           bool
	result         bool
	retry          int
	retry_interval time.Duration
	lastFinished   time.Time
	stdout         *helper.MaxLengthStringSliceAdaptor
	stderr         *helper.MaxLengthStringSliceAdaptor
	cfg            config.RepoConfig
	name           string
	signal         chan int
	logger         *log.Entry
	rwmutex        sync.RWMutex
	// ctx governs graceful shutdown: when cancelled, in-flight children
	// started via exec.CommandContext are killed (12-factor IX) and pending
	// retries abort. Defaults to context.Background() (today's behaviour).
	ctx context.Context
}

// creates a new executorInvokeWorker, which encapsules an executor
// into a Worker and provides functinalities like auto-retry and state
// management
func NewExecutorInvokeWorker(exector executor, status Status,
	cfg config.RepoConfig,
	signal chan int) (*executorInvokeWorker, error) {
	name, ok := cfg["name"].(string)
	if !ok {
		return nil, errors.New("no name in config")
	}
	w := &executorInvokeWorker{
		idle:           status.Idle,
		result:         status.Result,
		retry:          3,
		retry_interval: 3 * time.Second,
		lastFinished:   status.LastFinished,
		stdout:         helper.NewMaxLengthSlice(status.Stdout, 20),
		stderr:         helper.NewMaxLengthSlice(status.Stderr, 20),
		cfg:            cfg,
		signal:         signal,
		name:           name,
		logger:         log.WithField("repo", name),
		executor:       exector,
		ctx:            context.Background(),
	}
	if retry_generic, ok := cfg["retry"]; ok {
		if retry, ok := retry_generic.(int); ok {
			w.retry = retry
		} else {
			return nil, errors.New("retry should be an integer when present")
		}
	}

	if retry_interval_generic, ok := cfg["retry_interval"]; ok {
		if retry_interval, ok := retry_interval_generic.(int); ok {
			w.retry_interval = time.Duration(retry_interval) * time.Second
		} else {
			return nil, errors.New("retry_interval should be an integer when present")
		}
	}
	w.logger.Info(spew.Sprint(w))
	return w, nil
}

func (eiw *executorInvokeWorker) TriggerSync() {
	eiw.signal <- 1
}

// SetContext installs the shutdown context. The manager sets this to a
// context derived from its root (cancelled on SIGTERM/SIGINT) so the
// worker's child processes die with the daemon instead of orphaning.
func (eiw *executorInvokeWorker) SetContext(ctx context.Context) {
	eiw.ctx = ctx
}

func (eiw *executorInvokeWorker) GetStatus() Status {
	eiw.rwmutex.RLock()
	defer eiw.rwmutex.RUnlock()
	return Status{
		Idle:         eiw.idle,
		Result:       eiw.result,
		LastFinished: eiw.lastFinished,
		Stdout:       eiw.stdout.GetAll(),
		Stderr:       eiw.stderr.GetAll(),
	}
}

func (eiw *executorInvokeWorker) GetConfig() config.RepoConfig {
	eiw.rwmutex.RLock()
	defer eiw.rwmutex.RUnlock()
	return eiw.cfg
}

func (w *executorInvokeWorker) RunSync() {
	for {
		// Before the signal there is no sync_id yet; reuse the repo-scoped logger.
		logger := w.logger
		logger.WithField("event", "start_wait_signal").Debug("start waiting for signal")
		func() {
			w.rwmutex.Lock()
			defer w.rwmutex.Unlock()
			w.idle = true
		}()
		// Wait for a trigger OR shutdown — so a cancelled context exits RunSync
		// instead of blocking on the signal channel forever.
		select {
		case <-w.ctx.Done():
			logger.WithField("event", "shutdown").Info("worker shutting down")
			return
		case <-w.signal:
		}

		// A new sync run begins: stamp a sync_id so every line of this run
		// can be correlated in the event stream (see PLAN, Phase 0).
		syncID := shortID()
		logger = w.logger.WithField("sync_id", syncID)
		logger.WithField("event", "signal_received").Debug("finished waiting for signal")
		func() {
			w.rwmutex.Lock()
			defer w.rwmutex.Unlock()
			w.idle = false
		}()
		logger.WithField("event", "start_execution").Info("sync started")
		retry_limit := w.retry
		var result execResult
		var err error
		for retry_cnt := 1; retry_cnt <= retry_limit; retry_cnt++ {
			logger.WithField("event", "invoke_executor").WithField(
				"try_cnt", retry_cnt).Debugf("Invoke executor for the %v time", retry_cnt)
			utilities := []utility{newRlimit(w)}
			result, err = w.executor.RunOnce(w.ctx, logger, utilities)
			if err == nil {
				break
			}
			logger.WithField("event", "invoke_executor_fail").WithField(
				"try_cnt", retry_cnt).Infof(
				"Failed on the %v-th executor. Error: %v", retry_cnt, err.Error())
			logger.Debug("Stderr: ", result.Stderr)
			// Abort retry sleep on shutdown rather than blocking a full interval.
			select {
			case <-time.After(w.retry_interval):
			case <-w.ctx.Done():
				logger.WithField("event", "shutdown_during_retry").Info("worker shutting down")
				return
			}
		}
		if err != nil {
			logger.WithField("event", "execution_fail").Error(err.Error())
			exporter.GetInstance().SyncFail(w.name)
			func() {
				w.rwmutex.Lock()
				defer w.rwmutex.Unlock()
				w.result = false
				w.stdout.Put(result.Stdout)
				w.stderr.Put(result.Stderr)
				logger.Infof("Stderr: %s", result.Stderr)
				logger.Debugf("Stdout: %s", result.Stdout)
				w.idle = true
			}()
			logger.WithField("event", "execution_ended").Info("sync ended")
			continue
		}

		exporter.GetInstance().SyncSuccess(w.name)
		logger.WithField("event", "execution_succeed").Info("sync succeeded")
		logger.Infof("Stderr: %s", result.Stderr)
		func() {
			w.rwmutex.Lock()
			defer w.rwmutex.Unlock()
			w.stderr.Put(result.Stderr)
			logger.Debugf("Stdout: %s", result.Stdout)
			w.stdout.Put(result.Stdout)
			w.result = true
			w.lastFinished = time.Now()
		}()
		logger.WithField("event", "execution_ended").Info("sync ended")
	}
}

// shortID generates a short unique ID without external dependencies.
// It returns 4 bytes of crypto-random hex (8 chars), enough to correlate
// all log lines within a single sync attempt without collision risk.
func shortID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// rand.Read should not fail in practice; fall back to a fixed marker
		// so logging never breaks on ID generation.
		return "00000000"
	}
	return hex.EncodeToString(b)
}
