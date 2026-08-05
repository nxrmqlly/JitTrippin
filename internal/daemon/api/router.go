package api

import (
	"net/http"

	"github.com/nxrmqlly/jittrippin/internal/auth"
	"github.com/nxrmqlly/jittrippin/internal/daemon"
)

// Router is a http.Handler like object
type Router struct {
	mgr  *daemon.Manager
	mux  *http.ServeMux
	auth *auth.Service
}

// New returns a pointer to a new http.Handler-like Router object.
func New(mgr *daemon.Manager, auth *auth.Service) *Router {
	ro := &Router{
		mgr:  mgr,
		mux:  http.NewServeMux(),
		auth: auth,
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

	ro.mux.HandleFunc("POST /api/v1/auth/begin", ro.handleAuthBegin)
	ro.mux.HandleFunc("GET /api/v1/auth/{provider}/callback", ro.handleAuthCallback)
	ro.mux.HandleFunc("POST /api/v1/auth/exchange", ro.handleAuthExchange)
	ro.mux.HandleFunc("POST /api/v1/auth/logout", ro.handleAuthLogout)
}
