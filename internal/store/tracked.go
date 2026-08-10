package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type TrackedRepository struct {
	ID             string
	OwnerID        string
	GithubRepoID   int64
	GithubFullName string
	Branch         string
	CreatedAt      time.Time
}

func (s *Store) CreateTrackedRepository(ctx context.Context, repo *TrackedRepository) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO tracked_repositories (id, owner_id, github_repository_id, github_full_name, branch)
        VALUES ($1, $2, $3, $4, $5)`,
		repo.ID, repo.OwnerID, repo.GithubRepoID, repo.GithubFullName, repo.Branch,
	)

	if err != nil {
		return fmt.Errorf("cannot create tracked repo %q: %w", repo.ID, err)
	}

	return nil
}

func (s *Store) GetTrackedRepositoryByID(ctx context.Context, id string) (*TrackedRepository, error) {
	var repo TrackedRepository

	err := s.pool.QueryRow(ctx, `
        SELECT id, owner_id, github_repository_id, github_full_name, branch, created_at
        FROM tracked_repositories
        WHERE id = $1
    `, id).Scan(&repo.ID, &repo.OwnerID, &repo.GithubRepoID, &repo.GithubFullName, &repo.Branch, &repo.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("cannot get tracked repo id=%q: %w", id, err)
	}

	return &repo, nil
}

func (s *Store) GetTrackedRepositoryByGithubID(ctx context.Context, githubID int64) (*TrackedRepository, error) {
	var repo TrackedRepository

	err := s.pool.QueryRow(ctx, `
        SELECT id, owner_id, github_repository_id, github_full_name, branch, created_at
        FROM tracked_repositories
        WHERE github_repository_id = $1
    `, githubID).Scan(&repo.ID, &repo.OwnerID, &repo.GithubRepoID, &repo.GithubFullName, &repo.Branch, &repo.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("cannot get tracked repo: gh id=%d: %w", githubID, err)
	}

	return &repo, nil
}
