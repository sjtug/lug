package manager

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
	"github.com/sjtug/lug/pkg/worker"
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
		rest.Post("/lug/v1/admin/manager/start", r.startManager),
		rest.Post("/lug/v1/admin/manager/stop", r.stopManager),
		rest.Post("/lug/v1/admin/worker/:name/abort", r.abortWorker),
		rest.Post("/lug/v1/admin/worker/:name/sync", r.triggerSync),
		rest.Get("/lug/v1/admin/worker/:name/job", r.getWorkerJob),
		rest.Get("/lug/v1/admin/queue", r.getQueueStatus),
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
	r.manager.Exit()
}

func (r *RestfulAPI) abortWorker(w rest.ResponseWriter, req *rest.Request) {
	name := req.PathParam("name")
	if err := r.manager.AbortWorker(name); err != nil {
		rest.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (r *RestfulAPI) triggerSync(w rest.ResponseWriter, req *rest.Request) {
	name := req.PathParam("name")
	if err := r.manager.TriggerWorkerSync(name); err != nil {
		code := http.StatusNotFound
		if err.Error() != fmt.Sprintf("no worker named %s", name) {
			code = http.StatusConflict
		}
		rest.Error(w, err.Error(), code)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (r *RestfulAPI) getQueueStatus(w rest.ResponseWriter, req *rest.Request) {
	qs := r.manager.GetQueueStatus()
	if err := w.WriteJson(qs); err != nil {
		log.Error(err)
	}
}

// jobDetail is the response from the per-worker job inspection endpoint.
type jobDetail struct {
	Name        string                `json:"name"`
	ActiveJob   *worker.ActiveJobInfo `json:"active_job"`
	CgroupStats *worker.CgroupStats   `json:"cgroup_stats,omitempty"`
	AttachHint  string                `json:"attach_hint,omitempty"`
}

func (r *RestfulAPI) getWorkerJob(w rest.ResponseWriter, req *rest.Request) {
	name := req.PathParam("name")
	status := r.manager.GetStatus()
	ws, ok := status.WorkerStatus[name]
	if !ok {
		rest.Error(w, "no worker named "+name, http.StatusNotFound)
		return
	}
	if ws.ActiveJob == nil {
		rest.Error(w, "worker "+name+" is idle", http.StatusNotFound)
		return
	}
	resp := jobDetail{
		Name:      name,
		ActiveJob: ws.ActiveJob,
	}
	if ws.ActiveJob.CgroupPath != "" {
		stats := worker.ReadCgroupStats(ws.ActiveJob.CgroupPath)
		resp.CgroupStats = &stats
		resp.AttachHint = fmt.Sprintf(
			"nsenter --cgroup=%s --fork <command>",
			ws.ActiveJob.CgroupPath,
		)
	}
	if err := w.WriteJson(resp); err != nil {
		log.Error(err)
	}
}
