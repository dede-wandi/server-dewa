package transaction

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dede-wandi/server-dewa/internal/modules/product"
	"github.com/dede-wandi/server-dewa/internal/modules/vendor"
	"github.com/dede-wandi/server-dewa/internal/modules/wallet"
	"github.com/dede-wandi/server-dewa/internal/supabase"
)

func TestTransactionOrderLifecycle(t *testing.T) {
	// Setup dependencies
	walletRepo := wallet.NewMemoryRepository()
	pointSvc := wallet.NewPointService(walletRepo, nil)
	walletSvc := wallet.NewWalletService(walletRepo, pointSvc, nil)

	// Hubungkan DFlashAdapter asli ke mock server test untuk pengujian HTTP live
	mockVendorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("TSEL10 ke 081234567890 SUKSES. SN: 2026101511223344. Sal: 150.000"))
	}))
	defer mockVendorServer.Close()

	liveDFlash := vendor.NewDFlashAdapter("H2H02494", "112211", "DUNhil00@#", "")
	liveDFlash.SetCenters([]string{mockVendorServer.URL})
	vendor.GetGlobalManager().RegisterAdapter(liveDFlash)

	productRepo := product.NewCachedRepository(1 * time.Minute)
	productRepo.Save(&product.Product{
		ID:           "p-tsel10",
		CategoryID:   "cat-pulsa",
		Category:     product.CategoryPulsa,
		VendorCode:   "DFLASH",
		BuyerSKUCode: "TSEL10",
		Code:         "TSEL10",
		Name:         "Telkomsel 10.000",
		Brand:        "Telkomsel",
		CostPrice:    10200,
		Price:        11250,
		Status:       "ACTIVE",
	})
	productSvc := product.NewService(productRepo, nil)

	txRepo := NewMemoryRepository()
	txSvc := NewService(txRepo, walletSvc, productSvc, nil)

	testUser := &supabase.User{
		ID:    "user-test-uuid-123",
		Email: "user@test.com",
	}

	// 1. Top up wallet balance (pending -> approved)
	topTrx, _, err := walletSvc.TopUp(testUser.ID, testUser.Email, "", wallet.TopUpRequest{
		Amount:  50000,
		Channel: "Bank Transfer",
	})
	if err != nil {
		t.Fatalf("failed to topup: %v", err)
	}
	_, _, err = walletSvc.ApproveTopUp(nil, "", topTrx.TransactionCode, "Approved in test")
	if err != nil {
		t.Fatalf("failed to approve topup: %v", err)
	}

	bal, err := walletSvc.GetBalance(testUser.ID, testUser.Email, "")
	if err != nil || bal.Balance != 50000 {
		t.Fatalf("expected balance 50000, got %v (err: %v)", bal, err)
	}

	// 2. Create order for Telkomsel 10k (price 11.200)
	req := CreateOrderRequest{
		ProductCode:   "TSEL10",
		TargetAccount: "081234567890",
	}

	order, err := txSvc.CreateOrder(testUser, "", req)
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	if order.Status != StatusSuccess && order.Status != StatusProcessing {
		t.Errorf("expected status SUCCESS or PROCESSING, got %s", order.Status)
	}
	if order.ProductCode != "TSEL10" {
		t.Errorf("expected product code TSEL10, got %s", order.ProductCode)
	}

	// Check balance after payment (50000 - 11250 = 38750)
	balAfter, _ := walletSvc.GetBalance(testUser.ID, testUser.Email, "")
	if balAfter.Balance != 38750 {
		t.Errorf("expected remaining balance 38750, got %f", balAfter.Balance)
	}

	// 3. Test insufficient funds
	bigReq := CreateOrderRequest{
		ProductCode:   "PLN100", // price 102500 > 38800
		TargetAccount: "12345678901",
	}
	_, err = txSvc.CreateOrder(testUser, "", bigReq)
	if err == nil {
		t.Errorf("expected insufficient funds error, but order succeeded")
	}

	// 4. Test order detail retrieval
	fetched, err := txSvc.GetOrderDetail(order.OrderCode)
	if err != nil || fetched.OrderCode != order.OrderCode {
		t.Errorf("failed to retrieve order detail: %v", err)
	}

	// 5. Test user history
	history := txSvc.GetUserOrders(testUser.ID)
	if len(history) != 1 {
		t.Errorf("expected 1 order in history, got %d", len(history))
	}
}
