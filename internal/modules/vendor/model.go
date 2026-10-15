package vendor

import (
	"time"
)

// VendorOrderRequest payload pemesanan produk digital ke server vendor
type VendorOrderRequest struct {
	OrderID      string  `json:"order_id"`
	VendorCode   string  `json:"vendor_code"`   // e.g. DIGIFLAZZ, IAK, SERPUL, MANUAL
	BuyerSKUCode string  `json:"buyer_sku_code"` // e.g. FIBP14
	CustomerNo   string  `json:"customer_no"`   // Nomor HP / ID Pelanggan
	RefID        string  `json:"ref_id"`        // Unique internal transaction reference
	MaxPrice     float64 `json:"max_price"`     // Batas toleransi harga modal
	Testing      bool    `json:"testing"`       // Mode sandbox / staging
}

// VendorOrderResponse respon balik standar dari server vendor
type VendorOrderResponse struct {
	VendorCode   string                 `json:"vendor_code"`
	VendorTrxID  string                 `json:"vendor_trx_id"`
	RefID        string                 `json:"ref_id"`
	BuyerSKUCode string                 `json:"buyer_sku_code"`
	CustomerNo   string                 `json:"customer_no"`
	Price        float64                `json:"price"`
	Status       string                 `json:"status"` // SUCCESS, PENDING, FAILED
	SerialNumber string                 `json:"serial_number,omitempty"`
	Message      string                 `json:"message"`
	RawResponse  map[string]interface{} `json:"raw_response,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

// VendorCallbackPayload format terstandarisasi untuk webhook notifikasi status dari vendor
type VendorCallbackPayload struct {
	VendorCode   string                 `json:"vendor_code"`
	VendorTrxID  string                 `json:"vendor_trx_id"`
	RefID        string                 `json:"ref_id"`
	Status       string                 `json:"status"` // SUCCESS, PENDING, FAILED
	SerialNumber string                 `json:"serial_number,omitempty"`
	Message      string                 `json:"message"`
	RawPayload   map[string]interface{} `json:"raw_payload,omitempty"`
	ReceivedAt   time.Time              `json:"received_at"`
}
