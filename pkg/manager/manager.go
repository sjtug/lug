// Package manager provides definition of manager
package manager

import (
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"io"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sjtug/lug/pkg/config"
	"github.com/sjtug/lug/pkg/worker"
)

const (
	// SigStart is a signal sent to control channel of manager which starts sync of all container
	SigStart = iota
	// SigStop is a signal sent to control channel of manager which stops sync of all container
	SigStop
	// SigExit is a signal sent to control channel of manager which exits manager run loop
	SigExit
	// ExitFinish is a signal from finish channel of manager indicating exit finished
	ExitFinish
	// StopFinish is a signal from finish channel of manager indicating stopping job finished
	StopFinish
	// StartFinish is a signal from finish channel of manager indicating starting job finished
	StartFinish
)

// Manager holds worker instances
type Manager struct {
	config                *config.Config
	workers               []worker.Worker
	workersLastInvokeTime map[string]time.Time
	controlChan           chan int
	finishChan            chan int
	running               bool
	// storing index of worker to launch
	pendingQueue []int
	logger       *logrus.Entry
}

// Status holds the status of a manager and its workers
// WorkerStatus: key = worker's name, value = worker's status
type Status struct {
	Running      bool
	WorkerStatus map[string]worker.Status
}

type WorkerCheckPoint struct {
	LastInvokeTime time.Time  `json:"last_invoke_time"`
	LastFinished   *time.Time `json:"last_finished,omitempty"`
	Result         *bool      `json:"result,omitempty"`
}

type CheckPoint struct {
	WorkerInfo map[string]WorkerCheckPoint `json:"worker_info"`
}

// fromCheckpoint laods last invoke time from json
func fromCheckpoint(checkpointFile string) (*CheckPoint, error) {
	jsonFile, err := os.Open(checkpointFile)
	if err != nil {
		return nil, err
	}
	defer func() { _ = jsonFile.Close() }()

	data, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	var checkpoint CheckPoint

	err = json.Unmarshal(data, &checkpoint)
	if err != nil {
		return nil, err
	}

	return &checkpoint, nil
}

func workerFromCheckpoint(repoConfig config.RepoConfig, checkpoint *CheckPoint, name string, lastInvokeTime time.Time) (worker.Worker, error) {
	if checkpoint == nil {
		return worker.NewWorker(repoConfig, lastInvokeTime, true)
	}
	info, ok := checkpoint.WorkerInfo[name]
	if !ok {
		return worker.NewWorker(repoConfig, lastInvokeTime, true)
	}

	result := true
	if info.Result != nil {
		result = *info.Result
	}
	lastFinished := lastInvokeTime
	if info.LastFinished != nil {
		lastFinished = *info.LastFinished
	}
	return worker.NewWorker(repoConfig, lastFinished, result)
}

// NewManager creates a new manager with attached workers from config
func NewManager(config *config.Config) (*Manager, error) {
	logger := logrus.WithField("manager", "")
	checkpoint, err := fromCheckpoint(config.Checkpoint)
	workersLastInvokeTime := make(map[string]time.Time)
	if err != nil {
		logger.Info("failed to parse checkpoint file")
	} else {
		for name, info := range checkpoint.WorkerInfo {
			workersLastInvokeTime[name] = info.LastInvokeTime
		}
	}
	newManager := Manager{
		config:                config,
		workers:               []worker.Worker{},
		workersLastInvokeTime: workersLastInvokeTime,
		controlChan:           make(chan int),
		finishChan:            make(chan int),
		running:               true,
		logger:                logger,
	}
	for _, repoConfig := range config.Repos {
		if disabled, ok := repoConfig["disabled"].(bool); ok && disabled {
			continue
		}
		name, _ := repoConfig["name"].(string)
		if _, ok := newManager.workersLastInvokeTime[name]; !ok {
			newManager.workersLastInvokeTime[name] = time.Now().AddDate(-1, 0, 0)
		}
		w, err := workerFromCheckpoint(repoConfig, checkpoint, name, newManager.workersLastInvokeTime[name])
		if err != nil {
			return nil, err
		}
		newManager.workers = append(newManager.workers, w)
	}
	return &newManager, nil
}

func (m *Manager) checkpoint() error {
	ckptObj := &CheckPoint{WorkerInfo: make(map[string]WorkerCheckPoint)}
	for _, w := range m.workers {
		name := w.GetConfig()["name"].(string)
		status := w.GetStatus()
		lastInvokeTime, ok := m.workersLastInvokeTime[name]
		if !ok {
			lastInvokeTime = time.Now().AddDate(-1, 0, 0)
		}

		ckptObj.WorkerInfo[name] = WorkerCheckPoint{
			LastInvokeTime: lastInvokeTime,
			Result:         &status.Result,
			LastFinished:   &status.LastFinished,
		}
	}

	file, err := json.MarshalIndent(ckptObj, "", "  ")
	if err != nil {
		return err
	}
	ckpt := fmt.Sprintf("%s.tmp", m.config.Checkpoint)
	err = os.WriteFile(ckpt, file, 0644)
	if err != nil {
		return err
	}
	err = os.Rename(ckpt, m.config.Checkpoint)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) isAlreadyInPendingQueue(workerIdx int) bool {
	for _, wk := range m.pendingQueue {
		if wk == workerIdx {
			return true
		}
	}
	return false
}

func (m *Manager) launchWorkerFromPendingQueue(max_allowed int) {
	if max_allowed <= 0 {
		return
	}
	var new_idx int
	if max_allowed > len(m.pendingQueue) {
		new_idx = len(m.pendingQueue)
	} else {
		new_idx = max_allowed
	}
	m.logger.WithFields(logrus.Fields{
		"event":         "launch_worker_from_pending_queue",
		"max_allowed":   max_allowed,
		"new_idx":       new_idx,
		"pending_queue": spew.Sprint(m.pendingQueue),
	}).Debug("launch worker from pending queue")
	to_launch := m.pendingQueue[:new_idx]
	m.pendingQueue = m.pendingQueue[new_idx:]

	for _, w_idx := range to_launch {
		w := m.workers[w_idx]
		wConfig := w.GetConfig()
		m.logger.WithFields(logrus.Fields{
			"event":              "trigger_sync",
			"target_worker_name": wConfig["name"],
		}).Infof("trigger sync for worker %s from pendingQueue", wConfig["name"])
		m.workersLastInvokeTime[wConfig["name"].(string)] = time.Now()
		w.TriggerSync()
	}
}

// Run will block current routine
func (m *Manager) Run() {
	m.logger.Debugf("%p", m)
	c := time.Tick(time.Duration(m.config.Interval) * time.Second)
	for _, w := range m.workers {
		m.logger.WithFields(logrus.Fields{
			"event":         "call_runsync",
			"target_worker": w.GetConfig()["name"],
		}).Debugf("Calling RunSync() to w %s", w.GetConfig()["name"])
		go w.RunSync()
	}
	err := m.checkpoint()
	if err != nil {
		m.logger.WithFields(logrus.Fields{
			"event": "checkpoint_failed",
			"error": err,
		}).Error("Failed to checkpoint")
	}
	for {
		// wait until config.Interval seconds has elapsed
		select {
		case <-c:
			if m.running {
				m.logger.WithField("event", "poll_start").Info("Start polling workers")
				running_worker_cnt := 0
				shouldCheckpoint := false
				for i, w := range m.workers {
					wStatus := w.GetStatus()
					m.logger.WithFields(logrus.Fields{
						"event":                       "worker_status",
						"target_worker_idx":           i,
						"target_worker_idle":          wStatus.Idle,
						"target_worker_result":        wStatus.Result,
						"target_worker_last_finished": wStatus.LastFinished,
					})
					if !wStatus.Idle {
						running_worker_cnt++
						continue
					}
					wConfig := w.GetConfig()
					elapsed := time.Since(m.workersLastInvokeTime[wConfig["name"].(string)])
					sec2sync, ok := wConfig["interval"].(int)
					if !ok {
						sec2sync = 31536000 // if "interval" is not specified, then worker will launch once a year
					}
					if !m.isAlreadyInPendingQueue(i) && elapsed > time.Duration(sec2sync)*time.Second {
						m.logger.WithFields(logrus.Fields{
							"event":                  "trigger_pending",
							"target_worker_name":     wConfig["name"],
							"target_worker_interval": sec2sync,
						}).Infof("Interval of w %s (%d sec) elapsed, send it to pendingQueue", wConfig["name"], sec2sync)
						m.pendingQueue = append(m.pendingQueue, i)
						shouldCheckpoint = true
					}
				}
				m.launchWorkerFromPendingQueue(m.config.ConcurrentLimit - running_worker_cnt)
				m.logger.WithField("event", "poll_end").Info("Stop polling workers")

				// Here we do not checkpoint very concisely (e.g. every time after a successful sync).
				// We just want to minimize re-sync after restarting lug.
				if shouldCheckpoint {
					err := m.checkpoint()
					if err != nil {
						m.logger.WithFields(logrus.Fields{
							"event": "checkpoint_failed",
							"error": err,
						}).Error("Failed to checkpoint")
					}
				}
			}
		case sig, ok := <-m.controlChan:
			if ok {
				switch sig {
				default:
					m.logger.WithField("event", "unrecognized_control_signal").
						Warningf("Unrecognized Control Signal: %d", sig)
				case SigStart:
					m.running = true
					m.finishChan <- StartFinish
				case SigStop:
					m.running = false
					m.finishChan <- StopFinish
				case SigExit:
					m.logger.WithField("event", "exit_control_signal").Info("Exiting...")
					goto END_OF_FINISH
				}
			} else {
				m.logger.WithField("event", "control_channel_closed").Fatal("Control channel is closed!")
			}
		}
	}
END_OF_FINISH:
	m.logger.WithField("event", "send_exit_finish").Debug("Sending ExitFinish...")
	m.finishChan <- ExitFinish
	m.logger.WithField("event", "senf_exit_finish_end").Debug("Finished sending ExitFinish...")
}

// Run some specific worker once
func (m *Manager) RunSpecificWorker(worker string) {
	m.logger.Debugf("%p", m)

	for _, w := range m.workers {
		if worker == w.GetConfig()["name"] {
			m.logger.WithFields(logrus.Fields{
				"event":         "call_runsync",
				"target_worker": w.GetConfig()["name"],
			}).Debugf("Calling RunSync() to w %s", w.GetConfig()["name"])
			go w.RunSync()

			w.TriggerSync()
			for !w.GetStatus().Idle {
			}

			return
		}
	}

	m.logger.WithField("event", "unexpected_worker").Fatalf("%s worker not found!", worker)

}

func (m *Manager) expectChanVal(ch chan int, expected int) {
	exitMsg, ok := <-ch
	if ok {
		switch exitMsg {
		default:
			m.logger.WithFields(logrus.Fields{
				"event":        "unexpected_control_message",
				"expected_msg": expected,
				"received_msg": exitMsg,
			}).Fatalf("Unrecognized Msg: %d, expected %d", exitMsg, expected)
		case expected:
			m.logger.WithFields(logrus.Fields{
				"event":        "finish_receive_control_message",
				"expected_msg": expected,
				"received_msg": expected,
			}).Infof("Finished reading %d", expected)
		}
	} else {
		m.logger.WithField("event", "control_channel_closed").Fatalf("Channel has been closed, expected %d", expected)
	}
}

// Start polling, block until finish(may take several seconds)
func (m *Manager) Start() {
	m.controlChan <- SigStart
	m.expectChanVal(m.finishChan, StartFinish)
}

// Stop polling, block until finish(may take several seconds)
func (m *Manager) Stop() {
	m.controlChan <- SigStop
	m.expectChanVal(m.finishChan, StopFinish)
}

// Exit polling, block until finish(may take several seconds)
func (m *Manager) Exit() {
	m.Stop()
	m.controlChan <- SigExit
	m.expectChanVal(m.finishChan, ExitFinish)
}

// GetStatus gets status of Manager
func (m *Manager) GetStatus() *Status {
	status := Status{
		Running:      m.running,
		WorkerStatus: make(map[string]worker.Status),
	}
	for _, w := range m.workers {
		wConfig := w.GetConfig()
		wStatus := w.GetStatus()
		if hidden, ok := wConfig["hidden"].(bool); !(ok && hidden) {
			status.WorkerStatus[wConfig["name"].(string)] = wStatus
		}
	}
	return &status
}
