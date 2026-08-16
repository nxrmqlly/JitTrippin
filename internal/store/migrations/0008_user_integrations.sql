-- +goose Up
CREATE TABLE IF NOT EXISTS user_integrations (
    id                UUID PRIMARY KEY,
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          TEXT NOT NULL,
    provider_instance TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'active',
    installations     JSONB NOT NULL DEFAULT '[]',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (user_id, provider, provider_instance)
);

CREATE INDEX user_integrations_user_idx ON user_integrations (user_id);

-- +goose Down
DROP TABLE user_integrations;