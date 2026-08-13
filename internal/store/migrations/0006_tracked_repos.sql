-- +goose Up
CREATE TABLE IF NOT EXISTS tracked_repositories (
    id                     UUID PRIMARY KEY,
    owner_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider               TEXT NOT NULL,
    provider_instance      TEXT NOT NULL DEFAULT '',
    provider_repository_id TEXT NOT NULL,
    full_name              TEXT NOT NULL,
    branch                 TEXT NOT NULL DEFAULT 'main',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (owner_id, provider, provider_instance, provider_repository_id)
);

CREATE INDEX tracked_repositories_owner_id_idx
    ON tracked_repositories (owner_id);

CREATE INDEX tracked_repositories_provider_repo_idx
    ON tracked_repositories (provider, provider_instance, provider_repository_id);


-- +goose Down
DROP INDEX tracked_repositories_provider_repo_idx;
DROP INDEX tracked_repositories_owner_id_idx;
DROP TABLE tracked_repositories;