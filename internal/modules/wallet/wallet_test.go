package wallet

import (
	"testing"
)

func TestWalletAndPointsLifecycle(t *testing.T) {
	repo := NewMemoryRepository()
	pointSvc := NewPointService(repo, nil)
	walletSvc := NewWalletService(repo, pointSvc, nil)

	testUserID := "wallet-user-test-uuid"
	testEmail := "tester@dewacell.com"

	// 1. Initial balance check
	bal, err := walletSvc.GetBalance(testUserID, testEmail, "")
	if err != nil {
		t.Fatalf("failed to get balance: %v", err)
	}
	if bal.Balance != 0 || bal.Points != 0 {
		t.Errorf("expected new wallet to have 0 balance and 0 points, got bal=%f, pts=%d", bal.Balance, bal.Points)
	}

	// 2. Top up Rp 100.000 (status PENDING)
	trxTopup, _, err := walletSvc.TopUp(testUserID, testEmail, "", TopUpRequest{
		Amount:  100000,
		Channel: "QRIS",
	})
	if err != nil {
		t.Fatalf("failed to topup: %v", err)
	}
	if trxTopup.Status != "PENDING" {
		t.Errorf("expected status PENDING, got %s", trxTopup.Status)
	}

	// Super Admin approves the top up
	_, wApproved, err := walletSvc.ApproveTopUp(nil, "", trxTopup.TransactionCode, "Approved in test")
	if err != nil {
		t.Fatalf("failed to approve topup: %v", err)
	}
	if wApproved.Balance != 100000 {
		t.Errorf("expected balance 100000, got %f", wApproved.Balance)
	}

	// 3. Payment Rp 25.000 (earns 1% = 250 points cashback)
	_, wPay, err := walletSvc.Pay(testUserID, testEmail, "", PayRequest{
		Amount:      25000,
		Title:       "Beli Paket Data",
		Description: "Telkomsel 5GB",
		Category:    CategoryPayment,
		ReferenceID: "TRX-TEST-001",
	})
	if err != nil {
		t.Fatalf("failed to pay: %v", err)
	}
	if wPay.Balance != 75000 {
		t.Errorf("expected balance 75000, got %f", wPay.Balance)
	}
	if wPay.Points != 250 {
		t.Errorf("expected 250 cashback points, got %d", wPay.Points)
	}

	// 4. Award bonus 100 points
	_, err = pointSvc.AddRewardPoints(testUserID, 100, "Bonus Loyalty", "Bonus member baru", "BONUS-01")
	if err != nil {
		t.Fatalf("failed to award points: %v", err)
	}

	wWithPoints, err := walletSvc.GetBalance(testUserID, testEmail, "")
	if err != nil || wWithPoints.Points != 350 {
		t.Errorf("expected 350 points (250 cashback + 100 bonus), got %d (err: %v)", wWithPoints.Points, err)
	}

	// 5. Redeem 100 points
	_, wRedeemed, err := pointSvc.RedeemPoints(testUserID, testEmail, "", RedeemPointsRequest{
		Amount: 100,
	})
	if err != nil {
		t.Fatalf("failed to redeem points: %v", err)
	}
	if wRedeemed.Points != 250 {
		t.Errorf("expected 250 points remaining (350 - 100), got %d", wRedeemed.Points)
	}

	// 6. Test History
	wHistory := walletSvc.GetHistory(testUserID)
	if len(wHistory) != 2 { // 1 Topup + 1 Pay
		t.Errorf("expected 2 wallet transactions, got %d", len(wHistory))
	}

	ptHistory := pointSvc.GetHistory(testUserID)
	if len(ptHistory) != 3 { // 1 Cashback + 1 Bonus + 1 Redeem
		t.Errorf("expected 3 point transactions, got %d", len(ptHistory))
	}
}
