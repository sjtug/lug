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
)

// Manager holds worker instances
type Manager struct {
	config      *config.Config
	logger      *logging.Logger
	workers     []worker.Worker
	controlChan chan int
	running     bool
}

// ManagerStatus holds the status of a manager and its workers
// WorkerStatus: key = worker's name, value = worker's status
type Status struct {
	Running      bool
	WorkerStatus map[string]worker.Status
}

func NewManager(config *config.Config) (*Manager, error) {
	newManager := Manager{config, logging.MustGetLogger("manager"),
		[]worker.Worker{}, make(chan int), true}
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
				case SigStop:
					m.running = false
				case SigExit:
					break
				}
			} else {
				m.logger.Critical("Control channel is closed!")
			}
		}
	}
}

func (m *Manager) Start() {
	m.controlChan <- SigStart
}

func (m *Manager) Stop() {
	m.controlChan <- SigStop
}

func (m *Manager) Exit() {
	m.controlChan <- SigExit
}

func (m *Manager) GetStatus() Status {
	return Status{true, map[string]worker.Status{}}
}

func Foo() {
	fmt.Println("manager")
}
