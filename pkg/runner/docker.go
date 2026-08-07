package runner

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	// sdkContainer "github.com/docker/go-sdk/container"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type DockerRunner struct {
	client *client.Client
}

func NewDockerRunner() (*DockerRunner, error) {
	apiClient, err := client.New(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("unable to create docker client: %w", err)
	}
	return &DockerRunner{
		client: apiClient,
	}, nil
}

func envStrings(m map[string]string) []string {
	var e []string
	for k, v := range m {
		e = append(e, k+"="+v)
	}
	return e
}

func (r *DockerRunner) WorkDir() string {
	return "/workspace"
}

func (r *DockerRunner) Create(ctx context.Context, cfg ExecutionCreateConfig) (Execution, error) {
	wd := r.WorkDir()

	resp, err := r.client.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image:      cfg.Image,
			Cmd:        []string{"tail", "-f", "/dev/null"},
			Env:        envStrings(cfg.Env),
			WorkingDir: wd,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("unable to create container: %w", err)
	}

	if _, err := r.client.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		_, err := r.client.ContainerRemove(ctx, resp.ID, client.ContainerRemoveOptions{})
		if err != nil {
			return nil, fmt.Errorf("unable to remove failed container: %w", err)
		}
		return nil, fmt.Errorf("unable to start container: %w", err)
	}

	return &DockerExecution{
		client:      r.client,
		containerID: resp.ID,
		workDir:     wd,
	}, nil
}

type DockerExecution struct {
	client      *client.Client
	containerID string
	workDir     string
}


type ExecResult struct {
	ExitCode int
}

func (e *DockerExecution) Exec(ctx context.Context, cfg ExecConfig) (ExecResult, error) {
	// 0. create the exec defs
	ecRes, err := e.client.ExecCreate(ctx, e.containerID, client.ExecCreateOptions{
		User:         "root",
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"sh", "-c", cfg.Cmd},
	})
	if err != nil {
		return ExecResult{}, fmt.Errorf("error creating exec: %w", err)
	}

	// 1. attach and start an exec
	eaRes, err := e.client.ExecAttach(ctx, ecRes.ID, client.ExecAttachOptions{})
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
	eiRes, err := e.client.ExecInspect(ctx, ecRes.ID, client.ExecInspectOptions{})
	if err != nil {
		return ExecResult{}, fmt.Errorf("error inspecting exec: %w", err)
	}
	exitCode := eiRes.ExitCode

	if exitCode != 0 {
		return ExecResult{ExitCode: exitCode}, fmt.Errorf("command exited with code %d", exitCode)
	}
	return ExecResult{ExitCode: exitCode}, nil
}

func (e *DockerExecution) CopyIn(ctx context.Context, reader io.Reader, pathTo string) error {
	if !filepath.IsAbs(pathTo) {
		pathTo = filepath.Join(e.workDir, pathTo)
	}

	parent := filepath.Dir(pathTo)
	if _, err := e.Exec(ctx, ExecConfig{
		Cmd:    "mkdir -p " + parent,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}); err != nil {
		return fmt.Errorf("cannot create parent dir: %w", err)
	}

	_, err := e.client.CopyToContainer(ctx, e.containerID, client.CopyToContainerOptions{
		DestinationPath: parent,
		Content:         reader,
	})
	return err
}

func (e *DockerExecution) CopyOut(ctx context.Context, pathFrom string) (io.ReadCloser, error) {
	if !filepath.IsAbs(pathFrom) {
		pathFrom = filepath.Join(e.workDir, pathFrom)
	}

	res, err := e.client.CopyFromContainer(ctx, e.containerID, client.CopyFromContainerOptions{
		SourcePath: pathFrom,
	})
	if err != nil {
		return nil, err
	}

	return res.Content, nil
}

func (e *DockerExecution) Remove(ctx context.Context) error {
	cleanupCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		30*time.Second,
	)
	defer cancel()

	_, err := e.client.ContainerRemove(cleanupCtx, e.containerID, client.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	return err
}
