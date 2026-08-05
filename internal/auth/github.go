package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

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

// takes Request.Body
func tryFindEmail(r io.Reader) (string, error) {
	type email struct {
		Email    string `json:"email"`
		Verified bool   `json:"verified"`
		Primary  bool   `json:"primary"`
	}

	var emails []email

	if err := json.NewDecoder(r).Decode(&emails); err != nil {
		return "", err
	}

	var final string

	for _, e := range emails {
		if e.Primary && e.Verified {
			final = e.Email
			break
		}
	}

	return final, nil
}

func (g *Github) Exchange(ctx context.Context, code string) (*Identity, error) {
	tok, err := g.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	client := g.cfg.Client(ctx, tok)
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)

	if err != nil {
		return nil, err
	}

	type userResponse struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
		Name      string `json:"name"`
		Email     string `json:"email"`
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api GET /user/emails returned %q", res.Status)
	}

	var ur userResponse

	if err := json.NewDecoder(res.Body).Decode(&ur); err != nil {
		return nil, err
	}

	if ur.Email == "" {
		req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
		if err != nil {
			return nil, err
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("github api GET /user/emails returned %q", res.Status)
		}

		email, err := tryFindEmail(res.Body)
		if err != nil {
			return nil, err
		}

		ur.Email = email
	}

	if ur.Name == "" {
		ur.Name = ur.Login
	}

	return &Identity{
		Provider:   g.Name(),
		ProviderID: strconv.FormatInt(ur.ID, 10),

		Name:      ur.Name,
		Email:     ur.Email,
		AvatarURL: ur.AvatarURL,
	}, nil
}
