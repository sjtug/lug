package manager

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sjtug/lug/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestWorkerJobReportsNoActiveProcess(t *testing.T) {
	manager, err := NewManager(&config.Config{
		Repos: []config.RepoConfig{
			{
				"type":   "shell_script",
				"name":   "idle",
				"script": "true",
			},
		},
	})
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodGet,
		"/lug/v1/admin/worker/idle/job",
		nil,
	)
	response := httptest.NewRecorder()
	NewRestfulAPI(manager).GetAPIHandler().ServeHTTP(response, request)

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Contains(t, response.Body.String(), "worker idle has no active job")
}

func TestJobAttachHintTargetsProcessAndJoinsItsCgroup(t *testing.T) {
	hint := jobAttachHint(12345)

	assert.Equal(
		t,
		"nsenter --target=12345 --cgroup --join-cgroup -- <command>",
		hint,
	)
	assert.NotContains(t, hint, "--fork")
	assert.NotContains(t, hint, "/sys/fs/cgroup")
}
