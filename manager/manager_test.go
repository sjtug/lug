package manager

import (
	"fmt"
	"testing"
	"time"

	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
	"github.com/stretchr/testify/assert"
)

func TestManagerStartUp(t *testing.T) {
	manager, err := NewManager(&config.Config{
		Interval: 3,
		LogLevel: logging.DEBUG,
		Repos:    []config.RepoConfig{},
	})
	assert.Nil(t, err)
	if assert.NotNil(t, manager) {
		logging.MustGetLogger("ManagerTest").Debugf("Manager: %+v", manager)
		go manager.Run()
		ch := time.NewTicker(5 * time.Second).C
		<-ch
		status := manager.GetStatus()
		assert.True(t, status.Running)
		fmt.Printf("Manager status before Stop():\n%v\n", status)
		manager.Stop()
		manager.Exit()
		// Because Stop() and Exit() are currently async, we should wait first.
		ch = time.NewTicker(1 * time.Second).C
		<-ch
		status = manager.GetStatus()
		assert.False(t, status.Running)
	}
}
