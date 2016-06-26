package manager

import (
	"testing"
	"time"

	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
	"github.com/stretchr/testify/assert"
)

func TestManagerStartUp(t *testing.T) {
	manager, err := NewManager(&config.Config{3, logging.DEBUG,
		[]config.RepoConfig{}})
	assert.Nil(t, err)
	if assert.NotNil(t, manager) {
		logging.MustGetLogger("ManagerTest").Debugf("Manager: %+v", manager)
		go manager.Run()
		ch := time.NewTicker(5 * time.Second).C
		<-ch
		manager.Exit()
	}
}
