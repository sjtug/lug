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

// Manager holds worker instances
type Manager struct {
	config  *config.Config
	logger  *logging.Logger
	workers []worker.Worker
}

func NewManager(config *config.Config) (*Manager, error) {
	newManager := Manager{config, logging.MustGetLogger("manager"), []worker.Worker{}}
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
		<-c
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
			if elapsed > time.Duration(sec2sync) * time.Second {
				m.logger.Noticef("Interval of worker %s (%d sec) elapsed, trigger it to sync", wConfig["name"], sec2sync)
				worker.TriggerSync()
				m.logger.Noticef("Finished triggering worker %s", wConfig["name"])
			}
		}
		m.logger.Info("Stop polling workers")
	}
}

func Foo() {
	fmt.Println("manager")
}
