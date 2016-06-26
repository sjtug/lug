// Package manager provides definition of manager
package manager

import (
	"fmt"
	"strconv"
	"time"

	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
	"github.com/sjtug/lug/worker"
)

const (
	SigStart = iota
	SigStop
	SigExit
	ExitFinish
	StopFinish
	StartFinish
)

// Manager holds worker instances
type Manager struct {
	config      *config.Config
	logger      *logging.Logger
	workers     []worker.Worker
	controlChan chan int
	finishChan  chan int
	running     bool
}

// Status holds the status of a manager and its workers
// WorkerStatus: key = worker's name, value = worker's status
type Status struct {
	Running      bool
	WorkerStatus map[string]worker.Status
}

func NewManager(config *config.Config) (*Manager, error) {
	newManager := Manager{config, logging.MustGetLogger("manager"),
		[]worker.Worker{}, make(chan int), make(chan int), true}
	for _, repoConfig := range config.Repos {
		w, err := worker.NewWorker(repoConfig)
		if err != nil {
			return nil, err
		}
		newManager.workers = append(newManager.workers, w)
	}
	return &newManager, nil
}

// Run() will block current routine
func (m *Manager) Run() {
	m.logger.Debugf("%p", m)
	c := time.Tick(time.Duration(m.config.Interval) * time.Second)
	for _, worker := range m.workers {
		m.logger.Debugf("Calling RunSync() to worker %s", worker.GetConfig()["name"])
		go worker.RunSync()
	}
	for {
		// wait until config.Interval seconds has elapsed
		select {
		case <-c:
			if m.running {
				m.logger.Info("Start polling workers")
				for i, worker := range m.workers {
					wStatus := worker.GetStatus()
					m.logger.Debugf("worker %d: %+v", i, wStatus)
					if !wStatus.Idle {
						continue
					}
					wConfig := worker.GetConfig()
					elapsed := time.Since(wStatus.LastFinished)
					sec2sync, _ := strconv.Atoi(wConfig["interval"])
					if elapsed > time.Duration(sec2sync)*time.Second {
						m.logger.Noticef("Interval of worker %s (%d sec) elapsed, trigger it to sync", wConfig["name"], sec2sync)
						worker.TriggerSync()
					}
				}
				m.logger.Info("Stop polling workers")
			}
		case sig, ok := (<-m.controlChan):
			if ok {
				switch sig {
				default:
					m.logger.Warningf("Unrecognized Control Signal: %d", sig)
				case SigStart:
					m.running = true
					m.finishChan <- StartFinish
				case SigStop:
					m.running = false
					m.finishChan <- StopFinish
				case SigExit:
					m.logger.Info("Exiting...")
					goto END_OF_FINISH
				}
			} else {
				m.logger.Critical("Control channel is closed!")
			}
		}
	}
END_OF_FINISH:
	m.logger.Debug("Sending ExitFinish...")
	m.finishChan <- ExitFinish
	m.logger.Debug("Finished sending ExitFinish...")
}

func (m *Manager) expectChanVal(ch chan int, expected int) {
	exitMsg, ok := (<-ch)
	if ok {
		switch exitMsg {
		default:
			m.logger.Criticalf("Unrecognized Msg: %d, expected %d", exitMsg, expected)
		case expected:
			m.logger.Infof("Finished reading %d", expected)
		}
	} else {
		m.logger.Criticalf("Channel has been closed, expected %d", expected)
	}
}

func (m *Manager) Start() {
	m.controlChan <- SigStart
	m.expectChanVal(m.finishChan, StartFinish)
}

func (m *Manager) Stop() {
	m.controlChan <- SigStop
	m.expectChanVal(m.finishChan, StopFinish)
}

func (m *Manager) Exit() {
	m.Stop()
	m.controlChan <- SigExit
	m.expectChanVal(m.finishChan, ExitFinish)
}

// get Status from a Manager
func (m *Manager) GetStatus() *Status {
	status := Status{m.running, make(map[string]worker.Status)}
	for _, worker := range m.workers {
		wConfig := worker.GetConfig()
		wStatus := worker.GetStatus()
		status.WorkerStatus[wConfig["name"]] = wStatus
	}
	return &status
}

func Foo() {
	fmt.Println("manager")
}
