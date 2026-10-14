package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestSuperAdminUserControl(t *testing.T) {
	walletHandler := wallet.NewHandler(nil)
	walletSvc := walletHandler.GetWalletService()
	walletSvc.TopUp("target-user-1", "original@dewacell.com", "", wallet.TopUpRequest{
		Amount:  100000,
		Channel: "Bank",
	})

	adminHandler := NewHandler(nil, walletHandler)

	// 1. Test Reset Password
	pwdBody := `{"password":"newsecretpassword123"}`
	reqPwd := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/target-user-1/password", strings.NewReader(pwdBody))
	reqPwd.Header.Set("Content-Type", "application/json")
	rrPwd := httptest.NewRecorder()
	adminHandler.ResetUserPassword(rrPwd, reqPwd)
	if rrPwd.Code != http.StatusOK {
		t.Fatalf("expected status 200 for ResetUserPassword, got %d", rrPwd.Code)
	}

	// 2. Test Update Email
	emailBody := `{"email":"newemail@dewacell.com"}`
	reqEmail := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/target-user-1/email", strings.NewReader(emailBody))
	reqEmail.Header.Set("Content-Type", "application/json")
	rrEmail := httptest.NewRecorder()
	adminHandler.UpdateUserEmail(rrEmail, reqEmail)
	if rrEmail.Code != http.StatusOK {
		t.Fatalf("expected status 200 for UpdateUserEmail, got %d", rrEmail.Code)
	}

	// 3. Test Full Update User
	updateBody := `{"email":"updatedfull@dewacell.com","role":"agen"}`
	reqUpdate := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/target-user-1", strings.NewReader(updateBody))
	reqUpdate.Header.Set("Content-Type", "application/json")
	rrUpdate := httptest.NewRecorder()
	adminHandler.UpdateUser(rrUpdate, reqUpdate)
	if rrUpdate.Code != http.StatusOK {
		t.Fatalf("expected status 200 for UpdateUser, got %d", rrUpdate.Code)
	}

	// 4. Test Delete User
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/target-user-1", nil)
	rrDel := httptest.NewRecorder()
	adminHandler.DeleteUser(rrDel, reqDel)
	if rrDel.Code != http.StatusOK {
		t.Fatalf("expected status 200 for DeleteUser, got %d", rrDel.Code)
	}
}
