package artifact

import (
	"context"
	"io"
)

type Artifact struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Store interface {
	Save(ctx context.Context, key string, a Artifact, r io.Reader) error
	Load(ctx context.Context, key string, artifactName string) (io.ReadCloser, error)
}
