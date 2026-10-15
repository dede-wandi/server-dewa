package wallet

import "time"

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

// Wallet represents a user's digital wallet in DewaCell
type Wallet struct {
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	Role      string    `json:"role"`
	Balance   float64   `json:"balance"`
	Points    int64     `json:"points"`
	IsLocked  bool      `json:"is_locked"`
	IsPINSet  bool      `json:"is_pin_set"`
	PINHash   string    `json:"-"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WalletTransaction represents a single balance mutation in the platform ledger
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
	Channel         string    `json:"channel,omitempty"`
	ReferenceID     string    `json:"reference_id,omitempty"`
	AdminID         string    `json:"admin_id,omitempty"`
	AdminEmail      string    `json:"admin_email,omitempty"`
	AdminNotes      string    `json:"admin_notes,omitempty"`
	Status          string    `json:"status"` // SUCCESS, FAILED
	CreatedAt       time.Time `json:"created_at"`
}

// PayRequest represents a debit payment request
type PayRequest struct {
	Amount      float64 `json:"amount"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	ReferenceID string  `json:"reference_id"`
	PIN         string  `json:"pin,omitempty"`
}

// SetupPINRequest represents payload to create wallet PIN for the first time
type SetupPINRequest struct {
	PIN        string `json:"pin"`
	ConfirmPIN string `json:"confirm_pin"`
}

// ChangePINRequest represents payload to change existing wallet PIN
type ChangePINRequest struct {
	OldPIN     string `json:"old_pin"`
	NewPIN     string `json:"new_pin"`
	ConfirmPIN string `json:"confirm_pin"`
}

// VerifyPINRequest represents payload to verify wallet PIN
type VerifyPINRequest struct {
	PIN string `json:"pin"`
}

// TransferRequest represents payload to transfer balance to another user
type TransferRequest struct {
	TargetUserID string  `json:"target_user_id,omitempty"`
	TargetEmail  string  `json:"target_email,omitempty"`
	TargetPhone  string  `json:"target_phone,omitempty"`
	Amount       float64 `json:"amount"`
	PIN          string  `json:"pin"`
	Notes        string  `json:"notes,omitempty"`
}

// TopUpRequest represents a balance deposit request
type TopUpRequest struct {
	Amount        float64 `json:"amount"`
	Channel       string  `json:"channel,omitempty"`
	PaymentMethod string  `json:"payment_method,omitempty"`
	Notes         string  `json:"notes,omitempty"`
}

// ApproveTopUpRequest represents admin approval for pending topup
type ApproveTopUpRequest struct {
	TransactionCode string `json:"transaction_code"`
	Notes           string `json:"notes,omitempty"`
}

// RejectTopUpRequest represents admin rejection for pending topup
type RejectTopUpRequest struct {
	TransactionCode string `json:"transaction_code"`
	Reason          string `json:"reason,omitempty"`
}


// RefundRequest represents a refund request for a failed/cancelled transaction
type RefundRequest struct {
	TransactionCode string  `json:"transaction_code"`
	Amount          float64 `json:"amount,omitempty"`
	Reason          string  `json:"reason"`
	TargetUserID    string  `json:"target_user_id,omitempty"`
	UserID          string  `json:"user_id,omitempty"`
}

// AdjustRequest represents an administrative balance adjustment
type AdjustRequest struct {
	TargetUserID string  `json:"target_user_id"`
	UserID       string  `json:"user_id,omitempty"`
	Type         string  `json:"type"`   // CREDIT or DEBIT
	Amount       float64 `json:"amount"` // Nominal uang
	Reason       string  `json:"reason"` // Alasan koreksi (wajib)
	AdminNotes   string  `json:"admin_notes,omitempty"`
}
