package product

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dede-wandi/server-dewa/internal/middleware"
	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

// Handler represents the presentation HTTP controller for the product catalog
type Handler struct {
	service Service
}

// NewHandler creates a new instance of Handler with OOP Service & Cache Repository
func NewHandler(sbClient *supabase.Client) *Handler {
	repo := NewCachedRepository(5 * time.Minute)
	svc := NewService(repo, sbClient)

	// Fetch live products from Supabase
	go func() {
		_, _ = repo.SyncFromSupabase(sbClient)
	}()

	return &Handler{
		service: svc,
	}
}

// GetService returns the underlying Service instance
func (h *Handler) GetService() Service {
	return h.service
}

// Routes mounts all endpoints for products
// Routes mounts all endpoints for products (User / Public Catalog)
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public / General product catalog (User: hanya produk aktif)
	r.Get("/ping", h.Ping)
	r.Get("/info", h.Info)
	r.Get("/", h.ListUserProducts)
	r.Get("/categories", h.ListCategories)
	r.Get("/brands", h.ListBrands)
	r.Get("/{code}", h.GetUserProductDetail)

	// Admin-only / CRUD operations
	r.Group(func(adminOnly chi.Router) {
		adminOnly.Use(middleware.RequireRoles("admin", "superadmin"))
		adminOnly.Post("/", h.CreateProduct)
		adminOnly.Put("/{id}", h.UpdateProduct)
		adminOnly.Delete("/{id}", h.DeleteProduct)
		adminOnly.Post("/sync", h.SyncProducts)
	})

	return r
}

// AdminRoutes mounts dedicated administrative CRUD endpoints
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListAdminProducts)
	r.Get("/{code}", h.GetAdminProductDetail)
	r.Post("/", h.CreateProduct)
	r.Put("/{id}", h.UpdateProduct)
	r.Delete("/{id}", h.DeleteProduct)
	r.Post("/sync", h.SyncProducts)
	return r
}

// ListUserProducts returns ONLY active products for mobile app / end users
func (h *Handler) ListUserProducts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	category := r.URL.Query().Get("category")
	brand := r.URL.Query().Get("brand")
	search := r.URL.Query().Get("search")

	// activeOnly = true: hanya produk aktif yang tampil ke user
	products := h.service.ListProducts(category, brand, search, true)
	if products == nil {
		products = []*Product{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(products),
		"data":   products,
	})
}

// GetUserProductDetail returns details for an ACTIVE product (returns 404 if inactive)
func (h *Handler) GetUserProductDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	code := chi.URLParam(r, "code")

	p, err := h.service.GetProduct(code)
	if err != nil || !p.IsActive || !strings.EqualFold(p.Status, "ACTIVE") {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Produk sedang tidak aktif atau tidak ditemukan",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   p,
	})
}

// ListAdminProducts returns all products (active & inactive) for admin management
func (h *Handler) ListAdminProducts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	category := r.URL.Query().Get("category")
	brand := r.URL.Query().Get("brand")
	search := r.URL.Query().Get("search")
	status := strings.TrimSpace(r.URL.Query().Get("status"))

	// activeOnly = false: admin dapat melihat seluruh produk
	products := h.service.ListProducts(category, brand, search, false)

	if status != "" && !strings.EqualFold(status, "ALL") {
		filtered := make([]*Product, 0)
		for _, p := range products {
			if strings.EqualFold(p.Status, status) {
				filtered = append(filtered, p)
			}
		}
		products = filtered
	}
	if products == nil {
		products = []*Product{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(products),
		"data":   products,
	})
}

// GetAdminProductDetail returns details for any product for admin
func (h *Handler) GetAdminProductDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	code := chi.URLParam(r, "code")

	p, err := h.service.GetProduct(code)
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
		"data":   p,
	})
}

// ListCategories returns all available categories for active products
func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	categories := h.service.GetCategories(true)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(categories),
		"data":   categories,
	})
}

// ListBrands returns all brands for active products
func (h *Handler) ListBrands(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	category := r.URL.Query().Get("category")
	brands := h.service.GetBrands(category, true)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(brands),
		"data":   brands,
	})
}

// SyncProducts forces a cache sync from provider or database
func (h *Handler) SyncProducts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	adminID := "admin"
	if user != nil {
		adminID = user.ID
	}

	count, err := h.service.SyncProducts(adminID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"message":   fmt.Sprintf("Katalog produk berhasil disinkronisasi. Total %d produk aktif.", count),
		"synced_at": time.Now().Format(time.RFC3339),
	})
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":   "product-service",
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":     "product-service",
		"version":     "v2.0.0 (Clean OOP)",
		"cache":       "In-Memory TTL 5m (Sub-millisecond latency)",
		"endpoints":   []string{"GET /api/v1/products", "GET /api/v1/products/{code}", "GET /api/v1/products/categories", "GET /api/v1/products/brands", "POST /api/v1/admin/products", "PUT /api/v1/admin/products/{id}", "DELETE /api/v1/admin/products/{id}", "POST /api/v1/admin/products/sync"},
		"description": "Clean Architecture catalog engine for Pulsa, Data, PLN, E-Money, Game",
	})
}

// CreateProduct creates a new product within a mandatory category and validates vendor SKU
func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Payload JSON tidak valid: " + err.Error(),
		})
		return
	}

	prod, err := h.service.CreateProduct(req)
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
		"message": "Produk berhasil dibuat di dalam kategori",
		"data":    prod,
	})
}

// UpdateProduct updates product details by ID
func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Parameter ID produk wajib disertakan",
		})
		return
	}

	var req UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Payload JSON tidak valid: " + err.Error(),
		})
		return
	}

	prod, err := h.service.UpdateProduct(id, req)
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
		"message": "Produk berhasil diperbarui",
		"data":    prod,
	})
}

// DeleteProduct removes product by ID
func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Parameter ID produk wajib disertakan",
		})
		return
	}

	if err := h.service.DeleteProduct(id); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Produk berhasil dihapus",
	})
}
