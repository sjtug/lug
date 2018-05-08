package manager

import (
	"fmt"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/sjtug/lug/pkg/config"
)

func TestManagerStartUp(t *testing.T) {
	manager, err := NewManager(&config.Config{
		Interval: 3,
		Repos:    []config.RepoConfig{},
	})
	assert.Nil(t, err)
	if assert.NotNil(t, manager) {
		log.Debugf("Manager: %+v", manager)
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
