package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nxrmqlly/jittrippin/internal/config"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	"github.com/nxrmqlly/jittrippin/pkg/logs"
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

type runResult struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Error      *string    `json:"error"`
}

func (c *daemonClient) SubmitRun(p *engine.Pipeline) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	if err := c.do(http.MethodPost, "/api/v1/runs", p, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func (c *daemonClient) RunGet(id string) (*runResult, error) {
	var out runResult
	if err := c.do(http.MethodGet, "/api/v1/runs/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *daemonClient) CancelRun(id string) error {
	return c.do(http.MethodPost, "/api/v1/runs/"+id+"/cancel", nil, nil)
}

func (c *daemonClient) StreamLogs(runID, job string) (<-chan logs.Line, func(), error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/v1/runs/"+runID+"/logs/"+job+"/live", nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var eb struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&eb)
		resp.Body.Close()
		return nil, nil, apiErr{Status: resp.StatusCode, Msg: eb.Error}
	}

	ch := make(chan logs.Line, 256)
	stop := func() { resp.Body.Close() }
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := strings.TrimPrefix(sc.Text(), "data: ")
			if line == "" {
				continue
			}
			var ll logs.Line
			if err := json.Unmarshal([]byte(line), &ll); err != nil {
				continue
			}
			select {
			case ch <- ll:
			case <-req.Context().Done():
				return
			}
			if err := sc.Err(); err != nil {
				log.Printf("log stream failed run_id=%s job=%s: %s", runID, job, err.Error())
			}
		}
	}()
	return ch, stop, nil
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
	return c.do(http.MethodPost, "/api/v1/integrations/github/repos", cgBody{
		RepoFullName: fullName,
		Branch:       branch,
	}, nil)
}

func daemonURL(c *cli.Command) (string, error) {
	if u := c.String("daemon"); u != "" {
		return u, nil
	}
	cfg, err := config.LoadUserConfig()
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
	return c.do(http.MethodDelete, "/api/v1/integrations/"+provider, nil, nil)
}

type installAcct struct {
	ID          int64  `json:"id"`
	Login       string `json:"login"`
	AccountType string `json:"account_type"`
}

type installStatus struct {
	Installed  bool          `json:"installed"`
	Accounts   []installAcct `json:"accounts"`
	InstallURL string        `json:"install_url"`
}

func (c *daemonClient) InstallStatus() (*installStatus, error) {
	var out installStatus
	if err := c.do(http.MethodGet, "/api/v1/integrations/github/install-status", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *daemonClient) ListBranches(fullName string) ([]string, error) {
	var out struct {
		Branches []string `json:"branches"`
	}
	if err := c.do(http.MethodGet, "/api/v1/integrations/github/repos/"+fullName+"/branches", nil, &out); err != nil {
		return nil, err
	}
	return out.Branches, nil
}

type trackedRepo struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
	Branch   string `json:"branch"`
}

func (c *daemonClient) TrackedRepos() ([]trackedRepo, error) {
	var out []trackedRepo
	if err := c.do(http.MethodGet, "/api/v1/integrations/github/list", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *daemonClient) RemoveGithub(id string) error {
	return c.do(http.MethodDelete, "/api/v1/integrations/github/repos/"+id, nil, nil)
}

type installableRepo struct {
	ID            string `json:"id"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

func (c *daemonClient) InstallableRepos() ([]installableRepo, error) {
	var out struct {
		Repos []installableRepo `json:"repos"`
	}
	if err := c.do(http.MethodGet, "/api/v1/integrations/github/repos", nil, &out); err != nil {
		return nil, err
	}
	return out.Repos, nil
}

func (c *daemonClient) IntegrationProviders() ([]string, error) {
	var out struct {
		Integrations []string `json:"integrations"`
	}
	if err := c.do(http.MethodGet, "/api/v1/integrations/providers", nil, &out); err != nil {
		return nil, err
	}
	return out.Integrations, nil
}

type userInstallation struct {
	InstallationID string `json:"installation_id"`
	AccountLogin   string `json:"account_login"`
	AccountType    string `json:"account_type"`
}

type userIntegration struct {
	Provider         string             `json:"provider"`
	ProviderInstance string             `json:"provider_instance"`
	Status           string             `json:"status"`
	Installations    []userInstallation `json:"installations"`
}

func (c *daemonClient) MyIntegrations() ([]userIntegration, error) {
	var out struct {
		Integrations []userIntegration `json:"integrations"`
	}
	if err := c.do(http.MethodGet, "/api/v1/integrations", nil, &out); err != nil {
		return nil, err
	}
	return out.Integrations, nil
}

func (c *daemonClient) LinkIntegration(provider string) ([]installAcct, error) {
	var out struct {
		Accounts []installAcct `json:"accounts"`
	}
	if err := c.do(http.MethodPost, "/api/v1/integrations/"+provider, nil, &out); err != nil {
		return nil, err
	}
	return out.Accounts, nil
}
