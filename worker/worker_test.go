package worker

import (
	"testing"

	"github.com/sjtug/lug/config"
	"github.com/stretchr/testify/assert"
)

func TestNewWorker(t *testing.T) {
	const testStr = `interval: 25
loglevel: 5 # 1 - 5
repos:
- Config1: foo
  Config2: bar`
	c := config.Config{}
	err := c.Parse([]byte(testStr))

	assert := assert.New(t)
	assert.Nil(err)

	w := NewWorker(&c, &Log{})

	assert.Equal(true, w.GetStatus().Result)

	assert.Equal(25, w.getConfig().Interval)
	assert.Equal(5, int(w.getConfig().LogLevel))
	assert.Equal(1, len(w.getConfig().Repos))
	assert.Equal("foo", w.getConfig().Repos[0]["Config1"])
	assert.Equal("bar", w.getConfig().Repos[0]["Config2"])

}
