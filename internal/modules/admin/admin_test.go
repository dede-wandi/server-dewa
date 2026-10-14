package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dede-wandi/server-dewa/internal/modules/wallet"
)

func TestListUsersExcludesSuperAdmin(t *testing.T) {
	// Setup wallet handler and seed test users with various roles
	walletHandler := wallet.NewHandler(nil)
	walletSvc := walletHandler.GetWalletService()

	// Seed: regular user
	walletSvc.TopUp("user-1", "user1@gmail.com", "", wallet.TopUpRequest{
		Amount:  50000,
		Channel: "Bank",
	})

	// Seed: cashier
	walletSvc.TopUp("kasir-1", "kasir1@gmail.com", "", wallet.TopUpRequest{
		Amount:  20000,
		Channel: "Cash",
	})
	walletHandler.SetUserLock("kasir-1", false)

	// Seed: superadmin
	walletHandler.GetAllWallets()
	wallets := walletHandler.GetAllWallets()
	for _, w := range wallets {
		if w.UserID == "user-1" {
			w.Role = "user"
		} else if w.UserID == "kasir-1" {
			w.Role = "kasir"
		}
	}

	// Manually add a superadmin wallet to verify it is filtered out
	superAdminWal := &wallet.Wallet{
		UserID:    "superadmin-uuid-999",
		UserEmail: "superadmin@dewacell.com",
		Role:      "superadmin",
		Balance:   999999999,
		Points:    99999,
	}
	walletHandler.GetWalletService()
	// Top up superadmin to have it in the memory repo
	walletSvc.TopUp(superAdminWal.UserID, superAdminWal.UserEmail, "", wallet.TopUpRequest{
		Amount:  100000,
		Channel: "System",
	})
	for _, w := range walletHandler.GetAllWallets() {
		if w.UserID == superAdminWal.UserID {
			w.Role = "superadmin"
		}
	}

	// Initialize admin handler
	adminHandler := NewHandler(nil, walletHandler)

	// Make request to ListUsers
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rr := httptest.NewRecorder()

	adminHandler.ListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", rr.Code)
	}

	var res struct {
		Status string                   `json:"status"`
		Count  int                      `json:"count"`
		Data   []map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if res.Count == 0 {
		t.Fatalf("expected non-zero users, got %d", res.Count)
	}

	// Verify that NO user in the returned list is superadmin
	for _, u := range res.Data {
		roleStr, _ := u["role"].(string)
		email, _ := u["user_email"].(string)
		userID, _ := u["user_id"].(string)

		if roleStr == "superadmin" || userID == "superadmin-uuid-999" || email == "superadmin@dewacell.com" {
			t.Errorf("CRITICAL: superadmin was found in ListUsers! user: %v", u)
		}
	}
}
