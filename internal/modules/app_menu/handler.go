package app_menu

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/dede-wandi/server-dewa/internal/modules/app_category"
	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service Service
}

func NewHandler(sbClient *supabase.Client, catService app_category.Service) *Handler {
	return &Handler{
		service: NewService(sbClient, catService),
	}
}

func (h *Handler) GetService() Service {
	return h.service
}

// Routes mounts public read endpoints for mobile app users
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetActiveMenus)
	r.Get("/{id}", h.GetMenuDetail)
	return r
}

// AdminRoutes mounts admin full CRUD endpoints
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetAllMenus)
	r.Get("/{id}", h.GetMenuDetail)
	r.Post("/", h.CreateMenu)
	r.Put("/{id}", h.UpdateMenu)
	r.Delete("/{id}", h.DeleteMenu)
	return r
}

func parseMenuQueryParams(r *http.Request) (placement string, parentID string, level int, asTree bool) {
	placement = r.URL.Query().Get("placement")
	parentID = strings.TrimSpace(r.URL.Query().Get("parent_id"))
	if lvlStr := r.URL.Query().Get("level"); lvlStr != "" {
		if val, err := strconv.Atoi(lvlStr); err == nil {
			level = val
		}
	}
	treeParam := strings.ToLower(r.URL.Query().Get("tree"))
	asTree = treeParam == "true" || treeParam == "1"
	return
}

// GetActiveMenus returns active menus for home screen / mobile app
func (h *Handler) GetActiveMenus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	placement, parentID, level, asTree := parseMenuQueryParams(r)

	menus, err := h.service.GetActiveMenus(placement, parentID, level, asTree)
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
		"count":  len(menus),
		"data":   menus,
	})
}

// GetAllMenus returns all menus for Admin dashboard
func (h *Handler) GetAllMenus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	placement, parentID, level, asTree := parseMenuQueryParams(r)

	menus, err := h.service.GetAllMenus(placement, parentID, level, asTree)
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
		"count":  len(menus),
		"data":   menus,
	})
}

// GetMenuDetail returns detail of single menu
func (h *Handler) GetMenuDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")

	menu, err := h.service.GetMenuByID(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "menu tidak ditemukan",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   menu,
	})
}

// CreateMenu handles menu creation by admin
func (h *Handler) CreateMenu(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req CreateMenuRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Format data tidak valid: " + err.Error(),
		})
		return
	}

	created, err := h.service.CreateMenu(req)
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
		"message": "Menu layanan berhasil dibuat",
		"data":    created,
	})
}

// UpdateMenu handles menu modification by admin
func (h *Handler) UpdateMenu(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")

	var req UpdateMenuRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Format data tidak valid: " + err.Error(),
		})
		return
	}

	updated, err := h.service.UpdateMenu(id, req)
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
		"message": "Menu layanan berhasil diperbarui",
		"data":    updated,
	})
}

// DeleteMenu handles menu deletion by admin
func (h *Handler) DeleteMenu(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteMenu(id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Menu layanan berhasil dihapus",
	})
}
