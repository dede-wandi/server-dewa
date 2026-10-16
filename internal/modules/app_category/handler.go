package app_category

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/dede-wandi/server-dewa/internal/modules/product"
	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service Service
}

func NewHandler(sbClient *supabase.Client, prodService product.Service) *Handler {
	return &Handler{
		service: NewService(sbClient, prodService),
	}
}

func (h *Handler) GetService() Service {
	return h.service
}

// Routes mounts public read endpoints for mobile app users
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetActiveCategories)
	r.Get("/{id}", h.GetCategoryDetail)
	return r
}

// AdminRoutes mounts admin full CRUD endpoints
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetAllCategories)
	r.Get("/{id}", h.GetCategoryDetail)
	r.Post("/", h.CreateCategory)
	r.Put("/{id}", h.UpdateCategory)
	r.Delete("/{id}", h.DeleteCategory)
	return r
}

// GetActiveCategories handles public request for active categories (supports ?parent_id=...&level=...&tree=true)
func (h *Handler) GetActiveCategories(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	parentID := r.URL.Query().Get("parent_id")
	levelStr := r.URL.Query().Get("level")
	treeStr := strings.ToLower(r.URL.Query().Get("tree"))

	level := 0
	if levelStr != "" {
		level, _ = strconv.Atoi(levelStr)
	}

	asTree := treeStr == "true" || treeStr == "1"

	categories, err := h.service.GetActiveCategories(parentID, level, asTree)
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
		"count":  len(categories),
		"data":   categories,
	})
}

// GetAllCategories handles admin request for all categories (including inactive)
func (h *Handler) GetAllCategories(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	parentID := r.URL.Query().Get("parent_id")
	levelStr := r.URL.Query().Get("level")
	treeStr := strings.ToLower(r.URL.Query().Get("tree"))

	level := 0
	if levelStr != "" {
		level, _ = strconv.Atoi(levelStr)
	}

	asTree := treeStr == "true" || treeStr == "1"

	categories, err := h.service.GetAllCategories(parentID, level, asTree)
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
		"count":  len(categories),
		"data":   categories,
	})
}

// GetCategoryDetail returns single category by ID, code, or slug
func (h *Handler) GetCategoryDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	idOrSlug := chi.URLParam(r, "id")

	cat, err := h.service.GetCategoryByCodeOrSlug(idOrSlug)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "kategori tidak ditemukan",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   cat,
	})
}

// CreateCategory handles admin category creation
func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Format data tidak valid: " + err.Error(),
		})
		return
	}

	created, err := h.service.CreateCategory(req)
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
		"message": "Kategori berhasil dibuat",
		"data":    created,
	})
}

// UpdateCategory handles admin category update
func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")

	var req UpdateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Format data tidak valid: " + err.Error(),
		})
		return
	}

	updated, err := h.service.UpdateCategory(id, req)
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
		"message": "Kategori berhasil diperbarui",
		"data":    updated,
	})
}

// DeleteCategory handles admin category removal
func (h *Handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteCategory(id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Kategori berhasil dihapus",
	})
}
