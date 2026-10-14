package wallet

import (
	"strings"
	"sync"
	"time"
)

// Repository defines the storage contract for wallets, transactions, and points
type Repository interface {
	GetWallet(userID string) (*Wallet, bool)
	GetOrCreateWallet(userID, email string) *Wallet
	SaveWallet(w *Wallet)
	GetAllWallets() []*Wallet
	SetLock(userID string, locked bool) (*Wallet, error)

	AddTransaction(trx *WalletTransaction)
	GetTransactionsByUser(userID string) []*WalletTransaction
	FindTransactionByCode(code string) (*WalletTransaction, string)
	IsReferenceRefunded(refCode string) bool
	GetAllTransactions() []*WalletTransaction

	AddPointTransaction(ptrx *PointTransaction)
	GetPointTransactionsByUser(userID string) []*PointTransaction
	GetAllPointTransactions() []*PointTransaction

	GetStats() map[string]interface{}
}

// MemoryRepository provides a high-performance thread-safe in-memory store with O(1) indexing
type MemoryRepository struct {
	mu sync.RWMutex

	wallets            map[string]*Wallet
	transactions       map[string][]*WalletTransaction
	transactionsByCode map[string]*WalletTransaction
	transactionOwners  map[string]string
	refundedRefs       map[string]bool

	pointTransactions map[string][]*PointTransaction
}

// NewMemoryRepository initializes and seeds the repository with sample data
func NewMemoryRepository() *MemoryRepository {
	repo := &MemoryRepository{
		wallets:            make(map[string]*Wallet, 16),
		transactions:       make(map[string][]*WalletTransaction, 16),
		transactionsByCode: make(map[string]*WalletTransaction, 64),
		transactionOwners:  make(map[string]string, 64),
		refundedRefs:       make(map[string]bool, 16),
		pointTransactions:  make(map[string][]*PointTransaction, 16),
	}

	repo.seedInitialData()
	return repo
}

func (r *MemoryRepository) seedInitialData() {
	defaultUserID := "d4d709fb-a348-4066-a352-ee3ed0e03fa8"
	repoWallet := &Wallet{
		UserID:    defaultUserID,
		UserEmail: "user@gmail.com",
		Role:      "user",
		Balance:   2450000,
		Points:    12500,
		IsLocked:  false,
		UpdatedAt: time.Now(),
	}
	r.wallets[defaultUserID] = repoWallet

	initialTrxs := []*WalletTransaction{
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
			Description:     "No. Meter: 5123 4567 8901 - Sukses terbit Stroom Token",
			ReferenceID:     "PLN-TOKEN-4821",
			Status:          "SUCCESS",
			CreatedAt:       time.Now().Add(-24 * time.Hour),
		},
	}

	for _, t := range initialTrxs {
		r.addTransactionInternal(t)
	}

	initialPoints := []*PointTransaction{
		{
			ID:              "pt-01",
			TransactionCode: "PTS-20261014-101",
			UserID:          defaultUserID,
			Type:            PointTypeCredit,
			Category:        PointCategoryCashback,
			Amount:          750,
			PointsBefore:    11750,
			PointsAfter:     12500,
			Title:           "Cashback Transaksi Paket Data Telkomsel",
			Description:     "Cashback 1% otomatis dari transaksi TRX-20261014-998101",
			ReferenceID:     "TRX-20261014-998101",
			Status:          "SUCCESS",
			CreatedAt:       time.Now().Add(-2 * time.Hour),
		},
		{
			ID:              "pt-02",
			TransactionCode: "PTS-20261014-102",
			UserID:          defaultUserID,
			Type:            PointTypeCredit,
			Category:        PointCategoryReward,
			Amount:          2015,
			PointsBefore:    9735,
			PointsAfter:     11750,
			Title:           "Reward Transaksi Token Listrik PLN",
			Description:     "Reward loyalty point atas pembayaran listrik",
			ReferenceID:     "TRX-20261014-482199",
			Status:          "SUCCESS",
			CreatedAt:       time.Now().Add(-24 * time.Hour),
		},
		{
			ID:              "pt-03",
			TransactionCode: "PTS-20261014-103",
			UserID:          defaultUserID,
			Type:            PointTypeDebit,
			Category:        PointCategoryRedeem,
			Amount:          5000,
			PointsBefore:    14735,
			PointsAfter:     9735,
			Title:           "Penukaran Diskon Belanja PPOB",
			Description:     "Kupon potongan harga Rp 5.000 belanja DewaMart",
			ReferenceID:     "RED-MART-5000",
			Status:          "SUCCESS",
			CreatedAt:       time.Now().Add(-48 * time.Hour),
		},
	}

	for _, pt := range initialPoints {
		r.pointTransactions[defaultUserID] = append(r.pointTransactions[defaultUserID], pt)
	}
}

func (r *MemoryRepository) GetWallet(userID string) (*Wallet, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	w, ok := r.wallets[userID]
	return w, ok
}

func (r *MemoryRepository) GetOrCreateWallet(userID, email string) *Wallet {
	r.mu.Lock()
	defer r.mu.Unlock()

	w, exists := r.wallets[userID]
	if !exists {
		role := "user"
		emailLower := strings.ToLower(email)
		if strings.Contains(emailLower, "superadmin") || strings.Contains(emailLower, "admin") {
			role = "superadmin"
		} else if strings.Contains(emailLower, "kasir") {
			role = "kasir"
		} else if strings.Contains(emailLower, "merchant") || strings.Contains(emailLower, "fnb") {
			role = "fnb"
		}

		w = &Wallet{
			UserID:    userID,
			UserEmail: email,
			Role:      role,
			Balance:   1500000,
			Points:    5000,
			IsLocked:  false,
			UpdatedAt: time.Now(),
		}
		r.wallets[userID] = w

		// Seed starter transaction
		initTrx := &WalletTransaction{
			ID:              "trx-init-" + userID[:8],
			TransactionCode: "TOP-INIT-" + time.Now().Format("20060102150405"),
			UserID:          userID,
			Type:            TypeCredit,
			Category:        CategoryTopup,
			Amount:          1500000,
			BalanceBefore:   0,
			BalanceAfter:    1500000,
			Title:           "Saldo Awal Registrasi Akun DewaPay",
			Description:     "Saldo sambutan selamat datang untuk verifikasi akun & transaksi perdana",
			Status:          "SUCCESS",
			CreatedAt:       time.Now(),
		}
		r.addTransactionInternal(initTrx)
	} else if email != "" && w.UserEmail == "" {
		w.UserEmail = email
	}
	return w
}

func (r *MemoryRepository) SaveWallet(w *Wallet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w.UpdatedAt = time.Now()
	r.wallets[w.UserID] = w
}

func (r *MemoryRepository) GetAllWallets() []*Wallet {
	r.mu.Lock()
	// Ensure standard accounts exist
	if _, ok := r.wallets["d4d709fb-a348-4066-a352-ee3ed0e03fa8"]; !ok {
		r.wallets["d4d709fb-a348-4066-a352-ee3ed0e03fa8"] = &Wallet{
			UserID: "d4d709fb-a348-4066-a352-ee3ed0e03fa8", UserEmail: "user@gmail.com", Balance: 2450000, Points: 12500, UpdatedAt: time.Now(),
		}
	}
	if _, ok := r.wallets["afef2ecd-e069-4a2f-b5c3-0c277ad46a8f"]; !ok {
		r.wallets["afef2ecd-e069-4a2f-b5c3-0c277ad46a8f"] = &Wallet{
			UserID: "afef2ecd-e069-4a2f-b5c3-0c277ad46a8f", UserEmail: "superadmin@gmail.com", Balance: 100000000, Points: 500000, UpdatedAt: time.Now(),
		}
	}
	if _, ok := r.wallets["c3d809ab-b123-4567-8901-ee2ed0e03fa9"]; !ok {
		r.wallets["c3d809ab-b123-4567-8901-ee2ed0e03fa9"] = &Wallet{
			UserID: "c3d809ab-b123-4567-8901-ee2ed0e03fa9", UserEmail: "kasir_dewa@gmail.com", Balance: 5200000, Points: 15000, UpdatedAt: time.Now(),
		}
	}
	if _, ok := r.wallets["e5f909cd-d345-6789-0123-ff4ed0e03fb1"]; !ok {
		r.wallets["e5f909cd-d345-6789-0123-ff4ed0e03fb1"] = &Wallet{
			UserID: "e5f909cd-d345-6789-0123-ff4ed0e03fb1", UserEmail: "merchant_fnb@gmail.com", Balance: 18450000, Points: 28000, UpdatedAt: time.Now(),
		}
	}
	r.mu.Unlock()

	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]*Wallet, 0, len(r.wallets))
	for _, w := range r.wallets {
		res = append(res, w)
	}
	return res
}

func (r *MemoryRepository) SetLock(userID string, locked bool) (*Wallet, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, exists := r.wallets[userID]
	if !exists {
		return nil, nil
	}
	w.IsLocked = locked
	w.UpdatedAt = time.Now()
	return w, nil
}

// addTransactionInternal assumes mutex is already locked
func (r *MemoryRepository) addTransactionInternal(trx *WalletTransaction) {
	r.transactions[trx.UserID] = append([]*WalletTransaction{trx}, r.transactions[trx.UserID]...)

	// O(1) indexing for lightning fast lookups
	if trx.TransactionCode != "" {
		codeKey := strings.ToLower(trx.TransactionCode)
		r.transactionsByCode[codeKey] = trx
		r.transactionOwners[codeKey] = trx.UserID
	}
	if trx.ID != "" {
		idKey := strings.ToLower(trx.ID)
		r.transactionsByCode[idKey] = trx
		r.transactionOwners[idKey] = trx.UserID
	}
	if trx.ReferenceID != "" {
		refKey := strings.ToLower(trx.ReferenceID)
		if _, exists := r.transactionsByCode[refKey]; !exists {
			r.transactionsByCode[refKey] = trx
			r.transactionOwners[refKey] = trx.UserID
		}
		if trx.Category == CategoryRefund {
			r.refundedRefs[refKey] = true
		}
	}
}

func (r *MemoryRepository) AddTransaction(trx *WalletTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addTransactionInternal(trx)
}

func (r *MemoryRepository) GetTransactionsByUser(userID string) []*WalletTransaction {
	r.mu.RLock()
	defer r.mu.RUnlock()
	trxs := r.transactions[userID]
	res := make([]*WalletTransaction, len(trxs))
	copy(res, trxs)
	return res
}

func (r *MemoryRepository) FindTransactionByCode(code string) (*WalletTransaction, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clean := strings.ToLower(strings.TrimSpace(code))
	if trx, ok := r.transactionsByCode[clean]; ok {
		return trx, r.transactionOwners[clean]
	}

	// Fallback iterate in case of edge reference
	for uid, trxs := range r.transactions {
		for _, t := range trxs {
			if strings.EqualFold(t.TransactionCode, code) || strings.EqualFold(t.ID, code) ||
				(t.ReferenceID != "" && strings.EqualFold(t.ReferenceID, code)) {
				return t, uid
			}
		}
	}
	return nil, ""
}

func (r *MemoryRepository) IsReferenceRefunded(refCode string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	clean := strings.ToLower(strings.TrimSpace(refCode))
	if r.refundedRefs[clean] {
		return true
	}

	// Comprehensive check across all refunds
	for _, trxs := range r.transactions {
		for _, t := range trxs {
			if t.Category == CategoryRefund && strings.EqualFold(t.ReferenceID, refCode) {
				return true
			}
		}
	}
	return false
}

func (r *MemoryRepository) GetAllTransactions() []*WalletTransaction {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := 0
	for _, trxs := range r.transactions {
		total += len(trxs)
	}

	allTrxs := make([]*WalletTransaction, 0, total)
	for _, trxs := range r.transactions {
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

func (r *MemoryRepository) AddPointTransaction(ptrx *PointTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pointTransactions[ptrx.UserID] = append([]*PointTransaction{ptrx}, r.pointTransactions[ptrx.UserID]...)
}

func (r *MemoryRepository) GetPointTransactionsByUser(userID string) []*PointTransaction {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pts := r.pointTransactions[userID]
	res := make([]*PointTransaction, len(pts))
	copy(res, pts)
	return res
}

func (r *MemoryRepository) GetAllPointTransactions() []*PointTransaction {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := 0
	for _, pts := range r.pointTransactions {
		total += len(pts)
	}

	allPts := make([]*PointTransaction, 0, total)
	for _, pts := range r.pointTransactions {
		allPts = append(allPts, pts...)
	}

	// Sort descending
	for i := 0; i < len(allPts)-1; i++ {
		for j := i + 1; j < len(allPts); j++ {
			if allPts[i].CreatedAt.Before(allPts[j].CreatedAt) {
				allPts[i], allPts[j] = allPts[j], allPts[i]
			}
		}
	}
	return allPts
}

func (r *MemoryRepository) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var totalBalance float64
	var totalPoints int64
	var lockedUsers int
	activeUsers := len(r.wallets)

	for _, w := range r.wallets {
		totalBalance += w.Balance
		totalPoints += w.Points
		if w.IsLocked {
			lockedUsers++
		}
	}

	var totalCreditAmt, totalDebitAmt float64
	var totalTrxCount, totalPointTrxCount int

	for _, trxs := range r.transactions {
		totalTrxCount += len(trxs)
		for _, t := range trxs {
			if t.Type == TypeCredit {
				totalCreditAmt += t.Amount
			} else {
				totalDebitAmt += t.Amount
			}
		}
	}

	for _, pts := range r.pointTransactions {
		totalPointTrxCount += len(pts)
	}

	return map[string]interface{}{
		"active_wallets":          activeUsers,
		"locked_wallets":          lockedUsers,
		"total_circulating_idr":   totalBalance,
		"formatted_total_balance": formatRupiah(int64(totalBalance)),
		"total_points_issued":     totalPoints,
		"total_transactions":      totalTrxCount,
		"total_credit_volume":     totalCreditAmt,
		"total_debit_volume":      totalDebitAmt,
		"total_point_mutations":   totalPointTrxCount,
		"net_cashflow":            totalCreditAmt - totalDebitAmt,
	}
}
