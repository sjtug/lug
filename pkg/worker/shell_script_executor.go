package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cosiner/argv"
	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
	"github.com/sjtug/lug/pkg/config"
	"os"
	"os/exec"
	"strings"
)

// shellScriptExecutor implements executor interface
type shellScriptExecutor struct {
	cfg config.RepoConfig
}

func newShellScriptExecutor(cfg config.RepoConfig) *shellScriptExecutor {
	return &shellScriptExecutor{
		cfg: cfg,
	}
}

func convertMapToEnvVars(m map[string]interface{}) (map[string]string, error) {
	result := map[string]string{}
	for k, v := range m {
		switch v.(type) {
		case nil:
			// skip
		case bool:
			if v.(bool) {
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
		pair := strings.Split(e, "=")
		key := pair[0]
		val := pair[1]
		result[key] = val
	}
	return
}

// RunSync launches the worker
func (w *shellScriptExecutor) RunOnce(logger *logrus.Entry, utilities []utility) (execResult, error) {
	script, _ := w.cfg["script"]

	args, err := argv.Argv([]rune(script.(string)), getOsEnvsAsMap(), argv.Run)
	if err != nil {
		return execResult{"", ""}, errors.New(fmt.Sprint("Failed to parse argument:", err.Error()))
	}
	if len(args) > 1 {
		return execResult{"", ""}, errors.New("pipe is not supported in shell_script_worker")
	}
	invokeArgs := args[0]
	logger.Debug("Invoking args:", invokeArgs)
	cmd := exec.Command(invokeArgs[0], invokeArgs[1:]...)

	// Forwarding config items to shell script as environmental variables
	// Adds a LUG_ prefix to their key
	env := os.Environ()
	envvars, err := convertMapToEnvVars(w.cfg)
	if err != nil {
		return execResult{"", ""}, errors.New(fmt.Sprint("cannot convert w.cfg to env vars: ", err))
	}
	for k, v := range envvars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	for _, utility := range utilities {
		logger.WithField("event", "exec_prehook").Debug("Executing prehook of ", utility)
		if err := utility.preHook(); err != nil {
			logger.Error("Failed to execute preHook:", err)
		}
	}

	var bufErr, bufOut bytes.Buffer
	cmd.Stdout = &bufOut
	cmd.Stderr = &bufErr

	err = cmd.Start()

	for _, utility := range utilities {
		logger.WithField("event", "exec_posthook").Debug("Executing postHook of ", utility)
		if err := utility.postHook(); err != nil {
			logger.Error("Failed to execute postHook:", err)
		}
	}
	if err != nil {
		return execResult{"", ""}, errors.New("execution cannot start")
	}
	err = cmd.Wait()
	if err != nil {
		return execResult{bufOut.String(), bufErr.String()}, errors.New("execution failed")
	}
	return execResult{bufOut.String(), bufErr.String()}, nil
}
