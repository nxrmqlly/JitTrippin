package api

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/nxrmqlly/jittrippin/internal/daemon"
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
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	res, err := ro.mgr.Artifacts(daemon.ArtifactsConfig{
		RunID:  id,
		UserID: usr.ID,
	})
	if err != nil {
		runError(w, err)
		return
	}

	artifacts := make([]artifactsListItem, 0, len(res))
	for _, ar := range res {
		artifacts = append(artifacts, artifactsListItem{
			Job:      ar.JobName,
			Artifact: ar.ArtifactName,
			Download: fmt.Sprintf("/api/v1/runs/%s/artifacts/%s/%s", id, ar.JobName, ar.ArtifactName),
		})
	}

	httpx.WriteJSON(w, http.StatusOK, artifactsListResponse{
		RunID:     id,
		Artifacts: artifacts,
	})
}

func (ro *Router) handleArtifactsServe(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	job := r.PathValue("job")
	artifact := r.PathValue("artifact")

	reader, err := ro.mgr.GetArtifact(r.Context(), daemon.GetArtifactConfig{
		RunID:        id,
		UserID:       usr.ID,
		JobName:      job,
		ArtifactName: artifact,
	})
	if err != nil {
		runError(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%s`, artifact+".tar"))
	w.Header().Set("Content-Type", "application/x-tar")

	if _, err := io.Copy(w, reader); err != nil {
		log.Printf("Artifact stream cut short for %s/%s/%s: %v", id, job, artifact, err)
	}
}
