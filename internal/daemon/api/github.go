package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/nxrmqlly/jittrippin/internal/daemon/httpx"
	"github.com/nxrmqlly/jittrippin/internal/store"
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
	Ref        string     `json:"ref"`
	Before     string     `json:"before"`
	After      string     `json:"after"`
	Repository Repository `json:"repository"`
	Sender     User       `json:"sender"`
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

		tracked, err := ro.mgr.GetTrackedRepositoryByGithubID(event.Repository.ID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal server error")
			return
		}

		branch := strings.TrimPrefix(event.Ref, "refs/heads/")
		if tracked.Branch != branch {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, err = ro.mgr.SubmitRepository(*tracked, event.After)

	default:
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusOK)
}
