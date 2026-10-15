package vendor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDFlashSignatureAndAdapter(t *testing.T) {
	// 1. Verifikasi rumus signature DFLASH dengan data contoh dari user:
	// memberId = "DS0000"
	// product  = "TSP10"
	// dest     = "08223334455"
	// refID    = "3452123"
	// pin      = "8715"
	// password = "yunw)uc&@"
	// Template: OtomaX|DS0000|TSP10|08223334455|3452123|8715|yunw)uc&@
	// Expected sign: z4KNbX-NIUk0_GQb-hMCx17DBCU

	dflash := NewDFlashAdapter("DS0000", "8715", "yunw)uc&@", "")

	sign := dflash.GenerateSignature("TSP10", "08223334455", "3452123")
	expectedSign := "z4KNbX-NIUk0_GQb-hMCx17DBCU"

	if sign != expectedSign {
		t.Fatalf("Signature mismatch!\nExpected: %s\nGot:      %s", expectedSign, sign)
	}

	// 2. Verifikasi Cek Saldo (parameter product, dest, refID dikosongkan)
	// Template: OtomaX|DS0000||||8715|yunw)uc&@
	checkSign := dflash.GenerateCheckSignature("")
	if checkSign == "" {
		t.Fatalf("Expected non-empty check signature")
	}

	// 3. Verifikasi Validasi SKU DFLASH
	if !dflash.ValidateSKU("FIBP14") {
		t.Errorf("Expected SKU 'FIBP14' to be valid for DFLASH")
	}
	if !dflash.ValidateSKU("TSP10") {
		t.Errorf("Expected SKU 'TSP10' to be valid for DFLASH")
	}
	if dflash.ValidateSKU("AB") {
		t.Errorf("Expected SKU 'AB' to be invalid (too short)")
	}
	if dflash.ValidateSKU("TSP 10") {
		t.Errorf("Expected SKU with spaces to be invalid")
	}

	// 4. Verifikasi dispatch melalui VendorManager
	mockCenter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("FIBP14 ke 085712345678 SUKSES. SN: 9988776655. Saldo cukup."))
	}))
	defer mockCenter.Close()

	dflashCustom := NewDFlashAdapter("H2H02494", "112211", "DUNhil00@#", "")
	dflashCustom.SetCenters([]string{mockCenter.URL})

	mgr := GetGlobalManager()
	mgr.RegisterAdapter(dflashCustom)

	err := mgr.ValidateProductVendor("DFLASH", "FIBP14")
	if err != nil {
		t.Fatalf("expected VendorManager to validate DFLASH with FIBP14, got err: %v", err)
	}

	req := &VendorOrderRequest{
		VendorCode:   "DFLASH",
		BuyerSKUCode: "FIBP14",
		CustomerNo:   "085712345678",
		RefID:        "REF-DF-001",
		MaxPrice:     45000,
	}

	resp, err := mgr.ProcessOrder(context.Background(), req)
	if err != nil {
		t.Fatalf("failed to process order for DFLASH: %v", err)
	}

	if resp.VendorCode != "DFLASH" {
		t.Errorf("expected VendorCode DFLASH, got %s", resp.VendorCode)
	}
	if resp.Status != "SUCCESS" || resp.SerialNumber != "9988776655" {
		t.Errorf("expected status SUCCESS with SN 9988776655, got %+v", resp)
	}

	// 5. Verifikasi Webhook Callback Parsing dari DFLASH
	mockCallbackPayload := []byte(`{
		"ref_id": "REF-DF-001",
		"id": "TRX-DFLASH-9876",
		"status": "SUKSES",
		"sn": "085712345678/SN778899",
		"message": "Pengisian Sukses"
	}`)

	cb, err := dflash.ParseCallback(mockCallbackPayload, "")
	if err != nil {
		t.Fatalf("failed to parse DFLASH callback: %v", err)
	}

	if cb.RefID != "REF-DF-001" || cb.Status != "SUCCESS" || cb.SerialNumber != "085712345678/SN778899" {
		t.Errorf("parsed callback mismatch: %+v", cb)
	}

	// 6. Verifikasi Cek Saldo DFLASH
	balRes, err := dflash.CheckBalance(context.Background())
	if err != nil {
		t.Fatalf("failed to check balance: %v", err)
	}
	if balRes["vendor_code"] != "DFLASH" {
		t.Errorf("expected vendor_code DFLASH in balance response, got: %v", balRes["vendor_code"])
	}
}

func TestDFlashAutomaticFailover(t *testing.T) {
	// Buat mock server yang bertindak sebagai Center Cadangan yang hidup
	backupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","balance":5000000,"memberID":"DS0000"}`))
	}))
	defer backupServer.Close()

	// Inisialisasi adapter dengan Center 1 mati (port tidak aktif) dan Center 2 hidup
	adapter := NewDFlashAdapter("DS0000", "1234", "pass123", "")
	adapter.SetCenters([]string{
		"http://127.0.0.1:59999", // Center mati
		backupServer.URL,          // Center hidup (backup)
	})

	res, err := adapter.CheckBalance(context.Background())
	if err != nil {
		t.Fatalf("expected failover to succeed, got error: %v", err)
	}

	if res["active_center"] != backupServer.URL {
		t.Errorf("expected active_center %s, got %v", backupServer.URL, res["active_center"])
	}
}

func TestDFlashOfficialDocumentationPayloads(t *testing.T) {
	dflash := NewDFlashAdapter("DS0000", "8715", "pass", "")

	// 1. Test Callback Transaksi Sukses dari Dokumentasi (Status 20, SN, Harga, Saldo)
	successJSON := []byte(`{
		"refid": 1550305072,
		"tgl_entri": "2019-02-16T15:17:52.56",
		"tgl_status": "2019-02-16T15:17:54.057",
		"kode_produk": "TSPP10",
		"tujuan": "082130871971",
		"counter": 1,
		"status": 20,
		"status_text": "Sukses",
		"sn": "900241003386726321",
		"keterangan": "",
		"message": "17. TSPP10 ke 082130871971 SUKSES @15:18 16/02 SN: 900241003386726321. Sal: 163.852 - 10.055 = 153.797 #R1550305072 #*Transaksi Lancarrr...",
		"harga": 10055,
		"saldo": 153797220
	}`)

	cbSuccess, err := dflash.ParseCallback(successJSON, "")
	if err != nil {
		t.Fatalf("failed to parse DFLASH success callback: %v", err)
	}

	if cbSuccess.RefID != "1550305072" {
		t.Errorf("expected RefID 1550305072, got %s", cbSuccess.RefID)
	}
	if cbSuccess.Status != "SUCCESS" {
		t.Errorf("expected status SUCCESS, got %s", cbSuccess.Status)
	}
	if cbSuccess.SerialNumber != "900241003386726321" {
		t.Errorf("expected SN 900241003386726321, got %s", cbSuccess.SerialNumber)
	}

	// 2. Test Callback Transaksi Gagal Saldo Tidak Cukup (Status 43)
	failJSON := []byte(`{
		"refid": "113",
		"check": false,
		"double": false,
		"tgl_entri": "2018-07-13T23:01:27",
		"tgl_status": "2018-07-13T23:01:27",
		"kode_produk": "ovo500",
		"tujuan": "082219199696",
		"status": 43,
		"status_text": "Saldo tidak cukup",
		"message": "R#113 ovo500 ke 082219199696 Saldo tidak cukup"
	}`)

	cbFail, err := dflash.ParseCallback(failJSON, "")
	if err != nil {
		t.Fatalf("failed to parse DFLASH fail callback: %v", err)
	}

	if cbFail.RefID != "113" {
		t.Errorf("expected RefID 113, got %s", cbFail.RefID)
	}
	if cbFail.Status != "FAILED" {
		t.Errorf("expected status FAILED, got %s", cbFail.Status)
	}

	// 3. Test Plain Text Response Langsung dari DFLASH (Stok Tidak Cukup)
	realFailText := []byte(`INP5 ke 085861650213 R#TEST-REF-001 INP5.085861650213 tdk diproses. Stok tidak cukup. Sisa stok  0. Hrg:6.683. Antrian:0. Yo isi lg stoknya, Via bank ketik TIKET.JML.PIN send ke CENTER #*TRANSAKSI LANCARR`)
	status, _, msg, _ := dflash.parseDFlashResponse(realFailText, "http://45.76.183.72:666", "TEST-REF-001", "INP5", "085861650213")
	if status != "FAILED" {
		t.Errorf("expected status FAILED for real DFLASH out-of-stock text, got: %s", status)
	}
	if !strings.Contains(msg, "Stok / Saldo agen") {
		t.Errorf("expected informative error message, got: %s", msg)
	}

	// 4. Test Plain Text Response Sukses dari DFLASH dengan Serial Number
	successText := []byte(`INP5 ke 085861650213 R#ORD-1001 SUKSES. SN: 2026101511223344. Sal: 150.000 - 6.683 = 143.317. Trx lancar`)
	statusSucc, snSucc, _, _ := dflash.parseDFlashResponse(successText, "http://45.76.183.72:666", "ORD-1001", "INP5", "085861650213")
	if statusSucc != "SUCCESS" {
		t.Errorf("expected status SUCCESS for OtomaX success text, got: %s", statusSucc)
	}
	if snSucc != "2026101511223344" {
		t.Errorf("expected SN '2026101511223344', got: %s", snSucc)
	}
}
