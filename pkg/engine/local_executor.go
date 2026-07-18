package engine

import (
	"context"
	"io"
	"runtime"
	"sync"
)

const DEFAULTPARALLEL = 12


type LocalExecutor struct {
	Runner      Runner
	Stdout      io.Writer
	Stderr      io.Writer
	MaxParallel int
}

func (e *LocalExecutor) maxParallel() int {
	if e.MaxParallel > 0 {
		return e.MaxParallel
	}

	n := runtime.NumCPU()

	if n == 0 {
		return DEFAULTPARALLEL
	}

	return min(n, DEFAULTPARALLEL)

}

func (e *LocalExecutor) worker(ctx context.Context, jobs <-chan *Job, results chan<- JobResult, wg *sync.WaitGroup) {
	defer wg.Done()

	// worker keeps "polling" the channels
	for {
		select {
		case job, ok := <-jobs:
			if !ok {
				return // jobs channel died
			}

			// just in case: so that cancellation stays deterministic
			// while also getting fast responsive wait from select
			if err := ctx.Err(); err != nil {
				results <- JobResult{job: job, err: err}
				continue
			}
			err := e.Runner.RunJob(ctx, job, e.Stdout, e.Stderr)
			results <- JobResult{job: job, err: err}

		case <-ctx.Done():
			// context cancelled
			return
		}
	}
}

func (e *LocalExecutor) execute(ctx context.Context, p *Pipeline) error {
	scheduler := NewScheduler(p)
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

	for _, job := range scheduler.Ready() {
		jobs <- job
	}

	var firstErr error
	for !scheduler.Done() {
		res := <-results

		if res.err != nil {
			scheduler.Fail(res.job.Name)

			if firstErr == nil {
				firstErr = res.err
				cancel() // unblocks fast exit; stop queued jobs from starting
			}
			continue
		}

		// if any error appears, just drain the rest of the results
		if firstErr != nil {
			scheduler.Fail(res.job.Name)
			continue
		}

		// mark job as complete + get next
		nextJobs := scheduler.Complete(res.job.Name)
		for _, job := range nextJobs {
			jobs <- job
		}
	}

	close(jobs)
	wg.Wait()

	return firstErr
}

func (e *LocalExecutor) Run(ctx context.Context, p *Pipeline) error {
	if err := p.Validate(); err != nil {
		return err
	}
    
    if err := e.execute(ctx, p); err != nil {
        return err
    }

	return nil
}
