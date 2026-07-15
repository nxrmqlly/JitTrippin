package runner

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/go-sdk/container"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
)

type Runner interface {
	RunJob(ctx context.Context, job engine.Job, stdout io.Writer, stderr io.Writer) error
}

func RunJob(ctx context.Context, job engine.Job, stdout io.Writer, stderr io.Writer) error {
	cont, err := container.Run(
		ctx,
		container.WithImage(job.Image),
		container.WithCmd("tail", "-f", "/dev/null"),
		container.WithEnv(job.Env),
		// container.WithWaitStrategy(wait.ForExec())
	)

	if err != nil {
		return fmt.Errorf("unable to create container for job: '%s': %w", job.Name, err)
	}

	defer cont.Terminate(ctx)

	for idx, step := range job.Steps {
		// helper so I dont have to rewrite this all the time
		jobStepIdx := fmt.Sprintf("'%s/%s' (%d)", job.Name, step.Name, idx)

		exitCode, output, err := cont.Exec(
			ctx,
			[]string{"sh", "-c", step.Cmd},
		)
		if err != nil {
			return fmt.Errorf("step %s failed: %w", jobStepIdx, err)
		}

		// data, err := io.ReadAll(output)
		// if err != nil {
		// 	return fmt.Errorf("cannot read container output for step %s: %v", jobStepIdx, err)
		// }

		var writer io.Writer = stdout
		if exitCode != 0 {
			writer = stderr
		}

		if _, err := io.Copy(writer, output); err != nil {
			return fmt.Errorf("cannot return output stream for step %s: %w", jobStepIdx, err)
		}

		if exitCode != 0 {
			return fmt.Errorf("step %s failed with exit code %d", jobStepIdx, exitCode)
		}
	}

	return nil
}
