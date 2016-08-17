package manager

import (
	log "github.com/Sirupsen/logrus"
	"github.com/ant0ine/go-json-rest/rest"
	"net/http"
)

type RestfulAPI struct {
	manager *Manager
}

func NewRestfulAPI(m *Manager) *RestfulAPI {
	return &RestfulAPI{
		manager: m,
	}
}

func (r *RestfulAPI) GetAPIHandler() http.Handler {
	api := rest.NewApi()
	api.Use(rest.DefaultDevStack...)
	router, err := rest.MakeRouter(
		rest.Get("/lug/v1/manager", r.getManagerStatus),
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

func (r *RestfulAPI) getManagerStatus(w rest.ResponseWriter, req *rest.Request) {
	status := r.manager.GetStatus()
	w.WriteJson(status)
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
