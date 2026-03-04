// CLAUDE:SUMMARY Bearer token middleware for admin API authentication.
// CLAUDE:DEPENDS
// CLAUDE:EXPORTS BearerAuth

package admin

import (
	"net/http"
	"strings"
)

// BearerAuth returns a middleware that checks the Authorization header for a valid bearer token.
func BearerAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				http.Error(w, `{"error":"missing or invalid authorization"}`, http.StatusUnauthorized)
				return
			}
			if strings.TrimPrefix(auth, "Bearer ") != token {
				http.Error(w, `{"error":"invalid token"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
