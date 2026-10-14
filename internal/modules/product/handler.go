package product

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
	r.Get("/", h.ListSample)
	return r
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":   "product-service",
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "Product service is active",
	})
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":     "product-service",
		"version":     "v1.0.0",
		"endpoints":   []string{"GET /products", "GET /products/{id}", "POST /products/sync"},
		"description": "PPOB digital products (Pulsa, Data, PLN, Game, E-Money) catalog and pricing",
	})
}

func (h *Handler) ListSample(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data": []map[string]interface{}{
			{"code": "TSEL10", "name": "Telkomsel 10.000", "category": "pulsa", "price": 10250},
			{"code": "PLN20", "name": "Token PLN 20.000", "category": "pln", "price": 20500},
			{"code": "DANA50", "name": "Saldo DANA 50.000", "category": "emoney", "price": 50500},
		},
	})
}
