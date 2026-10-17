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

// SetAgencyResolver sets the agency markup resolver
func (h *Handler) SetAgencyResolver(ar AgencyResolver) {
	h.service.SetAgencyResolver(ar)
}

// Routes mounts all transaction endpoints
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public / Monitoring
	r.Get("/ping", h.Ping)
	r.Get("/info", h.Info)
	r.Post("/callback", h.Callback)

	// User order & inquiry endpoints (Require Auth)
	r.Post("/inquiry", h.InquiryBill)
	r.Post("/order", h.CreateOrder)
	r.Get("/history", h.GetHistory)
	r.Get("/{order_code}", h.GetOrderDetail)

	// Admin-only monitoring & fulfillment
	r.Group(func(adminOnly chi.Router) {
		adminOnly.Use(middleware.RequireRoles("admin", "superadmin"))
		adminOnly.Get("/admin/all", h.ListAllOrders)
		adminOnly.Post("/admin/orders/{code}/approve", h.ApproveOrder)
		adminOnly.Post("/admin/orders/{code}/reject", h.RejectOrder)
	})

	return r
}

func (h *Handler) GetService() Service {
	return h.service
}

// InquiryBill handles checking a postpaid bill (PDAM, PLN Pasca, BPJS, etc.)
func (h *Handler) InquiryBill(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())

	var req BillInquiryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Format payload JSON tidak valid",
		})
		return
	}

	log.Printf("🔍 [INQUIRY REQUEST] ProductCode=%s, CustomerNo=%s", req.ProductCode, req.CustomerNo)

	inq, err := h.service.InquiryBill(user, req)
	if err != nil {
		log.Printf("❌ [INQUIRY FAILED] ProductCode=%s, CustomerNo=%s, Error=%v", req.ProductCode, req.CustomerNo, err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	log.Printf("✅ [INQUIRY SUCCESS] ID=%s, Customer=%s, Bill=Rp %.0f, Admin=Rp %.0f, Total=Rp %.0f",
		inq.InquiryID, inq.CustomerName, inq.BillAmount, inq.AdminFee, inq.TotalAmount)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Pengecekan tagihan berhasil",
		"data":    inq,
	})
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

type ApproveOrderPayload struct {
	SerialNumber string `json:"serial_number"`
	Notes        string `json:"notes"`
}

type RejectOrderPayload struct {
	Reason string `json:"reason"`
}

// ApproveOrder handles admin approval for manual product orders
func (h *Handler) ApproveOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	code := chi.URLParam(r, "code")
	if code == "" {
		code = chi.URLParam(r, "order_code")
	}

	var req ApproveOrderPayload
	_ = json.NewDecoder(r.Body).Decode(&req)

	adminUser := middleware.GetUserFromContext(r.Context())
	order, err := h.service.ApproveOrder(adminUser, code, req.SerialNumber, req.Notes)
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
		"message": "Pesanan berhasil disetujui & diselesaikan",
		"data":    order,
	})
}

// RejectOrder handles admin rejection and auto-refund for manual product orders
func (h *Handler) RejectOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	code := chi.URLParam(r, "code")
	if code == "" {
		code = chi.URLParam(r, "order_code")
	}

	var req RejectOrderPayload
	_ = json.NewDecoder(r.Body).Decode(&req)

	adminUser := middleware.GetUserFromContext(r.Context())
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

	order, err := h.service.RejectOrder(adminUser, token, code, req.Reason)
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
		"message": "Pesanan ditolak dan saldo telah dikembalikan ke pengguna",
		"data":    order,
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
