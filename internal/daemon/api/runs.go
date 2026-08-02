package api

import (
	"errors"
	"net/http"
	"os"

	"github.com/nxrmqlly/jittrippin/internal/daemon"
	"github.com/nxrmqlly/jittrippin/internal/daemon/httpx"
	"github.com/nxrmqlly/jittrippin/internal/store"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
)

func (ro *Router) handleRunGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := ro.mgr.Get(id)
	if err != nil {

		switch {
		case errors.Is(err, store.ErrNotFound):
			httpx.ErrorJSON(w, http.StatusNotFound, "run not found")
		case errors.Is(err, daemon.ErrArtifactNotFound):
			httpx.ErrorJSON(w, http.StatusNotFound, "artifact not found")
		default:
			httpx.ErrorJSON(w, http.StatusInternalServerError, err.Error())
		}

		return
	}
	httpx.WriteJSON(w, http.StatusOK, run)
}

type runListResponse struct {
	Runs []*daemon.RunResult `json:"runs"`
}

func (ro *Router) handleRunList(w http.ResponseWriter, r *http.Request) {
	runs, err := ro.mgr.List()
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, runListResponse{
		Runs: runs,
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
		var verr engine.ValidationErrors

		switch {
		case errors.As(err, &verr):
			httpx.ErrorJSON(w, http.StatusBadRequest, err.Error())

		default:
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal server error")
		}

		return
	}

	httpx.WriteJSON(w, http.StatusCreated, runSubmitResponse{ID: run.ID()})
}
