package wallet

import (
	"strings"
	"sync"
	"time"

	"github.com/dede-wandi/server-dewa/internal/domain/role"
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

// NewMemoryRepository initializes a clean, real-time repository without dummy data
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		wallets:            make(map[string]*Wallet, 16),
		transactions:       make(map[string][]*WalletTransaction, 16),
		transactionsByCode: make(map[string]*WalletTransaction, 64),
		transactionOwners:  make(map[string]string, 64),
		refundedRefs:       make(map[string]bool, 16),
		pointTransactions:  make(map[string][]*PointTransaction, 16),
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
		def := role.GlobalRegistry.Resolve("", email)
		w = &Wallet{
			UserID:    userID,
			UserEmail: email,
			Role:      string(def.Role),
			Balance:   0,
			Points:    0,
			IsLocked:  false,
			UpdatedAt: time.Now(),
		}
		r.wallets[userID] = w
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
