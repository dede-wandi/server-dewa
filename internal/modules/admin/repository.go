package admin

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/dede-wandi/server-dewa/internal/supabase"
)

// Repository defines the contract for admin database and Supabase operations
type Repository interface {
	FetchRegisteredUsersWallets(token string) ([]map[string]interface{}, error)
	FetchAuthAdminUsers(page, perPage int) ([]supabase.User, error)
	FetchAuthAdminUser(userID string) (*supabase.User, error)
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

func (r *supabaseRepository) FetchAuthAdminUser(userID string) (*supabase.User, error) {
	if r.sbClient == nil {
		return nil, fmt.Errorf("supabase client not available")
	}
	return r.sbClient.AdminGetUser(userID)
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

	// Build user metadata from request
	userMeta := make(map[string]interface{})
	if r.sbClient.HasSecretKey() {
		if existingUser, err := r.sbClient.AdminGetUser(userID); err == nil && existingUser != nil && existingUser.UserMetadata != nil {
			for k, v := range existingUser.UserMetadata {
				userMeta[k] = v
			}
		}
	}

	// Merge any custom user_metadata provided
	if req.UserMetadata != nil {
		for k, v := range req.UserMetadata {
			userMeta[k] = v
		}
	}

	// Merge specific profile fields
	if req.FullName != "" {
		userMeta["full_name"] = req.FullName
	} else if req.DisplayName != "" {
		userMeta["full_name"] = req.DisplayName
		userMeta["display_name"] = req.DisplayName
	}
	if req.Phone != "" {
		userMeta["phone"] = req.Phone
	}
	if req.Address != "" {
		userMeta["address"] = req.Address
	}
	if req.BusinessName != "" {
		userMeta["business_name"] = req.BusinessName
	}
	if req.BusinessPhone != "" {
		userMeta["business_phone"] = req.BusinessPhone
	}
	if req.BusinessAddress != "" {
		userMeta["business_address"] = req.BusinessAddress
	}
	if req.WATarget1 != "" {
		userMeta["wa_target_1"] = req.WATarget1
	}
	if req.WATarget2 != "" {
		userMeta["wa_target_2"] = req.WATarget2
	}
	if req.WATarget3 != "" {
		userMeta["wa_target_3"] = req.WATarget3
	}

	// 1. Try Supabase Auth Admin API if secret key is present
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

		if len(userMeta) > 0 {
			updatePayload["user_metadata"] = userMeta
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
	if len(userMeta) > 0 {
		rpcPayload["p_user_metadata"] = userMeta
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

	enrichWithUsers := func(trxs []map[string]interface{}) {
		users, errUsers := r.FetchRegisteredUsersWallets("")
		if errUsers == nil && len(users) > 0 {
			userMap := make(map[string]map[string]string)
			for _, u := range users {
				uID, ok := u["user_id"].(string)
				if !ok || uID == "" {
					continue
				}
				userEmail, _ := u["user_email"].(string)
				userName := userEmail
				if meta, ok := u["user_metadata"].(map[string]interface{}); ok {
					if fn, ok := meta["full_name"].(string); ok && fn != "" {
						userName = fn
					} else if dn, ok := meta["display_name"].(string); ok && dn != "" {
						userName = dn
					}
				}
				userMap[uID] = map[string]string{
					"email": userEmail,
					"name":  userName,
				}
			}
			for _, trx := range trxs {
				if uID, ok := trx["user_id"].(string); ok {
					if info, found := userMap[uID]; found {
						trx["user_email"] = info["email"]
						trx["user_name"] = info["name"]
					}
				}
			}
		}
	}

	dbBytes, err := r.sbClient.CallRPC("get_all_app_transactions", map[string]interface{}{"p_limit": limit}, "")
	if err == nil && len(dbBytes) > 0 {
		var dbTrxs []map[string]interface{}
		if err := json.Unmarshal(dbBytes, &dbTrxs); err == nil && len(dbTrxs) > 0 {
			enrichWithUsers(dbTrxs)
			return dbTrxs, nil
		}
	}

	// PostgREST fallback: query directly from app_wallet_transactions
	filter := fmt.Sprintf("order=created_at.desc&limit=%d", limit)
	if respBytes, err := r.sbClient.GetDB("app_wallet_transactions", filter, ""); err == nil && len(respBytes) > 0 {
		var dbTrxs []map[string]interface{}
		if err := json.Unmarshal(respBytes, &dbTrxs); err == nil && len(dbTrxs) > 0 {
			enrichWithUsers(dbTrxs)
			return dbTrxs, nil
		}
	}

	// Cross-user RPC aggregation fallback:
	// Queries get_all_registered_users_wallets then gets each user's transactions via get_user_wallet_transactions RPC
	users, errUsers := r.FetchRegisteredUsersWallets("")
	if errUsers == nil && len(users) > 0 {
		var allTrx []map[string]interface{}
		for _, u := range users {
			uID, ok := u["user_id"].(string)
			if !ok || uID == "" {
				continue
			}
			userEmail, _ := u["user_email"].(string)
			userName := userEmail
			if meta, ok := u["user_metadata"].(map[string]interface{}); ok {
				if fn, ok := meta["full_name"].(string); ok && fn != "" {
					userName = fn
				} else if dn, ok := meta["display_name"].(string); ok && dn != "" {
					userName = dn
				}
			}

			userTrxBytes, errTrx := r.sbClient.CallRPC("get_user_wallet_transactions", map[string]interface{}{
				"p_user_id": uID,
				"p_limit":   limit,
			}, "")
			if errTrx == nil && len(userTrxBytes) > 0 {
				var uTrxs []map[string]interface{}
				if err := json.Unmarshal(userTrxBytes, &uTrxs); err == nil {
					for _, trx := range uTrxs {
						trx["user_email"] = userEmail
						trx["user_name"] = userName
						allTrx = append(allTrx, trx)
					}
				}
			}
		}

		sort.Slice(allTrx, func(i, j int) bool {
			ti, _ := allTrx[i]["created_at"].(string)
			tj, _ := allTrx[j]["created_at"].(string)
			return ti > tj
		})

		if len(allTrx) > limit {
			allTrx = allTrx[:limit]
		}
		if len(allTrx) > 0 {
			return allTrx, nil
		}
	}

	return []map[string]interface{}{}, nil
}

func (r *supabaseRepository) FetchAllPointTransactions(limit int) ([]map[string]interface{}, error) {
	if r.sbClient == nil {
		return nil, fmt.Errorf("supabase client not available")
	}

	enrichWithUsers := func(trxs []map[string]interface{}) {
		users, errUsers := r.FetchRegisteredUsersWallets("")
		if errUsers == nil && len(users) > 0 {
			userMap := make(map[string]map[string]string)
			for _, u := range users {
				uID, ok := u["user_id"].(string)
				if !ok || uID == "" {
					continue
				}
				userEmail, _ := u["user_email"].(string)
				userName := userEmail
				if meta, ok := u["user_metadata"].(map[string]interface{}); ok {
					if fn, ok := meta["full_name"].(string); ok && fn != "" {
						userName = fn
					} else if dn, ok := meta["display_name"].(string); ok && dn != "" {
						userName = dn
					}
				}
				userMap[uID] = map[string]string{
					"email": userEmail,
					"name":  userName,
				}
			}
			for _, trx := range trxs {
				if uID, ok := trx["user_id"].(string); ok {
					if info, found := userMap[uID]; found {
						trx["user_email"] = info["email"]
						trx["user_name"] = info["name"]
					}
				}
			}
		}
	}

	dbBytes, err := r.sbClient.CallRPC("get_all_point_transactions", map[string]interface{}{"p_limit": limit}, "")
	if err == nil && len(dbBytes) > 0 {
		var dbPts []map[string]interface{}
		if err := json.Unmarshal(dbBytes, &dbPts); err == nil && len(dbPts) > 0 {
			enrichWithUsers(dbPts)
			return dbPts, nil
		}
	}

	// PostgREST fallback: query directly from app_point_transactions
	filter := fmt.Sprintf("order=created_at.desc&limit=%d", limit)
	if respBytes, err := r.sbClient.GetDB("app_point_transactions", filter, ""); err == nil && len(respBytes) > 0 {
		var dbPts []map[string]interface{}
		if err := json.Unmarshal(respBytes, &dbPts); err == nil && len(dbPts) > 0 {
			enrichWithUsers(dbPts)
			return dbPts, nil
		}
	}

	return nil, err
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
