package article

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

// Routes mounts public articles endpoints for mobile app
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetArticles)
	r.Get("/{slugOrID}", h.GetArticleDetail)
	return r
}

// AdminRoutes mounts admin CRUD endpoints for web dashboard
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetAllArticlesAdmin)
	r.Get("/{slugOrID}", h.GetArticleDetail)
	r.Post("/", h.CreateArticle)
	r.Put("/{id}", h.UpdateArticle)
	r.Delete("/{id}", h.DeleteArticle)
	return r
}

// GetArticles returns published articles with category and search filtering
func (h *Handler) GetArticles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	category := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")

	articles, err := h.service.GetArticles(category, search)
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
		"count":  len(articles),
		"data":   articles,
	})
}

// GetAllArticlesAdmin returns all articles (including drafts) for Admin
func (h *Handler) GetAllArticlesAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	category := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")

	articles, err := h.service.GetAllArticlesAdmin(category, search)
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
		"count":  len(articles),
		"data":   articles,
	})
}

// GetArticleDetail returns single article by slug or ID
func (h *Handler) GetArticleDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	slugOrID := chi.URLParam(r, "slugOrID")

	article, err := h.service.GetArticleBySlugOrID(slugOrID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Artikel tidak ditemukan",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   article,
	})
}

// CreateArticle creates a new article
func (h *Handler) CreateArticle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req CreateArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Payload JSON tidak valid",
		})
		return
	}

	created, err := h.service.CreateArticle(req)
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
		"message": "Artikel berhasil dibuat",
		"data":    created,
	})
}

// UpdateArticle updates an existing article
func (h *Handler) UpdateArticle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")

	var req UpdateArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Payload JSON tidak valid",
		})
		return
	}

	updated, err := h.service.UpdateArticle(id, req)
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
		"message": "Artikel berhasil diperbarui",
		"data":    updated,
	})
}

// DeleteArticle deletes an article
func (h *Handler) DeleteArticle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteArticle(id); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Artikel berhasil dihapus",
	})
}
