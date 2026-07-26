package engine

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"

	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/checkout"
	"github.com/nxrmqlly/jittrippin/pkg/runner"
)

const DEFAULTPARALLEL = 12

type WorkItem struct {
	job *Job
	pe  *PipelineRuntime
}

type JobResult struct {
	job *Job
	err error
}

// PipelineRuntime is a state machine that asynchronously tracks
// the status of a job in a pipeline while it is running.
// It is safe to wait on by multiple goroutines
type PipelineRuntime struct {
	pipeline  *Pipeline
	scheduler *Scheduler
	results   chan JobResult
	stdout    io.Writer
	stderr    io.Writer
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	err       error
}

func newPipelineRuntime(parentCtx context.Context, p *Pipeline, stdout, stderr io.Writer) *PipelineRuntime {
	ctx, cancel := context.WithCancel(parentCtx)

	return &PipelineRuntime{
		pipeline:  p,
		scheduler: NewScheduler(p),
		results:   make(chan JobResult, len(p.Jobs)),
		stdout:    stdout,
		stderr:    stderr,
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}
}

// start drives the pipeline scheduler until completion..
// It must be called exactly once
func (pe *PipelineRuntime) start(queue chan<- WorkItem) {
	defer close(pe.done)
	defer func() {
		if r := recover(); r != nil {
			pe.err = fmt.Errorf("pipeline execution panic: %v", r)
		}
	}()

	for _, job := range pe.scheduler.Ready() {
		queue <- WorkItem{pe: pe, job: job}
	}

	var firstErr error
	for !pe.scheduler.Done() {
		res := <-pe.results

		if res.err != nil {
			pe.scheduler.Fail(res.job.Name)

			if firstErr == nil {
				firstErr = res.err
				pe.err = res.err

				pe.cancel() // unblocks fast exit; stop queued jobs from starting
			}
			continue
		}

		// if any error appears, just drain the rest of the results
		if firstErr != nil {
			pe.scheduler.Fail(res.job.Name)
			continue
		}

		// mark job as complete + get next
		nextJobs := pe.scheduler.Complete(res.job.Name)
		for _, job := range nextJobs {
			queue <- WorkItem{
				job: job,
				pe:  pe,
			}
		}
	}

}

func (pe *PipelineRuntime) Done() <-chan struct{} {
	return pe.done
}

func (pe *PipelineRuntime) Wait() error {
	<-pe.done
	return pe.err
}

func (pe *PipelineRuntime) Stop() {
	pe.cancel()
	pe.Wait()
}

type Executor struct {
	MaxParallel int

	Runner        runner.Runner
	Checkouter    checkout.Checkouter
	ArtifactStore artifact.Store

	queue chan WorkItem
	wg    sync.WaitGroup
}

type ExecutorConfig struct {
	// MaxParallel should be set to -1 for automatic detection
	MaxParallel int

	Runner        runner.Runner
	ArtifactStore artifact.Store
	Checkouter    checkout.Checkouter
}

func NewExecutor(cfg ExecutorConfig) *Executor {
	e := &Executor{
		MaxParallel:   cfg.MaxParallel,
		Runner:        cfg.Runner,
		ArtifactStore: cfg.ArtifactStore,
		queue:         make(chan WorkItem),
		Checkouter:    cfg.Checkouter,
	}
	e.spawnWorkers(e.maxParallel())

	return e
}

func (e *Executor) maxParallel() int {
	if e.MaxParallel > 0 {
		return e.MaxParallel
	}

	n := runtime.NumCPU()

	if n == 0 {
		return DEFAULTPARALLEL
	}

	return min(n, DEFAULTPARALLEL)

}

func (e *Executor) Shutdown() {
	close(e.queue)
	e.wg.Wait()
}

func (e *Executor) worker() {
	defer e.wg.Done()

	for work := range e.queue {
		if err := work.pe.ctx.Err(); err != nil {
			work.pe.results <- JobResult{
				job: work.job,
				err: err,
			}
			continue
		}

		err := ExecuteJob(work.pe.ctx, ExecuteJobConfig{
			runner:        e.Runner,
			job:           work.job,
			stdout:        work.pe.stdout,
			stderr:        work.pe.stderr,
			artifactStore: e.ArtifactStore,
			pipeline:      work.pe.pipeline,
			checkouter:    e.Checkouter,
		})
		work.pe.results <- JobResult{job: work.job, err: err}
	}
}

func (e *Executor) spawnWorkers(n int) {
	e.wg.Add(n)

	for range n {
		go e.worker()
	}
}

func (e *Executor) Submit(ctx context.Context, p *Pipeline, stdout, stderr io.Writer) (*PipelineRuntime, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	pe := newPipelineRuntime(ctx, p, stdout, stderr)
	go pe.start(e.queue)

	return pe, nil
}
