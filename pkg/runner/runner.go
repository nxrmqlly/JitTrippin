package runner

import (
	"context"
	"io"
)

// It's possible to make do with just Docker concrete type but this
// is important for future implementations of backend, like Podman.
//
// Every backend must implement both Runner and Execution. Like a plugin
// system where Runner is the plugin and Execution is the real driver
type Runner interface {
	Create(ctx context.Context, config ExecutionCreateConfig) (Execution, error)
}

type Execution interface {
	Exec(ctx context.Context, cfg ExecConfig) (ExecResult, error)
	CopyIn(ctx context.Context, reader io.Reader, pathTo string) error
	CopyOut(ctx context.Context, writer io.Writer, pathFrom string) error
	Remove(ctx context.Context) error
}

type ExecutionCreateConfig struct {
	Image string
	Env   map[string]string
}
