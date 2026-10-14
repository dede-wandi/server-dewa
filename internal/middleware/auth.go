package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dede-wandi/server-dewa/internal/supabase"
)

type contextKey string

const UserContextKey contextKey = "authenticated_user"

// RequireAuth ensures that requests have a valid Supabase Bearer token
func RequireAuth(sb *supabase.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeUnauthorized(w, "Unauthorized: Header 'Authorization: Bearer <token>' wajib disertakan")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				writeUnauthorized(w, "Unauthorized: Format header tidak valid. Gunakan 'Bearer <token>'")
				return
			}

			token := strings.TrimSpace(parts[1])
			if token == "" {
				writeUnauthorized(w, "Unauthorized: Token kosong")
				return
			}

			user, err := sb.GetUser(token)
			if err != nil {
				writeUnauthorized(w, "Unauthorized: Token tidak valid atau sudah kedaluwarsa ("+err.Error()+")")
				return
			}

			// Store authenticated user in request context
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext retrieves the authenticated user from the context
func GetUserFromContext(ctx context.Context) *supabase.User {
	if user, ok := ctx.Value(UserContextKey).(*supabase.User); ok {
		return user
	}
	return nil
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "error",
		"code":    http.StatusUnauthorized,
		"message": msg,
	})
}
