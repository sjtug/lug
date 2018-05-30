package worker

import (
	"errors"
	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"github.com/sjtug/lug/pkg/config"
	"github.com/sjtug/lug/pkg/exporter"
	"github.com/sjtug/lug/pkg/helper"
	"sync"
	"time"
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
}

// creates a new executorInvokeWorker, which encapsules an executor
// into a Worker and provides functinalities like auto-retry and state
// management
func NewExecutorInvokeWorker(exector executor, status Status,
	cfg config.RepoConfig,
	signal chan int) (*executorInvokeWorker, error) {
	name, ok := cfg["name"].(string)
	if !ok {
		return nil, errors.New("No name in config")
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
		logger:         log.WithField("worker", name),
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
		w.logger.WithField("event", "start_wait_signal").Debug("start waiting for signal")
		func() {
			w.rwmutex.Lock()
			defer w.rwmutex.Unlock()
			w.idle = true
		}()
		<-w.signal
		w.logger.WithField("event", "signal_received").Debug("finished waiting for signal")
		func() {
			w.rwmutex.Lock()
			defer w.rwmutex.Unlock()
			w.idle = false
		}()
		w.logger.WithField("event", "start_execution").Info("start execution")
		retry_limit := w.retry
		var result execResult
		var err error
		for retry_cnt := 1; retry_cnt <= retry_limit; retry_cnt++ {
			w.logger.WithField("event", "invoke_executor").WithField(
				"try_cnt", retry_cnt).Debugf("Invoke executor for the %v time", retry_cnt)
			utilities := []utility{newRlimit(w)}
			result, err = w.executor.RunOnce(w.logger, utilities)
			if err == nil {
				break
			}
			w.logger.WithField("event", "invoke_executor_fail").WithField(
				"try_cnt", retry_cnt).Infof(
				"Failed on the %v-th executor. Error: %v", retry_cnt, err.Error())
			w.logger.Debug("Stderr: ", result.Stderr)
			time.Sleep(w.retry_interval)
		}
		if err != nil {
			w.logger.WithField("event", "execution_fail").Error(err.Error())
			exporter.GetInstance().SyncFail(w.name)
			func() {
				w.rwmutex.Lock()
				defer w.rwmutex.Unlock()
				w.result = false
				w.stdout.Put(result.Stdout)
				w.stderr.Put(result.Stderr)
				w.logger.Infof("Stderr: %s", result.Stderr)
				w.logger.Debugf("Stdout: %s", result.Stdout)
				w.idle = true
			}()
			continue
		}

		exporter.GetInstance().SyncSuccess(w.name)
		w.logger.WithField("event", "execution_succeed").Info("succeed")
		w.logger.Infof("Stderr: %s", result.Stderr)
		func() {
			w.rwmutex.Lock()
			defer w.rwmutex.Unlock()
			w.stderr.Put(result.Stderr)
			w.logger.Debugf("Stdout: %s", result.Stdout)
			w.stdout.Put(result.Stdout)
			w.result = true
			w.lastFinished = time.Now()
		}()
	}
}
