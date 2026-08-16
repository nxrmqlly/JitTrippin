package api

import (
	"net/http"
	"net/url"

	"github.com/nxrmqlly/jittrippin/internal/auth"
	"github.com/nxrmqlly/jittrippin/internal/daemon/httpx"
)

type authBeginBody struct {
	Provider string `json:"provider"`
	Redirect string `json:"redirect"`
}

type authBeginResponse struct {
	URL string `json:"url"`
}

func (ro *Router) handleAuthBegin(w http.ResponseWriter, r *http.Request) {
	var b authBeginBody
	if !httpx.BindJSON(w, r, &b) {
		return
	}

	if b.Provider == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "query param provider is not set")
		return
	}
	if b.Redirect == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "query param redirect is not set")
		return
	}

	bres, err := ro.auth.Begin(r.Context(), auth.BeginOptions{
		Provider: b.Provider,
		Redirect: b.Redirect,
	})

	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, authBeginResponse{URL: bres.URL})

}

func (ro *Router) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()

	res, err := ro.auth.Callback(r.Context(), auth.CallbackOptions{
		Provider:     r.PathValue("provider"),
		State:        params.Get("state"),
		ProviderCode: params.Get("code"),
	})

	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	u, err := url.Parse(res.Redirect)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "invalid redirect url")
		return
	}
	q := u.Query()
	q.Set("auth_code", res.AuthCode)
	u.RawQuery = q.Encode()

	http.Redirect(w, r, u.String(), http.StatusFound)
}

type authExchangeBody struct {
	AuthCode string `json:"auth_code"`
}

type authExchangeResponse struct {
	SessionToken string `json:"session_token"`
}

func (ro *Router) handleAuthExchange(w http.ResponseWriter, r *http.Request) {
	var b authExchangeBody
	if !httpx.BindJSON(w, r, &b) {
		return
	}

	res, err := ro.auth.Exchange(r.Context(), auth.ExchangeOptions{
		AuthCode:  b.AuthCode,
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, authExchangeResponse{
		SessionToken: res.SessionToken,
	})
}

func (ro *Router) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	h := parseToken(r.Header.Get("Authorization"))
	if err := ro.auth.Logout(r.Context(), h); err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Write(nil)
}

func (ro *Router) handleAuthProviders(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, struct {
		Providers []string `json:"providers"`
	}{Providers: ro.auth.ProviderNames()})
}

type meResponse struct {
	User       userResponse       `json:"user"`
	Identities []identityResponse `json:"identities"`
}

type userResponse struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email,omitempty"`
}

type identityResponse struct {
	Provider string `json:"provider"`
	Login    string `json:"login,omitempty"`
}

func (ro *Router) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}
	me, err := ro.auth.Me(r.Context(), usr.ID)
	if err != nil {
		httpx.InternalServerError(w, err)
		return
	}
	resp := meResponse{
		User: userResponse{
			ID:    me.User.ID,
			Name:  me.User.DisplayName,
			Email: me.User.Email,
		},
	}
	for _, id := range me.Identities {
		resp.Identities = append(resp.Identities, identityResponse{
			Provider: id.Provider,
			Login:    id.ProviderLogin,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (ro *Router) handleAuthUnlink(w http.ResponseWriter, r *http.Request) {
	usr, ok := currentUser(w, r)
	if !ok {
		return
	}

	if err := ro.auth.Unlink(r.Context(), usr.ID, r.PathValue("provider")); err != nil {
		httpx.ErrorJSON(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

