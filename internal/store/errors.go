package store

import "errors"

var (
	ErrNotFound    = errors.New("not found")
	ErrRunNotFound = errors.New("run not found")
)
