package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"github.com/sjtug/lug/pkg/config"
	"mvdan.cc/sh/v3/shell"
)

// shellScriptExecutor implements executor interface
type shellScriptExecutor struct {
	cfg config.RepoConfig
	// memMax is the per-job memory.max in bytes (0 = unlimited), parsed
	// from `rlimit_mem` and enforced via the job cgroup.
	memMax uint64
	// timeout is the per-attempt wall clock budget (0 = no timeout), parsed
	// from `timeout` (seconds).
	timeout time.Duration
}

func newShellScriptExecutor(cfg config.RepoConfig) (*shellScriptExecutor, error) {
	e := &shellScriptExecutor{cfg: cfg}
	if raw, ok := cfg["rlimit_mem"]; ok {
		s, ok := raw.(string)
		if !ok {
			return nil, errors.New("rlimit_mem should be a size string when present")
		}
		bytes_, err := humanize.ParseBytes(s)
		if err != nil {
			return nil, fmt.Errorf("invalid rlimit_mem: %w", err)
		}
		e.memMax = bytes_
	}
	if raw, ok := cfg["timeout"]; ok {
		sec, ok := raw.(int)
		if !ok || sec < 0 {
			return nil, errors.New("timeout should be a non-negative integer (seconds) when present")
		}
		e.timeout = time.Duration(sec) * time.Second
	}
	return e, nil
}

func convertMapToEnvVars(m map[string]interface{}) (map[string]string, error) {
	result := map[string]string{}
	for k, v := range m {
		switch v := v.(type) {
		case nil:
			// skip
		case bool:
			if v {
				result["LUG_"+k] = "1"
			}
		case int, uint, float32, float64, string:
			result["LUG_"+k] = fmt.Sprint(v)
		default:
			return nil, errors.New("invalid type:" + spew.Sdump(v))
		}
	}
	marshal, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	result["LUG_config_json"] = string(marshal)
	return result, nil
}

func getOsEnvsAsMap() (result map[string]string) {
	envs := os.Environ()
	result = map[string]string{}
	for _, e := range envs {
		pair := strings.SplitN(e, "=", 2)
		key := pair[0]
		val := pair[1]
		result[key] = val
	}
	return
}

// RunOnce performs one sync attempt inside an ephemeral cgroup (when
// available): memory.max enforcement, whole-tree kill on ctx cancellation,
// and memory.peak / oom_kill telemetry are collected per run.
func (w *shellScriptExecutor) RunOnce(ctx context.Context, logger *logrus.Entry) (execResult, error) {
	script, ok := w.cfg["script"]
	if !ok {
		return execResult{}, errors.New("script not found in config")
	}

	// Split the command string into fields, respecting shell quoting rules
	fields, err := shell.Fields(script.(string), func(name string) string {
		return getOsEnvsAsMap()[name]
	})
	if err != nil {
		return execResult{}, fmt.Errorf("failed to parse command: %w", err)
	}

	if len(fields) == 0 {
		return execResult{}, errors.New("empty command")
	}

	if w.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.timeout)
		defer cancel()
	}

	logger.Debug("Invoking command:", fields[0], "with args:", fields[1:])
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	cmd.WaitDelay = 30 * time.Second // don't hang forever on inherited pipes

	// Forwarding config items to shell script as environmental variables
	// Adds a LUG_ prefix to their key
	env := os.Environ()
	envvars, err := convertMapToEnvVars(w.cfg)
	if err != nil {
		return execResult{}, fmt.Errorf("cannot convert w.cfg to env vars: %w", err)
	}
	for k, v := range envvars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	var bufErr, bufOut bytes.Buffer
	cmd.Stdout = &bufOut
	cmd.Stderr = &bufErr

	// Ephemeral per-job cgroup; nil when the environment lacks cgroup v2
	// delegation, in which case the job runs unconfined (plain subprocess).
	jobName := fmt.Sprintf("%v-%d", w.cfg["name"], time.Now().UnixNano())
	cg, err := newJobCgroup(jobName, w.memMax)
	if err != nil {
		logger.WithField("event", "job_cgroup_failed").WithError(err).
			Warn("failed to create job cgroup; running unconfined")
	}
	if cg != nil {
		defer cg.Close()
		attachToCgroup(cmd, cg)
		// Cancellation must reap the whole process tree, not just the direct
		// child (rsync wrappers fork). cgroup.kill does exactly that.
		cmd.Cancel = func() error {
			logger.WithField("event", "job_cgroup_kill").Info("killing job cgroup")
			return cg.Kill()
		}
	}

	start := time.Now()
	err = cmd.Start()
	if err != nil {
		return execResult{}, fmt.Errorf("execution cannot start: %w", err)
	}
	err = cmd.Wait()

	result := execResult{
		Stdout:   bufOut.String(),
		Stderr:   bufErr.String(),
		Duration: time.Since(start),
	}
	if cg != nil {
		result.PeakMemBytes, result.OOMKilled = cg.Collect()
	}
	logger.WithFields(logrus.Fields{
		"event":         "attempt_telemetry",
		"duration_sec":  result.Duration.Seconds(),
		"peak_mem":      result.PeakMemBytes,
		"oom_killed":    result.OOMKilled,
		"ctx_err":       ctx.Err(),
		"cgroup_scoped": cg != nil,
	}).Info("sync attempt finished")

	if err != nil {
		switch {
		case result.OOMKilled:
			return result, fmt.Errorf("execution failed: killed by OOM (memory.max=%d)", w.memMax)
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return result, fmt.Errorf("execution failed: timeout after %v", w.timeout)
		case ctx.Err() != nil:
			return result, fmt.Errorf("execution canceled: %w", ctx.Err())
		default:
			return result, fmt.Errorf("execution failed: %w", err)
		}
	}
	return result, nil
}
