package store

import (
	"context"
	"time"
)

type Installation struct {
	ID               string
	Provider         string
	ProviderInstance string
	AccountLogin     string
	AccountType      string
	SetupAction      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (s *Store) UpsertInstallation(ctx context.Context, inst *Installation) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO provider_installations
		(installation_id, provider, provider_instance, account_login, account_type, setup_action)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (provider, provider_instance, installation_id) DO UPDATE SET
			account_login = EXCLUDED.account_login,
			account_type  = EXCLUDED.account_type,
			setup_action  = EXCLUDED.setup_action,
			updated_at    = now()

		`, inst.ID, inst.Provider, inst.ProviderInstance, inst.AccountLogin, inst.AccountType, inst.SetupAction)
	return err
}
