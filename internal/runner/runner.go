package runner

import (
	"context"
	"fmt"
	"io"

	sdkContainer "github.com/docker/go-sdk/container"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
	"github.com/nxrmqlly/jittrippin/helpers"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
)

type JobRunner struct{}

func (jr *JobRunner) RunJob(ctx context.Context, job engine.Job, stdout io.Writer, stderr io.Writer) error {
	c, err := sdkContainer.Run(
		ctx,
		sdkContainer.WithImage(job.Image),
		sdkContainer.WithCmd("tail", "-f", "/dev/null"),
		sdkContainer.WithEnv(job.Env),
	)

	if err != nil {
		return fmt.Errorf("unable to create container for job: '%s': %w", job.Name, err)
	}

	defer c.Terminate(ctx, sdkContainer.TerminateTimeout(0))

	maxSteps := len(job.Steps)
	for idx, step := range job.Steps {
		// helper so I dont have to rewrite this all the time
		jobStepIdx := helpers.JobStepIndexMax(job.Name, step.Name, idx+1, maxSteps)

		// deprecated: is not correct and polls ExecInspect every 100ms. Also doesn't provide interface
		// for reading stdout/stderr ("output") in real time. This also risks hanging for long
		// commands that may take time to execute. The raw moby/moby/client implementation gives more
		// control.
		//
		// exitCode, output, err := c.Exec(
		// 	ctx,
		// 	[]string{"sh", "-c", step.Cmd},
		// )
		// if err != nil {
		// 	return fmt.Errorf("step %s failed: %w", jobStepIdx, err)
		// }

		contClient := c.Client()

		// 0. create the exec defs
		ecRes, err := contClient.ExecCreate(ctx, c.ID(), client.ExecCreateOptions{
			User:         "root",
			AttachStdout: true,
			AttachStderr: true,
			Cmd:          []string{"sh", "-c", step.Cmd},
		})
		if err != nil {
			return fmt.Errorf("error creating exec for step %s: %w", jobStepIdx, err)
		}

		// 1. attach and start an exec
		eaRes, err := contClient.ExecAttach(ctx, ecRes.ID, client.ExecAttachOptions{})
		if err != nil {
			return fmt.Errorf("error starting exec for step %s: %w", jobStepIdx, err)
		}
		output := eaRes.HijackedResponse.Reader

		// 2. drain output to stdout and stderr
		if _, err := stdcopy.StdCopy(stdout, stderr, output); err != nil {
			return fmt.Errorf("cannot return output stream for step %s: %w", jobStepIdx, err)
		}

		// 3. inspect for final exit code
		eiRes, err := contClient.ExecInspect(ctx, ecRes.ID, client.ExecInspectOptions{})
		if err != nil {
			return fmt.Errorf("error inspecting exec for step %s: %w", jobStepIdx, err)
		}
		exitCode := eiRes.ExitCode

		if exitCode != 0 {
			return fmt.Errorf("step %s failed with exit code %d", jobStepIdx, exitCode)
		}
		eaRes.HijackedResponse.Close()
	}

	return nil
}
