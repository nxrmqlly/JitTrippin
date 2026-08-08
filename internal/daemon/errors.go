package daemon

import "errors"

var (
	ErrForbidden        = errors.New("forbidden") // for mutation
	ErrArtifactNotFound = errors.New("artifact not found")
	ErrRunNotRunning    = errors.New("run not running")
)
