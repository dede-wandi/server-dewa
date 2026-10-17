package agency

import (
	"encoding/json"
	"net/http"

	"github.com/dede-wandi/server-dewa/internal/middleware"
	"github.com/dede-wandi/server-dewa/internal/modules/wallet"
	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

// Handler handles HTTP requests for agency features
type Handler struct {
	service  Service
	sbClient *supabase.Client
}

// NewHandler creates a new Agency Handler
func NewHandler(sbClient *supabase.Client, walletService wallet.WalletService) *Handler {
	svc := NewService(sbClient, walletService)
	return &Handler{
		service:  svc,
		sbClient: sbClient,
	}
}

// GetService exposes the agency service for cross-module integration
func (h *Handler) GetService() Service {
	return h.service
}

// Routes mounts all agency endpoints
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/dashboard", h.GetDashboard)
	r.Get("/profile", h.GetProfile)
	r.Put("/profile", h.UpdateProfile)

	// Member management
	r.Get("/members", h.ListMembers)
	r.Post("/members/register", h.RegisterMember)
	r.Get("/members/{id}", h.GetMember)
	r.Get("/members/{id}/markups", h.GetMemberMarkups)
	r.Put("/members/{id}/markups", h.UpdateMemberMarkups)

	// Default template markups
	r.Get("/markups/defaults", h.GetDefaultMarkups)
	r.Put("/markups/defaults", h.UpdateDefaultMarkups)
	r.Post("/markups/copy", h.CopyMarkups)

	// Commission earnings
	r.Get("/commissions", h.ListCommissions)

	return r
}

func (h *Handler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Sesi tidak valid",
		})
		return
	}

	data, err := h.service.GetDashboard(user.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   data,
	})
}

func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	userName := ""
	if user.UserMetadata != nil {
		if name, ok := user.UserMetadata["name"].(string); ok {
			userName = name
		}
	}

	prof, err := h.service.GetOrCreateProfile(user.ID, userName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   prof,
	})
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Payload tidak valid",
		})
		return
	}

	prof, err := h.service.UpdateProfile(user.ID, req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Profil keagenan berhasil diperbarui",
		"data":    prof,
	})
}

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	members, err := h.service.ListMembers(user.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(members),
		"data":   members,
	})
}

func (h *Handler) RegisterMember(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var req RegisterMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Payload JSON tidak valid",
		})
		return
	}

	member, err := h.service.RegisterMember(user.ID, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Anggota baru berhasil didaftarkan dan markup harga kategori telah aktif",
		"data":    member,
	})
}

func (h *Handler) GetMember(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	memberID := chi.URLParam(r, "id")
	member, err := h.service.GetMember(user.ID, memberID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   member,
	})
}

func (h *Handler) GetMemberMarkups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	memberID := chi.URLParam(r, "id")
	markups, err := h.service.GetMemberMarkups(user.ID, memberID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   markups,
	})
}

func (h *Handler) UpdateMemberMarkups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	memberID := chi.URLParam(r, "id")
	var req UpdateMarkupsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Payload JSON tidak valid",
		})
		return
	}

	err := h.service.UpdateMemberMarkups(user.ID, memberID, req.Markups)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Markup harga kategori anggota berhasil disimpan",
	})
}

func (h *Handler) GetDefaultMarkups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	markups, err := h.service.GetDefaultMarkups(user.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   markups,
	})
}

func (h *Handler) UpdateDefaultMarkups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var req UpdateMarkupsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Payload JSON tidak valid",
		})
		return
	}

	err := h.service.UpdateDefaultMarkups(user.ID, req.Markups)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Markup default kategori agen berhasil diperbarui",
	})
}

func (h *Handler) CopyMarkups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var req CopyMarkupsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Payload JSON tidak valid",
		})
		return
	}

	err := h.service.CopyMarkups(user.ID, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Markup kategori berhasil disalin ke anggota tujuan",
	})
}

func (h *Handler) ListCommissions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	commissions, err := h.service.ListCommissions(user.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(commissions),
		"data":   commissions,
	})
}
