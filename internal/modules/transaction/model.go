package transaction

import "time"

// OrderStatus represents the lifecycle of a PPOB order
type OrderStatus string

const (
	StatusPending    OrderStatus = "PENDING"
	StatusProcessing OrderStatus = "PROCESSING"
	StatusSuccess    OrderStatus = "SUCCESS"
	StatusFailed     OrderStatus = "FAILED"
	StatusRefunded   OrderStatus = "REFUNDED"
)

// Order represents a PPOB transaction order
type Order struct {
	ID              string      `json:"id"`
	OrderCode       string      `json:"order_code"`       // e.g. ORD-20261014-998811
	UserID          string      `json:"user_id"`
	UserEmail       string      `json:"user_email"`
	ProductCode     string      `json:"product_code"`
	ProductName     string      `json:"product_name"`
	ProductCategory string      `json:"product_category"` // pulsa, data, pln, emoney, game
	TargetAccount   string      `json:"target_account"`   // No HP, No Meter, Player ID
	Price           float64     `json:"price"`
	PointsEarned    int64       `json:"points_earned"`
	Status          OrderStatus `json:"status"`
	SerialNumber    string      `json:"serial_number,omitempty"` // Stroom token PLN / SN Pulsa
	WalletTrxCode   string      `json:"wallet_trx_code,omitempty"`
	ErrorMessage    string      `json:"error_message,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	CompletedAt     *time.Time  `json:"completed_at,omitempty"`
}

// CreateOrderRequest represents client payload to purchase a digital product (prepaid or postpaid)
type CreateOrderRequest struct {
	ProductCode   string `json:"product_code"`
	TargetAccount string `json:"target_account"`       // No HP / Meteran / ID Pelanggan
	PIN           string `json:"pin,omitempty"`
	InquiryID     string `json:"inquiry_id,omitempty"` // ID inquiry hasil cek tagihan pascabayar
}

// BillInquiryRequest payload untuk cek tagihan pascabayar (PDAM, PLN Pasca, BPJS, dll)
type BillInquiryRequest struct {
	ProductCode string `json:"product_code"` // Kode produk sistem (e.g. PDAM-BELU atau BPDAM177)
	CustomerNo  string `json:"customer_no"`  // ID Pelanggan / Nomor Meteran
}

// BillInquiryResponse rincian tagihan dari vendor untuk ditampilkan ke pelanggan
type BillInquiryResponse struct {
	InquiryID    string                 `json:"inquiry_id"`              // e.g. INQ-261016-12345
	ProductCode  string                 `json:"product_code"`            // e.g. PDAM-BELU
	ProductName  string                 `json:"product_name"`            // e.g. PDAM KAB. BELU
	VendorCode   string                 `json:"vendor_code"`             // e.g. DFLASH
	InquirySKU   string                 `json:"inquiry_sku"`             // e.g. CPDAM177
	PaymentSKU   string                 `json:"payment_sku"`             // e.g. BPDAM177
	CustomerNo   string                 `json:"customer_no"`             // e.g. 1234567890
	CustomerName string                 `json:"customer_name"`           // e.g. BUDI SANTOSO
	Period       string                 `json:"period,omitempty"`        // e.g. OKTOBER 2026
	BillAmount   float64                `json:"bill_amount"`             // Tagihan pokok dari vendor
	AdminFee     float64                `json:"admin_fee"`               // Biaya admin DewaCell
	TotalAmount  float64                `json:"total_amount"`            // Total bayar = BillAmount + AdminFee
	Description  string                 `json:"description,omitempty"`   // Keterangan / rincian lembar
	RawResponse  map[string]interface{} `json:"raw_response,omitempty"`  // Raw respons vendor
	CreatedAt    time.Time              `json:"created_at"`
	ExpiresAt    time.Time              `json:"expires_at"`              // Kedaluwarsa dalam 15 menit
}
