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

func (ro *Router) handleAuthLogout(w http.ResponseWriter, r *http.Request) {}
