package manager

import (
	"encoding/json"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// RestfulAPI is a JSON-like API of given manager.
type RestfulAPI struct {
	manager *Manager
}

// NewRestfulAPI returns a pointer to RestfulAPI of given manager.
func NewRestfulAPI(m *Manager) *RestfulAPI {
	return &RestfulAPI{
		manager: m,
	}
}

// GetAPIHandler returns handler that could be used for http package.
func (r *RestfulAPI) GetAPIHandler() http.Handler {
	mux := http.NewServeMux()

	// Public/frontend/healthcheck endpoint. Keep this shape stable: the
	// frontend consumes Running/WorkerStatus with Go's historical exported
	// field names.
	mux.HandleFunc("GET /lug/v1/manager/summary", r.getManagerStatusSummary)

	// Internal admin endpoints. docker-compose exposes port 7001 only to the
	// Compose networks; Caddy rejects /lug/v1/admin/* publicly.
	mux.HandleFunc("GET /lug/v1/admin/manager/detail", r.getManagerStatusDetail)
	mux.HandleFunc("GET /lug/v1/admin/repos/{name}", r.getRepoStatus)
	mux.HandleFunc("POST /lug/v1/admin/repos/{name}/sync", r.triggerRepoSync)
	mux.HandleFunc("POST /lug/v1/admin/manager/start", r.startManager)
	mux.HandleFunc("POST /lug/v1/admin/manager/stop", r.stopManager)
	mux.HandleFunc("DELETE /lug/v1/admin/manager", r.exitManager)

	return mux
}

type WorkerStatusSimple struct {
	// Result is true if sync succeed, else false
	Result bool
	// LastFinished indicates last success time
	LastFinished time.Time
	// Idle stands for whether worker is idle, false if syncing
	Idle bool
}

type MangerStatusSimple struct {
	Running      bool
	WorkerStatus map[string]WorkerStatusSimple
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error(err)
	}
}

func (r *RestfulAPI) getManagerStatusCommon(w http.ResponseWriter, detailed bool) {
	rawStatus := r.manager.GetStatus()
	if detailed {
		writeJSON(w, http.StatusOK, rawStatus)
		return
	}
	managerStatusSimple := MangerStatusSimple{
		Running:      rawStatus.Running,
		WorkerStatus: map[string]WorkerStatusSimple{},
	}
	// summary mode
	for workerKey, rawWorkerStatus := range rawStatus.WorkerStatus {
		managerStatusSimple.WorkerStatus[workerKey] = WorkerStatusSimple{
			Result:       rawWorkerStatus.Result,
			LastFinished: rawWorkerStatus.LastFinished,
			Idle:         rawWorkerStatus.Idle,
		}
	}
	writeJSON(w, http.StatusOK, managerStatusSimple)
}

func (r *RestfulAPI) getManagerStatusDetail(w http.ResponseWriter, _ *http.Request) {
	r.getManagerStatusCommon(w, true)
}

func (r *RestfulAPI) getManagerStatusSummary(w http.ResponseWriter, _ *http.Request) {
	r.getManagerStatusCommon(w, false)
}

func (r *RestfulAPI) startManager(w http.ResponseWriter, _ *http.Request) {
	r.manager.Start()
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (r *RestfulAPI) stopManager(w http.ResponseWriter, _ *http.Request) {
	r.manager.Stop()
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (r *RestfulAPI) exitManager(w http.ResponseWriter, _ *http.Request) {
	// Shutdown (not bare Exit()) so an API-triggered exit also cancels the
	// root context and kills in-flight rsync children — same disposable
	// behaviour as SIGTERM (12-factor IX). Run it asynchronously after the
	// acknowledgement so lugctl receives a clean response before the daemon exits.
	writeJSON(w, http.StatusOK, map[string]string{"status": "exiting"})
	go func() {
		time.Sleep(50 * time.Millisecond)
		r.manager.Shutdown()
	}()
}

// getRepoStatus returns the live status of a single repo, including the
// stdout/stderr ring buffer the operator inspects while a sync runs.
func (r *RestfulAPI) getRepoStatus(w http.ResponseWriter, req *http.Request) {
	name := req.PathValue("name")
	status, ok := r.manager.GetRepoStatus(name)
	if !ok {
		http.NotFound(w, req)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// triggerRepoSync schedules a one-off sync for one repo. It only enqueues
// the sync (TriggerSync on the live worker); the daemon's scheduler runs
// it under the same concurrency cap as a normal poll.
func (r *RestfulAPI) triggerRepoSync(w http.ResponseWriter, req *http.Request) {
	name := req.PathValue("name")
	if r.manager.TriggerRepoByName(name) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "triggered", "repo": name})
		return
	}
	http.NotFound(w, req)
}
