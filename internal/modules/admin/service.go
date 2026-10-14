package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dede-wandi/server-dewa/internal/domain/role"
	"github.com/dede-wandi/server-dewa/internal/modules/wallet"
	"github.com/dede-wandi/server-dewa/internal/supabase"
)

// Service defines the business logic interface for Super Admin operations
type Service interface {
	ListUsers(roleFilter, search, token string) ([]UserListItem, error)
	GetUserDetail(userID string) (*UserDetailResponse, error)
	ResetPassword(userID, newPassword, token string) error
	UpdateEmail(userID, newEmail, token string) error
	UpdateUser(userID string, req UpdateUserRequest, token string) error
	DeleteUser(userID, token string) error
	LockUser(userID string) (bool, error)
	UnlockUser(userID string) (bool, error)
	TopUpUser(adminUser *supabase.User, token string, req AdminTopUpRequest) (interface{}, error)
	ListAllTransactions() (interface{}, int)
	ListAllPointTransactions() (interface{}, int)
	AdjustPoints(targetUserID, adjType string, amount int64, reason, adminEmail string) (interface{}, error)
	AssignRole(email, role, token string) (interface{}, error)
	RemoveRole(email, role, token string) (interface{}, error)
}

type adminService struct {
	repo          Repository
	walletHandler *wallet.Handler
	sbClient      *supabase.Client
}

// NewService creates a new instance of Admin Service
func NewService(repo Repository, walletHandler *wallet.Handler, sbClient *supabase.Client) Service {
	return &adminService{
		repo:          repo,
		walletHandler: walletHandler,
		sbClient:      sbClient,
	}
}

func isSuperAdminUser(rStr, emailStr string) bool {
	cleanRole := strings.ToLower(strings.TrimSpace(rStr))
	cleanEmail := strings.ToLower(strings.TrimSpace(emailStr))

	if cleanRole == "superadmin" || strings.Contains(cleanRole, "superadmin") {
		return true
	}
	if strings.HasPrefix(cleanEmail, "superadmin") || strings.Contains(cleanEmail, "superadmin") {
		return true
	}
	return false
}

func (s *adminService) ListUsers(roleFilter, search, token string) ([]UserListItem, error) {
	// 1. Try querying real users & wallets directly from database via Supabase RPC
	if s.repo != nil {
		if dbUsers, err := s.repo.FetchRegisteredUsersWallets(token); err == nil && len(dbUsers) > 0 {
			filtered := make([]UserListItem, 0, len(dbUsers))
			for _, u := range dbUsers {
				userRole, _ := u["role"].(string)
				userEmail, _ := u["user_email"].(string)
				userID, _ := u["user_id"].(string)

				if isSuperAdminUser(userRole, userEmail) {
					continue
				}

				def := role.GlobalRegistry.Resolve(userRole, userEmail)
				if def.Role == role.RoleSuperAdmin {
					continue
				}

				if roleFilter != "" && !strings.EqualFold(string(def.Role), roleFilter) {
					continue
				}
				if search != "" && !strings.Contains(strings.ToLower(userEmail), search) {
					continue
				}

				var balance float64
				if b, ok := u["balance"].(float64); ok {
					balance = b
				}
				var points int64
				if p, ok := u["points"].(float64); ok {
					points = int64(p)
				}
				var isLocked bool
				if l, ok := u["is_locked"].(bool); ok {
					isLocked = l
				}
				createdAt, _ := u["created_at"].(string)
				updatedAt, _ := u["updated_at"].(string)

				filtered = append(filtered, UserListItem{
					UserID:    userID,
					UserEmail: userEmail,
					Role:      string(def.Role),
					RoleName:  def.Name,
					Category:  string(def.Category),
					Balance:   balance,
					Points:    points,
					IsLocked:  isLocked,
					CreatedAt: createdAt,
					UpdatedAt: updatedAt,
				})
			}
			return filtered, nil
		}

		// 2. Query all registered users directly from Supabase Auth Admin API
		if authUsers, err := s.repo.FetchAuthAdminUsers(1, 100); err == nil && len(authUsers) > 0 {
			list := make([]UserListItem, 0, len(authUsers))
			for _, u := range authUsers {
				if isSuperAdminUser(u.Role, u.Email) {
					continue
				}

				userRoles := u.GetRoles()
				isSuper := false
				for _, r := range userRoles {
					if isSuperAdminUser(r, u.Email) {
						isSuper = true
						break
					}
				}
				if isSuper {
					continue
				}

				primaryRole := "user"
				if len(userRoles) > 0 {
					primaryRole = userRoles[0]
				}
				def := role.GlobalRegistry.Resolve(primaryRole, u.Email)
				if def.Role == role.RoleSuperAdmin {
					continue
				}

				if roleFilter != "" && !strings.EqualFold(string(def.Role), roleFilter) {
					continue
				}
				if search != "" && !strings.Contains(strings.ToLower(u.Email), search) {
					continue
				}

				var balance float64
				var points int64
				var isLocked bool
				var updatedAt string
				if s.walletHandler != nil {
					wal, _ := s.walletHandler.GetWalletService().GetBalance(u.ID, u.Email, "")
					if wal != nil {
						balance = wal.Balance
						points = wal.Points
						isLocked = wal.IsLocked
						updatedAt = wal.UpdatedAt.Format(time.RFC3339)
					} else {
						updatedAt = u.CreatedAt
					}
				} else {
					updatedAt = u.CreatedAt
				}

				list = append(list, UserListItem{
					UserID:    u.ID,
					UserEmail: u.Email,
					Role:      string(def.Role),
					RoleName:  def.Name,
					Category:  string(def.Category),
					Balance:   balance,
					Points:    points,
					IsLocked:  isLocked,
					CreatedAt: u.CreatedAt,
					UpdatedAt: updatedAt,
				})
			}
			return list, nil
		}
	}

	// 3. Fallback to active runtime wallets
	if s.walletHandler != nil {
		wallets := s.walletHandler.GetAllWallets()
		userList := make([]UserListItem, 0, len(wallets))

		for _, wal := range wallets {
			if isSuperAdminUser(wal.Role, wal.UserEmail) {
				continue
			}
			def := role.GlobalRegistry.Resolve(wal.Role, wal.UserEmail)
			if def.Role == role.RoleSuperAdmin {
				continue
			}

			if roleFilter != "" && !strings.EqualFold(string(def.Role), roleFilter) {
				continue
			}
			if search != "" && !strings.Contains(strings.ToLower(wal.UserEmail), search) {
				continue
			}

			userList = append(userList, UserListItem{
				UserID:    wal.UserID,
				UserEmail: wal.UserEmail,
				Role:      string(def.Role),
				RoleName:  def.Name,
				Category:  string(def.Category),
				Balance:   wal.Balance,
				Points:    wal.Points,
				IsLocked:  wal.IsLocked,
				CreatedAt: wal.UpdatedAt.Format(time.RFC3339),
				UpdatedAt: wal.UpdatedAt.Format(time.RFC3339),
			})
		}
		return userList, nil
	}

	return []UserListItem{}, nil
}

func (s *adminService) GetUserDetail(userID string) (*UserDetailResponse, error) {
	cleanID := strings.TrimSpace(userID)
	if cleanID == "" {
		return nil, errors.New("User ID tidak boleh kosong")
	}

	var foundUser *UserListItem
	var transactions interface{} = []interface{}{}
	var userMeta map[string]interface{}

	// Always retrieve complete Supabase Auth User details and metadata
	var authUser *supabase.User
	if s.repo != nil {
		if directUser, err := s.repo.FetchAuthAdminUser(cleanID); err == nil && directUser != nil {
			authUser = directUser
		}
	}

	// 1. Try finding user from registered users & wallets RPC
	if s.repo != nil {
		if rawUsers, err := s.repo.FetchRegisteredUsersWallets(""); err == nil && len(rawUsers) > 0 {
			for _, u := range rawUsers {
				uID, _ := u["user_id"].(string)
				if strings.EqualFold(uID, cleanID) {
					userEmail, _ := u["user_email"].(string)
					roleStr, _ := u["role"].(string)
					def := role.GlobalRegistry.Resolve(roleStr, userEmail)

					var balance float64
					if b, ok := u["balance"].(float64); ok {
						balance = b
					}
					var points int64
					if p, ok := u["points"].(float64); ok {
						points = int64(p)
					}
					var isLocked bool
					if l, ok := u["is_locked"].(bool); ok {
						isLocked = l
					}
					createdAt, _ := u["created_at"].(string)
					updatedAt, _ := u["updated_at"].(string)

					if um, ok := u["user_metadata"].(map[string]interface{}); ok && len(um) > 0 {
						userMeta = um
					}

					foundUser = &UserListItem{
						UserID:    uID,
						UserEmail: userEmail,
						Role:      string(def.Role),
						RoleName:  def.Name,
						Category:  string(def.Category),
						Balance:   balance,
						Points:    points,
						IsLocked:  isLocked,
						CreatedAt: createdAt,
						UpdatedAt: updatedAt,
					}
					break
				}
			}
		}

		// 2. Fallback to Supabase Auth Admin user
		if foundUser == nil && authUser != nil {
			userRoles := authUser.GetRoles()
			primaryRole := "user"
			if len(userRoles) > 0 {
				primaryRole = userRoles[0]
			}
			def := role.GlobalRegistry.Resolve(primaryRole, authUser.Email)

			var balance float64
			var points int64
			var isLocked bool
			var updatedAt string

			if s.walletHandler != nil {
				wal, _ := s.walletHandler.GetWalletService().GetBalance(authUser.ID, authUser.Email, "")
				if wal != nil {
					balance = wal.Balance
					points = wal.Points
					isLocked = wal.IsLocked
					updatedAt = wal.UpdatedAt.Format(time.RFC3339)
				} else {
					updatedAt = authUser.CreatedAt
				}
			} else {
				updatedAt = authUser.CreatedAt
			}

			foundUser = &UserListItem{
				UserID:    authUser.ID,
				UserEmail: authUser.Email,
				Role:      string(def.Role),
				RoleName:  def.Name,
				Category:  string(def.Category),
				Balance:   balance,
				Points:    points,
				IsLocked:  isLocked,
				CreatedAt: authUser.CreatedAt,
				UpdatedAt: updatedAt,
			}
		}
	}

	// 3. Fallback to runtime wallet
	if foundUser == nil && s.walletHandler != nil {
		wal, trxs, err := s.walletHandler.GetUserWallet(cleanID)
		if err == nil && wal != nil {
			def := role.GlobalRegistry.Resolve(wal.Role, wal.UserEmail)
			foundUser = &UserListItem{
				UserID:    wal.UserID,
				UserEmail: wal.UserEmail,
				Role:      string(def.Role),
				RoleName:  def.Name,
				Category:  string(def.Category),
				Balance:   wal.Balance,
				Points:    wal.Points,
				IsLocked:  wal.IsLocked,
				CreatedAt: wal.UpdatedAt.Format(time.RFC3339),
				UpdatedAt: wal.UpdatedAt.Format(time.RFC3339),
			}
			transactions = trxs
		}
	}

	if foundUser == nil {
		return nil, errors.New("Pengguna tidak ditemukan")
	}

	// Fetch transaction history if not fetched yet
	if s.walletHandler != nil {
		history := s.walletHandler.GetWalletService().GetHistory(cleanID)
		if len(history) > 0 {
			transactions = history
		}
	}

	// Extract full profile information from user_metadata and app_metadata
	var fullName, phone, address, businessName, businessPhone, businessAddress, waTarget1, waTarget2, waTarget3 string

	if authUser != nil {
		if len(authUser.UserMetadata) > 0 {
			userMeta = authUser.UserMetadata
		}
		if authUser.Phone != "" {
			phone = authUser.Phone
		}
	}

	// Fallback to admin_get_user_detail RPC if userMeta is still nil
	if userMeta == nil && s.sbClient != nil {
		if respBytes, err := s.sbClient.CallRPC("admin_get_user_detail", map[string]interface{}{"p_user_id": cleanID}, ""); err == nil && len(respBytes) > 0 {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["user_id"] != nil {
				if um, ok := rpcRes["user_metadata"].(map[string]interface{}); ok && len(um) > 0 {
					userMeta = um
				}
			}
		}
	}

	if userMeta != nil {
		if fn, ok := userMeta["full_name"].(string); ok && fn != "" {
			fullName = fn
		} else if dn, ok := userMeta["display_name"].(string); ok && dn != "" {
			fullName = dn
		}
		if p, ok := userMeta["phone"].(string); ok && p != "" {
			phone = p
		}
		address, _ = userMeta["address"].(string)
		businessName, _ = userMeta["business_name"].(string)
		businessPhone, _ = userMeta["business_phone"].(string)
		businessAddress, _ = userMeta["business_address"].(string)
		waTarget1, _ = userMeta["wa_target_1"].(string)
		waTarget2, _ = userMeta["wa_target_2"].(string)
		waTarget3, _ = userMeta["wa_target_3"].(string)
	}

	return &UserDetailResponse{
		UserID:          foundUser.UserID,
		UserEmail:       foundUser.UserEmail,
		Role:            foundUser.Role,
		RoleName:        foundUser.RoleName,
		Category:        foundUser.Category,
		FullName:        fullName,
		Phone:           phone,
		Address:         address,
		BusinessName:    businessName,
		BusinessPhone:   businessPhone,
		BusinessAddress: businessAddress,
		WATarget1:       waTarget1,
		WATarget2:       waTarget2,
		WATarget3:       waTarget3,
		Balance:         foundUser.Balance,
		Points:          foundUser.Points,
		IsLocked:        foundUser.IsLocked,
		CreatedAt:       foundUser.CreatedAt,
		UpdatedAt:       foundUser.UpdatedAt,
		Transactions:    transactions,
	}, nil
}

func (s *adminService) TopUpUser(adminUser *supabase.User, token string, req AdminTopUpRequest) (interface{}, error) {
	req.TargetUserID = strings.TrimSpace(req.TargetUserID)
	if req.TargetUserID == "" {
		return nil, errors.New("Target user ID wajib disertakan")
	}
	if req.Amount <= 0 {
		return nil, errors.New("Nominal top up harus lebih besar dari 0")
	}

	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		channel = "Deposit Tunai / Transfer Admin"
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		adminEmail := "Super Admin"
		if adminUser != nil && adminUser.Email != "" {
			adminEmail = adminUser.Email
		}
		title = fmt.Sprintf("Top Up Saldo oleh Admin (%s)", adminEmail)
	}

	desc := strings.TrimSpace(req.Description)
	if desc == "" {
		desc = fmt.Sprintf("Penambahan saldo pengguna via %s", channel)
		if req.Notes != "" {
			desc += fmt.Sprintf(" [Catatan: %s]", req.Notes)
		}
	}

	refID := fmt.Sprintf("TOPUP-ADM-%s-%d", time.Now().Format("20060102"), time.Now().UnixNano()%100000)

	// 1. Try Supabase RPC process_app_wallet_mutation
	if s.sbClient != nil {
		rpcPayload := map[string]interface{}{
			"p_user_id":      req.TargetUserID,
			"p_type":         wallet.TypeCredit,
			"p_category":     wallet.CategoryTopup,
			"p_amount":       req.Amount,
			"p_title":        title,
			"p_description":  desc,
			"p_reference_id": refID,
		}
		if respBytes, err := s.sbClient.CallRPC("process_app_wallet_mutation", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				balAfter, _ := rpcRes["balance_after"].(float64)
				balBefore, _ := rpcRes["balance_before"].(float64)
				trxCode, _ := rpcRes["transaction_code"].(string)

				return map[string]interface{}{
					"status":  "success",
					"message": fmt.Sprintf("Saldo sebesar Rp %s berhasil ditambahkan ke akun pengguna!", formatRupiahAdmin(int64(req.Amount))),
					"transaction": map[string]interface{}{
						"transaction_code": trxCode,
						"user_id":          req.TargetUserID,
						"type":             wallet.TypeCredit,
						"category":         wallet.CategoryTopup,
						"amount":           req.Amount,
						"balance_before":   balBefore,
						"balance_after":    balAfter,
						"title":            title,
						"description":      desc,
						"reference_id":     refID,
						"status":           "SUCCESS",
						"created_at":       time.Now().Format(time.RFC3339),
					},
					"target_user_wallet": map[string]interface{}{
						"user_id":        req.TargetUserID,
						"balance_before": balBefore,
						"balance_after":  balAfter,
						"formatted":      fmt.Sprintf("Rp %s", formatRupiahAdmin(int64(balAfter))),
					},
				}, nil
			}
		}
	}

	// 2. Fallback to runtime wallet
	if s.walletHandler != nil {
		targetWal, _ := s.walletHandler.GetWalletService().GetBalance(req.TargetUserID, "", "")
		if targetWal == nil {
			return nil, errors.New("Pengguna tidak ditemukan")
		}
		balBefore := targetWal.Balance
		targetWal.Balance += req.Amount
		balAfter := targetWal.Balance

		return map[string]interface{}{
			"status":  "success",
			"message": fmt.Sprintf("Saldo sebesar Rp %s berhasil ditambahkan ke akun pengguna!", formatRupiahAdmin(int64(req.Amount))),
			"transaction": map[string]interface{}{
				"transaction_code": refID,
				"user_id":          req.TargetUserID,
				"type":             wallet.TypeCredit,
				"category":         wallet.CategoryTopup,
				"amount":           req.Amount,
				"balance_before":   balBefore,
				"balance_after":    balAfter,
				"title":            title,
				"description":      desc,
				"reference_id":     refID,
				"status":           "SUCCESS",
				"created_at":       time.Now().Format(time.RFC3339),
			},
			"target_user_wallet": map[string]interface{}{
				"user_id":        req.TargetUserID,
				"balance_before": balBefore,
				"balance_after":  balAfter,
				"formatted":      fmt.Sprintf("Rp %s", formatRupiahAdmin(int64(balAfter))),
			},
		}, nil
	}

	return nil, errors.New("Layanan dompet tidak tersedia")
}

func formatRupiahAdmin(amount int64) string {
	str := fmt.Sprintf("%d", amount)
	n := len(str)
	if n <= 3 {
		return str
	}
	var res []byte
	rem := n % 3
	if rem > 0 {
		res = append(res, []byte(str[:rem])...)
		if rem < n {
			res = append(res, '.')
		}
	}
	for i := rem; i < n; i += 3 {
		res = append(res, []byte(str[i:i+3])...)
		if i+3 < n {
			res = append(res, '.')
		}
	}
	return string(res)
}

func (s *adminService) ResetPassword(userID, newPassword, token string) error {
	if s.repo != nil {
		return s.repo.ResetPassword(userID, newPassword, token)
	}
	return nil
}

func (s *adminService) UpdateEmail(userID, newEmail, token string) error {
	if s.repo != nil {
		_ = s.repo.UpdateEmail(userID, newEmail, token)
	}
	if s.walletHandler != nil {
		if wal, _, err := s.walletHandler.GetUserWallet(userID); err == nil && wal != nil {
			wal.UserEmail = newEmail
		}
	}
	return nil
}

func (s *adminService) UpdateUser(userID string, req UpdateUserRequest, token string) error {
	if req.IsLocked != nil && s.walletHandler != nil {
		s.walletHandler.SetUserLock(userID, *req.IsLocked)
	}
	if s.repo != nil {
		return s.repo.UpdateUser(userID, req, token)
	}
	return nil
}

func (s *adminService) DeleteUser(userID, token string) error {
	if s.repo != nil {
		return s.repo.DeleteUser(userID, token)
	}
	return nil
}

func (s *adminService) LockUser(userID string) (bool, error) {
	if s.walletHandler == nil {
		return false, fmt.Errorf("wallet handler not available")
	}
	wal, err := s.walletHandler.SetUserLock(userID, true)
	if err != nil {
		return false, err
	}
	return wal.IsLocked, nil
}

func (s *adminService) UnlockUser(userID string) (bool, error) {
	if s.walletHandler == nil {
		return false, fmt.Errorf("wallet handler not available")
	}
	wal, err := s.walletHandler.SetUserLock(userID, false)
	if err != nil {
		return false, err
	}
	return wal.IsLocked, nil
}

func (s *adminService) ListAllTransactions() (interface{}, int) {
	if s.repo != nil {
		if trxs, err := s.repo.FetchAllTransactions(200); err == nil && len(trxs) > 0 {
			return trxs, len(trxs)
		}
	}
	if s.walletHandler != nil {
		trxs := s.walletHandler.GetAllTransactions()
		return trxs, len(trxs)
	}
	return []interface{}{}, 0
}

func (s *adminService) ListAllPointTransactions() (interface{}, int) {
	if s.repo != nil {
		if pts, err := s.repo.FetchAllPointTransactions(200); err == nil && len(pts) > 0 {
			return pts, len(pts)
		}
	}
	if s.walletHandler != nil {
		pts := s.walletHandler.GetAllPointTransactions()
		return pts, len(pts)
	}
	return []interface{}{}, 0
}

func (s *adminService) AdjustPoints(targetUserID, adjType string, amount int64, reason, adminEmail string) (interface{}, error) {
	if s.walletHandler == nil {
		return nil, fmt.Errorf("wallet handler not available")
	}
	return s.walletHandler.AdjustPoints(targetUserID, adjType, amount, reason, adminEmail)
}

func (s *adminService) AssignRole(email, roleName, token string) (interface{}, error) {
	if s.repo != nil {
		return s.repo.AssignRole(email, roleName, token)
	}
	return nil, nil
}

func (s *adminService) RemoveRole(email, roleName, token string) (interface{}, error) {
	if s.repo != nil {
		return s.repo.RemoveRole(email, roleName, token)
	}
	return nil, nil
}
