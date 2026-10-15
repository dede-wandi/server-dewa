package vendor

import (
	"context"
)

// IVendorAdapter interface kontrak OOP untuk integrasi beragam provider/vendor PPOB
type IVendorAdapter interface {
	// GetVendorCode mengidentifikasi kode vendor (misal: DIGIFLAZZ, IAK, SERPUL, MANUAL)
	GetVendorCode() string

	// ValidateSKU memastikan format SKU sesuai dengan spesifikasi vendor
	ValidateSKU(sku string) bool

	// OrderProduct mengirimkan instruksi pembayaran/pengisian produk ke server vendor
	OrderProduct(ctx context.Context, req *VendorOrderRequest) (*VendorOrderResponse, error)

	// CheckStatus mengecek status transaksi terbaru di server vendor
	CheckStatus(ctx context.Context, refID string) (*VendorOrderResponse, error)

	// CheckBalance mengecek saldo akun deposit di server vendor
	CheckBalance(ctx context.Context) (map[string]interface{}, error)

	// ParseCallback membedah dan memverifikasi payload webhook callback dari vendor
	ParseCallback(payload []byte, signature string) (*VendorCallbackPayload, error)
}
