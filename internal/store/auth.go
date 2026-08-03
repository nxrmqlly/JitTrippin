package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nxrmqlly/jittrippin/helpers"
)

type User struct {
	ID          string
	Email       *string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type UserIdentity struct {
	ID             string
	UserID         string
	Provider       string
	ProviderUserID string
	ProviderLogin  string
	ProviderEmail  *string
	AccessToken    []byte
	RefreshToken   []byte
	TokenExpiresAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Session struct {
	ID         string
	UserID     string
	TokenHash  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
	UserAgent  string
}


func (s *Store) CreateUser(ctx context.Context, usr *User) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO users (id, email, display_name, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5)`,
		usr.ID, usr.Email, usr.DisplayName, usr.CreatedAt, usr.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("cannot create user %s, %w", usr.ID, err)
	}

	return nil
}
func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	var u User

	err := s.pool.QueryRow(ctx, `
        SELECT id, email, display_name, created_at, updated_at
        FROM users
        WHERE id = $1
    `, id).Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("cannot get user %q, %w", id, err)
	}

	return &u, nil
}
func (s *Store) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	row := s.pool.QueryRow(ctx, `
        SELECT id, email, display_name, created_at, updated_at
        FROM users
        WHERE email = $1
    `, email)

	var u User

	err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("cannot get user with email %q, %w", email, err)
	}

	return &u, nil
}

func nilEmail(e string) *string {
	if e == "" {
		return nil
	} else {
		return &e
	}
}

func (s *Store) FindOrCreateIdentity(ctx context.Context, provider, providerUserID, providerLogin, providerEmail string) (*User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var uid string

	if providerEmail != "" {
		err := tx.QueryRow(ctx, `
            INSERT INTO users (id, email, display_name)
            VALUES ($1, $2, $3)
            ON CONFLICT (email) DO NOTHING
            RETURNING id
        `, helpers.MustUUIDV7(), providerEmail, providerLogin).
			Scan(&uid)

		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, providerEmail).Scan(&uid)
		}

		if err != nil {
			return nil, err
		}
	} else {
		err := tx.QueryRow(ctx, `
            INSERT INTO users (id, email, display_name)
            VALUES ($1, NULL, $2)
            RETURNING id
        `, helpers.MustUUIDV7(), providerLogin).Scan(&uid)

		if err != nil {
			return nil, err
		}
	}

	err = tx.QueryRow(ctx, `
        INSERT INTO user_identities
        (id, user_id, provider, provider_user_id, provider_login, provider_email)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (provider, provider_user_id) DO NOTHING
        RETURNING user_id
    `,
		helpers.MustUUIDV7(),
		uid,
		provider,
		providerUserID,
		providerLogin,
		nilEmail(providerEmail)).Scan(&uid)

	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			SELECT user_id FROM user_identities WHERE provider = $1 AND provider_user_id = $2
		`, provider, providerUserID).
			Scan(&uid)
	}

	if err != nil {
		return nil, err
	}

	var u User
	err = tx.QueryRow(ctx, `
        SELECT id, email, display_name, created_at, updated_at
        FROM users
        WHERE id = $1
    `, uid).Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &u, nil
}

func (s *Store) GetUserIdentities(ctx context.Context, userID string) ([]UserIdentity, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT 
            id,
            user_id,
            provider,
            provider_user_id,
            provider_login,
            provider_email,
            access_token,
            refresh_token,
            token_expires_at,
            created_at,
            updated_at
        FROM user_identities
        WHERE user_id = $1
        ORDER BY created_at   
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var identities []UserIdentity

	for rows.Next() {
		var i UserIdentity

		if err := rows.Scan(
			&i.ID,
			&i.UserID,
			&i.Provider,
			&i.ProviderUserID,
			&i.ProviderLogin,
			&i.ProviderEmail,
			&i.AccessToken,
			&i.RefreshToken,
			&i.TokenExpiresAt,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		identities = append(identities, i)
	}

	return identities, rows.Err()
}
