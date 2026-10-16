package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/dede-wandi/server-dewa/internal/domain/role"
	"github.com/dede-wandi/server-dewa/internal/middleware"
	"github.com/dede-wandi/server-dewa/internal/modules/vendor"
	"github.com/dede-wandi/server-dewa/internal/modules/wallet"
	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

var adminStartTime = time.Now()

// Handler acts as the HTTP Controller for the Admin module (OOP Architecture)
type Handler struct {
	adminService  Service
	walletHandler *wallet.Handler
	sbClient      *supabase.Client
}

// NewHandler constructs a new Admin Handler with injected Service and Repository
func NewHandler(sbClient *supabase.Client, walletHandler *wallet.Handler) *Handler {
	repo := NewRepository(sbClient)
	svc := NewService(repo, walletHandler, sbClient)

	return &Handler{
		adminService:  svc,
		walletHandler: walletHandler,
		sbClient:      sbClient,
	}
}

// Routes defines all /api/v1/admin endpoints
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Monitoring & Health
	r.Get("/ping", h.Ping)
	r.Get("/dashboard", h.DashboardStats)
	r.Get("/stats", h.DashboardStats)

	// User Management (Super Admin Full Authority)
	r.Get("/users", h.ListUsers)
	r.Get("/users/{id}", h.GetUserDetail)
	r.Put("/users/{id}", h.UpdateUser)
	r.Put("/users/{id}/password", h.ResetUserPassword)
	r.Post("/users/{id}/reset-password", h.ResetUserPassword)
	r.Put("/users/{id}/email", h.UpdateUserEmail)
	r.Delete("/users/{id}", h.DeleteUser)
	r.Put("/users/{id}/lock", h.LockUser)
	r.Put("/users/{id}/unlock", h.UnlockUser)

	// User-specific Wallet Shortcuts
	r.Post("/users/{id}/topup", h.AdminTopUp)
	r.Post("/users/{id}/adjust", h.walletHandler.AdjustBalance)
	r.Post("/users/{id}/refund", h.walletHandler.Refund)

	// Platform Wallets & Super Admin Ledger Operations
	r.Get("/wallets", h.ListAllWallets)
	r.Post("/topup", h.AdminTopUp)
	r.Post("/wallets/topup", h.AdminTopUp)
	r.Post("/adjust", h.walletHandler.AdjustBalance)
	r.Post("/wallets/adjust", h.walletHandler.AdjustBalance)
	r.Post("/refund", h.walletHandler.Refund)
	r.Post("/wallets/refund", h.walletHandler.Refund)

	// Top Up Approval & Verification (Super Admin)
	r.Get("/wallets/topup/pending", h.walletHandler.GetPendingTopUps)
	r.Post("/wallets/topup/approve", h.walletHandler.ApproveTopUp)
	r.Post("/wallets/topup/{code}/approve", h.walletHandler.ApproveTopUp)
	r.Post("/wallets/topup/reject", h.walletHandler.RejectTopUp)
	r.Post("/wallets/topup/{code}/reject", h.walletHandler.RejectTopUp)

	// Platform-wide Ledger & Transactions (All users)
	r.Get("/transactions", h.ListAllTransactions)
	r.Get("/ledger", h.ListAllTransactions)

	// Role Assignment via Supabase RPC
	r.Post("/roles/assign", h.AssignRole)
	r.Post("/roles/remove", h.RemoveRole)

	// Points Ledger & Adjustment (Admin)
	r.Get("/points/transactions", h.ListAllPointTransactions)
	r.Get("/points/ledger", h.ListAllPointTransactions)
	r.Post("/points/adjust", h.AdjustUserPoints)

	// Business Types & Extensible Merchant Models Registry
	r.Get("/business-types", h.ListBusinessTypes)
	r.Get("/roles", h.ListBusinessTypes)

	// Vendor Balance & Monitoring (DFLASH / Generic)
	r.Get("/vendor/balance", h.CheckVendorBalance)
	r.Get("/vendor/{vendor}/balance", h.CheckVendorBalance)
	r.Get("/vendor/status", h.CheckVendorStatus)
	r.Get("/vendor/{vendor}/status", h.CheckVendorStatus)

	return r
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	adminUser := middleware.GetUserFromContext(r.Context())
	adminEmail := "anonymous"
	if adminUser != nil {
		adminEmail = adminUser.Email
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":      "admin-service",
		"status":       "healthy",
		"architecture": "OOP Clean Architecture (Controller -> Service -> Repository)",
		"admin_email":  adminEmail,
		"uptime":       time.Since(adminStartTime).String(),
		"timestamp":    time.Now().Format(time.RFC3339),
	})
}

func (h *Handler) DashboardStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := h.walletHandler.GetStats()

	sbStatus := "connected"
	if h.sbClient != nil {
		if err := h.sbClient.Ping(); err != nil {
			sbStatus = "error: " + err.Error()
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"overview": stats,
			"system": map[string]interface{}{
				"database_status": sbStatus,
				"uptime":          time.Since(adminStartTime).String(),
				"memory_alloc_mb": float64(m.Alloc) / 1024 / 1024,
				"goroutines":      runtime.NumGoroutine(),
			},
		},
	})
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	roleFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("role")))
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

	users, err := h.adminService.ListUsers(roleFilter, search, token)
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
		"count":  len(users),
		"data":   users,
	})
}

func (h *Handler) GetUserDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := chi.URLParam(r, "id")

	detail, err := h.adminService.GetUserDetail(userID)
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
		"data":   detail,
	})
}

func (h *Handler) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := chi.URLParam(r, "id")

	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Format request tidak valid",
		})
		return
	}

	targetPwd := strings.TrimSpace(req.Password)
	if targetPwd == "" {
		targetPwd = strings.TrimSpace(req.NewPassword)
	}

	if len(targetPwd) < 6 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Password baru minimal 6 karakter",
		})
		return
	}

	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := h.adminService.ResetPassword(userID, targetPwd, token); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"user_id": userID,
		"message": "Password pengguna berhasil direset oleh Super Admin",
	})
}

func (h *Handler) UpdateUserEmail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := chi.URLParam(r, "id")

	var req UpdateEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Format request tidak valid",
		})
		return
	}

	newEmail := strings.ToLower(strings.TrimSpace(req.Email))
	if newEmail == "" || !strings.Contains(newEmail, "@") {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Alamat email baru tidak valid",
		})
		return
	}

	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := h.adminService.UpdateEmail(userID, newEmail, token); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"user_id":   userID,
		"new_email": newEmail,
		"message":   "Email pengguna berhasil diperbarui oleh Super Admin",
	})
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := chi.URLParam(r, "id")

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Format request tidak valid",
		})
		return
	}

	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := h.adminService.UpdateUser(userID, req, token); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"user_id": userID,
		"message": "Data pengguna berhasil diperbarui oleh Super Admin",
	})
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := chi.URLParam(r, "id")

	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := h.adminService.DeleteUser(userID, token); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"user_id": userID,
		"message": "Pengguna berhasil dihapus dari sistem oleh Super Admin",
	})
}

func (h *Handler) LockUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := chi.URLParam(r, "id")

	isLocked, err := h.adminService.LockUser(userID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"message":   "Dompet pengguna berhasil dibekukan (LOCKED)",
		"is_locked": isLocked,
	})
}

func (h *Handler) UnlockUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := chi.URLParam(r, "id")

	isLocked, err := h.adminService.UnlockUser(userID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"message":   "Dompet pengguna berhasil dibuka kembali (UNLOCKED)",
		"is_locked": isLocked,
	})
}

func (h *Handler) ListAllTransactions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	allTrxs, count := h.adminService.ListAllTransactions()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  count,
		"data":   allTrxs,
	})
}

func (h *Handler) AssignRole(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req RoleAssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))

	if req.Email == "" || req.Role == "" {
		http.Error(w, `{"status":"error","message":"Email dan Role wajib diisi"}`, http.StatusBadRequest)
		return
	}

	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	res, _ := h.adminService.AssignRole(req.Email, req.Role, token)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Role '%s' berhasil diberikan ke %s", req.Role, req.Email),
		"result":  res,
	})
}

func (h *Handler) RemoveRole(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req RoleAssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))

	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	res, _ := h.adminService.RemoveRole(req.Email, req.Role, token)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Role '%s' berhasil dicabut dari %s", req.Role, req.Email),
		"result":  res,
	})
}

func (h *Handler) ListAllPointTransactions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	allPts, count := h.adminService.ListAllPointTransactions()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  count,
		"data":   allPts,
	})
}

func (h *Handler) AdjustUserPoints(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	adminUser := middleware.GetUserFromContext(r.Context())
	if adminUser == nil || !adminUser.HasAnyRole("admin", "superadmin") {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    403,
			"message": "Akses ditolak. Fitur penyesuaian poin hanya dapat dilakukan oleh Super Admin.",
		})
		return
	}

	var req AdjustPointsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.TargetUserID == "" && req.UserID != "" {
		req.TargetUserID = req.UserID
	}
	if paramID := chi.URLParam(r, "id"); paramID != "" && req.TargetUserID == "" {
		req.TargetUserID = paramID
	}

	if req.TargetUserID == "" || req.Amount <= 0 || strings.TrimSpace(req.Reason) == "" {
		http.Error(w, `{"status":"error","message":"target_user_id, amount > 0, dan reason wajib diisi"}`, http.StatusBadRequest)
		return
	}

	ptTrx, err := h.adminService.AdjustPoints(req.TargetUserID, req.Type, req.Amount, req.Reason, adminUser.Email)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"message":     fmt.Sprintf("Penyesuaian %d Poin untuk user %s berhasil diproses", req.Amount, req.TargetUserID),
		"admin_by":    adminUser.Email,
		"transaction": ptTrx,
	})
}

func (h *Handler) ListBusinessTypes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	types := role.GlobalRegistry.GetAll()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(types),
		"data":   types,
	})
}

func (h *Handler) AdminTopUp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	adminUser := middleware.GetUserFromContext(r.Context())
	if adminUser == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusUnauthorized,
			"message": "Unauthorized: Super Admin login diperlukan",
		})
		return
	}

	var req AdminTopUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "Body request tidak valid: JSON diperlukan",
		})
		return
	}

	if req.TargetUserID == "" && req.UserID != "" {
		req.TargetUserID = req.UserID
	}
	// URL param fallback if endpoint is /users/{id}/topup
	paramID := chi.URLParam(r, "id")
	if paramID != "" && req.TargetUserID == "" {
		req.TargetUserID = paramID
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	res, err := h.adminService.TopUpUser(adminUser, token, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(res)
}

func (h *Handler) ListAllWallets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	users, err := h.adminService.ListUsers("", "", token)
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
		"count":  len(users),
		"data":   users,
	})
}

// CheckVendorBalance checks deposit balance on external vendor servers (e.g. DFLASH)
func (h *Handler) CheckVendorBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vendorCode := chi.URLParam(r, "vendor")
	if vendorCode == "" {
		vendorCode = r.URL.Query().Get("vendor")
		if vendorCode == "" {
			vendorCode = "DFLASH"
		}
	}

	bal, err := vendor.GetGlobalManager().CheckVendorBalance(r.Context(), vendorCode)
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
		"message": "Informasi saldo akun vendor berhasil diambil",
		"data":    bal,
	})
}

// CheckVendorStatus checks transaction status on external vendor servers (e.g. DFLASH) using ref_id
func (h *Handler) CheckVendorStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vendorCode := chi.URLParam(r, "vendor")
	if vendorCode == "" {
		vendorCode = r.URL.Query().Get("vendor")
		if vendorCode == "" {
			vendorCode = "DFLASH"
		}
	}

	refID := r.URL.Query().Get("ref_id")
	if refID == "" {
		refID = r.URL.Query().Get("refid")
	}
	if refID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Parameter ref_id wajib diisi (contoh: ?ref_id=TRX123456)",
		})
		return
	}

	res, err := vendor.GetGlobalManager().CheckVendorStatus(r.Context(), vendorCode, refID)
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
		"message": "Status transaksi vendor berhasil diperiksa",
		"data":    res,
	})
}

