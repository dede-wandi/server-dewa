package wallet

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

// Handler represents the HTTP presentation layer for wallet & point services
type Handler struct {
	sbClient      *supabase.Client
	walletService WalletService
	pointService  PointService
}

// NewHandler constructs and wires the Wallet Handler with OOP services
func NewHandler(sbClient *supabase.Client) *Handler {
	repo := NewMemoryRepository()
	pointSvc := NewPointService(repo, sbClient)
	walletSvc := NewWalletService(repo, pointSvc, sbClient)

	return &Handler{
		sbClient:      sbClient,
		walletService: walletSvc,
		pointService:  pointSvc,
	}
}

// GetWalletService returns the underlying WalletService instance
func (h *Handler) GetWalletService() WalletService {
	return h.walletService
}

// GetPointService returns the underlying PointService instance
func (h *Handler) GetPointService() PointService {
	return h.pointService
}

// Routes registers all endpoints for wallet and points
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public ping/info/callback
	r.Get("/ping", h.Ping)
	r.Get("/info", h.Info)
	r.Post("/topup/callback", h.TopUpCallback)

	// User endpoints (requires login token)
	r.Get("/balance", h.GetBalance)
	r.Get("/history", h.GetHistory)
	r.Get("/transactions", h.GetHistory)
	r.Post("/pay", h.Pay)
	r.Post("/topup", h.TopUp)
	r.Post("/refund", h.Refund)

	// Points endpoints (requires login token)
	r.Get("/points", h.GetPointHistory)
	r.Get("/points/history", h.GetPointHistory)
	r.Post("/points/redeem", h.RedeemPoints)

	// Admin-only endpoints (requires admin or superadmin role)
	r.Group(func(admin chi.Router) {
		admin.Use(middleware.RequireRoles("admin", "superadmin"))
		admin.Post("/adjust", h.AdjustBalance)
		admin.Get("/admin/users", h.AdminListUserWallets)
		admin.Get("/topup/pending", h.GetPendingTopUps)
		admin.Post("/topup/approve", h.ApproveTopUp)
		admin.Post("/topup/{code}/approve", h.ApproveTopUp)
		admin.Post("/topup/reject", h.RejectTopUp)
		admin.Post("/topup/{code}/reject", h.RejectTopUp)
	})

	return r
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sbStatus := "connected"
	if h.sbClient != nil {
		if err := h.sbClient.Ping(); err != nil {
			sbStatus = "error: " + err.Error()
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":         "wallet-service",
		"status":          "healthy",
		"architecture":    "OOP Clean Architecture (Controller -> Service -> Repository)",
		"supabase_status": sbStatus,
		"timestamp":       time.Now().Format(time.RFC3339),
	})
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": "wallet-service",
		"version": "v2.0.0",
		"endpoints": map[string]string{
			"GET /balance":        "Ambil informasi saldo IDR & DewaPoin aktif",
			"GET /history":        "Riwayat mutasi saldo lengkap",
			"POST /pay":           "Pembayaran produk digital dengan saldo",
			"POST /topup":         "Deposit isi ulang saldo instan",
			"POST /refund":        "Pengembalian saldo atas transaksi gagal",
			"GET /points/history": "Riwayat mutasi reward & tukar poin",
			"POST /points/redeem": "Tukar DewaPoin untuk voucher diskon belanja",
			"POST /adjust":        "Koreksi saldo oleh Super Admin",
		},
	})
}

func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"status":"error","message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	wal, err := h.walletService.GetBalance(user.ID, user.Email, token)
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
		"data": map[string]interface{}{
			"user_id":         wal.UserID,
			"user_email":      wal.UserEmail,
			"balance":         wal.Balance,
			"formatted":       fmt.Sprintf("Rp %s", formatRupiah(int64(wal.Balance))),
			"points":          wal.Points,
			"formatted_coins": fmt.Sprintf("%d Poin", wal.Points),
			"is_locked":       wal.IsLocked,
			"updated_at":      wal.UpdatedAt.Format(time.RFC3339),
		},
	})
}

func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"status":"error","message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	trxs := h.walletService.GetHistory(user.ID)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(trxs),
		"data":   trxs,
	})
}

func (h *Handler) Pay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"status":"error","message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req PayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	trx, wal, err := h.walletService.Pay(user.ID, user.Email, token, req)
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
		"message":     "Pembayaran berhasil diproses!",
		"transaction": trx,
		"wallet": map[string]interface{}{
			"balance":         wal.Balance,
			"formatted":       fmt.Sprintf("Rp %s", formatRupiah(int64(wal.Balance))),
			"points":          wal.Points,
			"formatted_coins": fmt.Sprintf("%d Poin", wal.Points),
		},
	})
}

func (h *Handler) TopUp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"status":"error","message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req TopUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	trx, wal, err := h.walletService.TopUp(user.ID, user.Email, token, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "pending",
		"message": fmt.Sprintf("Permintaan top up sebesar Rp %s berhasil dibuat dengan status PENDING. Silakan lakukan pembayaran ke kode referensi atau tunggu persetujuan Admin.", formatRupiah(int64(req.Amount))),
		"transaction": trx,
		"wallet": map[string]interface{}{
			"balance":   wal.Balance,
			"formatted": fmt.Sprintf("Rp %s", formatRupiah(int64(wal.Balance))),
			"points":    wal.Points,
		},
	})
}

func (h *Handler) GetPendingTopUps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	pending := h.walletService.GetPendingTopUps()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(pending),
		"data":   pending,
	})
}

func (h *Handler) ApproveTopUp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	adminUser := middleware.GetUserFromContext(r.Context())
	if adminUser == nil {
		http.Error(w, `{"status":"error","message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req ApproveTopUpRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	paramCode := chi.URLParam(r, "code")
	if paramCode != "" && req.TransactionCode == "" {
		req.TransactionCode = paramCode
	}

	if req.TransactionCode == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "transaction_code wajib disertakan",
		})
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	trx, targetWallet, err := h.walletService.ApproveTopUp(adminUser, token, req.TransactionCode, req.Notes)
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
		"message":     fmt.Sprintf("Top up transaksi %s sebesar Rp %s berhasil disetujui dan saldo telah ditambahkan!", trx.TransactionCode, formatRupiah(int64(trx.Amount))),
		"transaction": trx,
		"target_user_wallet": map[string]interface{}{
			"user_id":        targetWallet.UserID,
			"user_email":     targetWallet.UserEmail,
			"balance_before": trx.BalanceBefore,
			"balance_after":  trx.BalanceAfter,
			"formatted":      fmt.Sprintf("Rp %s", formatRupiah(int64(targetWallet.Balance))),
		},
	})
}

func (h *Handler) RejectTopUp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	adminUser := middleware.GetUserFromContext(r.Context())
	if adminUser == nil {
		http.Error(w, `{"status":"error","message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req RejectTopUpRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	paramCode := chi.URLParam(r, "code")
	if paramCode != "" && req.TransactionCode == "" {
		req.TransactionCode = paramCode
	}

	if req.TransactionCode == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "transaction_code wajib disertakan",
		})
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	trx, err := h.walletService.RejectTopUp(adminUser, token, req.TransactionCode, req.Reason)
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
		"message":     fmt.Sprintf("Permintaan top up %s berhasil ditolak.", trx.TransactionCode),
		"transaction": trx,
	})
}

func (h *Handler) TopUpCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req TopUpCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Payload callback tidak valid: JSON diperlukan",
		})
		return
	}

	trx, targetWallet, err := h.walletService.ProcessTopUpCallback(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	res := map[string]interface{}{
		"status":      "success",
		"message":     "Callback pembayaran top up berhasil diproses",
		"transaction": trx,
	}
	if targetWallet != nil {
		res["target_user_wallet"] = map[string]interface{}{
			"user_id":       targetWallet.UserID,
			"user_email":    targetWallet.UserEmail,
			"balance_after": targetWallet.Balance,
			"formatted":     fmt.Sprintf("Rp %s", formatRupiah(int64(targetWallet.Balance))),
		}
	}

	json.NewEncoder(w).Encode(res)
}

func (h *Handler) Refund(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	caller := middleware.GetUserFromContext(r.Context())
	if caller == nil {
		http.Error(w, `{"status":"error","message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req RefundRequest
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

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	trx, targetWallet, err := h.walletService.Refund(caller, token, req)
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
		"message":     fmt.Sprintf("Dana sebesar Rp %s atas transaksi %s berhasil di-refund kembali ke saldo pengguna!", formatRupiah(int64(trx.Amount)), req.TransactionCode),
		"transaction": trx,
		"wallet": map[string]interface{}{
			"balance":   targetWallet.Balance,
			"formatted": fmt.Sprintf("Rp %s", formatRupiah(int64(targetWallet.Balance))),
		},
		"target_user_wallet": map[string]interface{}{
			"user_id":        targetWallet.UserID,
			"user_email":     targetWallet.UserEmail,
			"balance_before": trx.BalanceBefore,
			"balance_after":  trx.BalanceAfter,
			"formatted":      fmt.Sprintf("Rp %s", formatRupiah(int64(targetWallet.Balance))),
		},
	})
}

func (h *Handler) AdjustBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	adminUser := middleware.GetUserFromContext(r.Context())
	if adminUser == nil {
		http.Error(w, `{"status":"error","message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req AdjustRequest
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

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	trx, targetWallet, err := h.walletService.AdjustBalance(adminUser, token, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	actionVerb := "ditambahkan ke"
	if req.Type == TypeDebit {
		actionVerb = "dipotong dari"
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"message":     fmt.Sprintf("Saldo sebesar Rp %s berhasil %s dompet pengguna!", formatRupiah(int64(req.Amount)), actionVerb),
		"transaction": trx,
		"target_user_wallet": map[string]interface{}{
			"user_id":        targetWallet.UserID,
			"user_email":     targetWallet.UserEmail,
			"balance_before": trx.BalanceBefore,
			"balance_after":  trx.BalanceAfter,
			"formatted":      fmt.Sprintf("Rp %s", formatRupiah(int64(targetWallet.Balance))),
		},
	})
}

type AdminWalletUserItem struct {
	UserID    string  `json:"user_id"`
	UserEmail string  `json:"user_email"`
	Balance   float64 `json:"balance"`
	Points    int64   `json:"points"`
	IsLocked  bool    `json:"is_locked"`
	UpdatedAt string  `json:"updated_at"`
}

func (h *Handler) AdminListUserWallets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	wallets := h.walletService.GetAllWallets()

	userList := make([]AdminWalletUserItem, 0, len(wallets))
	for _, wal := range wallets {
		cleanRole := strings.ToLower(strings.TrimSpace(wal.Role))
		cleanEmail := strings.ToLower(strings.TrimSpace(wal.UserEmail))
		if cleanRole == "superadmin" || strings.Contains(cleanRole, "superadmin") || strings.Contains(cleanEmail, "superadmin") {
			continue // Strictly exclude superadmin
		}

		userList = append(userList, AdminWalletUserItem{
			UserID:    wal.UserID,
			UserEmail: wal.UserEmail,
			Balance:   wal.Balance,
			Points:    wal.Points,
			IsLocked:  wal.IsLocked,
			UpdatedAt: wal.UpdatedAt.Format(time.RFC3339),
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(userList),
		"data":   userList,
	})
}

func (h *Handler) GetPointHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"status":"error","message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	pts := h.pointService.GetHistory(user.ID)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(pts),
		"data":   pts,
	})
}

func (h *Handler) RedeemPoints(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"status":"error","message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req RedeemPointsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	ptrx, wal, err := h.pointService.RedeemPoints(user.ID, user.Email, token, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":            "success",
		"message":           fmt.Sprintf("Berhasil menukarkan %d DewaPoin untuk potongan belanja!", req.Amount),
		"point_transaction": ptrx,
		"wallet": map[string]interface{}{
			"user_id":   wal.UserID,
			"points":    wal.Points,
			"formatted": fmt.Sprintf("%d Poin", wal.Points),
		},
	})
}

// ═══════════════════════════════════════════════════════════════════
// PUBLIC OOP SERVICE DELEGATE METHODS (Used by Admin Module)
// ═══════════════════════════════════════════════════════════════════

func (h *Handler) GetAllTransactions() []*WalletTransaction {
	return h.walletService.GetAllTransactions()
}

func (h *Handler) GetAllWallets() []*Wallet {
	return h.walletService.GetAllWallets()
}

func (h *Handler) GetUserWallet(userID string) (*Wallet, []*WalletTransaction, error) {
	return h.walletService.GetUserWallet(userID)
}

func (h *Handler) SetUserLock(userID string, locked bool) (*Wallet, error) {
	return h.walletService.SetUserLock(userID, locked)
}

func (h *Handler) GetStats() map[string]interface{} {
	return h.walletService.GetStats()
}

func (h *Handler) GetAllPointTransactions() []*PointTransaction {
	return h.pointService.GetAllPointTransactions()
}

func (h *Handler) AdjustPoints(userID string, pType string, amount int64, reason string, adminEmail string) (*PointTransaction, error) {
	return h.pointService.AdjustPoints(userID, pType, amount, reason, adminEmail)
}
