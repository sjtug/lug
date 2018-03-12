// Package manager provides definition of manager
package manager

import (
	"github.com/sirupsen/logrus"
	"github.com/sjtug/lug/config"
	"github.com/sjtug/lug/worker"
	"strconv"
	"time"
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
	workersLastInvokeTime []time.Time
	controlChan           chan int
	finishChan            chan int
	running               bool
	logger                *logrus.Entry
}

// Status holds the status of a manager and its workers
// WorkerStatus: key = worker's name, value = worker's status
type Status struct {
	Running      bool
	WorkerStatus map[string]worker.Status
}

// NewManager creates a new manager with attached workers from config
func NewManager(config *config.Config) (*Manager, error) {
	newManager := Manager{
		config:                config,
		workers:               []worker.Worker{},
		workersLastInvokeTime: []time.Time{},
		controlChan:           make(chan int),
		finishChan:            make(chan int),
		running:               true,
		logger:                logrus.WithField("manager", ""),
	}
	for _, repoConfig := range config.Repos {
		w, err := worker.NewWorker(repoConfig)
		if err != nil {
			return nil, err
		}
		newManager.workers = append(newManager.workers, w)
		newManager.workersLastInvokeTime = append(newManager.workersLastInvokeTime, time.Now().AddDate(-1, 0, 0))
	}
	return &newManager, nil
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
	for {
		// wait until config.Interval seconds has elapsed
		select {
		case <-c:
			if m.running {
				m.logger.WithField("event", "poll_start").Info("Start polling workers")
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
						continue
					}
					wConfig := w.GetConfig()
					elapsed := time.Since(m.workersLastInvokeTime[i])
					sec2sync, _ := strconv.Atoi(wConfig["interval"])
					if elapsed > time.Duration(sec2sync)*time.Second {
						m.logger.WithFields(logrus.Fields{
							"event":                  "trigger_sync",
							"target_worker_name":     wConfig["name"],
							"target_worker_interval": sec2sync,
						}).Infof("Interval of w %s (%d sec) elapsed, trigger it to sync", wConfig["name"], sec2sync)
						m.workersLastInvokeTime[i] = time.Now()
						w.TriggerSync()
					}
				}
				m.logger.WithField("event", "poll_end").Info("Stop polling workers")
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
		status.WorkerStatus[wConfig["name"]] = wStatus
	}
	return &status
}
