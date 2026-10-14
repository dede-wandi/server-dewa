package product

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
	r.Get("/", h.ListSample)
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
		"service":            "product-service",
		"status":             "healthy",
		"authenticated_user": userEmail,
		"timestamp":          time.Now().Format(time.RFC3339),
		"message":            "Product service is active (Protected by Supabase Auth)",
	})
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":     "product-service",
		"version":     "v1.1.0",
		"auth":        "Bearer Token Required",
		"endpoints":   []string{"GET /api/v1/products", "GET /api/v1/products/{id}", "POST /api/v1/products/sync"},
		"description": "PPOB digital products (Pulsa, Data, PLN, Game, E-Money) catalog and pricing",
	})
}

func (h *Handler) ListSample(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "success",
		"requester_id":    userID,
		"requester_email": user.Email,
		"data": []map[string]interface{}{
			{"code": "TSEL10", "name": "Telkomsel 10.000", "category": "pulsa", "price": 10250},
			{"code": "PLN20", "name": "Token PLN 20.000", "category": "pln", "price": 20500},
			{"code": "DANA50", "name": "Saldo DANA 50.000", "category": "emoney", "price": 50500},
			{"code": "ML86", "name": "Mobile Legends 86 Diamonds", "category": "game", "price": 19500},
		},
	})
}
