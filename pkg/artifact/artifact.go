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
	RunID        string
	JobName      string
	ArtifactName string
}

func (ar ArtifactRef) RelativePath() string {
	return filepath.Join(ar.RunID, ar.JobName, ar.ArtifactName)
}

type Store interface {
	Save(ctx context.Context, r io.Reader, ar ArtifactRef) error
	Load(ctx context.Context, ar ArtifactRef) (io.ReadCloser, error)
}
