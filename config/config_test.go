package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"strings"
)

func TestParseConfig(t *testing.T) {
	const testStr = `interval: 25
loglevel: 5 # 1 - 5
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
	asrt.Equal("shell_script", c.Repos[0]["type"])
	asrt.Equal("rsync -av rsync://rsync.chiark.greenend.org.uk/ftp/users/sgtatham/putty-website-mirror/ /tmp/putty", c.Repos[0]["script"])
	asrt.Equal("/mnt/putty", c.Repos[0]["path"])

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
}
