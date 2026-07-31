package server

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/checkout"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	"github.com/nxrmqlly/jittrippin/pkg/runner"

	"github.com/google/uuid"
)

type Run struct {
	mu sync.RWMutex

	// Unique UUIDv7
	id      string
	runtime *engine.PipelineRuntime

	status     Status
	createdAt  time.Time
	finishedAt *time.Time

	stdout io.Writer
	stderr io.Writer
}

func NewRun(pr *engine.PipelineRuntime, stdout, stderr io.Writer) *Run {
	return &Run{
		id:        uuid.Must(uuid.NewV7()).String(),
		runtime:   pr,
		createdAt: time.Now(),
		status:    StatusRunning,
		stdout:    stdout,
		stderr:    stderr,
	}
}

func (r *Run) ID() string {
	return r.id
}

func (r *Run) Status() Status {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.status
}

func (r *Run) SetStatus(s Status) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.status = s
}

func (r *Run) CreatedAt() time.Time {
	return r.createdAt
}

func (r *Run) FinishedAt() *time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.finishedAt
}

func (r *Run) SetFinishedAt(t *time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.finishedAt = t
}

type Manager struct {
	mu sync.RWMutex

	exec *engine.Executor
	runs map[string]*Run
}

func NewManager() (*Manager, error) {
	r, err := runner.NewDockerRunner()
	if err != nil {
		return nil, err
	}
	store := &artifact.LocalStore{
		Root: ".jt-pipelines",
	}
	c := &checkout.GitCheckouter{}

	e := engine.NewExecutor(engine.ExecutorConfig{
		MaxParallel: 0,

		Runner:        r,
		ArtifactStore: store,
		Checkouter:    c,
	})

	return &Manager{
		exec: e,
		runs: make(map[string]*Run),
	}, nil
}

func (m *Manager) Submit(ctx context.Context, p *engine.Pipeline, stdout, stderr io.Writer) (*Run, error) {
	pr, err := m.exec.Submit(ctx, p, stdout, stderr)
	if err != nil {
		return nil, err
	}

	run := NewRun(pr, stdout, stderr)

	go func() {
		err := pr.Wait()

		if err != nil {
			run.SetStatus(StatusFailed)
		} else {
			run.SetStatus(StatusSucceeded)
		}

		now := time.Now()
		run.SetFinishedAt(&now)
	}()

	m.mu.Lock()
	m.runs[run.id] = run
	m.mu.Unlock()

	return run, nil
}

type RunResult struct {
	// Unique UUIDv7
	ID         string
	Status     Status
	CreatedAt  time.Time
	FinishedAt *time.Time
}

func newRunResult(r *Run) *RunResult {
	return &RunResult{
		ID:         r.ID(),
		Status:     r.Status(),
		CreatedAt:  r.CreatedAt(),
		FinishedAt: r.FinishedAt(),
	}
}

func (m *Manager) Get(id string) (*RunResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if r, ok := m.runs[id]; ok {
		return newRunResult(r), nil
	}
	return nil, fmt.Errorf("run with ID %q not found", id)

}

func (m *Manager) List() []*RunResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rs []*RunResult
	for _, r := range m.runs {
		rs = append(rs, newRunResult(r))
	}
	return rs
}

func (m *Manager) Cancel(id string) error {
	m.mu.RLock()
	r, ok := m.runs[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("run %q not found", id)
	}
	r.runtime.Stop()
	return nil
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	r, ok := m.runs[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("run %q not found", id)
	}
	delete(m.runs, id)
	m.mu.Unlock()

	r.runtime.Stop()
	return nil
}

func (m *Manager) Teardown() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.runs))
	for id := range m.runs {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		_ = m.Delete(id)
	}
}
