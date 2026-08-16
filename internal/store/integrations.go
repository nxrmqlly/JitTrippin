package store

import (
	"context"
	"encoding/json"
	"time"
)

type IntegrationInstallation struct {
	InstallationID string `json:"installation_id"`
	AccountLogin   string `json:"account_login"`
	AccountType    string `json:"account_type"`
}

type UserIntegration struct {
	ID               string                    `json:"id"`
	UserID           string                    `json:"user_id"`
	Provider         string                    `json:"provider"`
	ProviderInstance string                    `json:"provider_instance"`
	Status           string                    `json:"status"`
	Installations    []IntegrationInstallation `json:"installations"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
}

func (s *Store) UpsertIntegration(ctx context.Context, ui *UserIntegration) error {
	insts, err := json.Marshal(ui.Installations)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO user_integrations
			(id, user_id, provider, provider_instance, status, installations)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, provider, provider_instance) DO UPDATE SET
			status        = EXCLUDED.status,
			installations = EXCLUDED.installations,
			updated_at    = now()
	`, ui.ID, ui.UserID, ui.Provider, ui.ProviderInstance, ui.Status, insts)
	return err
}

func (s *Store) ListUserIntegrations(ctx context.Context, userID string) ([]UserIntegration, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, provider, provider_instance, status, installations, created_at, updated_at
		FROM user_integrations
		WHERE user_id = $1
		ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserIntegration
	for rows.Next() {
		var ui UserIntegration
		var insts []byte
		if err := rows.Scan(&ui.ID, &ui.UserID, &ui.Provider, &ui.ProviderInstance, &ui.Status, &insts, &ui.CreatedAt, &ui.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(insts, &ui.Installations)
		out = append(out, ui)
	}
	return out, rows.Err()
}

func (s *Store) DeleteIntegration(ctx context.Context, userID, provider string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM user_integrations WHERE user_id=$1 AND provider=$2`, userID, provider)
	return err
}
