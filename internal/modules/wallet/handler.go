package wallet

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
		"service":   "wallet-service",
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "Wallet service is active",
	})
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":     "wallet-service",
		"version":     "v1.0.0",
		"endpoints":   []string{"GET /balance", "POST /topup", "POST /transfer"},
		"description": "User balance, ledger, topup invoices, and payment mutations",
	})
}
