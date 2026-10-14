package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/ping", h.Ping)
	r.Get("/info", h.Info)
	return r
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":   "auth-service",
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "Auth service is active",
	})
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":     "auth-service",
		"version":     "v1.0.0",
		"endpoints":   []string{"POST /login", "POST /register", "POST /refresh"},
		"description": "User authentication, JWT token issuance, and session management",
	})
}
