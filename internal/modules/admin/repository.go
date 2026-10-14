package admin

import (
	"encoding/json"
	"fmt"

	"github.com/dede-wandi/server-dewa/internal/supabase"
)

// Repository defines the contract for admin database and Supabase operations
type Repository interface {
	FetchRegisteredUsersWallets(token string) ([]map[string]interface{}, error)
	FetchAuthAdminUsers(page, perPage int) ([]supabase.User, error)
	ResetPassword(userID, newPassword, token string) error
	UpdateEmail(userID, newEmail, token string) error
	UpdateUser(userID string, req UpdateUserRequest, token string) error
	DeleteUser(userID, token string) error
	FetchAllTransactions(limit int) ([]map[string]interface{}, error)
	FetchAllPointTransactions(limit int) ([]map[string]interface{}, error)
	AssignRole(email, role, token string) (interface{}, error)
	RemoveRole(email, role, token string) (interface{}, error)
}

type supabaseRepository struct {
	sbClient *supabase.Client
}

// NewRepository creates a new instance of Admin Repository
func NewRepository(sbClient *supabase.Client) Repository {
	return &supabaseRepository{
		sbClient: sbClient,
	}
}

func (r *supabaseRepository) FetchRegisteredUsersWallets(token string) ([]map[string]interface{}, error) {
	if r.sbClient == nil {
		return nil, fmt.Errorf("supabase client not available")
	}

	dbBytes, err := r.sbClient.CallRPC("get_all_registered_users_wallets", nil, token)
	if err != nil || len(dbBytes) == 0 {
		return nil, err
	}

	var dbUsers []map[string]interface{}
	if err := json.Unmarshal(dbBytes, &dbUsers); err != nil {
		return nil, err
	}
	return dbUsers, nil
}

func (r *supabaseRepository) FetchAuthAdminUsers(page, perPage int) ([]supabase.User, error) {
	if r.sbClient == nil {
		return nil, fmt.Errorf("supabase client not available")
	}
	return r.sbClient.AdminListUsers(page, perPage)
}

func (r *supabaseRepository) ResetPassword(userID, newPassword, token string) error {
	if r.sbClient == nil {
		return nil
	}

	// 1. Try Supabase Auth Admin API if secret key is present
	if r.sbClient.HasSecretKey() {
		if _, err := r.sbClient.AdminUpdateUser(userID, map[string]interface{}{"password": newPassword}); err == nil {
			return nil
		}
	}

	// 2. Try Supabase PostgreSQL RPC (Security Definer)
	rpcPayload := map[string]interface{}{
		"p_user_id":      userID,
		"p_new_password": newPassword,
	}
	_, err := r.sbClient.CallRPC("admin_reset_user_password", rpcPayload, token)
	return err
}

func (r *supabaseRepository) UpdateEmail(userID, newEmail, token string) error {
	if r.sbClient == nil {
		return nil
	}

	// 1. Try Supabase Auth Admin API
	if r.sbClient.HasSecretKey() {
		if _, err := r.sbClient.AdminUpdateUser(userID, map[string]interface{}{"email": newEmail, "email_confirm": true}); err == nil {
			return nil
		}
	}

	// 2. Try Supabase PostgreSQL RPC
	rpcPayload := map[string]interface{}{
		"p_user_id":   userID,
		"p_new_email": newEmail,
	}
	_, err := r.sbClient.CallRPC("admin_update_user_email", rpcPayload, token)
	return err
}

func (r *supabaseRepository) UpdateUser(userID string, req UpdateUserRequest, token string) error {
	if r.sbClient == nil {
		return nil
	}

	// 1. Try Supabase Auth Admin API
	if r.sbClient.HasSecretKey() {
		updatePayload := make(map[string]interface{})
		if req.Email != "" {
			updatePayload["email"] = req.Email
			updatePayload["email_confirm"] = true
		}
		if req.Password != "" {
			updatePayload["password"] = req.Password
		}
		if req.Role != "" {
			updatePayload["app_metadata"] = map[string]interface{}{"role": req.Role}
		}
		if req.DisplayName != "" {
			updatePayload["user_metadata"] = map[string]interface{}{"full_name": req.DisplayName, "display_name": req.DisplayName}
		}

		if len(updatePayload) > 0 {
			if _, err := r.sbClient.AdminUpdateUser(userID, updatePayload); err == nil {
				return nil
			}
		}
	}

	// 2. Try Supabase PostgreSQL RPC
	rpcPayload := map[string]interface{}{
		"p_user_id": userID,
	}
	if req.Email != "" {
		rpcPayload["p_new_email"] = req.Email
	}
	if req.Password != "" {
		rpcPayload["p_new_password"] = req.Password
	}
	if req.Role != "" {
		rpcPayload["p_new_role"] = req.Role
	}

	_, err := r.sbClient.CallRPC("admin_update_user", rpcPayload, token)
	return err
}

func (r *supabaseRepository) DeleteUser(userID, token string) error {
	if r.sbClient == nil {
		return nil
	}

	if r.sbClient.HasSecretKey() {
		if err := r.sbClient.AdminDeleteUser(userID); err == nil {
			return nil
		}
	}

	rpcPayload := map[string]interface{}{
		"p_user_id": userID,
	}
	_, err := r.sbClient.CallRPC("admin_delete_user", rpcPayload, token)
	return err
}

func (r *supabaseRepository) FetchAllTransactions(limit int) ([]map[string]interface{}, error) {
	if r.sbClient == nil {
		return nil, fmt.Errorf("supabase client not available")
	}

	dbBytes, err := r.sbClient.CallRPC("get_all_app_transactions", map[string]interface{}{"p_limit": limit}, "")
	if err != nil || len(dbBytes) == 0 {
		return nil, err
	}

	var dbTrxs []map[string]interface{}
	if err := json.Unmarshal(dbBytes, &dbTrxs); err != nil {
		return nil, err
	}
	return dbTrxs, nil
}

func (r *supabaseRepository) FetchAllPointTransactions(limit int) ([]map[string]interface{}, error) {
	if r.sbClient == nil {
		return nil, fmt.Errorf("supabase client not available")
	}

	dbBytes, err := r.sbClient.CallRPC("get_all_point_transactions", map[string]interface{}{"p_limit": limit}, "")
	if err != nil || len(dbBytes) == 0 {
		return nil, err
	}

	var dbPts []map[string]interface{}
	if err := json.Unmarshal(dbBytes, &dbPts); err != nil {
		return nil, err
	}
	return dbPts, nil
}

func (r *supabaseRepository) AssignRole(email, role, token string) (interface{}, error) {
	if r.sbClient == nil {
		return nil, fmt.Errorf("supabase client not available")
	}

	rpcPayload := map[string]interface{}{
		"p_email": email,
		"p_role":  role,
	}
	respBytes, err := r.sbClient.CallRPC("assign_user_role", rpcPayload, token)
	if err != nil {
		return nil, err
	}

	var rpcRes interface{}
	json.Unmarshal(respBytes, &rpcRes)
	return rpcRes, nil
}

func (r *supabaseRepository) RemoveRole(email, role, token string) (interface{}, error) {
	if r.sbClient == nil {
		return nil, fmt.Errorf("supabase client not available")
	}

	rpcPayload := map[string]interface{}{
		"p_email": email,
		"p_role":  role,
	}
	respBytes, err := r.sbClient.CallRPC("remove_user_role", rpcPayload, token)
	if err != nil {
		return nil, err
	}

	var rpcRes interface{}
	json.Unmarshal(respBytes, &rpcRes)
	return rpcRes, nil
}
