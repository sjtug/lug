package worker

import (
	"testing"

	"github.com/sjtug/lug/config"
	"github.com/stretchr/testify/assert"
	"io"
	"os/exec"
	"time"
)

var rsyncW Worker

func TestNewExternalWorker(t *testing.T) {
	asrt := assert.New(t)
	c := config.RepoConfig{
		"blahblah": "foobar",
		"type":     "external",
	}
	_, err := NewWorker(c)
	// worker with no name is not allowed
	asrt.NotNil(err)

	c["name"] = "test_external"
	w, err := NewWorker(c)
	// config with name and dummy kv pairs should be allowed
	asrt.Nil(err)

	status := w.GetStatus()
	asrt.True(status.Result)
	asrt.False(status.Idle)
	asrt.NotNil(status.Stderr)
	asrt.NotNil(status.Stdout)
}

func TestNewRsyncWorker(t *testing.T) {
	asrt := assert.New(t)

	var c config.RepoConfig = make(map[string]string)
	c["type"] = "rsync"
	var err error
	rsyncW, err = NewWorker(c)

	asrt.Nil(rsyncW)
	asrt.NotNil(err)

	c["name"] = "putty"
	c["source"] = "source: rsync://rsync.chiark.greenend.org.uk/ftp/users/sgtatham/putty-website-mirror/"
	c["path"] = "/tmp/putty"
	c["interval"] = "6"
	c["rlimit_mem"] = "10M"
	c["exclude_hidden"] = "true"
	rsyncW, _ = NewWorker(c)

	asrt.True(rsyncW.GetStatus().Result)
	asrt.True(rsyncW.GetStatus().Idle)
	asrt.Equal("rsync", rsyncW.GetConfig()["type"])
	asrt.Equal("putty", rsyncW.GetConfig()["name"])
	asrt.Equal("source: rsync://rsync.chiark.greenend.org.uk/ftp/users/sgtatham/putty-website-mirror/", rsyncW.GetConfig()["source"])
	asrt.Equal("true", rsyncW.GetConfig()["exclude_hidden"])
	asrt.Equal("/tmp/putty", rsyncW.GetConfig()["path"])

}

func TestNewShellScriptWorker(t *testing.T) {
	var c config.RepoConfig = make(map[string]string)
	c["type"] = "shell_script"
	c["name"] = "shell"
	c["script"] = "script"

	asrt := assert.New(t)

	w, _ := NewWorker(c)

	asrt.Equal(true, w.GetStatus().Result)
	asrt.Equal("shell_script", w.GetConfig()["type"])
	asrt.Equal("shell", w.GetConfig()["name"])
	asrt.Equal("script", w.GetConfig()["script"])

}

type limitReader struct {
	cnt   int
	limit int
}

func newLimitReader(limit int) *limitReader {
	return &limitReader{
		cnt:   0,
		limit: limit,
	}
}
func (i *limitReader) Read(p []byte) (int, error) {
	if i.cnt > i.limit {
		return 0, io.EOF
	}
	i.cnt += len(p)
	for i := 0; i < len(p); i++ {
		p[i] = 5 // shouldn't use zero here, because sometimes pages filled with zero are not allocated
	}
	return len(p), nil
}

func TestUtilityRlimit(t *testing.T) {
	asrt := assert.New(t)
	rlimitUtility := newRlimit(rsyncW)

	cmd := exec.Command("rev")
	cmd.Stdin = newLimitReader(20000000) // > 10M = 10485760
	rlimitUtility.preHook()
	err1 := cmd.Start()
	rlimitUtility.postHook()
	var err2 error
	if err1 == nil {
		err2 = cmd.Wait()
	}
	asrt.True(err1 != nil || err2 != nil)
}

func TestShellScriptWorkerArgParse(t *testing.T) {
	c := map[string]string{
		"type":   "shell_script",
		"name":   "shell",
		"script": "wc -l /proc/stat",
	}
	w, err := NewWorker(c)

	asrt := assert.New(t)
	asrt.Nil(err)

	go w.RunSync()
	// workarounds
	time.Sleep(time.Millisecond * 500)
	w.TriggerSync()
	time.Sleep(time.Millisecond * 500)
	for !w.GetStatus().Idle {
		time.Sleep(time.Millisecond * 500)
	}
	asrt.True(w.GetStatus().Idle)
	asrt.True(w.GetStatus().Result)
}
