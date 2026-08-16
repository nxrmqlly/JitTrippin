package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type TrackedRepository struct {
	ID                   string    `json:"id"`
	OwnerID              string    `json:"owner_id"`
	Provider             string    `json:"provider"`
	ProviderInstance     string    `json:"provider_instance"`
	ProviderRepositoryID string    `json:"provider_repository_id"`
	FullName             string    `json:"full_name"`
	Branch               string    `json:"branch"`
	CreatedAt            time.Time `json:"created_at"`
}

func (s *Store) CreateTrackedRepository(ctx context.Context, repo *TrackedRepository) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO tracked_repositories
		(id, owner_id, provider, provider_instance, provider_repository_id, full_name, branch)
        VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		repo.ID, repo.OwnerID, repo.Provider, repo.ProviderInstance, repo.ProviderRepositoryID, repo.FullName, repo.Branch,
	)

	if err != nil {
		return fmt.Errorf("cannot create tracked repo %q: %w", repo.ID, err)
	}

	return nil
}

func (s *Store) GetTrackedRepositoryByID(ctx context.Context, id string) (*TrackedRepository, error) {
	var repo TrackedRepository

	err := s.pool.QueryRow(ctx, `
        SELECT id, owner_id, provider, provider_instance, provider_repository_id, full_name, branch, created_at
        FROM tracked_repositories
        WHERE id = $1
    `, id).
		Scan(&repo.ID, &repo.OwnerID, &repo.Provider, &repo.ProviderInstance, &repo.ProviderRepositoryID, &repo.FullName, &repo.Branch, &repo.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRepoNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("cannot get tracked repo id=%q: %w", id, err)
	}

	return &repo, nil
}

func (s *Store) GetTrackedRepositoriesByProviderRepo(ctx context.Context, provider string, instance string, provideRepoID string) ([]*TrackedRepository, error) {
	rows, err := s.pool.Query(ctx, `
	SELECT id, owner_id, provider, provider_instance, provider_repository_id, full_name, branch, created_at
	FROM tracked_repositories
	WHERE provider_repository_id = $1 AND provider = $2 AND provider_instance = $3
    `, provideRepoID, provider, instance)
	if err != nil {
		return nil, fmt.Errorf("cannot get tracked repo: provider=%s<%s> id=%s: %w", provider, instance, provideRepoID, err)
	}
	defer rows.Close()

	var repos []*TrackedRepository

	for rows.Next() {
		var repo TrackedRepository
		if err := rows.Scan(
			&repo.ID,
			&repo.OwnerID,
			&repo.Provider,
			&repo.ProviderInstance,
			&repo.ProviderRepositoryID,
			&repo.FullName,
			&repo.Branch,
			&repo.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("cannot get tracked repo: provider=%s<%s> id=%s: %w", provider, instance, provideRepoID, err)
		}
		repos = append(repos, &repo)
	}
	return repos, rows.Err()
}

func (s *Store) ListTrackedRepositoriesByOwner(ctx context.Context, userID string) ([]*TrackedRepository, error) {
	rows, err := s.pool.Query(ctx, `
	SELECT id, owner_id, provider, provider_instance, provider_repository_id, full_name, branch, created_at
	FROM tracked_repositories
	WHERE owner_id = $1
    `, userID)
	if err != nil {
		return nil, fmt.Errorf("cannot get tracked repo: owner=%s: %w", userID, err)
	}
	defer rows.Close()

	var repos []*TrackedRepository

	for rows.Next() {
		var repo TrackedRepository
		if err := rows.Scan(
			&repo.ID,
			&repo.OwnerID,
			&repo.Provider,
			&repo.ProviderInstance,
			&repo.ProviderRepositoryID,
			&repo.FullName,
			&repo.Branch,
			&repo.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("cannot get tracked repo: owner=%s: %w", userID, err)
		}
		repos = append(repos, &repo)
	}
	return repos, rows.Err()
}

func (s *Store) DeleteTrackedRepository(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM tracked_repositories
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRepoNotFound
	}

	return err
}

func (s *Store) DeleteTrackedRepositoriesByProvider(ctx context.Context, userID, provider string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM tracked_repositories WHERE owner_id=$1 AND provider=$2`, userID, provider)
	return err
}
