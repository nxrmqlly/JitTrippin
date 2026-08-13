package daemon

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/nxrmqlly/jittrippin/helpers"
	"github.com/nxrmqlly/jittrippin/internal/store"
	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/checkout"
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

type ManagerConfig struct {
	Executor        *engine.Executor
	Store           *store.Store
	LogsBroadcaster *logs.Broadcaster
}

func NewManager(ctx context.Context, cfg ManagerConfig) (*Manager, error) {
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

func (m *Manager) Submit(ownerID string, pipeline *engine.Pipeline) (*Run, error) {
	if pipeline.Visibility == "" {
		pipeline.Visibility = "private"
	}

	pr, err := m.exec.Submit(
		m.ctx,
		helpers.MustUUIDV7(),
		pipeline,
	)

	if err != nil {
		return nil, err
	}

	run := NewRun(pr)
	if err := m.store.CreateRun(m.ctx, store.RunRecord{
		ID:         run.ID(),
		OwnerID:    ownerID,
		Status:     string(run.Status()),
		CreatedAt:  run.CreatedAt(),
		Pipeline:   pipeline,
		Visibility: pipeline.Visibility,
	}); err != nil {
		pr.Stop() // stop if database fails
		log.Printf("failed to persist run %q: %v", run.ID(), err)
	}
	m.add(run)

	go func() {
		defer close(run.done)
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

func loadPipelines(r io.Reader) ([]*engine.Pipeline, error) {
	tr := tar.NewReader(r)

	var pipelines []*engine.Pipeline

	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read checkout archive: %w", err)
		}

		if h.Typeflag != tar.TypeReg {
			continue
		}

		// Only .jt/*.json
		name := path.Clean(h.Name)
		if !strings.HasPrefix(name, "_example/") ||
			!strings.HasSuffix(name, ".json") {
			continue
		}

		var p engine.Pipeline
		if err := json.NewDecoder(tr).Decode(&p); err != nil {
			return nil, fmt.Errorf("parse pipeline %q: %w", name, err)
		}

		pipelines = append(pipelines, &p)
	}

	return pipelines, nil
}

func (m *Manager) ListTrackedRepositoriesByProviderRepo(provider string, instance string, provideRepoID string) ([]*store.TrackedRepository, error) {
	return m.store.GetTrackedRepositoriesByProviderRepo(m.ctx, provider, instance, provideRepoID)
}

func (m *Manager) SubmitRepository(tracked store.TrackedRepository, commit string) ([]*Run, error) {
	url := "https://" + tracked.ProviderInstance + "/" + tracked.FullName + ".git"

	checkout, err := m.exec.Checkouter.Checkout(m.ctx, checkout.Checkout{
		URL: url,
		Ref: commit,
	})
	if err != nil {
		return nil, err
	}
	defer checkout.Close()

	pipelines, err := loadPipelines(checkout)
	if err != nil {
		return nil, err
	}

	var runs []*Run
	for _, p := range pipelines {
		run, err := m.Submit(tracked.OwnerID, p)
		if err != nil {
			return runs, fmt.Errorf("submit pipeline: %w", err)
		}

		runs = append(runs, run)
	}

	return runs, nil
}

func (m *Manager) QueueSubmitRepository(tracked store.TrackedRepository, commit string) {
	go func() {
		if _, err := m.SubmitRepository(tracked, commit); err != nil {
			log.Printf("repo submit failed: repo=%s commit=%s: %s", tracked.ID, commit, err.Error())
		}
	}()
}

type TrackRepositoryConfig struct {
	OwnerID              string
	Provider             string
	ProviderInstance     string
	ProviderRepositoryID string
	FullName             string
	Branch               string
}

func (m *Manager) TrackRepository(cfg TrackRepositoryConfig) error {
	err := m.store.CreateTrackedRepository(m.ctx, &store.TrackedRepository{
		ID:                   helpers.MustUUIDV7(),
		OwnerID:              cfg.OwnerID,
		Provider:             cfg.Provider,
		ProviderInstance:     cfg.ProviderInstance,
		ProviderRepositoryID: cfg.ProviderRepositoryID,
		FullName:             cfg.FullName,
		Branch:               cfg.Branch,
	})
	if err != nil {
		return err
	}
	return nil
}

type Run struct {
	mu sync.RWMutex

	runtime *engine.PipelineRuntime

	status     Status
	createdAt  time.Time
	finishedAt *time.Time

	done chan struct{}
}

func NewRun(pr *engine.PipelineRuntime) *Run {
	return &Run{
		runtime:   pr,
		createdAt: time.Now(),
		status:    StatusRunning,
		done:      make(chan struct{}),
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

func (r *Run) Done() <-chan struct{} {
	return r.done
}
func (r *Run) Wait() {
	<-r.done
}
