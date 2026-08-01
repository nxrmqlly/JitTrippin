package artifact

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalStore struct {
	Root string
	Extension string
}

func (s *LocalStore) path(a ArtifactRef) string {
	return filepath.Join(s.Root, a.RelativePath()+s.Extension)
}

func (s *LocalStore) Save(ctx context.Context, r io.Reader, ar ArtifactRef) error {
	path := s.path(ar)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("cannot create dest dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}

	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		os.Remove(path)
		return fmt.Errorf("cannot save file contents: %w", err)
	}

	return nil
}

func (s *LocalStore) Load(ctx context.Context, ar ArtifactRef) (io.ReadCloser, error) {
	return os.Open(s.path(ar))
}
