package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/nxrmqlly/jittrippin/internal/daemon/httpx"
	"github.com/nxrmqlly/jittrippin/internal/github"
	"github.com/nxrmqlly/jittrippin/internal/store"
)

func (ro *Router) userGithubToken(ctx context.Context, userID string) (string, error) {
	tok, err := ro.auth.Token(ctx, userID, "github")
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

func VerifyWebhook(secret string, payload []byte, signature string) bool {
	const pre = "sha256="
	if !strings.HasPrefix(signature, pre) {
		return false
	}

	sig, err := hex.DecodeString(strings.TrimPrefix(signature, pre))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := mac.Sum(nil)

	return len(sig) == len(expected) &&
		subtle.ConstantTimeCompare(sig, expected) == 1
}

type Release struct {
	ID              int64  `json:"id"`
	TagName         string `json:"tag_name"`
	TargetCommitish string `json:"target_commitish"`
	Draft           bool   `json:"draft"`
	Prerelease      bool   `json:"prerelease"`
}

type ReleaseEvent struct {
	Action       string       `json:"action"`
	Release      Release      `json:"release"`
	Repository   Repository   `json:"repository"`
	Sender       GHUser       `json:"sender"`
	Installation Installation `json:"installation"`
}

type PushEvent struct {
	Ref          string       `json:"ref"`
	Before       string       `json:"before"`
	After        string       `json:"after"`
	Repository   Repository   `json:"repository"`
	Sender       GHUser       `json:"sender"`
	Installation Installation `json:"installation"`
}

type Repository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

type GHUser struct {
	Login string `json:"login"`
}

type Installation struct {
	ID int64 `json:"id"`
}

func (ro *Router) handleGithubWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "invalid body")
		return
	}

	sig := r.Header.Get("X-Hub-Signature-256")
	if !VerifyWebhook(
		os.Getenv("GITHUB_WEBHOOK_SECRET"),
		body, sig,
	) {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "invalid webhook signature")
		return
	}

	event := r.Header.Get("X-GitHub-Event")
	slog.Debug("github: webhook: received event", "event", event)
	switch event {
	case "push":
		var event PushEvent
		err := json.Unmarshal(body, &event)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusInternalServerError, "error decoding json")
			return
		}

		branch := strings.TrimPrefix(event.Ref, "refs/heads/")
		tracked, err := ro.mgr.ListTrackedRepositoriesByProviderRepo(
			"github",
			"github.com",
			strconv.FormatInt(event.Repository.ID, 10),
		)
		if err != nil {
			httpx.InternalServerError(w, err)
			return
		}
		if len(tracked) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		submitted := false
		for _, t := range tracked {
			if t.Branch != "" && t.Branch != branch {
				continue
			}
			submitted = true
			go ro.submitPush(ro.mgr.Context(), *t, event.Ref, event.After, event.Installation.ID)
		}
		if !submitted {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	case "release":
		var event ReleaseEvent
		if err := json.Unmarshal(body, &event); err != nil {
			slog.Error("github: webhook: release: failed to unmarshal", "err", err)
			httpx.ErrorJSON(w, http.StatusInternalServerError, "error decoding json")
			return
		}

		slog.Debug("github: webhook: release: parsed",
			"action", event.Action,
			"tag", event.Release.TagName,
			"release_id", event.Release.ID,
			"draft", event.Release.Draft,
			"repo_id", event.Repository.ID,
			"repo_name", event.Repository.FullName,
			"install_id", event.Installation.ID,
		)

		if event.Release.Draft {
			slog.Debug("github: webhook: release: skipping draft")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		tracked, err := ro.mgr.ListTrackedRepositoriesByProviderRepo(
			"github", "github.com",
			strconv.FormatInt(event.Repository.ID, 10),
		)
		if err != nil {
			slog.Error("github: webhook: release: failed to list tracked repos", "err", err)
			httpx.InternalServerError(w, err)
			return
		}
		if len(tracked) == 0 {
			slog.Warn("github: webhook: release: no tracked repo found", "repo_id", event.Repository.ID)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		slog.Info("github: webhook: release: submitting", "tracked_count", len(tracked), "action", event.Action, "tag", event.Release.TagName)
		for _, t := range tracked {
			slog.Debug("github: webhook: release: spawning submitRelease", "repo", t.FullName, "branch", t.Branch, "owner", t.OwnerID)
			go ro.submitRelease(ro.mgr.Context(), *t, event.Action,
				event.Release.TagName, event.Release.ID, event.Installation.ID)
		}
		w.WriteHeader(http.StatusAccepted)
		return
	default:
		w.WriteHeader(http.StatusNoContent)
		return
	}
}

func (ro *Router) submitPush(ctx context.Context, tracked store.TrackedRepository, ref, sha string, installID int64) {
	if _, err := ro.gh.SubmitPush(ctx, tracked, ref, sha, installID); err != nil {
		slog.Error("github: push submit failed", "repo", tracked.FullName, "ref", ref, "err", err)
	}
}

func (ro *Router) submitRelease(ctx context.Context, tracked store.TrackedRepository, action, tag string, releaseID, installID int64) {
	slog.Info("github: submitRelease: starting", "repo", tracked.FullName, "tag", tag, "action", action, "release_id", releaseID, "install_id", installID)
	if _, err := ro.gh.SubmitRelease(ctx, tracked, installID, github.ReleaseInfo{
		Action: action, Tag: tag, ReleaseID: releaseID,
	}); err != nil {
		slog.Error("github: submitRelease: failed", "repo", tracked.FullName, "tag", tag, "err", err)
	}
}

type githubAddBody struct {
	RepoFullName string `json:"repo_fullname"`
	Branch       string `json:"branch"`
}

func (ro *Router) handleGithubAdd(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}

	var body githubAddBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "invalid body")
		return
	}
	token, err := ro.userGithubToken(r.Context(), usr.ID)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusForbidden, "github not linked")
		return
	}

	if err := ro.gh.TrackRepository(r.Context(), token, usr.ID, body.RepoFullName, body.Branch); err != nil {
		if errors.Is(err, github.ErrInstallNotFound) {
			httpx.ErrorJSON(w, http.StatusBadRequest, "app is not installed for this repository owner")
			return
		}
		httpx.InternalServerError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (ro *Router) handleGithubList(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}

	repos, err := ro.mgr.ListTrackedRepositoriesByOwner(usr.ID)
	if err != nil {
		httpx.InternalServerError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, repos)
}

func (ro *Router) handleGithubRemove(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}

	repo, err := ro.mgr.GetTrackedRepositoryByID(r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrRepoNotFound) {
			httpx.ErrorJSON(w, http.StatusNotFound, "not found")
			return
		}
		httpx.InternalServerError(w, err)
		return
	}
	if repo.OwnerID != usr.ID {
		httpx.ErrorJSON(w, http.StatusNotFound, "not found")
		return
	}
	if err := ro.mgr.DeleteTrackedRepository(repo.ID); err != nil {
		httpx.InternalServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func renderInstallPage(w http.ResponseWriter, title, msg string) {
	w.Header().Add("Content-Type", "text/plain; charset=utf8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%s\n%s\n\n%s", title, strings.Repeat("-", len(title)), msg)
}

func (ro *Router) handleGithubInstallCallback(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	if errMsg := params.Get("error"); errMsg != "" {
		renderInstallPage(w, "Installation failed", errMsg)
		return
	}
	action := params.Get("setup_action")
	idstr := params.Get("installation_id")
	if action != "install" || idstr == "" {
		renderInstallPage(w, "No installation recorded", "No setup_action=install parameter was present.")
		return
	}
	id, err := strconv.ParseInt(idstr, 10, 64)
	if err != nil {
		renderInstallPage(w, "Installation failed", "invalid installation_id")
		return
	}
	if err := ro.gh.RecordInstall(r.Context(), id, action); err != nil {
		renderInstallPage(w, "Installation failed", err.Error())
		return
	}
	renderInstallPage(w, "JitTrippin installed", "GitHub App installed successfully. You can close this tab now.")
}

type installAcct struct {
	ID          int64  `json:"id"`
	Login       string `json:"login"`
	AccountType string `json:"account_type"`
}

type githubInstallStatusResponse struct {
	Installed  bool          `json:"installed"`
	Accounts   []installAcct `json:"accounts"`
	InstallURL string        `json:"install_url"`
}

func (ro *Router) handleGithubInstallStatus(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}
	token, err := ro.userGithubToken(r.Context(), usr.ID)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusForbidden, "github not linked")
		return
	}
	res, err := ro.gh.InstallStatus(r.Context(), token)
	if err != nil {
		httpx.InternalServerError(w, err)
		return
	}
	resp := githubInstallStatusResponse{
		Installed:  res.Installed,
		InstallURL: res.InstallURL,
	}
	for _, a := range res.Accounts {
		resp.Accounts = append(resp.Accounts, installAcct{ID: a.ID, Login: a.AccountLogin, AccountType: a.AccountType})
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

type githubBranchesResponse struct {
	Branches []string `json:"branches"`
}

func (ro *Router) handleGithubBranches(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}
	token, err := ro.userGithubToken(r.Context(), usr.ID)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusForbidden, "github not linked")
		return
	}
	fullName := r.PathValue("owner") + "/" + r.PathValue("name")
	branches, err := ro.gh.ListBranches(r.Context(), token, fullName)
	if err != nil {
		if errors.Is(err, github.ErrInstallNotFound) {
			httpx.ErrorJSON(w, http.StatusBadRequest, "app is not installed for this repository owner")
			return
		}
		httpx.InternalServerError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, &githubBranchesResponse{Branches: branches})
}

type githubReposResponse struct {
	Repos []repositoryResponse `json:"repos"`
}

type repositoryResponse struct {
	ID            string `json:"id"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

func (ro *Router) handleGithubRepos(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}
	token, err := ro.userGithubToken(r.Context(), usr.ID)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusForbidden, "github not linked")
		return
	}
	repos, err := ro.gh.ListInstallableRepos(r.Context(), token)
	if err != nil {
		httpx.InternalServerError(w, err)
		return
	}
	var resp githubReposResponse
	for _, rp := range repos {
		resp.Repos = append(resp.Repos, repositoryResponse{
			ID:            rp.ID,
			FullName:      rp.FullName,
			DefaultBranch: rp.DefaultBranch,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}
