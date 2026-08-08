package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/nxrmqlly/jittrippin/helpers"
	"github.com/nxrmqlly/jittrippin/internal/store"
	"golang.org/x/oauth2"
)

var ErrProviderNotFound = errors.New("provider not found")

var ErrSessionRevoked = errors.New("session already revoked")
var ErrSessionExpired = errors.New("session has expired")


const (
	SessionLifetime    = 30 * 24 * time.Hour
	AuthCodeLifetime   = 5 * time.Minute
	OAuthStateLifetime = 10 * time.Minute
)

type Identity struct {
	Provider   string
	ProviderID string

	Name      string
	Email     string
	AvatarURL string
}

type Provider interface {
	Name() string

	AuthURL(state string, opts ...oauth2.AuthCodeOption) string

	Exchange(ctx context.Context, code string) (*Identity, error)
}

func randString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func newSessionToken() string {
	return "jt_se_" + randString(32)
}

func newAuthCode() string {
	return "jt_ac_" + randString(32)
}

func newState() string {
	return "jt_st_" + randString(32)
}

func hashToken(token string) string {
	b := []byte(token)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

type Service struct {
	store *store.Store

	providers map[string]Provider
}

func New(st *store.Store, providers ...Provider) *Service {
	pro := make(map[string]Provider)
	for _, p := range providers {
		pro[p.Name()] = p
	}
	return &Service{store: st, providers: pro}
}

type BeginOptions struct {
	Provider string
	Redirect string
}

type BeginResult struct {
	URL string
}

func (s *Service) Begin(ctx context.Context, opts BeginOptions) (BeginResult, error) {
	pro, ok := s.providers[opts.Provider]
	if !ok {
		return BeginResult{}, ErrProviderNotFound
	}

	state := newState()

	if err := s.store.CreateOAuthState(ctx, &store.OAuthState{
		State:     state,
		Redirect:  opts.Redirect,
		ExpiresAt: time.Now().Add(OAuthStateLifetime),
	}); err != nil {
		return BeginResult{}, err
	}
	url := pro.AuthURL(state)
	return BeginResult{URL: url}, nil
}

type CallbackOptions struct {
	Provider     string
	State        string
	ProviderCode string
}

type CallbackResult struct {
	Redirect string
	AuthCode string
}

func (s *Service) Callback(ctx context.Context, opts CallbackOptions) (CallbackResult, error) {
	pro, ok := s.providers[opts.Provider]
	if !ok {
		return CallbackResult{}, ErrProviderNotFound
	}

	oas, err := s.store.ConsumeOAuthState(ctx, opts.State)
	if err != nil {
		return CallbackResult{}, err
	}

	identity, err := pro.Exchange(ctx, opts.ProviderCode)
	if err != nil {
		return CallbackResult{}, err
	}

	user, err := s.store.FindOrCreateIdentity(
		ctx,
		pro.Name(),
		identity.ProviderID,
		identity.Name,
		identity.Email,
	)
	if err != nil {
		return CallbackResult{}, err
	}

	code := newAuthCode()
	if err := s.store.CreateAuthCode(ctx, &store.AuthCode{
		CodeHash:  hashToken(code),
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(AuthCodeLifetime),
	}); err != nil {
		return CallbackResult{}, err
	}

	return CallbackResult{
		Redirect: oas.Redirect,
		AuthCode: code,
	}, nil

}

type ExchangeOptions struct {
	AuthCode  string
	UserAgent string
}

type ExchangeResult struct {
	SessionToken string
}

func (s *Service) Exchange(ctx context.Context, opts ExchangeOptions) (ExchangeResult, error) {
	h := hashToken(opts.AuthCode)
	ac, err := s.store.ConsumeAuthCode(ctx, h)
	if err != nil {
		return ExchangeResult{}, err
	}

	seID := helpers.MustUUIDV7()
	seTK := newSessionToken()
	hash := hashToken(seTK)

	if err := s.store.CreateSession(ctx, &store.Session{
		ID:        seID,
		UserID:    ac.UserID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(SessionLifetime),
		UserAgent: opts.UserAgent,
	}); err != nil {
		return ExchangeResult{}, err
	}

	return ExchangeResult{SessionToken: seTK}, nil

}

func (s *Service) Authenticate(ctx context.Context, sessionToken string) (*store.User, error) {
	h := hashToken(sessionToken)

	se, err := s.store.SessionByTokenHash(ctx, h)
	if err != nil {
		return nil, err
	}
	if se.RevokedAt != nil {
		return nil, ErrSessionRevoked
	}
	// sess alr expired
	if se.ExpiresAt.Before(time.Now()) {
		return nil, ErrSessionExpired
	}

	usr, err := s.store.GetUserByID(ctx, se.UserID)
	if err != nil {
		return nil, err
	}

	// touch only after user is actually retreived
	if err := s.store.TouchSession(ctx, h); err != nil {
		return nil, err
	}

	return usr, nil
}

func (s *Service) Logout(ctx context.Context, sessionToken string) error {
	return s.store.RevokeSession(ctx, hashToken(sessionToken))
}
