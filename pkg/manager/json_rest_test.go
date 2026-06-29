package manager

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sjtug/lug/pkg/config"
	"github.com/stretchr/testify/require"
)

func newTestAPI(t *testing.T) http.Handler {
	t.Helper()
	m, err := NewManager(&config.Config{
		Interval: 1,
		Repos: []config.RepoConfig{
			{
				"type":   "shell_script",
				"name":   "repo-a",
				"script": "true",
			},
		},
	})
	require.NoError(t, err)
	return NewRestfulAPI(m).GetAPIHandler()
}

func TestAPISummaryPreservesFrontendShape(t *testing.T) {
	h := newTestAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/lug/v1/manager/summary", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Contains(t, body, "Running")
	require.Contains(t, body, "WorkerStatus")

	workerStatus, ok := body["WorkerStatus"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, workerStatus, "repo-a")

	repoStatus, ok := workerStatus["repo-a"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, repoStatus, "Result")
	require.Contains(t, repoStatus, "LastFinished")
	require.Contains(t, repoStatus, "Idle")
}

func TestAPIRepoStatusAndTrigger(t *testing.T) {
	h := newTestAPI(t)

	statusReq := httptest.NewRequest(http.MethodGet, "/lug/v1/admin/repos/repo-a", nil)
	statusRR := httptest.NewRecorder()
	h.ServeHTTP(statusRR, statusReq)
	require.Equal(t, http.StatusOK, statusRR.Code)

	triggerReq := httptest.NewRequest(http.MethodPost, "/lug/v1/admin/repos/repo-a/sync", nil)
	triggerRR := httptest.NewRecorder()
	h.ServeHTTP(triggerRR, triggerReq)
	require.Equal(t, http.StatusOK, triggerRR.Code)
	require.JSONEq(t, `{"repo":"repo-a","status":"triggered"}`, triggerRR.Body.String())

	missingReq := httptest.NewRequest(http.MethodPost, "/lug/v1/admin/repos/missing/sync", nil)
	missingRR := httptest.NewRecorder()
	h.ServeHTTP(missingRR, missingReq)
	require.Equal(t, http.StatusNotFound, missingRR.Code)
}
