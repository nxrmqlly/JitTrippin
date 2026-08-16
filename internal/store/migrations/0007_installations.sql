-- +goose Up
CREATE TABLE IF NOT EXISTS provider_installations (
    installation_id   TEXT NOT NULL,
    provider          TEXT NOT NULL,
    provider_instance TEXT NOT NULL DEFAULT '',
    account_login     TEXT NOT NULL,
    account_type      TEXT NOT NULL DEFAULT '',
    setup_action      TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (provider, provider_instance, installation_id)
);

CREATE INDEX provider_installations_account_idx
    ON provider_installations (provider, provider_instance, account_login);

-- +goose Down
DROP INDEX provider_installations_account_idx;
DROP TABLE provider_installations;
