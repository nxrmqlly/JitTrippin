package daemon

import "errors"

var (
	// ErrForbidden        = errors.New("forbidden")
	// ErrRunNotFound      = errors.New("run not found")
	ErrArtifactNotFound = errors.New("artifact not found")
	ErrRunNotRunning    = errors.New("run not running")
)
