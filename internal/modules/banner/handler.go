package banner

import (
	"encoding/json"
	"net/http"

	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service Service
}

func NewHandler(sbClient *supabase.Client) *Handler {
	return &Handler{
		service: NewService(sbClient),
	}
}

func (h *Handler) GetService() Service {
	return h.service
}

// Routes mounts public banner endpoints (for mobile app)
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetActiveBanners)
	r.Get("/{id}", h.GetBannerDetail)
	return r
}

// AdminRoutes mounts admin CRUD endpoints
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetAllBanners)
	r.Get("/{id}", h.GetBannerDetail)
	r.Post("/", h.CreateBanner)
	r.Put("/{id}", h.UpdateBanner)
	r.Delete("/{id}", h.DeleteBanner)
	return r
}

// GetActiveBanners returns active banners for home screen / mobile app
func (h *Handler) GetActiveBanners(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	bType := r.URL.Query().Get("type")
	placement := r.URL.Query().Get("placement")

	banners, err := h.service.GetActiveBanners(bType, placement)
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
		"count":  len(banners),
		"data":   banners,
	})
}

// GetAllBanners returns all banners for Admin dashboard
func (h *Handler) GetAllBanners(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	bType := r.URL.Query().Get("type")
	placement := r.URL.Query().Get("placement")

	banners, err := h.service.GetAllBanners(bType, placement)
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
		"count":  len(banners),
		"data":   banners,
	})
}

// GetBannerDetail returns detail of single banner
func (h *Handler) GetBannerDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")

	banner, err := h.service.GetBannerByID(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Banner tidak ditemukan",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   banner,
	})
}

// CreateBanner handles creating a new banner
func (h *Handler) CreateBanner(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req CreateBannerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Payload JSON tidak valid",
		})
		return
	}

	created, err := h.service.CreateBanner(req)
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
		"message": "Banner berhasil dibuat",
		"data":    created,
	})
}

// UpdateBanner handles updating an existing banner
func (h *Handler) UpdateBanner(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")

	var req UpdateBannerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Payload JSON tidak valid",
		})
		return
	}

	updated, err := h.service.UpdateBanner(id, req)
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
		"message": "Banner berhasil diperbarui",
		"data":    updated,
	})
}

// DeleteBanner handles deleting a banner
func (h *Handler) DeleteBanner(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteBanner(id); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Banner berhasil dihapus",
	})
}
