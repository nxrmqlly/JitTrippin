-- +goose Up
CREATE TABLE IF NOT EXISTS tracked_repositories (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    github_repository_id BIGINT NOT NULL,
    github_full_name TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT 'main',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (owner_id, github_repository_id)
);

CREATE INDEX tracked_repositories_owner_id_idx
    ON tracked_repositories (owner_id);

CREATE UNIQUE INDEX tracked_repositories_github_repository_idx
    ON tracked_repositories (github_repository_id);

-- +goose Down

DROP INDEX tracked_repositories_github_repository_idx;
DROP INDEX tracked_repositories_owner_id_idx;
DROP TABLE tracked_repositories;