package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/nxrmqlly/jittrippin/helpers"
	"github.com/nxrmqlly/jittrippin/internal/store"
	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
)

type Status string

const (
	StatusRunning   Status = "running"
	StatusFailed    Status = "failed"
	StatusSucceeded Status = "succeeded"
)

type Manager struct {
	ctx context.Context

	exec  *engine.Executor
	store *store.Store

	mu      sync.RWMutex
	running map[string]*Run
}

func NewManager(ctx context.Context, e *engine.Executor, st *store.Store) (*Manager, error) {
	return &Manager{
		ctx:     ctx,
		exec:    e,
		store:   st,
		running: make(map[string]*Run),
	}, nil
}

// Add sets running[run.id] to run
func (m *Manager) add(run *Run) {
	m.mu.Lock()
	m.running[run.ID()] = run
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

	if err := m.store.CreateRun(m.ctx, store.RunRecord{
		ID:        run.ID(),
		Status:    string(run.Status()),
		CreatedAt: run.CreatedAt(),
		Pipeline:  p,
	}); err != nil {
		pr.Stop() // stop if database fails
		log.Printf("failed to persist run %q: %v", run.ID(), err)
	}
	m.add(run)

	go func() {
		err := pr.Wait()

		var errStr *string
		if err != nil {
			run.SetStatus(StatusFailed)
			s := err.Error()
			errStr = &s
		} else {
			run.SetStatus(StatusSucceeded)
		}

		now := time.Now()
		run.SetFinishedAt(&now)

		m.mu.Lock()
		delete(m.running, run.ID())
		m.mu.Unlock()

		if err := m.store.UpdateRun(m.ctx, store.RunRecord{
			ID:         run.ID(),
			Status:     string(run.Status()),
			FinishedAt: run.FinishedAt(),
			Error:      errStr,
		}); err != nil {
			log.Printf("failed to persist completion of run %s: %v", run.ID(), err)
		}
	}()

	return run, nil
}

type RunResult struct {
	// Unique UUIDv7
	ID         string     `json:"id"`
	Status     Status     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Error      *string    `json:"error"`
}

// getRun is a helper function to get a Run from and ID,
// Run is not meant for public consumption, use RunResult instead.
func (m *Manager) getRun(id string) (*Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.running[id]
	if !ok {
		return nil, fmt.Errorf("run '%s' not found", id)
	}
	return r, nil
}

// Get returns a RunResult and returns an error if the run is not found
func (m *Manager) Get(id string) (*RunResult, error) {
	rec, err := m.store.GetRun(m.ctx, id)
	if err != nil {
		return nil, err
	}
	return &RunResult{
		ID:         rec.ID,
		Status:     Status(rec.Status),
		CreatedAt:  rec.CreatedAt,
		FinishedAt: rec.FinishedAt,
		Error:      rec.Error,
	}, nil
}

type ArtifactResult struct {
	JobName      string
	ArtifactName string
}

func (m *Manager) Artifacts(id string) ([]ArtifactResult, error) {
	rec, err := m.store.GetRun(m.ctx, id)
	if err != nil {
		return nil, err
	}

	var artifacts []ArtifactResult

	for _, j := range rec.Pipeline.Jobs {
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
	_, err := m.store.GetRun(m.ctx, id)
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

func (m *Manager) List() ([]*RunResult, error) {
	var rs []*RunResult
	recs, err := m.store.ListRuns(m.ctx, store.ListRunsConfig{})
	if err != nil {
		return nil, err
	}

	for _, r := range recs {
		rs = append(rs, &RunResult{
			ID:         r.ID,
			Status:     Status(r.Status),
			CreatedAt:  r.CreatedAt,
			FinishedAt: r.FinishedAt,
			Error:      r.Error,
		})
	}
	return rs, nil
}

func (m *Manager) Cancel(id string) error {
	m.mu.RLock()
	r, ok := m.running[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("run %q not found", id)
	}
	r.runtime.Stop()
	return nil
}

func (m *Manager) Delete(id string) error {
	if r, err := m.getRun(id); err == nil {
		r.runtime.Stop()

		m.mu.Lock()
		delete(m.running, id)
		m.mu.Unlock()
	}

	return m.store.DeleteRun(m.ctx, id)
}

func (m *Manager) Teardown() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.running))
	for id := range m.running {
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
