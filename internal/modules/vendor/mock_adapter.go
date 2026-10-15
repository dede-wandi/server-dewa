package vendor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MockVendorAdapter implementasi vendor simulasi (Sandbox/Staging)
// Digunakan sebelum API key resmi vendor aktif, sehingga arsitektur siap tanpa force close.
type MockVendorAdapter struct {
	vendorCode string
}

func NewMockVendorAdapter(code string) *MockVendorAdapter {
	if code == "" {
		code = "MANUAL"
	}
	return &MockVendorAdapter{
		vendorCode: strings.ToUpper(code),
	}
}

func (m *MockVendorAdapter) GetVendorCode() string {
	return m.vendorCode
}

func (m *MockVendorAdapter) ValidateSKU(sku string) bool {
	sku = strings.TrimSpace(sku)
	// SKU valid jika memiliki panjang minimal 3 karakter (misal: FIBP14, TSEL10)
	return len(sku) >= 3
}

func (m *MockVendorAdapter) OrderProduct(ctx context.Context, req *VendorOrderRequest) (*VendorOrderResponse, error) {
	if !m.ValidateSKU(req.BuyerSKUCode) {
		return nil, fmt.Errorf("kode SKU '%s' tidak sesuai dengan ketentuan vendor %s", req.BuyerSKUCode, m.vendorCode)
	}

	mockSN := fmt.Sprintf("SN%d%s", time.Now().Unix(), req.BuyerSKUCode)
	return &VendorOrderResponse{
		VendorCode:   m.vendorCode,
		VendorTrxID:  fmt.Sprintf("TRX-%s-%d", m.vendorCode, time.Now().UnixNano()),
		RefID:        req.RefID,
		BuyerSKUCode: req.BuyerSKUCode,
		CustomerNo:   req.CustomerNo,
		Price:        req.MaxPrice,
		Status:       "SUCCESS",
		SerialNumber: mockSN,
		Message:      fmt.Sprintf("Transaksi produk %s berhasil diproses oleh vendor %s (Mode Sandbox)", req.BuyerSKUCode, m.vendorCode),
		CreatedAt:    time.Now(),
	}, nil
}

func (m *MockVendorAdapter) CheckStatus(ctx context.Context, refID string) (*VendorOrderResponse, error) {
	return &VendorOrderResponse{
		VendorCode:   m.vendorCode,
		VendorTrxID:  fmt.Sprintf("TRX-%s-%d", m.vendorCode, time.Now().Unix()),
		RefID:        refID,
		Status:       "SUCCESS",
		SerialNumber: fmt.Sprintf("SN%d", time.Now().Unix()),
		Message:      "Transaksi sukses terverifikasi",
		CreatedAt:    time.Now(),
	}, nil
}

func (m *MockVendorAdapter) CheckBalance(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"vendor_code": m.vendorCode,
		"balance":     5000000.0,
		"currency":    "IDR",
		"status":      "ACTIVE",
		"message":     "Saldo akun deposit vendor (Mode Mock / Staging)",
		"checked_at":  time.Now().Format(time.RFC3339),
	}, nil
}

func (m *MockVendorAdapter) ParseCallback(payload []byte, signature string) (*VendorCallbackPayload, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}

	refID, _ := raw["ref_id"].(string)
	status, _ := raw["status"].(string)
	if status == "" {
		status = "SUCCESS"
	}
	sn, _ := raw["sn"].(string)
	msg, _ := raw["message"].(string)

	return &VendorCallbackPayload{
		VendorCode:   m.vendorCode,
		VendorTrxID:  fmt.Sprintf("%v", raw["trx_id"]),
		RefID:        refID,
		Status:       status,
		SerialNumber: sn,
		Message:      msg,
		RawPayload:   raw,
		ReceivedAt:   time.Now(),
	}, nil
}
