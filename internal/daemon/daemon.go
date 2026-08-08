package daemon

import (
	"context"
	"io"
	"log"
	"sync"
	"time"

	"github.com/nxrmqlly/jittrippin/helpers"
	"github.com/nxrmqlly/jittrippin/internal/store"
	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	"github.com/nxrmqlly/jittrippin/pkg/logs"
)

type Status string

const (
	StatusRunning   Status = "running"
	StatusFailed    Status = "failed"
	StatusSucceeded Status = "succeeded"
)

type Manager struct {
	ctx context.Context

	exec            *engine.Executor
	store           *store.Store
	logsBroadcaster *logs.Broadcaster

	mu      sync.RWMutex
	running map[string]*Run
}

type NewManagerConfig struct {
	Executor        *engine.Executor
	Store           *store.Store
	LogsBroadcaster *logs.Broadcaster
}

func NewManager(ctx context.Context, cfg NewManagerConfig) (*Manager, error) {
	if err := cfg.Store.MarkInterruptedRuns(ctx); err != nil {
		return nil, err
	}

	return &Manager{
		ctx:             ctx,
		exec:            cfg.Executor,
		store:           cfg.Store,
		logsBroadcaster: cfg.LogsBroadcaster,
		running:         make(map[string]*Run),
	}, nil
}

// Add sets running[run.id] to run
func (m *Manager) add(run *Run) {
	m.mu.Lock()
	m.running[run.ID()] = run
	m.mu.Unlock()
}

type SubmitConfig struct {
	OwnerID  string
	Pipeline *engine.Pipeline
}

func (m *Manager) Submit(cfg SubmitConfig) (*Run, error) {
	if cfg.Pipeline.Visibility == "" {
		cfg.Pipeline.Visibility = "private"
	}

	pr, err := m.exec.Submit(
		m.ctx,
		helpers.MustUUIDV7(),
		cfg.Pipeline,
	)

	if err != nil {
		return nil, err
	}

	run := NewRun(pr)
	if err := m.store.CreateRun(m.ctx, store.RunRecord{
		ID:         run.ID(),
		OwnerID:    cfg.OwnerID,
		Status:     string(run.Status()),
		CreatedAt:  run.CreatedAt(),
		Pipeline:   cfg.Pipeline,
		Visibility: cfg.Pipeline.Visibility,
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

type GetConfig struct {
	RunID  string
	UserID string
}

// Get returns a RunResult and returns an error if the run is not found
func (m *Manager) Get(cfg GetConfig) (*RunResult, error) {
	rec, err := m.store.GetRunSecure(m.ctx, cfg.RunID, cfg.UserID)
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

type ArtifactsConfig struct {
	RunID  string
	UserID string
}

type ArtifactResult struct {
	JobName      string
	ArtifactName string
}

func (m *Manager) Artifacts(cfg ArtifactsConfig) ([]ArtifactResult, error) {
	rec, err := m.store.GetRunSecure(m.ctx, cfg.RunID, cfg.UserID)
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

type GetArtifactConfig struct {
	RunID        string
	UserID       string
	JobName      string
	ArtifactName string
}

func (m *Manager) GetArtifact(ctx context.Context, cfg GetArtifactConfig) (io.ReadCloser, error) {
	if _, err := m.store.GetRunSecure(m.ctx, cfg.RunID, cfg.UserID); err != nil {
		return nil, err
	}

	r, err := m.exec.ArtifactStore.Load(ctx, artifact.Ref{
		RunID: cfg.RunID,
		Path:  artifact.ArtifactPath(cfg.JobName, cfg.ArtifactName),
	})
	if err != nil {
		return nil, ErrArtifactNotFound
	}
	return r, nil
}

type GetLogsConfig struct {
	RunID    string
	UserID   string
	JobName  string
	Filename string
}

func (m *Manager) GetLogs(ctx context.Context, cfg GetLogsConfig) (io.ReadCloser, error) {
	if _, err := m.store.GetRunSecure(m.ctx, cfg.RunID, cfg.UserID); err != nil {
		return nil, err
	}
	return m.exec.ArtifactStore.Load(ctx, artifact.Ref{
		RunID: cfg.RunID,
		Path:  artifact.LogPath(cfg.JobName, cfg.Filename),
	})
}

type SubscribeLogsConfig struct {
	UserID string
	Ref    logs.Ref
}

func (m *Manager) SubscribeLogs(cfg SubscribeLogsConfig) (schan <-chan logs.Line, unsubscribe func(), err error) {
	if _, err := m.store.GetRunSecure(m.ctx, cfg.Ref.RunID, cfg.UserID); err != nil {
		return nil, nil, err
	}

	ch, unsub := m.logsBroadcaster.Subscribe(cfg.Ref)
	return ch, unsub, nil
}

type ListConfig struct {
	UserID string
}

func (m *Manager) List(cfg ListConfig) ([]*RunResult, error) {
	var rs []*RunResult
	recs, err := m.store.ListRuns(m.ctx, store.ListRunsConfig{
		OwnerID: cfg.UserID,
	})
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

type CancelConfig struct {
	RunID  string
	UserID string
}

func (m *Manager) Cancel(cfg CancelConfig) error {
	run, err := m.store.GetRunSecure(m.ctx, cfg.RunID, cfg.UserID)
	if err != nil {
		return err
	}

	if cfg.UserID != run.OwnerID {
		return ErrForbidden
	}

	m.mu.RLock()
	r, ok := m.running[cfg.RunID]
	m.mu.RUnlock()

	if !ok {
		return ErrRunNotRunning // 409
	}
	r.runtime.Stop()
	return nil
}

type Run struct {
	mu sync.RWMutex

	runtime *engine.PipelineRuntime

	status     Status
	createdAt  time.Time
	finishedAt *time.Time
}

func NewRun(pr *engine.PipelineRuntime) *Run {
	return &Run{
		runtime:   pr,
		createdAt: time.Now(),
		status:    StatusRunning,
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
