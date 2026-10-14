package admin

import (
	"fmt"
	"strings"
	"time"

	"github.com/dede-wandi/server-dewa/internal/domain/role"
	"github.com/dede-wandi/server-dewa/internal/modules/wallet"
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
	ListAllTransactions() (interface{}, int)
	ListAllPointTransactions() (interface{}, int)
	AdjustPoints(targetUserID, adjType string, amount int64, reason, adminEmail string) (interface{}, error)
	AssignRole(email, role, token string) (interface{}, error)
	RemoveRole(email, role, token string) (interface{}, error)
}

type adminService struct {
	repo          Repository
	walletHandler *wallet.Handler
}

// NewService creates a new instance of Admin Service
func NewService(repo Repository, walletHandler *wallet.Handler) Service {
	return &adminService{
		repo:          repo,
		walletHandler: walletHandler,
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
	if s.walletHandler == nil {
		return nil, fmt.Errorf("wallet handler not available")
	}

	wal, trxs, err := s.walletHandler.GetUserWallet(userID)
	if err != nil {
		return nil, err
	}

	return &UserDetailResponse{
		UserID:       wal.UserID,
		UserEmail:    wal.UserEmail,
		Balance:      wal.Balance,
		Points:       wal.Points,
		IsLocked:     wal.IsLocked,
		UpdatedAt:    wal.UpdatedAt.Format(time.RFC3339),
		Transactions: trxs,
	}, nil
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
