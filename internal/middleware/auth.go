package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"udayana-backend/internal/service"
)

type identity struct {
	userID int64
	role   string
}

type contextKey struct{}

func RequireRole(store *service.Store, roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(header, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "autentikasi diperlukan")
				return
			}
			userID, role, err := store.VerifyToken(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
			if err != nil {
				writeError(w, http.StatusUnauthorized, "token tidak valid atau kedaluwarsa")
				return
			}
			if !allowed[role] {
				writeError(w, http.StatusForbidden, "akses tidak diizinkan")
				return
			}
			ctx := context.WithValue(r.Context(), contextKey{}, identity{userID: userID, role: role})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		http.Error(w, "response encode failed", http.StatusInternalServerError)
	}
}

func Identity(r *http.Request) (int64, string) {
	value, _ := r.Context().Value(contextKey{}).(identity)
	return value.userID, value.role
}
