package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/nxrmqlly/jittrippin/internal/daemon"
	"github.com/nxrmqlly/jittrippin/internal/daemon/httpx"
)

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
	Sender       User         `json:"sender"`
	Installation Installation `json:"installation"`
}

type Repository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

type User struct {
	Login string `json:"login"`
}

type Installation struct {
	ID string `json:"id"`
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

		for _, t := range tracked {
			if t.Branch != "" && t.Branch != branch {
				continue
			}
			ro.mgr.QueueSubmitRepository(*t, event.After)
		}

	default:
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusOK)
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

	ro.mgr.TrackRepository(daemon.TrackRepositoryConfig{
		OwnerID:              usr.ID,
		Provider:             "",
		ProviderInstance:     "github.com",
		ProviderRepositoryID: "",
		FullName:             "",
		Branch:               "",
	})
}
func (ro *Router) handleGithubList(w http.ResponseWriter, r *http.Request)
func (ro *Router) handleGithubRemove(w http.ResponseWriter, r *http.Request)
