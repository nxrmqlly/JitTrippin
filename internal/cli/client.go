package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

type apiErr struct {
	Status int
	Msg    string
}

func (e apiErr) Error() string {
	return fmt.Sprintf("%s (%d)", e.Msg, e.Status)
}

type daemonClient struct {
	baseURL string
	http    *http.Client
	token   string
}

func newDaemonClient(baseURL string) *daemonClient {
	return &daemonClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *daemonClient) do(method, path string, body, out any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}

	req, err := http.NewRequest(method, c.baseURL+path, &buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var eb struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&eb)
		return apiErr{Status: resp.StatusCode, Msg: eb.Error}
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// Begin starts the authentication process with the daemon and returns the next URL
func (c *daemonClient) Begin(provider, redirect string) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	type beginBody struct {
		Provider string `json:"provider"`
		Redirect string `json:"redirect"`
	}
	if err := c.do(
		http.MethodPost, "/api/v1/auth/begin", beginBody{
			Provider: provider,
			Redirect: redirect,
		}, &out,
	); err != nil {
		return "", err
	}
	return out.URL, nil
}

// Exchange exchanges the authCode for a long lived session token and returns it
func (c *daemonClient) Exchange(authCode string) (string, error) {
	var out struct {
		SessionToken string `json:"session_token"`
	}
	type exchangeBody struct {
		AuthCode string `json:"auth_code"`
	}
	if err := c.do(
		http.MethodPost, "/api/v1/auth/exchange", exchangeBody{AuthCode: authCode}, &out,
	); err != nil {
		return "", err
	}
	return out.SessionToken, nil
}

// Logout invalidates the current session
func (c *daemonClient) Logout() error {
	return c.do(http.MethodPost, "/api/v1/auth/logout", nil, nil)
}

// ConnectGithub connects user's github repositories to server controlled GitHub App.
func (c *daemonClient) ConnectGithub(fullName, branch string) error {
	type cgBody struct {
		RepoFullName string `json:"repo_fullname"`
		Branch       string `json:"branch"`
	}
	return c.do(http.MethodPost, "/api/v1/integrations/github", cgBody{
		RepoFullName: fullName,
		Branch:       branch,
	}, nil)
}

func daemonURL(c *cli.Command) (string, error) {
	if u := c.String("daemon"); u != "" {
		return u, nil
	}
	cfg, err := loadCfg()
	if err != nil {
		return "", err
	}
	return cfg.Daemon, nil
}

func (c *daemonClient) Providers() ([]string, error) {
	var out struct {
		Providers []string `json:"providers"`
	}
	if err := c.do(http.MethodGet, "/api/v1/auth/providers", nil, &out); err != nil {
		return nil, err
	}
	return out.Providers, nil
}

type meResult struct {
	User struct {
		ID    string  `json:"id"`
		Name  string  `json:"name"`
		Email *string `json:"email,omitempty"`
	} `json:"user"`
	Identities []struct {
		Provider string `json:"provider"`
		Login    string `json:"login,omitempty"`
	} `json:"identities"`
}

func (c *daemonClient) Me() (*meResult, error) {
	var out meResult
	if err := c.do(http.MethodGet, "/api/v1/auth/me", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *daemonClient) RemoveIntegration(provider string) error {
	return c.do(http.MethodDelete, "/api/v1/auth/"+provider, nil, nil)
}
