package worker

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sjtug/lug/pkg/config"
)

func TestCgroupMemoryLimitConfiguration(t *testing.T) {
	const limit = 20 * 1024 * 1024
	cg, err := newJobCgroup("cg_limit_test", limit)
	if err != nil {
		t.Fatal(err)
	}
	if cg == nil {
		t.Skip("cgroup delegation unavailable in this environment")
	}
	defer cg.Close()

	data, err := os.ReadFile(cg.Path() + "/memory.max")
	if err != nil {
		t.Fatal(err)
	}
	got, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if got != limit {
		t.Fatalf("memory.max = %d, want %d", got, limit)
	}
}

// TestCgroupOOMEnforcement verifies kernel OOM enforcement and telemetry. It
// intentionally invokes the kernel OOM killer, so it is excluded from normal
// test runs. Run it explicitly in a disposable delegated scope with:
//
//	LUG_RUN_CGROUP_OOM_TEST=1 go test -run TestCgroupOOMEnforcement ./pkg/worker/
func TestCgroupOOMEnforcement(t *testing.T) {
	if os.Getenv("LUG_RUN_CGROUP_OOM_TEST") != "1" {
		t.Skip("set LUG_RUN_CGROUP_OOM_TEST=1 to run the destructive OOM integration test")
	}

	logrus.SetLevel(logrus.DebugLevel)
	e, err := newShellScriptExecutor(config.RepoConfig{
		"name":       "cg_oom_test",
		"script":     `python3 -c "x = bytearray(100*1024*1024); import time; time.sleep(1)"`,
		"rlimit_mem": "20M",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := e.RunOnce(context.Background(), logrus.WithField("repo", "cg_oom_test"))
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
