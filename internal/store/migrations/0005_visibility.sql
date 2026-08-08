-- +goose Up
ALTER TABLE runs ADD COLUMN visibility TEXT NOT NULL DEFAULT 'private';

-- +goose Down
ALTER TABLE runs DROP COLUMN visibility;