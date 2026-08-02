package api

import (
	"fmt"
	"net/http"

	"github.com/nxrmqlly/jittrippin/internal/daemon/httpx"
)

type artifactsListItem struct {
	Job      string `json:"job"`
	Artifact string `json:"artifact"`
	Download string `json:"download"`
}
type artifactsListResponse struct {
	RunID     string              `json:"run_id"`
	Artifacts []artifactsListItem `json:"artifacts"`
}

func (ro *Router) handleArtifactsList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	res, err := ro.mgr.Artifacts(id)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	artifacts := make([]artifactsListItem, 0, len(res))
	for _, ar := range res {
		artifacts = append(artifacts, artifactsListItem{
			Job:      ar.JobName,
			Artifact: ar.ArtifactName,
			Download: fmt.Sprintf("/api/v1/runs/%s/artifacts/%s/%s",
				id,
				ar.JobName,
				ar.ArtifactName,
			),
		})
	}

	httpx.WriteJSON(w, http.StatusOK, artifactsListResponse{
		RunID:     id,
		Artifacts: artifacts,
	})
}

func (ro *Router) handleArtifactsServe(w http.ResponseWriter, r *http.Request) {
    
}
