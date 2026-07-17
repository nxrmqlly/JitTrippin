package engine

import (
	"context"
	"io"
	"runtime"
	"sync"
)

const DEFAULTPARALLEL = 4

type Engine struct {
	Runner      Runner
	Stdout      io.Writer
	Stderr      io.Writer
	MaxParallel int
}

type JobResult struct {
	job *Job
	err error
}

func (e *Engine) maxParallel() int {
	if e.MaxParallel > 0 {
		return e.MaxParallel
	}

	n := runtime.NumCPU()

	if n == 0 {
		return DEFAULTPARALLEL
	}

	return min(n, DEFAULTPARALLEL)

}

func (e *Engine) worker(ctx context.Context, jobs <-chan *Job, results chan<- JobResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		if err := ctx.Err(); err != nil {
			results <- JobResult{job: job, err: err}
			continue
		}

		err := e.Runner.RunJob(ctx, *job, e.Stdout, e.Stderr)
		results <- JobResult{job: job, err: err}
	}
}

// execute schedules and runs all jobs in a pipeline
func (e *Engine) execute(ctx context.Context, p *Pipeline) error {
	// Internally: we use Kahn's Algorithm to schedule jobs in a pipeline
	// based on dependencies. We execute all parent jobs (on which other
	// jobs depend on) and then execute the next set of dependent
	// processes and so on until all of them execute completely or return
	// an error.
	jobMap := make(map[string]*Job, len(p.Jobs))
	indegree := make(map[string]int, len(p.Jobs))
	children := make(map[string][]string)

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

	n := len(p.Jobs)
	jobs := make(chan *Job, n)
	results := make(chan JobResult, n)

	ctx, cancel := context.WithCancel(ctx) // (ctx) is parent ctx :P
	defer cancel()

	var wg sync.WaitGroup

	for range e.maxParallel() {
		wg.Add(1)
		go e.worker(ctx, jobs, results, &wg)
	}

	dispatch := func(job *Job) {
		jobs <- job
	}

	dispatched := 0
	for i := range p.Jobs {
		job := &p.Jobs[i]
		if indegree[job.Name] == 0 {
			dispatch(job)
			dispatched++
		}
	}

	var firstErr error = nil
	processed := 0

	for processed < dispatched {
		res := <-results
		processed++

		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
				cancel() // unblocks fast exit; stop queued jobs from starting
			}
			continue
		}

		// if any error appears, just drain the rest of the results
		if firstErr != nil {
			continue
		}

		for _, child := range children[res.job.Name] {
			indegree[child]--

			if indegree[child] == 0 {
				// jobs <- jobMap[child]
				dispatch(jobMap[child])
				dispatched++
			}
		}
	}

	close(jobs)
	wg.Wait()

	return firstErr

	// ===== LINEAR SCHEDULER =====

	// ready := []*Job{}

	// ? QOL: we dont use this as this is randomly ordered
	// ? It's just nicer for CI/CD pipelines
	// for jobName, deg := range indegree

	// for i := range p.Jobs {
	// 	job := &p.Jobs[i]
	// 	if indegree[job.Name] == 0 {
	// 		ready = append(ready, job)
	// 	}
	// }

	// for len(ready) != 0 {
	// 	if next, ok := helpers.PopBack(&ready); ok {
	// 		if err := e.Runner.RunJob(ctx, *next, e.Stdout, e.Stderr); err != nil {
	// 			return err
	// 		}

	// 		processed++

	// 		for _, child := range children[next.Name] {
	// 			if indegree[child]--; indegree[child] == 0 {
	// 				ready = append(ready, jobMap[child])
	// 			}
	// 		}
	// 	}
	// }

	// if processed != len(p.Jobs) {
	// 	panic("scheduler bug: pipeline was not a DAG")
	// }
}

// Run validates the pipeline and executes all jobs in a pipeline
func (e *Engine) Run(ctx context.Context, p *Pipeline) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if err := e.execute(ctx, p); err != nil {
		return err
	}
	return nil
}
