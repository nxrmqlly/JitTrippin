package api

import (
	"net/http"
	"os"

	"github.com/nxrmqlly/jittrippin/internal/daemon"
	"github.com/nxrmqlly/jittrippin/internal/daemon/httpx"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
)

func (ro *Router) handleRunGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := ro.mgr.Get(id)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, run)
}

type runListResponse struct {
	Runs []*daemon.RunResult `json:"runs"`
}

func (ro *Router) handleRunList(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, runListResponse{
		Runs: ro.mgr.List(),
	})
}

type runSubmitResponse struct {
	ID string `json:"id"`
}

func (ro *Router) handleRunSubmit(w http.ResponseWriter, r *http.Request) {
	var p engine.Pipeline

	if !httpx.BindJSON(w, r, &p) {
		return
	}

	run, err := ro.mgr.Submit(&p, os.Stdout, os.Stderr)
	if err != nil {
		// can only ever return engine.ValidationErrors
		httpx.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, runSubmitResponse{ID: run.ID()})
}
