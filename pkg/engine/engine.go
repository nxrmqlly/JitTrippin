package engine

import (
	"context"
	"io"

	"github.com/nxrmqlly/jittrippin/helpers"
)

type Engine struct {
	Runner Runner
	Stdout io.Writer
	Stderr io.Writer
}

func (e *Engine) execute(ctx context.Context, p *Pipeline) error {
	jobMap := make(map[string]*Job, len(p.Jobs))
	indegree := make(map[string]int, len(p.Jobs))
	children := make(map[string][]string)

	processed := 0

	for i := range p.Jobs {
		job := &p.Jobs[i]

		jobMap[job.Name] = job
		indegree[job.Name] = len(job.DependsOn)

		for _, dep := range job.DependsOn {
			// we dont have to worry about duplicates because our validator
			// handles that for us. yay!
			children[dep] = append(children[dep], job.Name)
		}
	}

	ready := []*Job{}

	// ? QOL: we dont use this as this is randomly ordered
	// ? It's just nicer for CI/CD pipelines
	// for jobName, deg := range indegree

	for i := range p.Jobs {
		job := &p.Jobs[i]
		if indegree[job.Name] == 0 {
			ready = append(ready, job)
		}
	}

	for len(ready) != 0 {
		if next, ok := helpers.PopBack(&ready); ok {
			if err := e.Runner.RunJob(ctx, *next, e.Stdout, e.Stderr); err != nil {
				return err
			}

			processed++

			for _, child := range children[next.Name] {
				if indegree[child]--; indegree[child] == 0 {
					ready = append(ready, jobMap[child])
				}
			}
		}
	}

	if processed != len(p.Jobs) {
		panic("scheduler bug: pipeline was not a DAG")
	}
	return nil
}

func (e *Engine) Run(ctx context.Context, p *Pipeline) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if err := e.execute(ctx, p); err != nil {
		return err
	}
	return nil
}
