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

func ArtifactPath(jobName, artifactName string) string {
	return filepath.Join("artifacts", jobName, artifactName)
}

type LogPathResult struct {
	StdoutPath   string
	StderrPath   string
	CombinedPath string
}

func LogPath(jobName, logJob string) LogPathResult {
	return LogPathResult{
		StdoutPath:   filepath.Join("logs", jobName, "stdout.log"),
		StderrPath:   filepath.Join("logs", jobName, "stderr.log"),
		CombinedPath: filepath.Join("logs", jobName, "combined.log"),
	}
}

type Ref struct {
	RunID string

	// use LogPath or ArtifactPath
	Path  string
}

func (r Ref) RelativePath() string {
	return filepath.Join(r.RunID, r.Path)
}

type Store interface {
	Create(ctx context.Context, ref Ref) (io.WriteCloser, error)
	Load(ctx context.Context, ref Ref) (io.ReadCloser, error)
}
