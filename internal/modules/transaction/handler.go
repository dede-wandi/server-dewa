package transaction

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dede-wandi/server-dewa/internal/middleware"
	"github.com/dede-wandi/server-dewa/internal/modules/product"
	"github.com/dede-wandi/server-dewa/internal/modules/wallet"
	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

// Handler represents the presentation layer for transaction orders and billing
type Handler struct {
	service Service
}

// NewHandler constructs a new Handler instance with full OOP dependencies
func NewHandler(
	walletService wallet.WalletService,
	productService product.Service,
	sbClient *supabase.Client,
) *Handler {
	repo := NewMemoryRepository()
	svc := NewService(repo, walletService, productService, sbClient)

	return &Handler{
		service: svc,
	}
}

// Routes mounts all transaction endpoints
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public / Monitoring
	r.Get("/ping", h.Ping)
	r.Get("/info", h.Info)
	r.Post("/callback", h.Callback)

	// User order endpoints (Require Auth)
	r.Post("/order", h.CreateOrder)
	r.Get("/history", h.GetHistory)
	r.Get("/{order_code}", h.GetOrderDetail)

	// Admin-only monitoring
	r.Group(func(adminOnly chi.Router) {
		adminOnly.Use(middleware.RequireRoles("admin", "superadmin"))
		adminOnly.Get("/admin/all", h.ListAllOrders)
	})

	return r
}

// CreateOrder handles creating a new PPOB digital product order
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Sesi autentikasi tidak ditemukan. Harap sertakan token Bearer.",
		})
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Format payload JSON tidak valid",
		})
		return
	}

	log.Printf("🛒 [ORDER REQUEST] User=%s, ProductCode=%s, TargetAccount=%s", user.Email, req.ProductCode, req.TargetAccount)

	order, err := h.service.CreateOrder(user, token, req)
	if err != nil {
		log.Printf("❌ [ORDER FAILED] ProductCode=%s, Target=%s, Error=%v", req.ProductCode, req.TargetAccount, err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
			"order":   order,
		})
		return
	}

	log.Printf("✅ [ORDER SUCCESS] Code=%s, Product=%s, Target=%s, Price=Rp %.0f, SN=%s", order.OrderCode, order.ProductName, order.TargetAccount, order.Price, order.SerialNumber)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Pesanan produk berhasil diproses!",
		"data":    order,
	})
}

// GetHistory returns order history for the authenticated user
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Unauthorized",
		})
		return
	}

	orders := h.service.GetUserOrders(user.ID)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(orders),
		"data":   orders,
	})
}

// GetOrderDetail returns order details by order_code
func (h *Handler) GetOrderDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	orderCode := chi.URLParam(r, "order_code")

	order, err := h.service.GetOrderDetail(orderCode)
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
		"data":   order,
	})
}

// ListAllOrders returns all orders across the platform (Super Admin only)
func (h *Handler) ListAllOrders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	orders := h.service.GetAllOrders()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(orders),
		"data":   orders,
	})
}

// Callback handles vendor switching webhooks
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"message":  "Webhook callback acknowledged",
		"received": time.Now().Format(time.RFC3339),
	})
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":   "transaction-service",
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":     "transaction-service",
		"version":     "v2.0.0 (Clean Architecture OOP)",
		"endpoints":   []string{"POST /api/v1/transactions/order", "GET /api/v1/transactions/history", "GET /api/v1/transactions/{order_code}", "POST /api/v1/transactions/callback"},
		"description": "Clean Architecture order processing, billing orchestrator, and auto-refund engine",
	})
}
