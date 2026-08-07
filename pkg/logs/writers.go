package logs

import (
	"context"
	"fmt"
	"io"
)

type WritersStore struct {
	stdout io.Writer
	stderr io.Writer
}

func NewWritersStore(stdout, stderr io.Writer) *WritersStore {
	return &WritersStore{
		stdout: stdout, stderr: stderr,
	}
}

func (s *WritersStore) Open(ctx context.Context, ref Ref) (*JobLog, error) {

	lwo := NewLineWriter("stdout", func(l Line) error {
		_, err := fmt.Fprintf(s.stdout, "[%s] %s", l.Stream, l.Data)
		if err != nil {
			return err
		}
		return nil
	})
	lwe := NewLineWriter("stderr", func(l Line) error {
		_, err := fmt.Fprintf(s.stderr, "[%s] %s", l.Stream, l.Data)
		if err != nil {
			return err
		}
		return nil
	})

	return &JobLog{
		Stderr: lwe,
		Stdout: lwo,

		Close: func() error {
			err1 := lwe.Close()
			err2 := lwo.Close()
			if err1 != nil {
				return err1
			}
			if err2 != nil {
				return err2
			}
			return nil
		},
	}, nil
}

func (s *WritersStore) Read(ctx context.Context, ref Ref) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not supported")
}
