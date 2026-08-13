package api

import (
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/nxrmqlly/jittrippin/internal/auth"
	"github.com/nxrmqlly/jittrippin/internal/daemon"
	"github.com/nxrmqlly/jittrippin/internal/daemon/httpx"
	"github.com/nxrmqlly/jittrippin/internal/store"
)

// Router is a http.Handler like object
type Router struct {
	mgr  *daemon.Manager
	mux  *http.ServeMux
	auth *auth.Service

	GlobalMws []Middleware
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

func (ro *Router) routes() {
	ro.GlobalMws = append(ro.GlobalMws, ro.Logging)

	ro.Handle("GET /api/v1/health", ro.handleHealth)

	ro.Handle("GET    /api/v1/runs/{id}", ro.handleRunGet, ro.Authentication)
	ro.Handle("GET    /api/v1/runs", ro.handleRunList, ro.Authentication)
	ro.Handle("POST   /api/v1/runs", ro.handleRunSubmit, ro.Authentication)

	ro.Handle("GET    /api/v1/runs/{id}/artifacts/", ro.handleArtifactsList, ro.Authentication)
	ro.Handle("GET    /api/v1/runs/{id}/artifacts/{job}/{artifact}", ro.handleArtifactsServe, ro.Authentication)

	ro.Handle("GET    /api/v1/runs/{id}/logs/{job}/", ro.handleLogsGet, ro.Authentication)
	ro.Handle("GET    /api/v1/runs/{id}/logs/{job}/live", ro.handleLogsLive, ro.Authentication)

	ro.Handle("POST   /api/v1/runs/{id}/cancel", ro.handleRunCancel, ro.Authentication)

	ro.Handle("POST   /api/v1/auth/begin", ro.handleAuthBegin)
	ro.Handle("GET    /api/v1/auth/{provider}/callback", ro.handleAuthCallback)
	ro.Handle("POST   /api/v1/auth/exchange", ro.handleAuthExchange)
	ro.Handle("POST   /api/v1/auth/logout", ro.handleAuthLogout)

	ro.Handle("POST   /api/v1/integrations/github/webhook", ro.handleGithubWebhook)
	ro.Handle("POST   /api/v1/integrations/github", ro.handleGithubAdd, ro.Authentication)
	ro.Handle("GET    /api/v1/integrations/github/list", ro.handleGithubList, ro.Authentication)
	ro.Handle("DELETE /api/v1/integrations/github", ro.handleGithubRemove, ro.Authentication)

}

func (ro *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ro.mux.ServeHTTP(w, r)
}

func (ro *Router) Handle(pattern string, f http.HandlerFunc, mws ...Middleware) {
	ro.mux.Handle(pattern, Chain(f, append(ro.GlobalMws, mws...)...))
}

type Group struct {
	ro     *Router
	prefix string
	mws    []Middleware
}

func (ro *Router) Group(prefix string, mws ...Middleware) *Group {
	return &Group{ro: ro, prefix: prefix, mws: mws}
}

func resolvePattern(pre string, suf string) string {
	parts := strings.Fields(suf)
	var method string
	var suffix string

	if len(parts) >= 2 {
		method = parts[0]
		suffix = parts[1]
	} else {
		panic("invalid pattern")
	}

	return method + " " + path.Join(pre, suffix)
}

func (g *Group) Handle(pattern string, f http.HandlerFunc, mws ...Middleware) {
	all := make([]Middleware, 0, len(g.mws)+len(mws))
	all = append(all, g.mws...)
	all = append(all, mws...)
	g.ro.Handle(resolvePattern(g.prefix, pattern), f, all...)
}

func runError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		httpx.ErrorJSON(w, http.StatusNotFound, "not found") // 404 generic

	case errors.Is(err, store.ErrRunNotFound):
		httpx.ErrorJSON(w, http.StatusNotFound, "run not found") // 404

	case errors.Is(err, daemon.ErrArtifactNotFound):
		httpx.ErrorJSON(w, http.StatusNotFound, "artifact not found") // 404

	case errors.Is(err, daemon.ErrRunNotRunning):
		httpx.ErrorJSON(w, http.StatusConflict, "run is not running") // 409

	case errors.Is(err, daemon.ErrForbidden):
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden") // 403

	default:
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal server error") // 500
	}
}
