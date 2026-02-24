package auth

import (
	"net/http"
	"strings"

	"github.com/go-chi/jwtauth/v5"
)

// NewMiddleware создает middleware для проверки JWT токена
func NewMiddleware(jwtSecret string) func(next http.Handler) http.Handler {
	tAuth := jwtauth.New("HS256", []byte(jwtSecret), nil)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			if authHeader != "" && !strings.HasPrefix(authHeader, "Bearer ") {
				authHeader = "Bearer " + authHeader
				r.Header.Set("Authorization", authHeader)
			}

			token, err := jwtauth.VerifyRequest(tAuth, r, jwtauth.TokenFromHeader)
			if err != nil || token == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"Forbidden"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
