package worker

import (
	"testing"

	"github.com/sjtug/lug/config"
	"github.com/stretchr/testify/assert"
)

func TestNewRsyncWorker(t *testing.T) {
	assert := assert.New(t)

	var c config.RepoConfig = make(map[string]string)
	c["type"] = "rsync"
	w, err := NewWorker(c)

	assert.Nil(w)
	assert.NotNil(err)

	c["name"] = "putty"
	c["source"] = "source: rsync://rsync.chiark.greenend.org.uk/ftp/users/sgtatham/putty-website-mirror/"
	c["path"] = "/tmp/putty"
	c["interval"] = "6"
	w, _ = NewWorker(c)

	assert.True(w.GetStatus().Result)
	assert.True(w.GetStatus().Idle)
	assert.Equal("rsync", w.GetConfig()["type"])
	assert.Equal("putty", w.GetConfig()["name"])
	assert.Equal("source: rsync://rsync.chiark.greenend.org.uk/ftp/users/sgtatham/putty-website-mirror/", w.GetConfig()["source"])
	assert.Equal("/tmp/putty", w.GetConfig()["path"])

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
