package worker

import (
	"testing"

	"github.com/sjtug/lug/config"
	"github.com/stretchr/testify/assert"
)

func TestNewRsyncWorker(t *testing.T) {
	var c config.RepoConfig = make(map[string]string)
	c["type"] = "rsync"
	c["name"] = "putty"
	c["source"] = "source"
	c["path"] = "path"
	c["b"] = "bbb"

	assert := assert.New(t)

	w, _ := NewWorker(c)

	assert.Equal(true, w.GetStatus().Result)
	assert.Equal("rsync", w.GetConfig()["type"])
	assert.Equal("putty", w.GetConfig()["name"])
	assert.Equal("path", w.GetConfig()["path"])
	assert.Equal("source", w.GetConfig()["source"])
	assert.Equal("bbb", w.GetConfig()["b"])

}

func TestNewShellScriptWorker(t *testing.T) {
	var c config.RepoConfig = make(map[string]string)
	c["type"] = "shell_script"
	c["name"] = "shell"
	c["script"] = "script"

	assert := assert.New(t)

	w, _ := NewWorker(c)

	assert.Equal(true, w.GetStatus().Result)
	assert.Equal("shell_script", w.GetConfig()["type"])
	assert.Equal("shell", w.GetConfig()["name"])
	assert.Equal("script", w.GetConfig()["script"])

}
