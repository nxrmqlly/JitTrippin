package runner

import (
	"context"
	"io"
)

type ExecutionCreateConfig struct {
	Image string
	Env   map[string]string
}

type ExecConfig struct {
	Cmd    string
	Stdout io.Writer
	Stderr io.Writer
}

// It's possible to make do with just Docker concrete type but this
// is important for future implementations of backend, like Podman.
//
// Every backend must implement both Runner and Execution. Like a plugin
// system where Runner is the plugin and Execution is the real driver
type Runner interface {
	Create(ctx context.Context, cfg ExecutionCreateConfig) (Execution, error)
	WorkDir() string
}

// Execution is the environment where a Job is executed, it can be a docker
// container, a podman container, a VM or something else based on implementation.
type Execution interface {
	// Exec executes a shell command inside the environment
	Exec(ctx context.Context, cfg ExecConfig) (ExecResult, error)

	// CopyIn copies the contents of reader into pathTo inside the environment
	CopyIn(ctx context.Context, reader io.Reader, pathTo string) error

	// CopyOut returns the file contents from the container at pathFrom.
	// If err is non-nil, the returned ReadCloser is nil and must not be used.
	CopyOut(ctx context.Context, pathFrom string) (io.ReadCloser, error)

	// Remove stops and deletes an execution Environment
	Remove(ctx context.Context) error
}
