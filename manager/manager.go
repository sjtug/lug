// Package manager provides definition of manager
package manager

import (
	"fmt"

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
	return &newManager, nil
}

// Run() will block current routine
func (m *Manager) Run() {

}

func Foo() {
	fmt.Println("manager")
}
