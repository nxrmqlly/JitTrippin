package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
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

type PushEvent struct {
	Ref          string       `json:"ref"`
	Before       string       `json:"before"`
	After        string       `json:"after"`
	Repository   Repository   `json:"repository"`
	Sender       GHUser         `json:"sender"`
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
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal server error")
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
			go ro.submitPush(ro.mgr.Context(), *t, event.After, event.Installation.ID)
		}
		if !submitted {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return

	default:
		w.WriteHeader(http.StatusNoContent)
		return
	}
}

func (ro *Router) submitPush(ctx context.Context, tracked store.TrackedRepository, sha string, installID int64) {
	if _, err := ro.gh.SubmitPush(ctx, tracked, sha, installID); err != nil {
		log.Printf("github: push submit failed: repo=%s sha=%s: %v", tracked.ID, sha, err)
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
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal server error")
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
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, repos)
}

type githubRemoveBody struct {
	ID string `json:"id"`
}

func (ro *Router) handleGithubRemove(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}

	var body githubRemoveBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "invalid body")
		return
	}

	repo, err := ro.mgr.GetTrackedRepositoryByID(body.ID)
	if err != nil {
		if errors.Is(err, store.ErrRepoNotFound) {
			httpx.ErrorJSON(w, http.StatusNotFound, "not found")
			return
		}
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if repo.OwnerID != usr.ID {
		httpx.ErrorJSON(w, http.StatusNotFound, "not found")
		return
	}
	if err := ro.mgr.DeleteTrackedRepository(body.ID); err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
