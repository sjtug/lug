package worker

import (
	"testing"

	"github.com/sjtug/lug/config"
	"github.com/stretchr/testify/assert"
	"io"
	"os/exec"
	"time"
)

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
	external_worker, ok := NewExternalWorker(config.RepoConfig{
		"name":       "test_worker",
		"rlimit_mem": "10M",
	})
	asrt.Nil(ok)

	rlimitUtility := newRlimit(external_worker)

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
