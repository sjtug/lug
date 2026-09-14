package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/sjtug/lug/pkg/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
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

func (d *dummyExecutor) RunOnce(ctx context.Context, logger *logrus.Entry) (execResult, error) {
	atomic.AddInt32(&d.RunCnt, 1)
	return execResult{}, errors.New("dummy error")
}

type failOnceExecutor struct {
	RunCnt int32
}

func (d *failOnceExecutor) RunOnce(ctx context.Context, logger *logrus.Entry) (execResult, error) {
	if atomic.AddInt32(&d.RunCnt, 1) == 1 {
		return execResult{}, errors.New("first attempt failed")
	}
	return execResult{}, nil
}

func TestExecutorInvokeWorkerLogsEveryAttemptStart(t *testing.T) {
	asrt := assert.New(t)
	d := &failOnceExecutor{}
	cfg := config.RepoConfig{
		"retry":          2,
		"retry_interval": 0,
		"name":           "retry-log-test",
	}
	control := make(chan int, 1)
	w, err := NewExecutorInvokeWorker(d, Status{Idle: true}, cfg, control)
	asrt.NoError(err)

	logger, hook := logrustest.NewNullLogger()
	w.logger = logger.WithField("repo", "retry-log-test")
	go w.RunSync()
	w.TriggerSync()

	asrt.Eventually(func() bool {
		return atomic.LoadInt32(&d.RunCnt) == 2 && w.GetStatus().Idle
	}, time.Second, 10*time.Millisecond)

	var attempts []int
	for _, entry := range hook.AllEntries() {
		if entry.Data["event"] == "start_execution" {
			attempt, ok := entry.Data["try_cnt"].(int)
			asrt.True(ok)
			attempts = append(attempts, attempt)
		}
	}
	asrt.Equal([]int{1, 2}, attempts)
}

func TestExecutorInvokeWorkerDoesNotWaitAfterFinalAttempt(t *testing.T) {
	asrt := assert.New(t)
	d := &dummyExecutor{}
	control := make(chan int, 1)
	w, err := NewExecutorInvokeWorker(d, Status{Idle: true}, config.RepoConfig{
		"retry":          1,
		"retry_interval": 3600,
		"name":           "no-final-wait-test",
	}, control)
	asrt.NoError(err)

	go w.RunSync()
	w.TriggerSync()

	asrt.Eventually(func() bool {
		return atomic.LoadInt32(&d.RunCnt) == 1 && w.GetStatus().Idle
	}, time.Second, 10*time.Millisecond)
}

func TestExecutorInvokeWorkerAbortInterruptsRetryWait(t *testing.T) {
	asrt := assert.New(t)
	d := &dummyExecutor{}
	control := make(chan int, 1)
	w, err := NewExecutorInvokeWorker(d, Status{Idle: true}, config.RepoConfig{
		"retry":          2,
		"retry_interval": 3600,
		"name":           "cancel-retry-wait-test",
	}, control)
	asrt.NoError(err)

	go w.RunSync()
	w.TriggerSync()
	asrt.Eventually(func() bool {
		return atomic.LoadInt32(&d.RunCnt) == 1 && !w.GetStatus().Idle
	}, time.Second, 10*time.Millisecond)

	w.AbortSync()

	asrt.Eventually(func() bool {
		return w.GetStatus().Idle
	}, time.Second, 10*time.Millisecond)
	asrt.Equal(int32(1), atomic.LoadInt32(&d.RunCnt))
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

func TestShellScriptExecutorTimeout(t *testing.T) {
	asrt := assert.New(t)
	e, err := newShellScriptExecutor(config.RepoConfig{
		"name":    "timeout_test",
		"script":  "sleep 30",
		"timeout": 1,
	})
	asrt.Nil(err)
	start := time.Now()
	result, err := e.RunOnce(context.Background(), logrus.WithField("repo", "timeout_test"))
	asrt.NotNil(err)
	asrt.Contains(err.Error(), "timeout")
	asrt.Less(time.Since(start), 10*time.Second)
	asrt.Greater(result.Duration, time.Duration(0))
}

func TestShellScriptExecutorCancel(t *testing.T) {
	asrt := assert.New(t)
	e, err := newShellScriptExecutor(config.RepoConfig{
		"name":   "cancel_test",
		"script": "sleep 30",
	})
	asrt.Nil(err)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err = e.RunOnce(ctx, logrus.WithField("repo", "cancel_test"))
	asrt.NotNil(err)
	asrt.Less(time.Since(start), 10*time.Second)
}

func TestShellScriptExecutorCancelKillsDescendants(t *testing.T) {
	asrt := assert.New(t)
	pidFile := t.TempDir() + "/child.pid"
	e, err := newShellScriptExecutor(config.RepoConfig{
		"name": "cancel_descendants_test",
		"script": fmt.Sprintf(
			`sh -c 'sleep 30 & echo $! > %s; wait'`, pidFile,
		),
	})
	asrt.NoError(err)
	// Force the production fallback used by restricted containers where cgroup
	// delegation is unavailable.
	e.newJobCgroup = func(string, uint64) (*jobCgroup, error) { return nil, nil }

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, runErr := e.RunOnce(ctx, logrus.WithField("repo", "cancel_descendants_test"))
		errCh <- runErr
	}()

	var childPID int
	asrt.Eventually(func() bool {
		data, readErr := os.ReadFile(pidFile)
		if readErr != nil {
			return false
		}
		childPID, readErr = strconv.Atoi(strings.TrimSpace(string(data)))
		return readErr == nil
	}, 2*time.Second, 10*time.Millisecond)
	if childPID == 0 {
		cancel()
		t.Fatal("child process did not publish its PID")
	}

	start := time.Now()
	cancel()
	select {
	case runErr := <-errCh:
		asrt.Error(runErr)
		asrt.Contains(runErr.Error(), "canceled")
	case <-time.After(5 * time.Second):
		t.Fatal("executor did not return promptly after cancellation")
	}
	asrt.Less(time.Since(start), 5*time.Second)
	asrt.Eventually(func() bool {
		err := syscall.Kill(childPID, 0)
		return errors.Is(err, syscall.ESRCH)
	}, 2*time.Second, 10*time.Millisecond, "descendant process %d survived cancellation", childPID)
}

func TestShellScriptExecutorTelemetry(t *testing.T) {
	asrt := assert.New(t)
	e, err := newShellScriptExecutor(config.RepoConfig{
		"name":   "telemetry_test",
		"script": "true",
	})
	asrt.Nil(err)
	result, err := e.RunOnce(context.Background(), logrus.WithField("repo", "telemetry_test"))
	asrt.Nil(err)
	asrt.Greater(result.Duration, time.Duration(0))
	// PeakMemBytes > 0 only when cgroup v2 delegation is available; do not assert.
}

func TestTelemetryObserve(t *testing.T) {
	asrt := assert.New(t)
	var tel Telemetry
	tel.Observe(10*time.Second, 1000, false)
	asrt.Equal(10*time.Second, tel.DurationEWMA)
	asrt.Equal(uint64(1000), tel.PeakMemEWMA)
	tel.Observe(20*time.Second, 2000, true)
	asrt.Equal(uint64(1), tel.OOMKills)
	asrt.Equal(uint64(2), tel.Runs)
	// EWMA moves toward the new sample but not all the way
	asrt.Greater(tel.DurationEWMA, 10*time.Second)
	asrt.Less(tel.DurationEWMA, 20*time.Second)
	asrt.Greater(tel.PeakMemEWMA, uint64(1000))
	asrt.Less(tel.PeakMemEWMA, uint64(2000))
	// Zero samples must not disturb the EWMAs
	prev := tel
	tel.Observe(0, 0, false)
	asrt.Equal(prev.DurationEWMA, tel.DurationEWMA)
	asrt.Equal(prev.PeakMemEWMA, tel.PeakMemEWMA)
}

func TestRlimitMemRejected(t *testing.T) {
	asrt := assert.New(t)
	_, err := newShellScriptExecutor(config.RepoConfig{
		"name":       "bad_rlimit",
		"script":     "true",
		"rlimit_mem": "not-a-size",
	})
	asrt.NotNil(err)
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
