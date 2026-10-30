package middleware

import (
	"context"
	"net/http"

	"github.com/magicelemental11/go-musthave-diploma-tpl/internal/auth"
)

// SessionCookie is the cookie that carries the signed user ID.
const SessionCookie = "session"

type userIDKey struct{}

// RequireAuth rejects requests without a valid session cookie with 401 and
// stores the user ID in the request context otherwise.
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var id int64
			var ok bool
			if c, err := r.Cookie(SessionCookie); err == nil {
				id, ok = auth.Parse(secret, c.Value)
			}
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, id)))
		})
	}
}

// UserID returns the authenticated user ID stored by RequireAuth.
func UserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey{}).(int64)
	return id, ok
}
