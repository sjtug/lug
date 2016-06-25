package manager

import (
	"testing"

	"github.com/sjtug/lug/config"
	"github.com/stretchr/testify/assert"
)

func TestManagerStartUp(t *testing.T) {
	manager := NewManager(&config.Config{})
	assert.NotNil(t, manager)
}
