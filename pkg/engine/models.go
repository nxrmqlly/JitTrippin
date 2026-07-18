package engine

import (
	"context"
	"io"
)

type Pipeline struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Jobs        []Job  `json:"jobs"`
}

type Job struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Steps     []Step            `json:"steps"`
	DependsOn []string          `json:"depends_on"`
	Env       map[string]string `json:"env"`
}

type Step struct {
	Name string `json:"name"`
	Cmd  string `json:"cmd"`
}

type Runner interface {
	RunJob(ctx context.Context, job Job, stdout io.Writer, stderr io.Writer) error
}
