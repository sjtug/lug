package worker

import (
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"errors"
	"github.com/sirupsen/logrus"
	"github.com/sjtug/lug/pkg/config"
	"sync/atomic"
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
	asrt.True(status.Idle)
	asrt.NotNil(status.Stderr)
	asrt.NotNil(status.Stdout)
}

func TestNewShellScriptWorker(t *testing.T) {
	var c config.RepoConfig = make(map[string]interface{})
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

func TestShellScriptWorkerEnvVarsConvert(t *testing.T) {
	type TestCase struct {
		Str          string
		Expected     map[string]string
		ExpectedJSON string
	}
	testCases := []TestCase{
		{
			Str: `
env1: 2
env2: true
env3: false
env4: null
env5: /tmp/bbc`,
			Expected: map[string]string{
				"LUG_env1": "2",
				"LUG_env2": "1",
				"LUG_env5": "/tmp/bbc",
			},
			ExpectedJSON: `{"env1": 2, "env2": true, "env3": false, "env5": "/tmp/bbc"}`,
		},
	}
	asrt := assert.New(t)
	for _, testcase := range testCases {
		cfgViper := viper.New()
		cfgViper.SetConfigType("yaml")
		asrt.Nil(cfgViper.ReadConfig(strings.NewReader(testcase.Str)))
		actual_interfaces := map[string]interface{}{}
		asrt.Nil(cfgViper.Unmarshal(&actual_interfaces))
		actual, err := convertMapToEnvVars(actual_interfaces)
		asrt.Nil(err, spew.Sdump(actual_interfaces)+"\n"+spew.Sdump(cfgViper.AllSettings()))
		asrt.Contains(actual, "LUG_config_json")
		actual_json := actual["LUG_config_json"]
		delete(actual, "LUG_config_json")
		asrt.Equal(testcase.Expected, actual)
		asrt.JSONEq(testcase.ExpectedJSON, actual_json)
	}
}

type dummyExecutor struct {
	RunCnt int32
}

func (d *dummyExecutor) RunOnce(logger *logrus.Entry, utilities []utility) (execResult, error) {
	atomic.AddInt32(&d.RunCnt, 1)
	return execResult{"", ""}, errors.New("dummy error")
}

func TestExecutorInvokeWorker(t *testing.T) {
	asrt := assert.New(t)
	d := &dummyExecutor{}
	cfg := config.RepoConfig{
		"interval":       100,
		"retry":          2,
		"retry_interval": 1,
		"name":           "dummy",
	}
	logrus.SetLevel(logrus.DebugLevel)
	control := make(chan int, 1)
	w, err := NewExecutorInvokeWorker(d, Status{
		Idle:         true,
		Result:       true,
		LastFinished: time.Now().AddDate(-1, -1, -1),
	}, cfg, control)
	asrt.Nil(err)
	go w.RunSync()
	w.TriggerSync()
	time.Sleep(time.Millisecond * 100)
	// should be retrying...
	status1 := w.GetStatus()
	asrt.False(status1.Idle)
	asrt.Equal(1, int(atomic.LoadInt32(&d.RunCnt)))
	time.Sleep(time.Second * 2)
	// should abort..
	status2 := w.GetStatus()
	asrt.True(status2.Idle)
	asrt.False(status2.Result)
	asrt.Equal(2, int(atomic.LoadInt32(&d.RunCnt)))
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
	c := map[string]interface{}{
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
