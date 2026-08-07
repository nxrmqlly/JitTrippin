package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/nxrmqlly/jittrippin/pkg/artifact"
)

type FileStore struct {
	artifacts artifact.Store
	broadcast *Broadcaster

	mu   sync.Mutex
	open map[Ref]io.Closer
}

// NewFileStore returns a pointer to a new FileStore.
//
// artifactStore is required but broadcaster may be nil, if there is no use for it.
func NewFileStore(artifactStore artifact.Store, broadcaster *Broadcaster) *FileStore {

	fmt.Printf("NewFileStore broadcaster=%p\n", broadcaster)

	return &FileStore{
		artifacts: artifactStore,
		broadcast: broadcaster,
		open:      make(map[Ref]io.Closer),
	}
}

func (s *FileStore) Open(ctx context.Context, ref Ref) (*JobLog, error) {
	w, err := s.artifacts.Create(ctx, artifact.Ref{
		RunID: ref.RunID,
		Path:  artifact.LogPath(ref.Job, "combined"),
	})
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.open[ref] = w
	s.mu.Unlock()

	enc := json.NewEncoder(w)

	emit := func(l Line) error {
		if err := enc.Encode(l); err != nil {
			return err
		}

		if s.broadcast != nil {
			s.broadcast.Publish(ref, l)
		}
		return nil
	}

	stdout := NewLineWriter("stdout", emit)
	stderr := NewLineWriter("stderr", emit)
	return &JobLog{
		Stderr: stderr,
		Stdout: stdout,
		Close: func() error {
			var ferr error
			if err := stdout.Close(); err != nil {
				ferr = err
			}
			if err := stderr.Close(); err != nil {
				ferr = err
			}

			s.mu.Lock()
			delete(s.open, ref)
			s.mu.Unlock()

			if err := w.Close(); err != nil {
				ferr = err
			}
			return ferr
		},
	}, nil
}

func (s *FileStore) Read(ctx context.Context, ref Ref) (io.ReadCloser, error) {
	return s.artifacts.Load(ctx, artifact.Ref{
		RunID: ref.RunID,
		Path:  artifact.LogPath(ref.Job, "combined.log"),
	})
}
