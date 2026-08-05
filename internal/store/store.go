package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
)

var ErrNotFound = errors.New("generic: not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type RunRecord struct {
	ID         string
	Status     string
	CreatedAt  time.Time
	FinishedAt *time.Time
	Error      *string
	Pipeline   *engine.Pipeline
}

func (s *Store) CreateRun(ctx context.Context, r RunRecord) error {
	b, err := json.Marshal(r.Pipeline)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
			INSERT INTO runs (id, status, created_at, pipeline)
			VALUES ($1, $2, $3, $4)
		`, r.ID, r.Status, r.CreatedAt, b)

	if err != nil {
		return fmt.Errorf("cannot create run %q, %w", r.ID, err)
	}

	return nil
}
func (s *Store) UpdateRun(ctx context.Context, r RunRecord) error {
	tag, err := s.pool.Exec(ctx, `
			UPDATE runs
			SET status = $2, finished_at = $3, error = $4
			WHERE id = $1
		`, r.ID, r.Status, r.FinishedAt, r.Error)
	if err != nil {
		return fmt.Errorf("cannot update run %q, %w", r.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
func (s *Store) GetRun(ctx context.Context, id string) (*RunRecord, error) {
	row := s.pool.QueryRow(ctx, `
			SELECT id, status, created_at, finished_at, error, pipeline
			FROM runs
			WHERE id = $1
		`, id)

	var r RunRecord
	var raw []byte

	err := row.Scan(&r.ID, &r.Status, &r.CreatedAt, &r.FinishedAt, &r.Error, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("cannot get run %q, %w", id, err)
	}

	var p engine.Pipeline
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("cannot process pipeline for run %q: %w", id, err)
	}
	r.Pipeline = &p
	return &r, nil
}

type ListRunsConfig struct{}

func (s *Store) ListRuns(ctx context.Context, cfg ListRunsConfig) ([]RunRecord, error) {
	rows, err := s.pool.Query(ctx, `
			SELECT id, status, created_at, finished_at, error, pipeline
			FROM runs
			ORDER BY created_at DESC
		`)
	if err != nil {
		return nil, fmt.Errorf("cannt list runs: %w", err)
	}
	var raw []byte
	var out []RunRecord
	for rows.Next() {
		var r RunRecord
		if err := rows.Scan(
			&r.ID, &r.Status, &r.CreatedAt, &r.FinishedAt, &r.Error, &raw,
		); err != nil {
			return nil, ErrNotFound
		}

		var p engine.Pipeline
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("cannot process pipeline for run %q: %w", r.ID, err)
		}
		r.Pipeline = &p

		out = append(out, r)
	}

	return out, rows.Err()
}
func (s *Store) DeleteRun(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `
			DELETE FROM runs
			WHERE id = $1
		`, id)
	if err != nil {
		return fmt.Errorf("cannot delete run %q: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MarkInterruptedRuns(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE runs
		SET status = 'failed', error = 'interrupted by daemon'
		WHERE status = 'running'
	`)
	if err != nil {
		return fmt.Errorf("cannot mark stale runs: %w", err)
	}
	return nil
}
