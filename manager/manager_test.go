package manager

import (
	"testing"

	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
	"github.com/stretchr/testify/assert"
)

func TestManagerStartUp(t *testing.T) {
	manager := NewManager(&config.Config{}, logging.MustGetLogger("manager"))
	assert.NotNil(t, manager)
}
