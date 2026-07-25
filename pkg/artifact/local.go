package artifact

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const extension = ".tar"

type LocalStore struct {
	Root string
}

func (s *LocalStore) path(k string, a string) string {
	return filepath.Join(s.Root, k, a+extension)
}

func (s *LocalStore) Save(ctx context.Context, key string, a Artifact, r io.Reader) error {
	path := s.path(key, a.Name)

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

func (s *LocalStore) Load(ctx context.Context, key string, artifactName string) (io.ReadCloser, error) {
	return os.Open(s.path(key, artifactName))
}
