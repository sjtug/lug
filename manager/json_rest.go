package manager

import (
	log "github.com/Sirupsen/logrus"
	"github.com/ant0ine/go-json-rest/rest"
	"net/http"
	"time"
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
		rest.Get("/lug/v1/manager", r.getManagerStatusDetail),
		rest.Get("/lug/v1/manager/summary", r.getManagerStatusSummary),
		rest.Post("/lug/v1/manager/start", r.startManager),
		rest.Post("/lug/v1/manager/stop", r.stopManager),
		rest.Delete("/lug/v1/manager", r.exitManager),
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
		w.WriteJson(rawStatus)
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
	w.WriteJson(managerStatusSimple)
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
	r.manager.Exit()
}
