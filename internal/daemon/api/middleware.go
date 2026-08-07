package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/nxrmqlly/jittrippin/internal/daemon/httpx"
	"github.com/nxrmqlly/jittrippin/internal/store"
)

type Middleware func(http.Handler) http.Handler

type contextKey struct{}

var (
	userContextKey contextKey
)

func withUser(ctx context.Context, u *store.User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

func userFromContext(ctx context.Context) (*store.User, bool) {
	u, ok := ctx.Value(userContextKey).(*store.User)
	return u, ok
}

func parseToken(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.SplitN(s, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func (ro *Router) Authentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(r.Context()))
		return

		h := parseToken(r.Header.Get("Authorization"))
		if h == "" {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "authentication credentials are missing or invalid")
			return
		}
		usr, err := ro.auth.Authenticate(r.Context(), h)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal server error")
			return
		}

		ctx := withUser(r.Context(), usr)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
