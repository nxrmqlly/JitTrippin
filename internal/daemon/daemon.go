package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/nxrmqlly/jittrippin/helpers"
	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
)

type Status int

const (
	StatusRunning = iota
	StatusFailed
	StatusSucceeded
)

type Manager struct {
	ctx context.Context

	exec *engine.Executor

	mu   sync.RWMutex
	runs map[string]*Run
}

func NewManager(ctx context.Context, e *engine.Executor) (*Manager, error) {
	return &Manager{
		ctx:  ctx,
		exec: e,
		runs: make(map[string]*Run),
	}, nil
}

// Add sets runs[run.id] to run
func (m *Manager) add(run *Run) {
	m.mu.Lock()
	m.runs[run.ID()] = run
	m.mu.Unlock()
}

func (m *Manager) Submit(p *engine.Pipeline, stdout, stderr io.Writer) (*Run, error) {
	pr, err := m.exec.Submit(
		m.ctx,
		helpers.MustUUIDV7(),
		p,
		stdout,
		stderr)
	if err != nil {
		return nil, err
	}

	run := NewRun(pr, stdout, stderr)
	m.add(run)

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

	return run, nil
}

type RunResult struct {
	// Unique UUIDv7
	ID         string     `json:"id"`
	Status     Status     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

func newRunResult(r *Run) *RunResult {
	return &RunResult{
		ID:         r.ID(),
		Status:     r.Status(),
		CreatedAt:  r.CreatedAt(),
		FinishedAt: r.FinishedAt(),
	}
}

// getRun is a helper function to get a Run from and ID,
// Run is not meant for public consumption, use RunResult instead.
func (m *Manager) getRun(id string) (*Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.runs[id]
	if !ok {
		return nil, fmt.Errorf("run '%s' not found", id)
	}
	return r, nil
}

// Get returns a RunResult and returns an error if the run is not found
func (m *Manager) Get(id string) (*RunResult, error) {
	r, err := m.getRun(id)
	if err != nil {
		return nil, fmt.Errorf("run with ID %q not found", id)
	}
	return newRunResult(r), nil
}

type ArtifactResult struct {
	JobName      string
	ArtifactName string
}

func (m *Manager) Artifacts(id string) ([]ArtifactResult, error) {
	run, err := m.getRun(id)
	if err != nil {
		return nil, err
	}

	var artifacts []ArtifactResult

	p := run.runtime.Pipeline()
	for _, j := range p.Jobs {
		for _, a := range j.Artifacts {
			artifacts = append(artifacts, ArtifactResult{
				JobName:      j.Name,
				ArtifactName: a.Name,
			})
		}
	}

	return artifacts, nil
}

var ErrArtifactNotFound = errors.New("artifact not found")

func (m *Manager) GetArtifact(ctx context.Context, id, jobName, artifactName string) (io.ReadCloser, error) {
	// check if run exists
	_, err := m.getRun(id)
	if err != nil {
		return nil, err
	}

	r, err := m.exec.ArtifactStore.Load(
		ctx,
		artifact.ArtifactRef{
			RunID:        id,
			JobName:      jobName,
			ArtifactName: artifactName,
		},
	)
	if err != nil {
		return nil, ErrArtifactNotFound
	}
	return r, nil
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

type Run struct {
	mu sync.RWMutex

	runtime *engine.PipelineRuntime

	status     Status
	createdAt  time.Time
	finishedAt *time.Time

	stdout io.Writer
	stderr io.Writer
}

func NewRun(pr *engine.PipelineRuntime, stdout, stderr io.Writer) *Run {
	return &Run{
		runtime:   pr,
		createdAt: time.Now(),
		status:    StatusRunning,
		stdout:    stdout,
		stderr:    stderr,
	}
}

func (r *Run) ID() string {
	return r.runtime.ID()
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
