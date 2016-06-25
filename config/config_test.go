package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseConfig(t *testing.T) {
	const testStr = `interval: 25
loglevel: 5 # 1 - 5
repos:
- type: rsync
  source: vim.org
  interval: 600 # Interval between sync
  path: /mnt/vim
`
	c := Config{}
	err := c.Parse([]byte(testStr))

	assert := assert.New(t)
	assert.Nil(err)
	assert.Equal(25, c.Interval)
	assert.Equal(5, int(c.LogLevel))
	assert.Equal(1, len(c.Repos))
	assert.Equal("rsync", c.Repos[0]["type"])
	assert.Equal("vim.org", c.Repos[0]["source"])
	assert.Equal("600", c.Repos[0]["interval"])
	assert.Equal("/mnt/vim", c.Repos[0]["path"])

}

func TestWrongManagerConfig(t *testing.T) {
	var testStr string = `interval: -1
loglevel: 5 # 1 - 5
repos:
- type: rsync
  source: vim.org
  interval: 600 # Interval between sync
  path: /mnt/vim
`
	c := Config{}
	var err error
	err = c.Parse([]byte(testStr))

	assert := assert.New(t)
	assert.Equal("Interval can't be negative", err.Error())

	testStr = `interval: 25
loglevel: 6
repos:
- type: rsync
  source: vim.org
  interval: 600 # Interval between sync
  path: /mnt/vim
`
	c = Config{}
	err = c.Parse([]byte(testStr))

	assert.Equal("loglevel must be 0-5", err.Error())
}
