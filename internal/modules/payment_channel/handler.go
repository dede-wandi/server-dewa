package payment_channel

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

// Handler handles HTTP requests for payment channels
type Handler struct {
	service  Service
	sbClient *supabase.Client
}

// NewHandler creates a new Handler instance
func NewHandler(sbClient *supabase.Client) *Handler {
	return &Handler{
		service:  NewService(sbClient),
		sbClient: sbClient,
	}
}

// GetService exposes the underlying service
func (h *Handler) GetService() Service {
	return h.service
}

// PublicRoutes registers public routes for mobile apps and clients
func (h *Handler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetActiveChannels)
	r.Get("/{id}", h.GetChannelByID)
	return r
}

// AdminRoutes registers administrative management routes
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetAllChannelsAdmin)
	r.Get("/{id}", h.GetChannelByID)
	r.Post("/", h.CreateChannel)
	r.Put("/{id}", h.UpdateChannel)
	r.Delete("/{id}", h.DeleteChannel)
	return r
}

// GetActiveChannels returns all active & coming soon payment channels
func (h *Handler) GetActiveChannels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	channels, err := h.service.GetActiveChannels()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(PaymentChannelListResponse{
		Status: "success",
		Count:  len(channels),
		Data:   channels,
	})
}

// GetAllChannelsAdmin returns all payment channels including inactive
func (h *Handler) GetAllChannelsAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	channels, err := h.service.GetAllChannelsAdmin()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(PaymentChannelListResponse{
		Status: "success",
		Count:  len(channels),
		Data:   channels,
	})
}

// GetChannelByID returns channel details by ID or code
func (h *Handler) GetChannelByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")
	ch, err := h.service.GetChannelByID(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(PaymentChannelResponse{
		Status: "success",
		Data:   ch,
	})
}

// CreateChannel creates a new payment channel
func (h *Handler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req CreatePaymentChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Invalid request body: JSON required",
		})
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	ch, err := h.service.CreateChannel(req, token)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(PaymentChannelResponse{
		Status:  "success",
		Message: "Channel pembayaran berhasil ditambahkan",
		Data:    ch,
	})
}

// UpdateChannel modifies existing payment channel
func (h *Handler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")

	var req UpdatePaymentChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Invalid request body: JSON required",
		})
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	ch, err := h.service.UpdateChannel(id, req, token)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(PaymentChannelResponse{
		Status:  "success",
		Message: "Channel pembayaran berhasil diperbarui",
		Data:    ch,
	})
}

// DeleteChannel removes a payment channel
func (h *Handler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	if err := h.service.DeleteChannel(id, token); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Channel pembayaran berhasil dihapus",
	})
}
