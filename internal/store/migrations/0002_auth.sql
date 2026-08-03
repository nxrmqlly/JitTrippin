-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id           UUID PRIMARY KEY,
    email        TEXT UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_identities (
    id               UUID PRIMARY KEY,
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    provider_login   TEXT NOT NULL DEFAULT '',
    provider_email   TEXT,
    access_token     BYTEA,
    refresh_token    BYTEA,
    token_expires_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id)
);
CREATE INDEX user_identities_user_id_idx ON user_identities (user_id);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID      PRIMARY KEY,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    user_agent   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

-- +goose Down
DROP TABLE sessions;
DROP TABLE user_identities;
DROP TABLE users;