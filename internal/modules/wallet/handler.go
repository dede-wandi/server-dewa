package wallet

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dede-wandi/server-dewa/internal/middleware"
	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

// Transaction categories
const (
	CategoryTopup       = "TOPUP"
	CategoryPayment     = "PAYMENT"
	CategoryRefund      = "REFUND"
	CategoryAdjustment  = "ADJUSTMENT"
	CategoryTransferIn  = "TRANSFER_IN"
	CategoryTransferOut = "TRANSFER_OUT"
)

// Transaction types
const (
	TypeCredit = "CREDIT" // Saldo Masuk
	TypeDebit  = "DEBIT"  // Saldo Keluar
)

type Wallet struct {
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	Balance   float64   `json:"balance"`
	Points    int64     `json:"points"`
	IsLocked  bool      `json:"is_locked"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WalletTransaction struct {
	ID              string    `json:"id"`
	TransactionCode string    `json:"transaction_code"`
	UserID          string    `json:"user_id"`
	Type            string    `json:"type"`     // CREDIT or DEBIT
	Category        string    `json:"category"` // TOPUP, PAYMENT, REFUND, ADJUSTMENT, etc.
	Amount          float64   `json:"amount"`
	BalanceBefore   float64   `json:"balance_before"`
	BalanceAfter    float64   `json:"balance_after"`
	Title           string    `json:"title"`       // Nama jelas transaksi
	Description     string    `json:"description"` // Keterangan rinci
	ReferenceID     string    `json:"reference_id,omitempty"`
	AdminID         string    `json:"admin_id,omitempty"`
	AdminEmail      string    `json:"admin_email,omitempty"`
	AdminNotes      string    `json:"admin_notes,omitempty"`
	Status          string    `json:"status"` // SUCCESS, FAILED
	CreatedAt       time.Time `json:"created_at"`
}

type PointTransaction struct {
	ID              string    `json:"id"`
	TransactionCode string    `json:"transaction_code"`
	UserID          string    `json:"user_id"`
	Type            string    `json:"type"`     // CREDIT (Poin Masuk) or DEBIT (Poin Keluar/Belanja)
	Category        string    `json:"category"` // REWARD, CASHBACK, REDEEM, ADJUSTMENT
	Amount          int64     `json:"amount"`
	PointsBefore    int64     `json:"points_before"`
	PointsAfter     int64     `json:"points_after"`
	Title           string    `json:"title"`       // e.g. "Cashback Belanja Pulsa", "Diskon Belanja Pakai Poin"
	Description     string    `json:"description"` // Keterangan transaksi poin
	ReferenceID     string    `json:"reference_id,omitempty"`
	Status          string    `json:"status"` // SUCCESS
	CreatedAt       time.Time `json:"created_at"`
}

type Handler struct {
	sbClient          *supabase.Client
	mu                sync.RWMutex
	wallets           map[string]*Wallet
	transactions      map[string][]*WalletTransaction
	pointTransactions map[string][]*PointTransaction
}

func NewHandler(sbClient *supabase.Client) *Handler {
	h := &Handler{
		sbClient:          sbClient,
		wallets:           make(map[string]*Wallet),
		transactions:      make(map[string][]*WalletTransaction),
		pointTransactions: make(map[string][]*PointTransaction),
	}

	// Pre-seed sample wallet for existing user (user@gmail.com)
	defaultUserID := "d4d709fb-a348-4066-a352-ee3ed0e03fa8"
	h.wallets[defaultUserID] = &Wallet{
		UserID:    defaultUserID,
		UserEmail: "user@gmail.com",
		Balance:   2450000,
		Points:    12500,
		IsLocked:  false,
		UpdatedAt: time.Now(),
	}

	h.transactions[defaultUserID] = []*WalletTransaction{
		{
			ID:              "trx-01",
			TransactionCode: "TRX-20261014-998101",
			UserID:          defaultUserID,
			Type:            TypeDebit,
			Category:        CategoryPayment,
			Amount:          75000,
			BalanceBefore:   2525000,
			BalanceAfter:    2450000,
			Title:           "Pembayaran Paket Data Telkomsel 50GB",
			Description:     "Pembelian kuota internet 0812-3456-7890 melalui DewaCell PPOB",
			ReferenceID:     "ORD-TSEL-50GB",
			Status:          "SUCCESS",
			CreatedAt:       time.Now().Add(-2 * time.Hour),
		},
		{
			ID:              "trx-02",
			TransactionCode: "TOP-20261014-880123",
			UserID:          defaultUserID,
			Type:            TypeCredit,
			Category:        CategoryTopup,
			Amount:          500000,
			BalanceBefore:   2025000,
			BalanceAfter:    2525000,
			Title:           "Top Up Saldo via BCA Virtual Account",
			Description:     "Deposit saldo berhasil diverifikasi otomatis oleh payment gateway",
			ReferenceID:     "VA-BCA-880123456789",
			Status:          "SUCCESS",
			CreatedAt:       time.Now().Add(-5 * time.Hour),
		},
		{
			ID:              "trx-03",
			TransactionCode: "TRX-20261014-482199",
			UserID:          defaultUserID,
			Type:            TypeDebit,
			Category:        CategoryPayment,
			Amount:          201500,
			BalanceBefore:   2226500,
			BalanceAfter:    2025000,
			Title:           "Pembayaran Token Listrik PLN Rp 200.000",
			Description:     "No. Meter: 5123 4567 8901 - Token Berhasil Terbit",
			ReferenceID:     "PLN-200K-01",
			Status:          "SUCCESS",
			CreatedAt:       time.Now().Add(-24 * time.Hour),
		},
	}

	h.pointTransactions[defaultUserID] = []*PointTransaction{
		{
			ID:              "pt-01",
			TransactionCode: "PTS-20261014-998101",
			UserID:          defaultUserID,
			Type:            TypeCredit,
			Category:        "CASHBACK",
			Amount:          750,
			PointsBefore:    11750,
			PointsAfter:     12500,
			Title:           "Reward Poin Cashback Paket Data",
			Description:     "Cashback 1% dari pembayaran paket data Telkomsel 50GB",
			ReferenceID:     "TRX-20261014-998101",
			Status:          "SUCCESS",
			CreatedAt:       time.Now().Add(-2 * time.Hour),
		},
		{
			ID:              "pt-02",
			TransactionCode: "PTS-20261014-482199",
			UserID:          defaultUserID,
			Type:            TypeCredit,
			Category:        "CASHBACK",
			Amount:          2015,
			PointsBefore:    9735,
			PointsAfter:     11750,
			Title:           "Reward Poin Cashback Token PLN",
			Description:     "Cashback 1% dari pembelian token listrik PLN Rp 200.000",
			ReferenceID:     "TRX-20261014-482199",
			Status:          "SUCCESS",
			CreatedAt:       time.Now().Add(-24 * time.Hour),
		},
		{
			ID:              "pt-03",
			TransactionCode: "PTS-20261013-112233",
			UserID:          defaultUserID,
			Type:            TypeDebit,
			Category:        "REDEEM",
			Amount:          5000,
			PointsBefore:    14735,
			PointsAfter:     9735,
			Title:           "Diskon Belanja Pakai DewaPoin",
			Description:     "Penukaran 5.000 Poin untuk potongan harga belanja sayur di DewaMart",
			ReferenceID:     "TRX-20261014-112233",
			Status:          "SUCCESS",
			CreatedAt:       time.Now().Add(-30 * time.Hour),
		},
		{
			ID:              "pt-04",
			TransactionCode: "PTS-20261001-000001",
			UserID:          defaultUserID,
			Type:            TypeCredit,
			Category:        "REWARD",
			Amount:          10000,
			PointsBefore:    0,
			PointsAfter:     10000,
			Title:           "Bonus Reward Pembukaan Akun",
			Description:     "Selamat datang di DewaCell! Bonus 10.000 DewaPoin untuk akun baru",
			Status:          "SUCCESS",
			CreatedAt:       time.Now().Add(-14 * 24 * time.Hour),
		},
	}

	return h
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public ping/info
	r.Get("/ping", h.Ping)
	r.Get("/info", h.Info)

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
	})

	return r
}

func (h *Handler) getOrCreateWalletLocked(userID, email string) *Wallet {
	w, exists := h.wallets[userID]
	if !exists {
		w = &Wallet{
			UserID:    userID,
			UserEmail: email,
			Balance:   500000, // Saldo awal pembukaan akun
			Points:    5000,
			IsLocked:  false,
			UpdatedAt: time.Now(),
		}
		h.wallets[userID] = w

		// Catat mutasi awal
		initTrx := &WalletTransaction{
			ID:              fmt.Sprintf("trx-init-%d", time.Now().UnixNano()),
			TransactionCode: fmt.Sprintf("TOP-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000),
			UserID:          userID,
			Type:            TypeCredit,
			Category:        CategoryTopup,
			Amount:          500000,
			BalanceBefore:   0,
			BalanceAfter:    500000,
			Title:           "Bonus Saldo Awal Akun DewaCell",
			Description:     "Saldo deposit awal resmi pembukaan akun DewaCell",
			Status:          "SUCCESS",
			CreatedAt:       time.Now(),
		}
		h.transactions[userID] = []*WalletTransaction{initTrx}
	} else if w.UserEmail == "" && email != "" {
		w.UserEmail = email
	}
	return w
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	userEmail := "anonymous"
	if user != nil {
		userEmail = user.Email
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":            "wallet-service",
		"status":             "healthy",
		"authenticated_user": userEmail,
		"timestamp":          time.Now().Format(time.RFC3339),
		"message":            "Wallet service is active with full transaction ledger & admin balance adjustment",
	})
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": "wallet-service",
		"version": "v1.2.0",
		"features": []string{
			"Cek saldo & poin real-time (tabel app_wallets)",
			"Buku besar mutasi kas (tabel app_wallet_transactions) transparan dengan nama transaksi jelas",
			"Pembayaran debit saldo (PPOB & FnB)",
			"Top up saldo instan",
			"Refund dana transaksi gagal / batal",
			"Penyesuaian saldo oleh Super Admin jika terjadi kekeliruan / salah input",
		},
		"endpoints": map[string]string{
			"GET /balance":     "Melihat saldo akun login",
			"GET /history":     "Riwayat mutasi saldo debit & kredit dengan nama jelas",
			"POST /pay":        "Pembayaran debit saldo",
			"POST /topup":      "Isi saldo credit",
			"POST /refund":     "Pengembalian saldo atas transaksi gagal",
			"POST /adjust":     "[Admin] Koreksi & penyesuaian saldo user (salah input/kompensasi)",
			"GET /admin/users": "[Admin] Daftar saldo seluruh user",
		},
	})
}

// GetBalance returns the balance of the logged-in user
func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"status":"error","message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	h.mu.Lock()
	wallet := h.getOrCreateWalletLocked(user.ID, user.Email)
	h.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "success",
		"user_id":         wallet.UserID,
		"user_email":      wallet.UserEmail,
		"balance":         wallet.Balance,
		"formatted":       fmt.Sprintf("Rp %s", formatRupiah(int64(wallet.Balance))),
		"points":          wallet.Points,
		"formatted_coins": fmt.Sprintf("%d Poin", wallet.Points),
		"is_locked":       wallet.IsLocked,
		"currency":        "IDR",
		"updated_at":      wallet.UpdatedAt.Format(time.RFC3339),
	})
}

// GetHistory returns user transaction mutations with clear names and descriptions
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"status":"error","message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	h.mu.RLock()
	trxs, exists := h.transactions[user.ID]
	if !exists {
		trxs = []*WalletTransaction{}
	}
	res := make([]*WalletTransaction, len(trxs))
	copy(res, trxs)
	h.mu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(res),
		"data":   res,
	})
}

type PayRequest struct {
	Amount      float64 `json:"amount"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	ReferenceID string  `json:"reference_id"`
}

// Pay handles debit payment from wallet
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

	if req.Amount <= 0 {
		http.Error(w, `{"status":"error","message":"Nominal pembayaran harus lebih besar dari 0"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		req.Title = "Pembayaran Transaksi DewaCell"
	}
	if strings.TrimSpace(req.Description) == "" {
		req.Description = fmt.Sprintf("Pembayaran sebesar Rp %s", formatRupiah(int64(req.Amount)))
	}
	if req.Category == "" {
		req.Category = CategoryPayment
	}

	// Coba panggil RPC Supabase jika database function sudah tersedia
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if h.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_user_id":      user.ID,
			"p_type":         TypeDebit,
			"p_category":     req.Category,
			"p_amount":       req.Amount,
			"p_title":        req.Title,
			"p_description":  req.Description,
			"p_reference_id": req.ReferenceID,
		}
		if respBytes, err := h.sbClient.CallRPC("process_app_wallet_mutation", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				h.mu.Lock()
				wallet := h.getOrCreateWalletLocked(user.ID, user.Email)
				balAfter, _ := rpcRes["balance_after"].(float64)
				wallet.Balance = balAfter
				wallet.Points += int64(req.Amount * 0.01)
				wallet.UpdatedAt = time.Now()

				trxCode, _ := rpcRes["transaction_code"].(string)
				trx := &WalletTransaction{
					ID:              fmt.Sprintf("%v", rpcRes["transaction_id"]),
					TransactionCode: trxCode,
					UserID:          user.ID,
					Type:            TypeDebit,
					Category:        req.Category,
					Amount:          req.Amount,
					BalanceBefore:   rpcRes["balance_before"].(float64),
					BalanceAfter:    balAfter,
					Title:           req.Title,
					Description:     req.Description,
					ReferenceID:     req.ReferenceID,
					Status:          "SUCCESS",
					CreatedAt:       time.Now(),
				}
				h.transactions[user.ID] = append([]*WalletTransaction{trx}, h.transactions[user.ID]...)
				h.mu.Unlock()

				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":      "success",
					"message":     "Pembayaran berhasil diproses!",
					"source":      "supabase_db",
					"transaction": trx,
					"wallet": map[string]interface{}{
						"balance":         wallet.Balance,
						"formatted":       fmt.Sprintf("Rp %s", formatRupiah(int64(wallet.Balance))),
						"points":          wallet.Points,
						"formatted_coins": fmt.Sprintf("%d Poin", wallet.Points),
					},
				})
				return
			}
		}
	}

	// Fallback in-memory jika function Supabase belum di-run di SQL Editor
	h.mu.Lock()
	defer h.mu.Unlock()

	wallet := h.getOrCreateWalletLocked(user.ID, user.Email)
	if wallet.IsLocked {
		http.Error(w, `{"status":"error","message":"Dompet sedang dikunci"}`, http.StatusBadRequest)
		return
	}

	if wallet.Balance < req.Amount {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "error",
			"code":            "INSUFFICIENT_BALANCE",
			"message":         fmt.Sprintf("Saldo tidak mencukupi. Saldo saat ini: Rp %s, Dibutuhkan: Rp %s", formatRupiah(int64(wallet.Balance)), formatRupiah(int64(req.Amount))),
			"current_balance": wallet.Balance,
			"required_amount": req.Amount,
		})
		return
	}

	balanceBefore := wallet.Balance
	wallet.Balance -= req.Amount
	balanceAfter := wallet.Balance
	ptsBefore := wallet.Points
	earnedPts := int64(req.Amount * 0.01) // 1% points reward
	wallet.Points += earnedPts
	wallet.UpdatedAt = time.Now()

	trxCode := fmt.Sprintf("TRX-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	trx := &WalletTransaction{
		ID:              fmt.Sprintf("trx-%d", time.Now().UnixNano()),
		TransactionCode: trxCode,
		UserID:          user.ID,
		Type:            TypeDebit,
		Category:        req.Category,
		Amount:          req.Amount,
		BalanceBefore:   balanceBefore,
		BalanceAfter:    balanceAfter,
		Title:           req.Title,
		Description:     req.Description,
		ReferenceID:     req.ReferenceID,
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}

	h.transactions[user.ID] = append([]*WalletTransaction{trx}, h.transactions[user.ID]...)

	// Catat riwayat mutasi poin reward
	if earnedPts > 0 {
		ptTrx := &PointTransaction{
			ID:              fmt.Sprintf("pt-%d", time.Now().UnixNano()),
			TransactionCode: fmt.Sprintf("PTS-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000),
			UserID:          user.ID,
			Type:            TypeCredit,
			Category:        "CASHBACK",
			Amount:          earnedPts,
			PointsBefore:    ptsBefore,
			PointsAfter:     wallet.Points,
			Title:           fmt.Sprintf("Reward 1%% Poin: %s", req.Title),
			Description:     fmt.Sprintf("Cashback reward poin dari transaksi %s", trxCode),
			ReferenceID:     trxCode,
			Status:          "SUCCESS",
			CreatedAt:       time.Now(),
		}
		h.pointTransactions[user.ID] = append([]*PointTransaction{ptTrx}, h.pointTransactions[user.ID]...)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"message":     "Pembayaran berhasil diproses!",
		"transaction": trx,
		"wallet": map[string]interface{}{
			"balance":         wallet.Balance,
			"formatted":       fmt.Sprintf("Rp %s", formatRupiah(int64(wallet.Balance))),
			"points":          wallet.Points,
			"formatted_coins": fmt.Sprintf("%d Poin", wallet.Points),
		},
	})
}

type TopUpRequest struct {
	Amount  float64 `json:"amount"`
	Channel string  `json:"channel"`
}

// TopUp adds money to user's balance
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

	if req.Amount <= 0 {
		http.Error(w, `{"status":"error","message":"Nominal top up harus lebih besar dari 0"}`, http.StatusBadRequest)
		return
	}

	channelName := "Virtual Account Bank"
	if req.Channel != "" {
		channelName = req.Channel
	}

	title := fmt.Sprintf("Top Up Saldo via %s", channelName)
	desc := fmt.Sprintf("Penambahan saldo dompet Rp %s berhasil diverifikasi", formatRupiah(int64(req.Amount)))

	// Coba panggil RPC Supabase jika database function sudah ada
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if h.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_user_id":     user.ID,
			"p_type":        TypeCredit,
			"p_category":    CategoryTopup,
			"p_amount":      req.Amount,
			"p_title":       title,
			"p_description": desc,
		}
		if respBytes, err := h.sbClient.CallRPC("process_app_wallet_mutation", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				h.mu.Lock()
				wallet := h.getOrCreateWalletLocked(user.ID, user.Email)
				balAfter, _ := rpcRes["balance_after"].(float64)
				wallet.Balance = balAfter
				wallet.UpdatedAt = time.Now()

				trxCode, _ := rpcRes["transaction_code"].(string)
				trx := &WalletTransaction{
					ID:              fmt.Sprintf("%v", rpcRes["transaction_id"]),
					TransactionCode: trxCode,
					UserID:          user.ID,
					Type:            TypeCredit,
					Category:        CategoryTopup,
					Amount:          req.Amount,
					BalanceBefore:   rpcRes["balance_before"].(float64),
					BalanceAfter:    balAfter,
					Title:           title,
					Description:     desc,
					Status:          "SUCCESS",
					CreatedAt:       time.Now(),
				}
				h.transactions[user.ID] = append([]*WalletTransaction{trx}, h.transactions[user.ID]...)
				h.mu.Unlock()

				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":      "success",
					"message":     "Top up saldo berhasil!",
					"source":      "supabase_db",
					"transaction": trx,
					"wallet": map[string]interface{}{
						"balance":   wallet.Balance,
						"formatted": fmt.Sprintf("Rp %s", formatRupiah(int64(wallet.Balance))),
						"points":    wallet.Points,
					},
				})
				return
			}
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	wallet := h.getOrCreateWalletLocked(user.ID, user.Email)
	balanceBefore := wallet.Balance
	wallet.Balance += req.Amount
	balanceAfter := wallet.Balance
	wallet.UpdatedAt = time.Now()

	trxCode := fmt.Sprintf("TOP-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	trx := &WalletTransaction{
		ID:              fmt.Sprintf("top-%d", time.Now().UnixNano()),
		TransactionCode: trxCode,
		UserID:          user.ID,
		Type:            TypeCredit,
		Category:        CategoryTopup,
		Amount:          req.Amount,
		BalanceBefore:   balanceBefore,
		BalanceAfter:    balanceAfter,
		Title:           title,
		Description:     desc,
		ReferenceID:     fmt.Sprintf("VA-%d", time.Now().Unix()),
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}

	h.transactions[user.ID] = append([]*WalletTransaction{trx}, h.transactions[user.ID]...)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"message":     "Top up saldo berhasil!",
		"transaction": trx,
		"wallet": map[string]interface{}{
			"balance":   wallet.Balance,
			"formatted": fmt.Sprintf("Rp %s", formatRupiah(int64(wallet.Balance))),
			"points":    wallet.Points,
		},
	})
}

type RefundRequest struct {
	TransactionCode string  `json:"transaction_code"`
	Amount          float64 `json:"amount,omitempty"`
	Reason          string  `json:"reason"`
	TargetUserID    string  `json:"target_user_id,omitempty"`
}

// Refund refunds a failed/cancelled transaction
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

	req.TransactionCode = strings.TrimSpace(req.TransactionCode)
	if req.TransactionCode == "" {
		http.Error(w, `{"status":"error","message":"Kode transaksi (transaction_code) wajib disertakan"}`, http.StatusBadRequest)
		return
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "Transaksi gagal diproses oleh sistem / provider partner"
	}

	isAdmin := caller.HasAnyRole("admin", "superadmin")

	// 1. Coba panggil RPC Supabase jika database function sudah ada
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if h.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_transaction_code": req.TransactionCode,
			"p_reason":           reason,
		}
		if respBytes, err := h.sbClient.CallRPC("refund_app_wallet_transaction", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				trxCode, _ := rpcRes["transaction_code"].(string)
				balAfter, _ := rpcRes["balance_after"].(float64)
				balBefore, _ := rpcRes["balance_before"].(float64)
				amt, _ := rpcRes["amount"].(float64)
				targetUID, _ := rpcRes["user_id"].(string)
				if targetUID == "" {
					targetUID = caller.ID
				}

				h.mu.Lock()
				targetWallet := h.getOrCreateWalletLocked(targetUID, "")
				targetWallet.Balance = balAfter
				targetWallet.UpdatedAt = time.Now()

				trx := &WalletTransaction{
					ID:              fmt.Sprintf("%v", rpcRes["transaction_id"]),
					TransactionCode: trxCode,
					UserID:          targetUID,
					Type:            TypeCredit,
					Category:        CategoryRefund,
					Amount:          amt,
					BalanceBefore:   balBefore,
					BalanceAfter:    balAfter,
					Title:           rpcRes["title"].(string),
					Description:     rpcRes["description"].(string),
					ReferenceID:     req.TransactionCode,
					AdminID:         caller.ID,
					AdminEmail:      caller.Email,
					AdminNotes:      reason,
					Status:          "SUCCESS",
					CreatedAt:       time.Now(),
				}
				h.transactions[targetUID] = append([]*WalletTransaction{trx}, h.transactions[targetUID]...)
				h.mu.Unlock()

				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":      "success",
					"message":     fmt.Sprintf("Dana Rp %s atas transaksi %s berhasil di-refund!", formatRupiah(int64(amt)), req.TransactionCode),
					"source":      "supabase_db",
					"transaction": trx,
					"wallet": map[string]interface{}{
						"balance":   balAfter,
						"formatted": fmt.Sprintf("Rp %s", formatRupiah(int64(balAfter))),
					},
					"target_user_wallet": map[string]interface{}{
						"user_id":   targetUID,
						"balance":   balAfter,
						"formatted": fmt.Sprintf("Rp %s", formatRupiah(int64(balAfter))),
					},
				})
				return
			}
		}
	}

	// 2. Fallback in-memory: Cari transaksi asli di SELURUH TRANSAKSI (lintas pengguna)
	h.mu.Lock()
	defer h.mu.Unlock()

	var origTrx *WalletTransaction
	var targetUserID string

	for uid, trxs := range h.transactions {
		for _, t := range trxs {
			if strings.EqualFold(t.TransactionCode, req.TransactionCode) ||
				strings.EqualFold(t.ID, req.TransactionCode) ||
				(t.ReferenceID != "" && strings.EqualFold(t.ReferenceID, req.TransactionCode)) {
				origTrx = t
				targetUserID = uid
				break
			}
		}
		if origTrx != nil {
			break
		}
	}

	// Jika pemanggil bukan admin, pastikan transaksi milik dia sendiri
	if !isAdmin && origTrx != nil && targetUserID != caller.ID {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    403,
			"message": "Anda tidak memiliki izin untuk me-refund transaksi milik pengguna lain",
		})
		return
	}

	// Tentukan target user ID jika transaksi tidak ditemukan di memori
	if targetUserID == "" {
		if req.TargetUserID != "" {
			targetUserID = req.TargetUserID
		} else {
			targetUserID = caller.ID
		}
	}

	// Validasi & ambil nominal refund
	refundAmount := req.Amount
	origTitle := req.TransactionCode
	if origTrx != nil {
		if origTrx.Type != TypeDebit {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Hanya transaksi DEBIT (pembayaran/pengeluaran) yang dapat di-refund",
			})
			return
		}

		// Pastikan transaksi belum pernah di-refund sebelumnya
		for _, t := range h.transactions[targetUserID] {
			if t.Category == CategoryRefund &&
				(strings.EqualFold(t.ReferenceID, origTrx.TransactionCode) || strings.EqualFold(t.ReferenceID, origTrx.ID)) {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":  "error",
					"message": fmt.Sprintf("Transaksi %s sudah pernah di-refund sebelumnya!", origTrx.TransactionCode),
				})
				return
			}
		}

		refundAmount = origTrx.Amount
		origTitle = origTrx.Title
	} else if refundAmount <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Transaksi '%s' tidak ditemukan di buku besar dan nominal refund tidak diisi.", req.TransactionCode),
		})
		return
	}

	// KREDITKAN KEMBALI UANG KE SALDO TARGET USER
	targetWallet := h.getOrCreateWalletLocked(targetUserID, "")
	balanceBefore := targetWallet.Balance
	targetWallet.Balance += refundAmount
	balanceAfter := targetWallet.Balance
	targetWallet.UpdatedAt = time.Now()

	refundCode := fmt.Sprintf("REF-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	refundTrx := &WalletTransaction{
		ID:              fmt.Sprintf("ref-%d", time.Now().UnixNano()),
		TransactionCode: refundCode,
		UserID:          targetUserID,
		Type:            TypeCredit,
		Category:        CategoryRefund,
		Amount:          refundAmount,
		BalanceBefore:   balanceBefore,
		BalanceAfter:    balanceAfter,
		Title:           fmt.Sprintf("Refund Dana: %s", origTitle),
		Description:     fmt.Sprintf("Pengembalian dana saldo atas transaksi %s. Alasan: %s", req.TransactionCode, reason),
		ReferenceID:     req.TransactionCode,
		AdminID:         caller.ID,
		AdminEmail:      caller.Email,
		AdminNotes:      reason,
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}

	h.transactions[targetUserID] = append([]*WalletTransaction{refundTrx}, h.transactions[targetUserID]...)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"message":     fmt.Sprintf("Dana sebesar Rp %s atas transaksi %s berhasil di-refund kembali ke saldo pengguna!", formatRupiah(int64(refundAmount)), req.TransactionCode),
		"transaction": refundTrx,
		"wallet": map[string]interface{}{
			"balance":   balanceAfter,
			"formatted": fmt.Sprintf("Rp %s", formatRupiah(int64(balanceAfter))),
		},
		"target_user_wallet": map[string]interface{}{
			"user_id":        targetWallet.UserID,
			"user_email":     targetWallet.UserEmail,
			"balance_before": balanceBefore,
			"balance_after":  balanceAfter,
			"formatted":      fmt.Sprintf("Rp %s", formatRupiah(int64(balanceAfter))),
		},
	})
}

type AdjustRequest struct {
	TargetUserID string  `json:"target_user_id"`
	Type         string  `json:"type"`   // CREDIT or DEBIT
	Amount       float64 `json:"amount"` // Nominal uang
	Reason       string  `json:"reason"` // Alasan koreksi (wajib)
	AdminNotes   string  `json:"admin_notes,omitempty"`
}

// AdjustBalance allows Super Admin / Admin to adjust a user's balance
// for error corrections, wrong inputs, or compensation
func (h *Handler) AdjustBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	adminUser := middleware.GetUserFromContext(r.Context())
	if adminUser == nil || !adminUser.HasAnyRole("admin", "superadmin") {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    403,
			"message": "Akses ditolak. Fitur penyesuaian saldo ini hanya dapat dilakukan oleh Super Admin.",
		})
		return
	}

	var req AdjustRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.TargetUserID) == "" {
		http.Error(w, `{"status":"error","message":"target_user_id wajib disertakan"}`, http.StatusBadRequest)
		return
	}

	req.Type = strings.ToUpper(strings.TrimSpace(req.Type))
	if req.Type != TypeCredit && req.Type != TypeDebit {
		http.Error(w, `{"status":"error","message":"Tipe mutasi tidak valid. Harus 'CREDIT' (tambah saldo) atau 'DEBIT' (potong saldo)"}`, http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, `{"status":"error","message":"Nominal penyesuaian harus lebih besar dari 0"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Reason) == "" {
		http.Error(w, `{"status":"error","message":"Alasan penyesuaian saldo (reason) wajib diisi untuk kepatuhan audit pembukuan"}`, http.StatusBadRequest)
		return
	}

	// Coba panggil RPC Supabase jika database function sudah ada
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if h.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_target_user_id": req.TargetUserID,
			"p_type":           req.Type,
			"p_amount":         req.Amount,
			"p_reason":         req.Reason,
			"p_admin_id":       adminUser.ID,
			"p_admin_notes":    req.Reason,
		}
		if respBytes, err := h.sbClient.CallRPC("admin_adjust_app_wallet", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				h.mu.Lock()
				targetWallet := h.getOrCreateWalletLocked(req.TargetUserID, "")
				balBefore, _ := rpcRes["balance_before"].(float64)
				balAfter, _ := rpcRes["balance_after"].(float64)
				targetWallet.Balance = balAfter
				targetWallet.UpdatedAt = time.Now()

				trxCode, _ := rpcRes["transaction_code"].(string)
				trx := &WalletTransaction{
					ID:              fmt.Sprintf("%v", rpcRes["transaction_id"]),
					TransactionCode: trxCode,
					UserID:          req.TargetUserID,
					Type:            req.Type,
					Category:        CategoryAdjustment,
					Amount:          req.Amount,
					BalanceBefore:   balBefore,
					BalanceAfter:    balAfter,
					Title:           rpcRes["title"].(string),
					Description:     rpcRes["description"].(string),
					AdminID:         adminUser.ID,
					AdminEmail:      adminUser.Email,
					AdminNotes:      req.Reason,
					Status:          "SUCCESS",
					CreatedAt:       time.Now(),
				}
				h.transactions[req.TargetUserID] = append([]*WalletTransaction{trx}, h.transactions[req.TargetUserID]...)
				h.mu.Unlock()

				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":      "success",
					"message":     fmt.Sprintf("Penyesuaian saldo sebesar Rp %s untuk user %s berhasil dicatat di Supabase DB!", formatRupiah(int64(req.Amount)), req.TargetUserID),
					"source":      "supabase_db",
					"admin_by":    adminUser.Email,
					"transaction": trx,
					"target_user_wallet": map[string]interface{}{
						"user_id":        targetWallet.UserID,
						"balance_before": balBefore,
						"balance_after":  balAfter,
						"formatted":      fmt.Sprintf("Rp %s", formatRupiah(int64(balAfter))),
					},
				})
				return
			}
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	targetWallet := h.getOrCreateWalletLocked(req.TargetUserID, "")
	balanceBefore := targetWallet.Balance

	var balanceAfter float64
	var title string
	var desc string

	if req.Type == TypeCredit {
		balanceAfter = balanceBefore + req.Amount
		title = "Penyesuaian Saldo oleh Super Admin (Kredit/Penambahan)"
		desc = fmt.Sprintf("Penambahan saldo sebesar Rp %s oleh Admin (%s). Alasan: %s", formatRupiah(int64(req.Amount)), adminUser.Email, req.Reason)
	} else {
		if balanceBefore < req.Amount {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":          "error",
				"code":            "INSUFFICIENT_BALANCE",
				"message":         fmt.Sprintf("Gagal memotong saldo: Saldo target user (Rp %s) tidak mencukupi untuk pemotongan Rp %s", formatRupiah(int64(balanceBefore)), formatRupiah(int64(req.Amount))),
				"current_balance": balanceBefore,
				"requested_cut":   req.Amount,
			})
			return
		}
		balanceAfter = balanceBefore - req.Amount
		title = "Penyesuaian Saldo oleh Super Admin (Debet/Koreksi Salah Input)"
		desc = fmt.Sprintf("Pemotongan saldo sebesar Rp %s karena koreksi pembukuan oleh Admin (%s). Alasan: %s", formatRupiah(int64(req.Amount)), adminUser.Email, req.Reason)
	}

	targetWallet.Balance = balanceAfter
	targetWallet.UpdatedAt = time.Now()

	adjCode := fmt.Sprintf("ADJ-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	trx := &WalletTransaction{
		ID:              fmt.Sprintf("adj-%d", time.Now().UnixNano()),
		TransactionCode: adjCode,
		UserID:          req.TargetUserID,
		Type:            req.Type,
		Category:        CategoryAdjustment,
		Amount:          req.Amount,
		BalanceBefore:   balanceBefore,
		BalanceAfter:    balanceAfter,
		Title:           title,
		Description:     desc,
		AdminID:         adminUser.ID,
		AdminEmail:      adminUser.Email,
		AdminNotes:      req.Reason,
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}

	h.transactions[req.TargetUserID] = append([]*WalletTransaction{trx}, h.transactions[req.TargetUserID]...)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"message":     fmt.Sprintf("Penyesuaian saldo sebesar Rp %s untuk user %s berhasil dicatat di buku besar mutasi!", formatRupiah(int64(req.Amount)), req.TargetUserID),
		"admin_by":    adminUser.Email,
		"transaction": trx,
		"target_user_wallet": map[string]interface{}{
			"user_id":        targetWallet.UserID,
			"balance_before": balanceBefore,
			"balance_after":  balanceAfter,
			"formatted":      fmt.Sprintf("Rp %s", formatRupiah(int64(balanceAfter))),
		},
	})
}

// AdminListUserWallets allows Super Admin to list all users and their balances
func (h *Handler) AdminListUserWallets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	h.mu.Lock()
	// Pastikan user-user utama terdaftar
	h.getOrCreateWalletLocked("d4d709fb-a348-4066-a352-ee3ed0e03fa8", "user@gmail.com")
	h.getOrCreateWalletLocked("afef2ecd-e069-4a2f-b5c3-0c277ad46a8f", "superadmin@gmail.com")
	h.getOrCreateWalletLocked("c3d809ab-b123-4567-8901-ee2ed0e03fa9", "kasir_dewa@gmail.com")
	h.getOrCreateWalletLocked("e5f909cd-d345-6789-0123-ff4ed0e03fb1", "merchant_fnb@gmail.com")
	h.mu.Unlock()

	h.mu.RLock()
	defer h.mu.RUnlock()

	userList := make([]map[string]interface{}, 0, len(h.wallets))
	for _, wal := range h.wallets {
		userList = append(userList, map[string]interface{}{
			"user_id":    wal.UserID,
			"user_email": wal.UserEmail,
			"balance":    wal.Balance,
			"formatted":  fmt.Sprintf("Rp %s", formatRupiah(int64(wal.Balance))),
			"points":     wal.Points,
			"is_locked":  wal.IsLocked,
			"updated_at": wal.UpdatedAt.Format(time.RFC3339),
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(userList),
		"data":   userList,
	})
}

func (h *Handler) GetAllTransactions() []*WalletTransaction {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var allTrxs []*WalletTransaction
	for _, trxs := range h.transactions {
		allTrxs = append(allTrxs, trxs...)
	}

	// Sort descending by CreatedAt
	for i := 0; i < len(allTrxs)-1; i++ {
		for j := i + 1; j < len(allTrxs); j++ {
			if allTrxs[i].CreatedAt.Before(allTrxs[j].CreatedAt) {
				allTrxs[i], allTrxs[j] = allTrxs[j], allTrxs[i]
			}
		}
	}

	return allTrxs
}

func (h *Handler) GetAllWallets() []*Wallet {
	h.mu.Lock()
	h.getOrCreateWalletLocked("d4d709fb-a348-4066-a352-ee3ed0e03fa8", "user@gmail.com")
	h.getOrCreateWalletLocked("afef2ecd-e069-4a2f-b5c3-0c277ad46a8f", "superadmin@gmail.com")
	h.getOrCreateWalletLocked("c3d809ab-b123-4567-8901-ee2ed0e03fa9", "kasir_dewa@gmail.com")
	h.getOrCreateWalletLocked("e5f909cd-d345-6789-0123-ff4ed0e03fb1", "merchant_fnb@gmail.com")
	h.mu.Unlock()

	h.mu.RLock()
	defer h.mu.RUnlock()

	wallets := make([]*Wallet, 0, len(h.wallets))
	for _, w := range h.wallets {
		wallets = append(wallets, w)
	}
	return wallets
}

func (h *Handler) GetUserWallet(userID string) (*Wallet, []*WalletTransaction, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	w, exists := h.wallets[userID]
	if !exists {
		return nil, nil, fmt.Errorf("user with ID %s not found", userID)
	}
	trxs := h.transactions[userID]
	return w, trxs, nil
}

func (h *Handler) SetUserLock(userID string, locked bool) (*Wallet, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	w, exists := h.wallets[userID]
	if !exists {
		return nil, fmt.Errorf("user with ID %s not found", userID)
	}
	w.IsLocked = locked
	w.UpdatedAt = time.Now()
	return w, nil
}

func (h *Handler) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var totalBalance float64
	var totalPoints int64
	for _, w := range h.wallets {
		totalBalance += w.Balance
		totalPoints += w.Points
	}

	var totalTrxs int
	for _, trxs := range h.transactions {
		totalTrxs += len(trxs)
	}

	return map[string]interface{}{
		"total_users":                  len(h.wallets),
		"total_circulating_balance":    totalBalance,
		"formatted_circulating_balance": fmt.Sprintf("Rp %s", FormatRupiah(int64(totalBalance))),
		"total_points":                 totalPoints,
		"total_transactions":           totalTrxs,
	}
}

func FormatRupiah(n int64) string {
	return formatRupiah(n)
}

func formatRupiah(n int64) string {
	str := fmt.Sprintf("%d", n)
	if n < 0 {
		str = fmt.Sprintf("%d", -n)
	}
	var res []byte
	length := len(str)
	for i, c := range str {
		if (length-i)%3 == 0 && i != 0 {
			res = append(res, '.')
		}
		res = append(res, byte(c))
	}
	if n < 0 {
		return "-" + string(res)
	}
	return string(res)
}

func (h *Handler) GetPointHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"status":"error","message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	h.mu.RLock()
	pts, exists := h.pointTransactions[user.ID]
	if !exists {
		pts = []*PointTransaction{}
	}
	var ptsBalance int64 = 0
	if wal, ok := h.wallets[user.ID]; ok {
		ptsBalance = wal.Points
	}
	res := make([]*PointTransaction, len(pts))
	copy(res, pts)
	h.mu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "success",
		"points_balance":  ptsBalance,
		"formatted_coins": fmt.Sprintf("%d Poin", ptsBalance),
		"point_value_idr": ptsBalance, // 1 Poin bernilai Rp 1,-
		"count":           len(res),
		"data":            res,
	})
}

type RedeemPointsRequest struct {
	Points      int64  `json:"points"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
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

	if req.Points <= 0 {
		http.Error(w, `{"status":"error","message":"Jumlah poin yang ditukarkan harus lebih dari 0"}`, http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	wallet := h.getOrCreateWalletLocked(user.ID, user.Email)
	if wallet.Points < req.Points {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "error",
			"code":            "INSUFFICIENT_POINTS",
			"message":         fmt.Sprintf("Poin tidak mencukupi. Poin Anda: %d, dibutuhkan: %d Poin", wallet.Points, req.Points),
			"current_points":  wallet.Points,
			"required_points": req.Points,
		})
		return
	}

	ptsBefore := wallet.Points
	wallet.Points -= req.Points
	ptsAfter := wallet.Points
	wallet.UpdatedAt = time.Now()

	title := "Penukaran Poin untuk Diskon Belanja"
	if req.Title != "" {
		title = req.Title
	}
	desc := fmt.Sprintf("Penukaran %d DewaPoin senilai Rp %s untuk potongan harga", req.Points, formatRupiah(req.Points))
	if req.Description != "" {
		desc = req.Description
	}

	ptCode := fmt.Sprintf("PTS-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	ptTrx := &PointTransaction{
		ID:              fmt.Sprintf("pt-%d", time.Now().UnixNano()),
		TransactionCode: ptCode,
		UserID:          user.ID,
		Type:            TypeDebit,
		Category:        "REDEEM",
		Amount:          req.Points,
		PointsBefore:    ptsBefore,
		PointsAfter:     ptsAfter,
		Title:           title,
		Description:     desc,
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}

	h.pointTransactions[user.ID] = append([]*PointTransaction{ptTrx}, h.pointTransactions[user.ID]...)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "success",
		"message":         fmt.Sprintf("%d Poin berhasil ditukarkan untuk belanja!", req.Points),
		"points_redeemed": req.Points,
		"points_balance":  ptsAfter,
		"transaction":     ptTrx,
	})
}

func (h *Handler) GetAllPointTransactions() []*PointTransaction {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var allTrxs []*PointTransaction
	for _, trxs := range h.pointTransactions {
		allTrxs = append(allTrxs, trxs...)
	}

	// Sort descending by CreatedAt
	for i := 0; i < len(allTrxs)-1; i++ {
		for j := i + 1; j < len(allTrxs); j++ {
			if allTrxs[i].CreatedAt.Before(allTrxs[j].CreatedAt) {
				allTrxs[i], allTrxs[j] = allTrxs[j], allTrxs[i]
			}
		}
	}

	return allTrxs
}

func (h *Handler) AdjustPoints(userID string, pType string, amount int64, reason string, adminEmail string) (*PointTransaction, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	wal, exists := h.wallets[userID]
	if !exists {
		return nil, fmt.Errorf("user ID %s not found", userID)
	}

	ptsBefore := wal.Points
	var ptsAfter int64
	var title string

	pType = strings.ToUpper(strings.TrimSpace(pType))
	if pType == TypeCredit {
		ptsAfter = ptsBefore + amount
		title = fmt.Sprintf("Penyesuaian Tambah Poin oleh Super Admin (%s)", adminEmail)
	} else {
		if ptsBefore < amount {
			return nil, fmt.Errorf("poin user (%d) tidak mencukupi untuk pemotongan %d poin", ptsBefore, amount)
		}
		ptsAfter = ptsBefore - amount
		title = fmt.Sprintf("Koreksi Pemotongan Poin oleh Super Admin (%s)", adminEmail)
	}

	wal.Points = ptsAfter
	wal.UpdatedAt = time.Now()

	ptCode := fmt.Sprintf("PTS-ADJ-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	ptTrx := &PointTransaction{
		ID:              fmt.Sprintf("pt-adj-%d", time.Now().UnixNano()),
		TransactionCode: ptCode,
		UserID:          userID,
		Type:            pType,
		Category:        "ADJUSTMENT",
		Amount:          amount,
		PointsBefore:    ptsBefore,
		PointsAfter:     ptsAfter,
		Title:           title,
		Description:     reason,
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}

	h.pointTransactions[userID] = append([]*PointTransaction{ptTrx}, h.pointTransactions[userID]...)
	return ptTrx, nil
}


