package manager

import (
	"testing"

	"github.com/sjtug/lug/config"
	"github.com/stretchr/testify/assert"
)

func TestManagerStartUp(t *testing.T) {
	manager, err := NewManager(&config.Config{})
	assert.Nil(t, err)
	assert.NotNil(t, manager)
}
