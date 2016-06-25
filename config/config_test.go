package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseConfig(t *testing.T) {
	const testStr = `interval: 25
loglevel: 5 # 1 - 5
repos:
- Config1: foo
  Config2: bar`
	c := Config{}
	err := c.Parse([]byte(testStr))

	assert := assert.New(t)
	assert.Nil(err)
	assert.Equal(25, c.Interval)
	assert.Equal(5, int(c.LogLevel))
	assert.Equal(1, len(c.Repos))
	assert.Equal("foo", c.Repos[0]["Config1"])
	assert.Equal("bar", c.Repos[0]["Config2"])

}
