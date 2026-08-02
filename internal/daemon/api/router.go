package api

import (
	"net/http"

	"github.com/nxrmqlly/jittrippin/internal/daemon"
)

// Router is a http.Handler like object
type Router struct {
	mgr *daemon.Manager
	mux *http.ServeMux
}

// NewRouter returns a pointer to a new http.Handler-like Router object.
func NewRouter(mgr *daemon.Manager) *Router {
	ro := &Router{
		mgr: mgr,
		mux: http.NewServeMux(),
	}
	ro.routes()
	return ro
}

func (ro *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ro.mux.ServeHTTP(w, r)
}

func (ro *Router) routes() {
	ro.mux.HandleFunc("GET /api/v1/health", ro.handleHealth)

	ro.mux.HandleFunc("GET /api/v1/runs/{id}", ro.handleRunGet)
	ro.mux.HandleFunc("GET /api/v1/runs", ro.handleRunList)
	ro.mux.HandleFunc("POST /api/v1/runs", ro.handleRunSubmit)

	ro.mux.HandleFunc("GET /api/v1/runs/{id}/artifacts/", ro.handleArtifactsList)
	ro.mux.HandleFunc("GET /api/v1/runs/{id}/artifacts/{job}/{artifact}", ro.handleArtifactsServe)
}
