package artifact

import (
	"context"
	"io"
	"path/filepath"
)

type Artifact struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type ArtifactRef struct {
	runID        string
	jobName      string
	artifactName string
}

func NewArtifactRef(runID, jobName, artifactName string) ArtifactRef {
	return ArtifactRef{
		runID:        runID,
		jobName:      jobName,
		artifactName: artifactName,
	}
}

func (ar ArtifactRef) RelativePath() string {
	return filepath.Join(ar.runID, ar.jobName, ar.artifactName)
}

type Store interface {
	Save(ctx context.Context, r io.Reader, ar ArtifactRef) error
	Load(ctx context.Context, ar ArtifactRef) (io.ReadCloser, error)
}
