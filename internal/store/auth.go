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

// DeviceCode is future
type DeviceCode struct {
	Code      string
	UserCode  string
	Status    string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type OAuthState struct {
	State      string
	DeviceCode *string
	Redirect   string
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

type AuthCode struct {
	CodeHash  string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
}

func (s *Store) CreateUser(ctx context.Context, usr *User) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO users (id, email, display_name)
        VALUES ($1, $2, $3)`,
		usr.ID, usr.Email, usr.DisplayName,
	)

	if err != nil {
		return fmt.Errorf("cannot create user %s: %w", usr.ID, err)
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

		if len(i.AccessToken) > 0 {
			pt, err := helpers.Decrypt(s.encKey, i.AccessToken)
			if err != nil {
				return nil, fmt.Errorf("cannot decrypt access token for identitiy %s: %w", i.ID, err)
			}
			i.AccessToken = pt
		}
		if len(i.RefreshToken) > 0 {
			pt, err := helpers.Decrypt(s.encKey, i.RefreshToken)
			if err != nil {
				return nil, fmt.Errorf("cannot decrypt refresh token for identitiy %s: %w", i.ID, err)
			}
			i.RefreshToken = pt
		}
		identities = append(identities, i)
	}

	return identities, rows.Err()
}

func (s *Store) UpdateIdentityTokens(ctx context.Context, provider, providerUserID, accessToken, refreshToken string, expiresAt *time.Time) error {
	encAccess, err := helpers.Encrypt(s.encKey, []byte(accessToken))
	if err != nil {
		return err
	}
	var encRefresh []byte
	if refreshToken != "" {
		encRefresh, err = helpers.Encrypt(s.encKey, []byte(refreshToken))
		if err != nil {
			return err
		}
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE user_identities
		SET access_token = $1, refresh_token = $2, token_expires_at = $3
		WHERE provider = $4 AND provider_user_id = $5
	`, encAccess, encRefresh, expiresAt, provider, providerUserID)

	if err != nil {
		return fmt.Errorf("cannot update identity tokens provider=%s user=%s: %w", provider, providerUserID, err)
	}
	return nil
}

// CreateSession creates a sessions database entry.
// CreatedAt timestamp is managed by database.
//
// Must specify: ID, UserID, TokenHash, ExpiresAt, UserAgent
//
// Do not specify, or specify zero/nil: CreatedAt, RevokedAt, LastUsedAt
func (s *Store) CreateSession(ctx context.Context, se *Session) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO sessions
		(id, user_id, token_hash, expires_at, user_agent)
        VALUES ($1, $2, $3, $4, $5)`,
		se.ID, se.UserID, se.TokenHash, se.ExpiresAt, se.UserAgent,
	)

	if err != nil {
		return fmt.Errorf("cannot create session %s: %w", se.ID, err)
	}

	return nil
}

func (s *Store) SessionByTokenHash(ctx context.Context, hash string) (*Session, error) {
	var se Session

	err := s.pool.QueryRow(ctx, `
        SELECT id, user_id, token_hash, created_at, expires_at, revoked_at, last_used_at, user_agent
        FROM sessions
        WHERE token_hash = $1
    `, hash).
		Scan(&se.ID, &se.UserID, &se.TokenHash, &se.CreatedAt, &se.ExpiresAt, &se.RevokedAt, &se.LastUsedAt, &se.UserAgent)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("cannot get session by hash %q, %w", hash, err)
	}

	return &se, nil
}

func (s *Store) RevokeSession(ctx context.Context, tokenHash string) error {
	tag, err := s.pool.Exec(ctx, `
       	UPDATE sessions
		SET revoked_at = now()
		WHERE token_hash = $1
		`, tokenHash)

	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("cannot revoke session with hash %s: %w", tokenHash, err)
	}

	return nil
}

func (s *Store) TouchSession(ctx context.Context, tokenHash string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE sessions
		SET last_used_at = now()
		WHERE token_hash = $1
		`, tokenHash)

	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("cannot revoke session with hash %s: %w", tokenHash, err)
	}

	return nil
}

// CreateOAuthState creates a new oauth_states database entry.
// CreatedAt timestamp is managed by database.
//
// Must specify: State, Redirect, ExpiresAt
//
// Do not specify, or specify zero/nil: DeviceCode, CreatedAt
func (s *Store) CreateOAuthState(ctx context.Context, oas *OAuthState) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO oauth_states
		(state, redirect, expires_at)
        VALUES ($1, $2, $3)`,
		oas.State, oas.Redirect, oas.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("cannot create OAuth State %s: %w", oas.State, err)
	}

	return nil
}

// ConsumeOAuthState deletes a oauth_states database entry,
// provided a state.
//
// Returns pointer to a OAuthState with State and Redirect populated
func (s *Store) ConsumeOAuthState(ctx context.Context, state string) (*OAuthState, error) {
	row := s.pool.QueryRow(ctx, `
		DELETE FROM oauth_states
		WHERE state = $1 AND expires_at > now()
		RETURNING state, redirect
	`, state)

	var oas OAuthState

	err := row.Scan(&oas.State, &oas.Redirect)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("cannot consume OAuth state %q, %w", state, err)
	}

	return &oas, nil
}

// CreateAuthCode creates a auth_codes database entry.
// CreatedAt timestamp is managed by database.
//
// Must specify: CodeHash, UserID, ExpiresAt
//
// Do not specify, or specify zero/nil: CreatedAt, UsedAt
func (s *Store) CreateAuthCode(ctx context.Context, ac *AuthCode) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO auth_codes
		(code_hash, user_id, expires_at)
        VALUES ($1, $2, $3)`,
		ac.CodeHash, ac.UserID, ac.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("cannot create OAuth State %s: %w", ac.CodeHash, err)
	}

	return nil
}

func (s *Store) ConsumeAuthCode(ctx context.Context, codeHash string) (*AuthCode, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE auth_codes
		SET used_at = now()
		WHERE code_hash = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING code_hash, user_id, expires_at, used_at
	`, codeHash)

	var ac AuthCode

	err := row.Scan(&ac.CodeHash, &ac.UserID, &ac.ExpiresAt, &ac.UsedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("cannot consume OAuth state %q, %w", codeHash, err)
	}

	return &ac, nil
}
