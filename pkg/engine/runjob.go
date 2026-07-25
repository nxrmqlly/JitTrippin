package engine

import (
	"context"
	"fmt"
	"io"

	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/runner"
)

type ExecuteJobConfig struct {
	runner        runner.Runner
	job           *Job
	stdout        io.Writer
	stderr        io.Writer
	artifactStore artifact.Store
	pipeline      *Pipeline
}

func (p *Pipeline) LookupArtifact(jobName, artifactName string) (artifact.Artifact, error) {
	for _, job := range p.Jobs {
		if jobName != job.Name {
			continue
		}

		for _, a := range job.Artifacts {
			if a.Name == artifactName {
				return a, nil
			}
		}
		return artifact.Artifact{}, fmt.Errorf("job '%s' does not produce artifact '%s", jobName, artifactName)
	}
	return artifact.Artifact{}, fmt.Errorf("job '%s' does not exist", jobName)
}

func ExecuteJob(ctx context.Context, cfg ExecuteJobConfig) error {
	exec, err := cfg.runner.Create(ctx, runner.ExecutionCreateConfig{
		Image: cfg.job.Image,
		Env:   cfg.job.Env,
	})
	if err != nil {
		return err
	}

	defer exec.Remove(ctx)

	for _, dep := range cfg.job.DependsOn {
		for _, req := range dep.Requires {
			a, err := cfg.pipeline.LookupArtifact(dep.Job, req)
			if err != nil {
				return err
			}

			r, err := cfg.artifactStore.Load(ctx, dep.Job, req)
			if err != nil {
				return err
			}

			if err := exec.CopyIn(ctx, r, a.Path); err != nil {
				r.Close()
				return err
			}

			r.Close()
		}
	}

	for _, step := range cfg.job.Steps {
		_, err := exec.Exec(ctx, runner.ExecConfig{
			Cmd:    step.Cmd,
			Stdout: cfg.stdout,
			Stderr: cfg.stderr,
		})
		if err != nil {
			return err
		}
	}

	for _, a := range cfg.job.Artifacts {
		r, err := exec.CopyOut(ctx, a.Path)
		if err != nil {
			return err
		}

		if err := cfg.artifactStore.Save(ctx, cfg.job.Name, a, r); err != nil {
			return err
		}

		if err := r.Close(); err != nil {
			return err
		}
	}

	return nil
}
