package api

import (
	"net/http"

	"github.com/nxrmqlly/jittrippin/internal/daemon/httpx"
	"github.com/nxrmqlly/jittrippin/internal/store"
)

func (ro *Router) handleIntegrationProviders(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, struct {
		Integrations []string `json:"integrations"`
	}{Integrations: []string{ro.gh.Name()}})
}

func (ro *Router) handleIntegrationList(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}
	integrations, err := ro.gh.ListUserIntegrations(r.Context(), usr.ID)
	if err != nil {
		httpx.InternalServerError(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, struct {
		Integrations []store.UserIntegration `json:"integrations"`
	}{Integrations: integrations})
}

func (ro *Router) handleIntegrationAdd(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}
	if r.PathValue("provider") != "github" {
		httpx.ErrorJSON(w, http.StatusNotFound, "unknown integration")
		return
	}
	token, err := ro.userGithubToken(r.Context(), usr.ID)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusForbidden, "github not linked: run jt auth login first")
		return
	}
	installs, err := ro.gh.LinkUserIntegration(r.Context(), usr.ID, token)
	if err != nil {
		httpx.InternalServerError(w)
		return
	}
	accounts := make([]installAcct, 0, len(installs))
	for _, ins := range installs {
		accounts = append(accounts, installAcct{
			ID:          ins.ID,
			Login:       ins.AccountLogin,
			AccountType: ins.AccountType,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, struct {
		Accounts []installAcct `json:"accounts"`
	}{Accounts: accounts})
}

func (ro *Router) handleIntegrationRemove(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}
	if err := ro.gh.UnlinkUserIntegration(r.Context(), usr.ID, r.PathValue("provider")); err != nil {
		httpx.InternalServerError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
