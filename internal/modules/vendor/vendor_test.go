package vendor

import (
	"context"
	"testing"
)

func TestVendorAdapterAndManager(t *testing.T) {
	mgr := GetGlobalManager()

	// 1. Test Validasi Vendor Terdaftar & SKU Valid (Mock / FIBP14)
	err := mgr.ValidateProductVendor("MOCK", "FIBP14")
	if err != nil {
		t.Fatalf("expected SKU FIBP14 to be valid for MOCK vendor, got err: %v", err)
	}

	// 2. Test Vendor Tidak Dikenal
	errUnknown := mgr.ValidateProductVendor("UNKNOWN_VENDOR", "FIBP14")
	if errUnknown == nil {
		t.Fatalf("expected error for unknown vendor, got nil")
	}

	// 3. Test SKU Kosong / Terlalu Pendek
	errShort := mgr.ValidateProductVendor("MOCK", "AB")
	if errShort == nil {
		t.Fatalf("expected error for short SKU 'AB', got nil")
	}

	// 4. Test Manual / Default SKU Validation (SKU valid)
	errManual := mgr.ValidateProductVendor("MANUAL", "FIBP14")
	if errManual != nil {
		t.Fatalf("expected MANUAL vendor to accept SKU FIBP14, got: %v", errManual)
	}

	// 5. Test Mock Order Execution (OOP Strategy Dispatch)
	req := &VendorOrderRequest{
		VendorCode:   "MOCK",
		BuyerSKUCode: "FIBP14",
		CustomerNo:   "085712345678",
		RefID:        "REF-TEST-001",
		MaxPrice:     45000,
		Testing:      true,
	}

	resp, err := mgr.ProcessOrder(context.Background(), req)
	if err != nil {
		t.Fatalf("failed to process mock order: %v", err)
	}

	if resp.RefID != "REF-TEST-001" {
		t.Errorf("expected RefID REF-TEST-001, got %s", resp.RefID)
	}
	if resp.Status != "SUCCESS" {
		t.Errorf("expected status SUCCESS, got %s", resp.Status)
	}
	if resp.SerialNumber == "" {
		t.Errorf("expected non-empty serial number for mock success")
	}

	// 6. Test Callback Parsing
	adapter, err := mgr.GetAdapter("MOCK")
	if err != nil {
		t.Fatalf("failed to get mock adapter: %v", err)
	}

	mockPayload := []byte(`{"ref_id":"REF-TEST-001","status":"SUCCESS","sn":"085712345678/OK","message":"Transaksi Sukses"}`)
	cb, err := adapter.ParseCallback(mockPayload, "")
	if err != nil {
		t.Fatalf("failed to parse callback: %v", err)
	}
	if cb.RefID != "REF-TEST-001" || cb.Status != "SUCCESS" {
		t.Errorf("callback parsing mismatch: %+v", cb)
	}

	// 7. Test CheckVendorBalance
	bal, err := mgr.CheckVendorBalance(context.Background(), "MOCK")
	if err != nil {
		t.Fatalf("failed to check vendor balance: %v", err)
	}
	if bal["balance"] == nil {
		t.Errorf("expected balance field in response")
	}
}
