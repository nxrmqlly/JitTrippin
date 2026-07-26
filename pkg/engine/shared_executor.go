package engine

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"

	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/runner"
)

const DEFAULTPARALLEL = 12

type WorkItem struct {
	job *Job
	pe  *PipelineRuntime
}

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

func NewPipelineRuntime(parentCtx context.Context, p *Pipeline, stdout, stderr io.Writer) *PipelineRuntime {
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

type Executor struct {
	MaxParallel int
	Runner      runner.Runner

	queue chan WorkItem
	wg    sync.WaitGroup

	ArtifactStore artifact.Store
}

func NewExecutor(runner runner.Runner, maxParallel int, artifactStore artifact.Store) *Executor {
	e := &Executor{
		MaxParallel:   maxParallel,
		Runner:        runner,
		queue:         make(chan WorkItem),
		ArtifactStore: artifactStore,
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

	pe := NewPipelineRuntime(ctx, p, stdout, stderr)
	go pe.start(e.queue)

	return pe, nil
}

func (pe *PipelineRuntime) Stop() {
	pe.cancel()
	pe.Wait()
}
