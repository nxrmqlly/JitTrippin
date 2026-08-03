package auth

import (
	"context"

	"golang.org/x/oauth2"
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
