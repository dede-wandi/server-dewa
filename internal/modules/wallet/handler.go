package wallet

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
	r.Get("/balance", h.GetBalance)
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
		"service":            "wallet-service",
		"status":             "healthy",
		"authenticated_user": userEmail,
		"timestamp":          time.Now().Format(time.RFC3339),
		"message":            "Wallet service is active (Protected by Supabase Auth)",
	})
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":     "wallet-service",
		"version":     "v1.1.0",
		"auth":        "Bearer Token Required",
		"endpoints":   []string{"GET /api/v1/wallet/balance", "POST /api/v1/wallet/topup", "POST /api/v1/wallet/transfer"},
		"description": "User balance, ledger, topup invoices, and payment mutations",
	})
}

func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	userID := ""
	userEmail := ""
	if user != nil {
		userID = user.ID
		userEmail = user.Email
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"user_id":    userID,
		"user_email": userEmail,
		"balance":    500000,
		"currency":   "IDR",
	})
}
