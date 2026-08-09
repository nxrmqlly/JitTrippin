package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

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

func currentUser(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	usr, ok := userFromContext(r.Context())
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	return usr, ok
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

func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func (ro *Router) Authentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := parseToken(r.Header.Get("Authorization"))
		if h == "" {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "authentication credentials are missing or invalid")
			return
		}
		usr, err := ro.auth.Authenticate(r.Context(), h)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := withUser(r.Context(), usr)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type logResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *logResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (ro *Router) Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &logResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // go defaults to 200 OK
		}
		next.ServeHTTP(rw, r)
		elapsed := time.Since(start)

		log.Printf("%d %s %s %s", rw.statusCode, elapsed, r.Method, r.URL.Path)
	})
}
