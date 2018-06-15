package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseConfig(t *testing.T) {
	const testStr = `interval: 25
loglevel: 5 # 1 - 5
concurrent_limit: 6
repos:
- type: shell_script
  script: rsync -av rsync://rsync.chiark.greenend.org.uk/ftp/users/sgtatham/putty-website-mirror/ /tmp/putty
  name: putty
  path: /mnt/putty
`
	c := Config{}
	err := c.Parse(strings.NewReader(testStr))

	asrt := assert.New(t)
	asrt.Nil(err)
	asrt.Equal(25, c.Interval)
	asrt.Equal(5, int(c.LogLevel))
	asrt.Equal(1, len(c.Repos))
	asrt.Equal(6, c.ConcurrentLimit)
	asrt.EqualValues("shell_script", c.Repos[0]["type"])
	asrt.EqualValues("rsync -av rsync://rsync.chiark.greenend.org.uk/ftp/users/sgtatham/putty-website-mirror/ /tmp/putty", c.Repos[0]["script"])
	asrt.EqualValues("/mnt/putty", c.Repos[0]["path"])
}

func TestParseRepo(t *testing.T) {
	const testStr = `interval: 25
loglevel: 5
repos:
- type: shell_script
  number: 2
  float: 2.5
  nullValue: null
  str: string
  boolean: false
`
	c := Config{}
	err := c.Parse(strings.NewReader(testStr))

	asrt := assert.New(t)
	asrt.Nil(err)
	asrt.Equal(1, len(c.Repos))
	asrt.EqualValues(2, c.Repos[0]["number"])
	asrt.EqualValues(2.5, c.Repos[0]["float"])
	asrt.EqualValues(nil, c.Repos[0]["nullValue"])
	asrt.EqualValues("string", c.Repos[0]["str"])
	asrt.EqualValues(false, c.Repos[0]["boolean"])
}

func TestParseYamlAnchor(t *testing.T) {
	const testStr = `interval: 25
loglevel: 5
dummy:
  common1: &common1
    interval: 3600
  common2: &common2
    retry: 5
repos:
  - type: shell_script
    <<: *common1
    str: string
    retry: 3
  - type: shell_script
    <<: *common1
    <<: *common2
`
	c := Config{}
	var err error
	err = c.Parse(strings.NewReader(testStr))
	asrt := assert.New(t)
	asrt.NoError(err)
	asrt.Equal(2, len(c.Repos))
	asrt.EqualValues("shell_script", c.Repos[0]["type"])
	asrt.EqualValues("string", c.Repos[0]["str"])
	asrt.EqualValues(3600, c.Repos[0]["interval"])
	asrt.EqualValues("shell_script", c.Repos[0]["type"])
	asrt.EqualValues(3600, c.Repos[1]["interval"])
	asrt.EqualValues(5, c.Repos[1]["retry"])
}

func TestWrongManagerConfig(t *testing.T) {
	var testStr = `interval: -1
loglevel: 5 # 1 - 5
repos:
- type: shell_script
  script: rsync -av rsync://rsync.chiark.greenend.org.uk/ftp/users/sgtatham/putty-website-mirror/ /tmp/putty
  name: putty
  path: /mnt/putty
`
	c := Config{}
	var err error
	err = c.Parse(strings.NewReader(testStr))

	asrt := assert.New(t)
	asrt.Equal("Interval can't be negative", err.Error())

	testStr = `interval: 25
loglevel: 6
repos:
- type: shell_script
  script: rsync -av rsync://rsync.chiark.greenend.org.uk/ftp/users/sgtatham/putty-website-mirror/ /tmp/putty
  name: putty
  path: /mnt/putty
`
	c = Config{}
	err = c.Parse(strings.NewReader(testStr))

	asrt.Equal("loglevel must be 0-5", err.Error())

	testStr = `interval: 25
loglevel: 4
concurrent_limit: 0
repos:
- type: shell_script
  script: rsync -av rsync://rsync.chiark.greenend.org.uk/ftp/users/sgtatham/putty-website-mirror/ /tmp/putty
  name: putty
  path: /mnt/putty
`
	c = Config{}
	err = c.Parse(strings.NewReader(testStr))

	asrt.Equal("concurrent limit must be positive", err.Error())
}
