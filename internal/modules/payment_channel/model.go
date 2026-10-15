package payment_channel

import "time"

// Channel status constants
const (
	StatusActive     = "ACTIVE"
	StatusComingSoon = "COMING_SOON"
	StatusInactive   = "INACTIVE"
)

// Channel type constants
const (
	TypeManual         = "MANUAL"
	TypeQRIS           = "QRIS"
	TypeVirtualAccount = "VIRTUAL_ACCOUNT"
	TypeEwallet        = "EWALLET"
	TypeRetail         = "RETAIL"
)

// PaymentChannel represents a payment channel entity
type PaymentChannel struct {
	ID              string    `json:"id"`
	Code            string    `json:"code"`                        // contoh: 'fund_credit', 'qris_duitku', 'qris_dana'
	Name            string    `json:"name"`                        // contoh: 'Fund Credit (Transfer Manual Admin)', 'QRIS Duitku'
	ChannelType     string    `json:"channel_type"`                // MANUAL, QRIS, VIRTUAL_ACCOUNT, EWALLET
	Vendor          string    `json:"vendor"`                      // manual, duitku, dana, midtrans, xendit, tripay
	AccountNumber   string    `json:"account_number,omitempty"`    // nomor rekening / nomor VA contoh: '8801 2345 6789'
	AccountName     string    `json:"account_name,omitempty"`      // atas nama akun contoh: 'PT DEWA CELLULER INDONESIA'
	IconName        string    `json:"icon_name"`                   // nama icon Ionicons contoh: 'wallet-outline', 'qr-code-outline'
	IconURL         string    `json:"icon_url,omitempty"`          // logo URL resmi vendor
	AdminFee        float64   `json:"admin_fee"`                   // biaya admin tetap (Rp)
	AdminFeePercent float64   `json:"admin_fee_percent"`           // persentase fee (0.7% dll)
	MinAmount       float64   `json:"min_amount"`                  // minimum deposit (default: 10.000)
	MaxAmount       float64   `json:"max_amount"`                  // maksimum deposit (default: 10.000.000)
	Status          string    `json:"status"`                      // ACTIVE, COMING_SOON, INACTIVE
	IsPG            bool      `json:"is_pg"`                       // true jika Payment Gateway otomatis, false jika manual
	Badge           string    `json:"badge"`                       // 'AKTIF', 'Coming Soon', '0% Fee'
	Instructions    string    `json:"instructions"`                // petunjuk pembayaran jelas
	SortOrder       int       `json:"sort_order"`                  // urutan tampilan
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreatePaymentChannelRequest payload for Admin creating a payment channel
type CreatePaymentChannelRequest struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	ChannelType     string  `json:"channel_type"`
	Vendor          string  `json:"vendor"`
	AccountNumber   string  `json:"account_number,omitempty"`
	AccountName     string  `json:"account_name,omitempty"`
	IconName        string  `json:"icon_name,omitempty"`
	IconURL         string  `json:"icon_url,omitempty"`
	AdminFee        float64 `json:"admin_fee"`
	AdminFeePercent float64 `json:"admin_fee_percent"`
	MinAmount       float64 `json:"min_amount"`
	MaxAmount       float64 `json:"max_amount"`
	Status          string  `json:"status"` // ACTIVE, COMING_SOON, INACTIVE
	IsPG            *bool   `json:"is_pg,omitempty"`
	Badge           string  `json:"badge,omitempty"`
	Instructions    string  `json:"instructions,omitempty"`
	SortOrder       int     `json:"sort_order"`
}

// UpdatePaymentChannelRequest payload for Admin updating a payment channel
type UpdatePaymentChannelRequest struct {
	Name            *string  `json:"name,omitempty"`
	ChannelType     *string  `json:"channel_type,omitempty"`
	Vendor          *string  `json:"vendor,omitempty"`
	AccountNumber   *string  `json:"account_number,omitempty"`
	AccountName     *string  `json:"account_name,omitempty"`
	IconName        *string  `json:"icon_name,omitempty"`
	IconURL         *string  `json:"icon_url,omitempty"`
	AdminFee        *float64 `json:"admin_fee,omitempty"`
	AdminFeePercent *float64 `json:"admin_fee_percent,omitempty"`
	MinAmount       *float64 `json:"min_amount,omitempty"`
	MaxAmount       *float64 `json:"max_amount,omitempty"`
	Status          *string  `json:"status,omitempty"`
	IsPG            *bool    `json:"is_pg,omitempty"`
	Badge           *string  `json:"badge,omitempty"`
	Instructions    *string  `json:"instructions,omitempty"`
	SortOrder       *int     `json:"sort_order,omitempty"`
}

// PaymentChannelListResponse wrapper for list
type PaymentChannelListResponse struct {
	Status  string           `json:"status"`
	Count   int              `json:"count"`
	Data    []PaymentChannel `json:"data"`
	Message string           `json:"message,omitempty"`
}

// PaymentChannelResponse wrapper for single item
type PaymentChannelResponse struct {
	Status  string          `json:"status"`
	Data    *PaymentChannel `json:"data,omitempty"`
	Message string          `json:"message,omitempty"`
}
