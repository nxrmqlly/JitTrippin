package engine

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/checkout"
	"github.com/nxrmqlly/jittrippin/pkg/logs"
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
//
// PipelineRuntime is not meant to be mutated, use getters.
type PipelineRuntime struct {
	// ID is daemon metadata, has no significance for engine
	// It is an unique UUIDv7
	id string

	pipeline  *Pipeline
	scheduler *Scheduler
	results   chan JobResult
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	err       error
}

func newPipelineRuntime(parentCtx context.Context, id string, p *Pipeline) *PipelineRuntime {
	ctx, cancel := context.WithCancel(parentCtx)

	return &PipelineRuntime{
		id:        id,
		pipeline:  p,
		scheduler: NewScheduler(p),
		results:   make(chan JobResult, len(p.Jobs)),
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}
}

// ID returns the ID associaated with the PipelineRuntime
func (pe *PipelineRuntime) ID() string {
	return pe.id
}

// Pipeline returns the Pipeline associated with the PipelineRuntime
func (pe *PipelineRuntime) Pipeline() *Pipeline {
	return pe.pipeline
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
	LogsStore     logs.Store

	queue chan WorkItem
	wg    sync.WaitGroup
}

type ExecutorConfig struct {
	// MaxParallel should be set to -1 for automatic detection
	MaxParallel int

	Runner        runner.Runner
	ArtifactStore artifact.Store
	Checkouter    checkout.Checkouter
	LogsStore     logs.Store
}

func NewExecutor(cfg ExecutorConfig) *Executor {
	e := &Executor{
		MaxParallel:   cfg.MaxParallel,
		Runner:        cfg.Runner,
		ArtifactStore: cfg.ArtifactStore,
		LogsStore:     cfg.LogsStore,
		Checkouter:    cfg.Checkouter,
		queue:         make(chan WorkItem),
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

		lref := logs.Ref{
			RunID: work.pe.ID(),
			Job:   work.job.Name,
		}

		jl, err := e.LogsStore.Open(work.pe.ctx, lref)
		
		if err != nil {
			work.pe.results <- JobResult{
				job: work.job,
				err: err,
			}
			continue
		}

		err = ExecuteJob(work.pe.ctx, ExecuteJobConfig{
			runner:        e.Runner,
			job:           work.job,
			stdout:        jl.Stdout,
			stderr:        jl.Stderr,
			artifactStore: e.ArtifactStore,
			runtime:       work.pe,
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

// Submit validates a pipeline, then adds a pipeline to worker queue.
// Returns ValidationErrors if validation fails
func (e *Executor) Submit(ctx context.Context, id string, p *Pipeline) (*PipelineRuntime, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	pe := newPipelineRuntime(ctx, id, p)
	go pe.start(e.queue)

	return pe, nil
}
