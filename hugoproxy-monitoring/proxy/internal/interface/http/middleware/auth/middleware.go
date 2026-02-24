package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/jwtauth/v5"
)

type contextKey string

const userIDKey contextKey = "user_id"

func NewMiddleware(jwtSecret string) func(next http.Handler) http.Handler {
	tokenAuth := jwtauth.New("HS256", []byte(jwtSecret), nil)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			if authHeader != "" && !strings.HasPrefix(authHeader, "Bearer ") {
				authHeader = "Bearer " + authHeader
				r.Header.Set("Authorization", authHeader)
			}

			token, err := jwtauth.VerifyRequest(tokenAuth, r, jwtauth.TokenFromHeader)
			if err != nil || token == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"Forbidden"}`))
				return
			}

			if userID, exists := token.Get("user_id"); exists {
				if userIDInt, ok := userID.(int); ok {
					ctx := context.WithValue(r.Context(), userIDKey, userIDInt)
					r = r.WithContext(ctx)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func GetUserID(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(userIDKey).(int)
	return userID, ok
}
