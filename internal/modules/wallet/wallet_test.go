package wallet

import (
	"testing"

	"github.com/dede-wandi/server-dewa/internal/supabase"
)

func supabaseUserMock(id, email string) *supabase.User {
	return &supabase.User{
		ID:    id,
		Email: email,
	}
}

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

func TestWalletPINAndTransfer(t *testing.T) {
	repo := NewMemoryRepository()
	pointSvc := NewPointService(repo, nil)
	walletSvc := NewWalletService(repo, pointSvc, nil)

	senderID := "sender-uuid-001"
	senderEmail := "sender@dewacell.com"
	recipientID := "recipient-uuid-002"
	recipientEmail := "recipient@dewacell.com"

	// 1. Give sender Rp 100.000 via direct adjustment
	_, _, err := walletSvc.AdjustBalance(nil, "", AdjustRequest{
		TargetUserID: senderID,
		Type:         TypeCredit,
		Amount:       100000,
		Reason:       "Initial test balance",
	})
	if err != nil {
		t.Fatalf("failed to seed sender balance: %v", err)
	}

	// 2. Validate PIN input formats
	if err := ValidatePIN("12345"); err == nil {
		t.Error("expected error for 5 digit PIN")
	}
	if err := ValidatePIN("1234567"); err == nil {
		t.Error("expected error for 7 digit PIN")
	}
	if err := ValidatePIN("abcdef"); err == nil {
		t.Error("expected error for non-numeric PIN")
	}
	if err := ValidatePIN("123456"); err != nil {
		t.Errorf("expected valid 6-digit PIN, got error: %v", err)
	}

	// 3. Try transfer before PIN is set -> Must fail
	_, _, err = walletSvc.Transfer(supabaseUserMock(senderID, senderEmail), "", TransferRequest{
		TargetUserID: recipientID,
		Amount:       25000,
		PIN:          "123456",
	})
	if err == nil {
		t.Fatal("expected transfer to fail when PIN is not yet set")
	}

	// 4. Setup PIN (123456)
	err = walletSvc.SetupPIN(senderID, "", SetupPINRequest{
		PIN:        "123456",
		ConfirmPIN: "123456",
	})
	if err != nil {
		t.Fatalf("failed to setup PIN: %v", err)
	}

	// Verify wallet reflects is_pin_set = true
	senderWal, _ := walletSvc.GetBalance(senderID, senderEmail, "")
	if !senderWal.IsPINSet {
		t.Errorf("expected is_pin_set=true on sender wallet")
	}

	// Setup PIN again should fail
	err = walletSvc.SetupPIN(senderID, "", SetupPINRequest{
		PIN:        "654321",
		ConfirmPIN: "654321",
	})
	if err == nil {
		t.Error("expected duplicate setup PIN to fail")
	}

	// 5. Change PIN with wrong old PIN -> Must fail
	err = walletSvc.ChangePIN(senderID, "", ChangePINRequest{
		OldPIN:     "999999",
		NewPIN:     "654321",
		ConfirmPIN: "654321",
	})
	if err == nil {
		t.Error("expected ChangePIN to fail with wrong old PIN")
	}

	// 6. Change PIN successfully to 654321
	err = walletSvc.ChangePIN(senderID, "", ChangePINRequest{
		OldPIN:     "123456",
		NewPIN:     "654321",
		ConfirmPIN: "654321",
	})
	if err != nil {
		t.Fatalf("failed to change PIN: %v", err)
	}

	// Verify old PIN no longer works
	if err := walletSvc.VerifyPIN(senderID, "123456"); err == nil {
		t.Error("expected old PIN to be invalid after change")
	}
	// Verify new PIN works
	if err := walletSvc.VerifyPIN(senderID, "654321"); err != nil {
		t.Errorf("expected new PIN to be valid: %v", err)
	}

	// 7. Transfer with wrong PIN -> Must fail
	_, _, err = walletSvc.Transfer(supabaseUserMock(senderID, senderEmail), "", TransferRequest{
		TargetUserID: recipientID,
		Amount:       25000,
		PIN:          "000000",
	})
	if err == nil {
		t.Error("expected transfer to fail with incorrect PIN")
	}

	// 8. Self-transfer -> Must fail
	_, _, err = walletSvc.Transfer(supabaseUserMock(senderID, senderEmail), "", TransferRequest{
		TargetUserID: senderID,
		Amount:       25000,
		PIN:          "654321",
	})
	if err == nil {
		t.Error("expected transfer to self to fail")
	}

	// 9. Transfer with correct PIN Rp 25.000 -> Must succeed
	trxTransfer, walSenderAfter, err := walletSvc.Transfer(supabaseUserMock(senderID, senderEmail), "", TransferRequest{
		TargetUserID: recipientID,
		Amount:       25000,
		PIN:          "654321",
		Notes:        "Bayar makan siang",
	})
	if err != nil {
		t.Fatalf("failed to execute transfer: %v", err)
	}

	if walSenderAfter.Balance != 75000 {
		t.Errorf("expected sender balance to be 75000, got %f", walSenderAfter.Balance)
	}
	if trxTransfer.Amount != 25000 || trxTransfer.Type != TypeDebit || trxTransfer.Category != CategoryTransferOut {
		t.Errorf("unexpected transfer transaction: %+v", trxTransfer)
	}

	// Verify recipient received balance
	walRecipientAfter, _ := walletSvc.GetBalance(recipientID, recipientEmail, "")
	if walRecipientAfter.Balance != 25000 {
		t.Errorf("expected recipient balance to be 25000, got %f", walRecipientAfter.Balance)
	}

	// 10. Transfer exceeding balance -> Must fail
	_, _, err = walletSvc.Transfer(supabaseUserMock(senderID, senderEmail), "", TransferRequest{
		TargetUserID: recipientID,
		Amount:       1000000,
		PIN:          "654321",
	})
	if err == nil {
		t.Error("expected transfer exceeding balance to fail")
	}
}
