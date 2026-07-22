package engine

import (
	"context"
	"io"

	"github.com/nxrmqlly/jittrippin/pkg/runner"
)

func RunJob(ctx context.Context, r runner.Runner, job *Job, stdout, stderr io.Writer) error {
	exec, err := r.Create(ctx, runner.ExecutionCreateConfig{
		Image: job.Image,
		Env:   job.Env,
	})
	if err != nil {
		return err
	}
	defer exec.Remove(ctx)

	for _, step := range job.Steps {
		_, err := exec.Exec(ctx, runner.ExecConfig{
			Cmd:    step.Cmd,
			Stdout: stdout,
			Stderr: stderr,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
