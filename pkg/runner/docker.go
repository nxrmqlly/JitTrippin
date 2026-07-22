package runner

import (
	"context"
	"fmt"
	"io"
	"time"

	sdkContainer "github.com/docker/go-sdk/container"
	sdkClient "github.com/docker/go-sdk/client"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
)

type DockerRunner struct{
	client  sdkClient.SDKClient
}

type DockerExecution struct {
	container *sdkContainer.Container
}

func (r *DockerRunner) Create(ctx context.Context, cfg ExecutionCreateConfig) (Execution, error) {
	c, err := sdkContainer.Run(
		ctx,
		sdkContainer.WithImage(cfg.Image),
		sdkContainer.WithCmd("tail", "-f", "/dev/null"),
		sdkContainer.WithEnv(cfg.Env),
	)

	if err != nil {
		return nil, fmt.Errorf("unable to create container: %w", err)
	}

	return &DockerExecution{
		container: c,
	}, nil
}

type ExecConfig struct {
	Cmd    string
	Stdout io.Writer
	Stderr io.Writer
}

type ExecResult struct {
	ExitCode int
}

func (e *DockerExecution) Exec(ctx context.Context, cfg ExecConfig) (ExecResult, error) {
	c := e.container
	contClient := c.Client()

	// 0. create the exec defs
	ecRes, err := contClient.ExecCreate(ctx, c.ID(), client.ExecCreateOptions{
		User:         "root",
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"sh", "-c",cfg.Cmd},
	})
	if err != nil {
		return ExecResult{}, fmt.Errorf("error creating exec: %w", err)
	}

	// 1. attach and start an exec
	eaRes, err := contClient.ExecAttach(ctx, ecRes.ID, client.ExecAttachOptions{})
	if err != nil {
		return ExecResult{}, fmt.Errorf("error starting exec: %w", err)
	}
	defer eaRes.HijackedResponse.Close()
	output := eaRes.HijackedResponse.Reader

	// 2. drain output to stdout and stderr
	if _, err := stdcopy.StdCopy(cfg.Stdout, cfg.Stderr, output); err != nil {
		return ExecResult{}, err
	}

	// 3. inspect for final exit code
	eiRes, err := contClient.ExecInspect(ctx, ecRes.ID, client.ExecInspectOptions{})
	if err != nil {
		return ExecResult{}, fmt.Errorf("error inspecting exec: %w", err)
	}
	exitCode := eiRes.ExitCode

	finalResult := ExecResult{ExitCode: exitCode}

	if exitCode != 0 {
		return finalResult, fmt.Errorf("command exited with code %d", exitCode)
		// return fmt.Errorf("step %s failed with exit code %d", jobStepIdx, exitCode)
	}
	return finalResult, nil
}

func (e *DockerExecution) CopyIn(ctx context.Context, reader io.Reader, pathTo string) error
func (e *DockerExecution) CopyOut(ctx context.Context, writer io.Writer, pathFrom string) error

func (e *DockerExecution) Remove(ctx context.Context) error {
	cleanupCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		30*time.Second,
	)
	defer cancel()

	return e.container.Terminate(
		cleanupCtx,
		sdkContainer.TerminateTimeout(0),
	)
}
