package vendor

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// VendorManager mengelola seluruh adapter vendor PPOB secara dinamis (OOP Factory / Registry)
type VendorManager struct {
	mu       sync.RWMutex
	adapters map[string]IVendorAdapter
}

var (
	globalManager *VendorManager
	once          sync.Once
)

// GetGlobalManager mengembalikan singleton instance VendorManager
func GetGlobalManager() *VendorManager {
	once.Do(func() {
		globalManager = NewVendorManager()
		// 1. Daftarkan adapter vendor resmi DFLASH
		globalManager.RegisterAdapter(NewDFlashAdapter("H2H02494", "112211", "DUNhil00@#", ""))
		// 2. Daftarkan adapter untuk produk MANUAL (tanpa menembak API vendor luar)
		globalManager.RegisterAdapter(NewManualAdapter())
		// NOTE: Untuk vendor baru berikutnya (misal Digiflazz, IAK, Serpul, dsb.), cukup daftarkan di sini:
		// globalManager.RegisterAdapter(NewDigiflazzAdapter(...))
	})
	return globalManager
}

// NewVendorManager inisialisasi registry adapter
func NewVendorManager() *VendorManager {
	return &VendorManager{
		adapters: make(map[string]IVendorAdapter),
	}
}

// RegisterAdapter mendaftarkan implementasi adapter baru
func (m *VendorManager) RegisterAdapter(adapter IVendorAdapter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.ToUpper(strings.TrimSpace(adapter.GetVendorCode()))
	m.adapters[key] = adapter
}

// GetAdapter mengambil adapter berdasarkan nama/kode vendor secara ketat (Strict Vendor Routing)
func (m *VendorManager) GetAdapter(vendorCode string) (IVendorAdapter, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := strings.ToUpper(strings.TrimSpace(vendorCode))
	if key == "" {
		key = "MANUAL" // Default aman ke MANUAL (tidak akan pernah menembak DFLASH)
	}

	adapter, exists := m.adapters[key]
	if !exists {
		return nil, fmt.Errorf("vendor '%s' tidak didukung atau belum terdaftar di sistem. Transaksi tidak diteruskan demi keamanan", vendorCode)
	}
	return adapter, nil
}

// ValidateProductVendor memeriksa apakah kode SKU cocok dengan vendor yang dituju.
// Jika tidak sesuai, transaksi TIDAK DITERUSKAN ke API vendor.
func (m *VendorManager) ValidateProductVendor(vendorCode, buyerSKUCode string) error {
	adapter, err := m.GetAdapter(vendorCode)
	if err != nil {
		return err
	}

	if !adapter.ValidateSKU(buyerSKUCode) {
		return fmt.Errorf("kode produk/SKU '%s' tidak valid atau tidak cocok dengan format server vendor %s. Transaksi tidak diteruskan ke vendor", buyerSKUCode, strings.ToUpper(vendorCode))
	}

	return nil
}

// ProcessOrder meneruskan order ke adapter vendor terkait jika validasi lolos
func (m *VendorManager) ProcessOrder(ctx context.Context, req *VendorOrderRequest) (*VendorOrderResponse, error) {
	if err := m.ValidateProductVendor(req.VendorCode, req.BuyerSKUCode); err != nil {
		return nil, err
	}

	adapter, err := m.GetAdapter(req.VendorCode)
	if err != nil {
		return nil, err
	}

	return adapter.OrderProduct(ctx, req)
}

// CheckVendorBalance mengecek sisa saldo deposit agen pada server vendor
func (m *VendorManager) CheckVendorBalance(ctx context.Context, vendorCode string) (map[string]interface{}, error) {
	adapter, err := m.GetAdapter(vendorCode)
	if err != nil {
		return nil, err
	}
	return adapter.CheckBalance(ctx)
}

// CheckVendorStatus mengecek status transaksi ke server vendor berdasarkan refID
func (m *VendorManager) CheckVendorStatus(ctx context.Context, vendorCode, refID string) (*VendorOrderResponse, error) {
	adapter, err := m.GetAdapter(vendorCode)
	if err != nil {
		return nil, err
	}
	return adapter.CheckStatus(ctx, refID)
}

