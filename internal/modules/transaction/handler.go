package transaction

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dede-wandi/server-dewa/internal/middleware"
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
	user := middleware.GetUserFromContext(r.Context())
	userEmail := "anonymous"
	if user != nil {
		userEmail = user.Email
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":            "transaction-service",
		"status":             "healthy",
		"authenticated_user": userEmail,
		"timestamp":          time.Now().Format(time.RFC3339),
		"message":            "Transaction service is active (Protected by Supabase Auth)",
	})
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":     "transaction-service",
		"version":     "v1.1.0",
		"auth":        "Bearer Token Required",
		"endpoints":   []string{"POST /api/v1/transactions/order", "GET /api/v1/transactions/history", "POST /api/v1/transactions/callback"},
		"description": "Order processing, bill payment switching, and vendor callback handling",
	})
}
