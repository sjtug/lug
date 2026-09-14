package worker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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
	telemetry      Telemetry
	stdout         *helper.MaxLengthStringSliceAdaptor
	stderr         *helper.MaxLengthStringSliceAdaptor
	cfg            config.RepoConfig
	name           string
	signal         chan int
	// cancelMu guards cancelRun, which aborts the in-flight sync (if any).
	cancelMu  sync.Mutex
	cancelRun context.CancelFunc
	logger    *log.Entry
	rwmutex   sync.RWMutex
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
		telemetry:      status.Telemetry,
		stdout:         helper.NewMaxLengthSlice(status.Stdout, 20),
		stderr:         helper.NewMaxLengthSlice(status.Stderr, 20),
		cfg:            cfg,
		signal:         signal,
		name:           name,
		logger:         log.WithField("repo", name),
		executor:       exector,
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

// AbortSync cancels the in-flight sync run, if any. The executor kills the
// whole cgroup tree, or the Linux process group when cgroups are unavailable.
func (eiw *executorInvokeWorker) AbortSync() {
	eiw.cancelMu.Lock()
	defer eiw.cancelMu.Unlock()
	if eiw.cancelRun != nil {
		eiw.cancelRun()
	}
}

func (eiw *executorInvokeWorker) GetStatus() Status {
	eiw.rwmutex.RLock()
	defer eiw.rwmutex.RUnlock()
	status := Status{
		Idle:         eiw.idle,
		Result:       eiw.result,
		LastFinished: eiw.lastFinished,
		Telemetry:    eiw.telemetry,
		Stdout:       eiw.stdout.GetAll(),
		Stderr:       eiw.stderr.GetAll(),
	}
	if inspector, ok := eiw.executor.(jobInspector); ok {
		status.ActiveJob = inspector.ActiveJob()
	}
	return status
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
		<-w.signal

		// A new sync run begins: stamp a sync_id so every line of this run
		// can be correlated in the event stream.
		syncID := shortID()
		logger = w.logger.WithField("sync_id", syncID)
		logger.WithField("event", "signal_received").Debug("finished waiting for signal")
		func() {
			w.rwmutex.Lock()
			defer w.rwmutex.Unlock()
			w.idle = false
		}()

		ctx, cancel := context.WithCancel(context.Background())
		w.cancelMu.Lock()
		w.cancelRun = cancel
		w.cancelMu.Unlock()

		retry_limit := w.retry
		var result execResult
		var err error
	retryLoop:
		for retry_cnt := 1; retry_cnt <= retry_limit; retry_cnt++ {
			logger.WithFields(log.Fields{
				"event":   "start_execution",
				"try_cnt": retry_cnt,
			}).Info("sync attempt started")
			logger.WithField("event", "invoke_executor").WithField(
				"try_cnt", retry_cnt).Debugf("Invoke executor for the %v time", retry_cnt)
			result, err = w.executor.RunOnce(ctx, logger)
			if err == nil || ctx.Err() != nil {
				// success, or the whole run was aborted: retrying is pointless
				break
			}
			logger.WithField("event", "invoke_executor_fail").WithField(
				"try_cnt", retry_cnt).Infof(
				"Failed on the %v-th executor. Error: %v", retry_cnt, err.Error())
			logger.Debug("Stderr: ", result.Stderr)
			if retry_cnt == retry_limit {
				break
			}

			logger.WithFields(log.Fields{
				"event":        "retry_wait",
				"try_cnt":      retry_cnt,
				"duration_sec": w.retry_interval.Seconds(),
			}).Info("waiting before retry")
			timer := time.NewTimer(w.retry_interval)
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				err = fmt.Errorf("execution canceled during retry wait: %w", ctx.Err())
				break retryLoop
			}
		}

		w.cancelMu.Lock()
		w.cancelRun = nil
		w.cancelMu.Unlock()
		cancel()

		if err != nil {
			logger.WithField("event", "execution_fail").Error(err.Error())
			exporter.GetInstance().SyncFail(w.name)
			func() {
				w.rwmutex.Lock()
				defer w.rwmutex.Unlock()
				w.result = false
				w.telemetry.Observe(result.Duration, result.PeakMemBytes, result.OOMKilled)
				w.stdout.Put(result.Stdout)
				w.stderr.Put(result.Stderr)
				logger.Infof("Stderr: %s", result.Stderr)
				logger.Debugf("Stdout: %s", result.Stdout)
				w.idle = true
			}()
			w.exportTelemetry()
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
			w.telemetry.Observe(result.Duration, result.PeakMemBytes, result.OOMKilled)
		}()
		w.exportTelemetry()
		logger.WithField("event", "execution_ended").Info("sync ended")
	}
}

func (w *executorInvokeWorker) exportTelemetry() {
	t := w.GetStatus().Telemetry
	exporter.GetInstance().SyncTelemetry(w.name, exporter.TelemetrySample{
		LastDuration:     t.LastDuration,
		DurationEWMA:     t.DurationEWMA,
		LastPeakMemBytes: t.LastPeakMem,
		PeakMemEWMABytes: t.PeakMemEWMA,
		OOMKills:         t.OOMKills,
	})
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
