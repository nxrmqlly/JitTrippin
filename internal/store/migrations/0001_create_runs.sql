-- +goose Up
CREATE TABLE IF NOT EXISTS runs (
    id          UUID PRIMARY KEY,
    status      TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    pipeline    JSONB NOT NULL,
    error       TEXT
);

-- +goose Down 
DROP TABLE runs;