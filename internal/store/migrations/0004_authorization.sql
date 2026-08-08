-- +goose Up
ALTER TABLE runs 
ADD COLUMN owner_id UUID NOT NULL
REFERENCES users(id);

CREATE INDEX runs_owner_id_idx
ON runs(owner_id);

-- +goose Down 
DROP INDEX runs_owner_id_idx;

ALTER TABLE runs
DROP COLUMN owner_id;