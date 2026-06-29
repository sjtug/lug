package worker

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/sirupsen/logrus"
	"github.com/sjtug/lug/pkg/config"
)

func TestNewExternalWorker(t *testing.T) {
	asrt := assert.New(t)
	c := config.RepoConfig{
		"blahblah": "foobar",
		"type":     "external",
	}
	_, err := NewWorker(c, time.Now(), true)
	// worker with no name is not allowed
	asrt.NotNil(err)

	c["name"] = "test_external"
	w, err := NewWorker(c, time.Now(), true)
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

	w, _ := NewWorker(c, time.Now(), true)

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

func (d *dummyExecutor) RunOnce(ctx context.Context, logger *logrus.Entry, utilities []utility) (execResult, error) {
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

func TestUtilityRlimitWrapsCommand(t *testing.T) {
	asrt := assert.New(t)
	t.Setenv("LUG_RLIMIT_HELPER", "/bin/lug-rlimit-helper")
	external_worker, ok := NewExternalWorker(config.RepoConfig{
		"name":       "test_worker",
		"rlimit_mem": "10M",
	})
	asrt.Nil(ok)

	rlimitUtility := newRlimit(external_worker)

	cmd := exec.Command("rev", "arg")
	asrt.NoError(rlimitUtility.apply(cmd))
	asrt.Equal("/bin/lug-rlimit-helper", cmd.Path)
	asrt.Equal([]string{"/bin/lug-rlimit-helper", "__run-with-rlimit", "10M", "--", "rev", "arg"}, cmd.Args)
}

func TestShellScriptWorkerArgParse(t *testing.T) {
	c := map[string]interface{}{
		"type":   "shell_script",
		"name":   "shell",
		"script": "wc -l /proc/stat",
	}
	w, err := NewWorker(c, time.Now(), true)

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
