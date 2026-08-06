package artifact

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalStore struct {
	Root      string
	Extension string
}

func (s *LocalStore) path(r Ref) string {
	return filepath.Join(s.Root, r.RelativePath()+s.Extension)
}

func (s *LocalStore) Create(ctx context.Context, r Ref) (io.WriteCloser, error) {
	path := s.path(r)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("cannot create dest dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("cannot create file: %w", err)
	}

	return f, nil
}

func (s *LocalStore) Load(ctx context.Context, r Ref) (io.ReadCloser, error) {
	return os.Open(s.path(r))
}
