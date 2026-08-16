package store

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrRunNotFound      = errors.New("run not found")
	ErrRepoNotFound     = errors.New("repo not found")
	ErrIdentityNotFound = errors.New("identity not found")
)
