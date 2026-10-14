package wallet

import "time"

// Point Transaction types
const (
	PointTypeCredit = "CREDIT" // Poin Masuk (Reward/Cashback/Penyesuaian Tambah)
	PointTypeDebit  = "DEBIT"  // Poin Keluar (Belanja/Tukar Poin/Penyesuaian Kurang)
)

// Point Transaction categories
const (
	PointCategoryReward     = "REWARD"     // Bonus poin dari transaksi/loyalty
	PointCategoryCashback   = "CASHBACK"   // Cashback 1% dari belanja
	PointCategoryRedeem     = "REDEEM"     // Penggunaan poin untuk diskon belanja
	PointCategoryAdjustment = "ADJUSTMENT" // Koreksi oleh Super Admin
)

// PointTransaction represents a point mutation ledger entry
type PointTransaction struct {
	ID              string    `json:"id"`
	TransactionCode string    `json:"transaction_code"`
	UserID          string    `json:"user_id"`
	Type            string    `json:"type"`     // CREDIT or DEBIT
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

// RedeemPointsRequest represents a request to spend DewaPoin for discounts
type RedeemPointsRequest struct {
	Amount      int64  `json:"amount"`
	Description string `json:"description"`
}

// PointAdjustRequest represents an administrative point adjustment request
type PointAdjustRequest struct {
	TargetUserID string `json:"target_user_id"`
	Type         string `json:"type"`   // CREDIT or DEBIT
	Amount       int64  `json:"amount"` // Jumlah poin
	Reason       string `json:"reason"` // Alasan penyesuaian (wajib)
}
