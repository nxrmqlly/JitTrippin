package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/nxrmqlly/jittrippin/internal/daemon"
	"github.com/nxrmqlly/jittrippin/internal/daemon/httpx"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	"github.com/nxrmqlly/jittrippin/pkg/logs"
)

func (ro *Router) handleRunGet(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	run, err := ro.mgr.Get(daemon.GetConfig{
		RunID:  id,
		UserID: usr.ID,
	})
	if err != nil {
		runError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, run)
}

type runListResponse struct {
	Runs []*daemon.RunResult `json:"runs"`
}

func (ro *Router) handleRunList(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}
	runs, err := ro.mgr.List(daemon.ListConfig{UserID: usr.ID})
	if err != nil {
		runError(w, err)
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
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}

	var p engine.Pipeline
	if !httpx.BindJSON(w, r, &p) {
		return
	}
	run, err := ro.mgr.Submit(daemon.SubmitConfig{Pipeline: &p, OwnerID: usr.ID})
	if err != nil {
		var verr engine.ValidationErrors
		switch {
		case errors.As(err, &verr):
			httpx.ErrorJSON(w, http.StatusBadRequest, err.Error())
		default:
			runError(w, err)
		}
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, runSubmitResponse{ID: run.ID()})
}

func (ro *Router) handleLogsGet(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	jo := r.PathValue("job")
	rc, err := ro.mgr.GetLogs(r.Context(), daemon.GetLogsConfig{
		RunID:    id,
		UserID:   usr.ID,
		JobName:  jo,
		Filename: "combined",
	})
	if err != nil {
		runError(w, err)
	}
	defer rc.Close()

	w.Header().Set("Content-Type", "application/x-ndjson")
	io.Copy(w, rc)
}

func (ro *Router) handleLogsLive(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ch, unsub, err := ro.mgr.SubscribeLogs(daemon.SubscribeLogsConfig{
		Ref: logs.Ref{
			RunID: r.PathValue("id"),
			Job:   r.PathValue("job"),
		},
		UserID: usr.ID,
	})
	if err != nil {
		runError(w, err)
		return
	}
	defer unsub()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	enc := json.NewEncoder(w)
	for {
		select {
		case <-r.Context().Done():
			return
		case line := <-ch:
			fmt.Fprint(w, "data: ")
			enc.Encode(line)
			fmt.Fprintf(w, "\n\n")
			flusher.Flush()
		}
	}
}

func (ro *Router) handleRunCancel(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}

	if err := ro.mgr.Cancel(daemon.CancelConfig{
		RunID:  r.PathValue("id"),
		UserID: usr.ID,
	}); err != nil {
		runError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
