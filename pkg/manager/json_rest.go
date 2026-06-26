package manager

import (
	"net/http"
	"time"

	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
)

// RestfulAPI is a JSON-like API of given manager
type RestfulAPI struct {
	manager *Manager
}

// NewRestfulAPI returns a pointer to RestfulAPI of given manager
func NewRestfulAPI(m *Manager) *RestfulAPI {
	return &RestfulAPI{
		manager: m,
	}
}

// GetAPIHandler returns handler that could be used for http package
func (r *RestfulAPI) GetAPIHandler() http.Handler {
	api := rest.NewApi()
	api.Use(rest.DefaultDevStack...)
	router, err := rest.MakeRouter(
		rest.Get("/lug/v1/admin/manager/detail", r.getManagerStatusDetail),
		rest.Get("/lug/v1/manager/summary", r.getManagerStatusSummary),
		rest.Get("/lug/v1/admin/repos/:name", r.getRepoStatus),
		rest.Post("/lug/v1/admin/repos/:name/sync", r.triggerRepoSync),
		rest.Post("/lug/v1/admin/manager/start", r.startManager),
		rest.Post("/lug/v1/admin/manager/stop", r.stopManager),
		rest.Delete("/lug/v1/admin/manager", r.exitManager),
	)
	if err != nil {
		log.Fatal(err)
	}
	api.SetApp(router)
	return api.MakeHandler()
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

func (r *RestfulAPI) getManagerStatusCommon(w rest.ResponseWriter, req *rest.Request, detailed bool) {
	rawStatus := r.manager.GetStatus()
	if detailed {
		if err := w.WriteJson(rawStatus); err != nil {
			log.Error(err)
		}
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
	if err := w.WriteJson(managerStatusSimple); err != nil {
		log.Error(err)
	}
}

func (r *RestfulAPI) getManagerStatusDetail(w rest.ResponseWriter, req *rest.Request) {
	r.getManagerStatusCommon(w, req, true)
}

func (r *RestfulAPI) getManagerStatusSummary(w rest.ResponseWriter, req *rest.Request) {
	r.getManagerStatusCommon(w, req, false)
}

func (r *RestfulAPI) startManager(w rest.ResponseWriter, req *rest.Request) {
	r.manager.Start()
}

func (r *RestfulAPI) stopManager(w rest.ResponseWriter, req *rest.Request) {
	r.manager.Stop()
}

func (r *RestfulAPI) exitManager(w rest.ResponseWriter, req *rest.Request) {
	// Shutdown (not bare Exit()) so an API-triggered exit also cancels the
	// root context and kills in-flight rsync children — same disposable
	// behaviour as SIGTERM (12-factor IX).
	if err := w.WriteJson(map[string]string{"status": "exiting"}); err != nil {
		log.Error(err)
	}
	go r.manager.Shutdown()
}

// getRepoStatus returns the live status of a single repo, including the
// stdout/stderr ring buffer the operator inspects while a sync runs.
func (r *RestfulAPI) getRepoStatus(w rest.ResponseWriter, req *rest.Request) {
	name := req.PathParam("name")
	status, ok := r.manager.GetRepoStatus(name)
	if !ok {
		rest.NotFound(w, req)
		return
	}
	if err := w.WriteJson(status); err != nil {
		log.Error(err)
	}
}

// triggerRepoSync schedules a one-off sync for one repo. It only enqueues
// the sync (TriggerSync on the live worker); the daemon's scheduler runs
// it under the same concurrency cap as a normal poll.
func (r *RestfulAPI) triggerRepoSync(w rest.ResponseWriter, req *rest.Request) {
	name := req.PathParam("name")
	if r.manager.TriggerRepoByName(name) {
		if err := w.WriteJson(map[string]string{"status": "triggered", "repo": name}); err != nil {
			log.Error(err)
		}
		return
	}
	rest.NotFound(w, req)
}
