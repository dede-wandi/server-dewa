package vendor

import (
	"context"
	"fmt"
	"time"
)

// ManualAdapter menangani produk yang diproses secara manual oleh admin/CS
// tanpa mengirimkan permintaan API ke provider pihak ketiga seperti DFLASH.
type ManualAdapter struct{}

func NewManualAdapter() *ManualAdapter {
	return &ManualAdapter{}
}

func (a *ManualAdapter) GetVendorCode() string {
	return "MANUAL"
}

func (a *ManualAdapter) ValidateSKU(sku string) bool {
	return true
}

func (a *ManualAdapter) OrderProduct(ctx context.Context, req *VendorOrderRequest) (*VendorOrderResponse, error) {
	return &VendorOrderResponse{
		VendorCode:   "MANUAL",
		RefID:        req.RefID,
		Status:       "PROCESSING",
		Message:      "Pesanan manual diterima, menunggu pemrosesan oleh tim admin",
		CreatedAt:    time.Now(),
	}, nil
}

func (a *ManualAdapter) CheckStatus(ctx context.Context, refID string) (*VendorOrderResponse, error) {
	return &VendorOrderResponse{
		VendorCode: "MANUAL",
		RefID:      refID,
		Status:     "PROCESSING",
		Message:    "Pesanan manual sedang dalam antrean admin",
	}, nil
}

func (a *ManualAdapter) CheckBalance(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"vendor":  "MANUAL",
		"balance": 0,
		"message": "Vendor manual tidak memerlukan saldo deposit API",
	}, nil
}

func (a *ManualAdapter) ParseCallback(payload []byte, signature string) (*VendorCallbackPayload, error) {
	return nil, fmt.Errorf("vendor manual tidak mendukung callback webhook otomatis")
}
