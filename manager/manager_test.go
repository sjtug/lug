package manager

import (
	"testing"
	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
	"github.com/stretchr/testify/assert"
)

func TestManagerStartUp(t *testing.T) {
	manager, err := NewManager(&config.Config{3, logging.INFO,
		[]config.RepoConfig{
			config.RepoConfig {"type": "rsync"}}})
	assert.Nil(t, err)
	assert.NotNil(t, manager)
	manager.Run()
}
