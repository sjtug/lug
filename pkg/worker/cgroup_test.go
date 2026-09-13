package worker

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sjtug/lug/pkg/config"
)

// TestCgroupEnforcement verifies memory.max enforcement and OOM telemetry.
// It skips unless run under a delegated cgroup subtree, e.g.:
//
//	systemd-run --user --scope -p Delegate=yes go test -run TestCgroup ./pkg/worker/
func TestCgroupEnforcement(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	e, err := newShellScriptExecutor(config.RepoConfig{
		"name":       "cg_mem_test",
		"script":     `python3 -c "x = bytearray(100*1024*1024); import time; time.sleep(1)"`,
		"rlimit_mem": "20M",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := e.RunOnce(context.Background(), logrus.WithField("repo", "cg_mem_test"))
	t.Logf("err=%v peak=%d oom=%v dur=%v", err, result.PeakMemBytes, result.OOMKilled, result.Duration)
	if result.PeakMemBytes == 0 {
		t.Skip("cgroup delegation unavailable in this environment")
	}
	if err == nil || !result.OOMKilled {
		t.Fatal("expected OOM kill under 20M memory.max")
	}
}

func TestCgroupTelemetryHappy(t *testing.T) {
	e, err := newShellScriptExecutor(config.RepoConfig{
		"name":   "cg_ok_test",
		"script": "sleep 0.2",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := e.RunOnce(context.Background(), logrus.WithField("repo", "cg_ok_test"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Duration < 100*time.Millisecond {
		t.Fatal("duration not measured")
	}
	t.Logf("peak=%d dur=%v", result.PeakMemBytes, result.Duration)
}
