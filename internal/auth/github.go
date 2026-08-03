package auth

import (
	"context"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type GithubConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type Github struct {
	cfg *oauth2.Config
}

func NewGithub(cfg GithubConfig) *Github {
	return &Github{cfg: &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,

		Endpoint: github.Endpoint,

		Scopes: []string{
			"read:user",
			"read:email",
		},
	}}
}

func (g *Github) Name() string {
	return "github"
}

func (g *Github) AuthURL(state string, opts ...oauth2.AuthCodeOption) string {
	return g.cfg.AuthCodeURL(state, opts...)
}

func (g *Github) Exchange(ctx context.Context, code string) (*Identity, error) {
	// tok, err := g.cfg.Exchange(ctx, code)
	// if err != nil {
	// 	return nil, err
	// }
	return nil, nil
}
