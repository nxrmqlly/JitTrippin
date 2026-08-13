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

type Store struct {
	pool   *pgxpool.Pool
	encKey []byte
}

func New(pool *pgxpool.Pool, encKey []byte) *Store {
	return &Store{pool: pool, encKey: encKey}
}

type RunRecord struct {
	ID         string
	OwnerID    string
	Status     string
	CreatedAt  time.Time
	FinishedAt *time.Time
	Error      *string
	Visibility string
	Pipeline   *engine.Pipeline
}

func (s *Store) CreateRun(ctx context.Context, r RunRecord) error {
	b, err := json.Marshal(r.Pipeline)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
			INSERT INTO runs (id, owner_id, status, created_at, pipeline, visibility)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, r.ID, r.OwnerID, r.Status, r.CreatedAt, b, r.Visibility)

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
		return ErrRunNotFound
	}

	return nil
}

func (s *Store) GetRunSecure(ctx context.Context, id, userID string) (*RunRecord, error) {
	row := s.pool.QueryRow(ctx, `
			SELECT id, owner_id, status, created_at, finished_at, error, pipeline
			FROM runs
			WHERE id = $1 AND (owner_id = $2 OR visibility = 'public')
		`, id, userID)

	var r RunRecord
	var raw []byte

	err := row.Scan(&r.ID, &r.OwnerID, &r.Status, &r.CreatedAt, &r.FinishedAt, &r.Error, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRunNotFound // missing or not visible to you
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

type ListRunsConfig struct {
	OwnerID string
}

func (s *Store) ListRuns(ctx context.Context, cfg ListRunsConfig) ([]RunRecord, error) {
	rows, err := s.pool.Query(ctx, `
			SELECT id, status, created_at, finished_at, error, pipeline, visibility
			FROM runs
			WHERE owner_id = $1
			ORDER BY created_at DESC
		`, cfg.OwnerID)
	if err != nil {
		return nil, fmt.Errorf("cannt list runs: %w", err)
	}
	var raw []byte
	var out []RunRecord
	for rows.Next() {
		var r RunRecord
		if err := rows.Scan(
			&r.ID, &r.Status, &r.CreatedAt, &r.FinishedAt, &r.Error, &raw, &r.Visibility,
		); err != nil {
			return nil, ErrRunNotFound
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
