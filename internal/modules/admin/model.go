package admin

// UserListItem represents a user in the superadmin user list with fixed JSON key order
type UserListItem struct {
	UserID    string  `json:"user_id"`
	UserEmail string  `json:"user_email"`
	Role      string  `json:"role"`
	RoleName  string  `json:"role_name"`
	Category  string  `json:"category"`
	Balance   float64 `json:"balance"`
	Points    int64   `json:"points"`
	IsLocked  bool    `json:"is_locked"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// UserDetailResponse represents a single user's detail with fixed JSON key order and full profile metadata
type UserDetailResponse struct {
	UserID          string                 `json:"user_id"`
	UserEmail       string                 `json:"user_email"`
	Role            string                 `json:"role"`
	RoleName        string                 `json:"role_name"`
	Category        string                 `json:"category"`
	FullName        string                 `json:"full_name,omitempty"`
	Phone           string                 `json:"phone,omitempty"`
	Address         string                 `json:"address,omitempty"`
	BusinessName    string                 `json:"business_name,omitempty"`
	BusinessPhone   string                 `json:"business_phone,omitempty"`
	BusinessAddress string                 `json:"business_address,omitempty"`
	WATarget1       string                 `json:"wa_target_1,omitempty"`
	WATarget2       string                 `json:"wa_target_2,omitempty"`
	WATarget3       string                 `json:"wa_target_3,omitempty"`
	Balance         float64                `json:"balance"`
	Points          int64                  `json:"points"`
	IsLocked        bool                   `json:"is_locked"`
	CreatedAt       string                 `json:"created_at"`
	UpdatedAt       string                 `json:"updated_at"`
	UserMetadata    map[string]interface{} `json:"user_metadata,omitempty"`
	AppMetadata     map[string]interface{} `json:"app_metadata,omitempty"`
	Transactions    interface{}            `json:"transactions"`
}

// ResetPasswordRequest represents payload to reset user password
type ResetPasswordRequest struct {
	Password    string `json:"password"`
	NewPassword string `json:"new_password"`
}

// UpdateEmailRequest represents payload to update user email
type UpdateEmailRequest struct {
	Email string `json:"email"`
}

// UpdateUserRequest represents payload to comprehensively edit user details and profile metadata
type UpdateUserRequest struct {
	Email           string                 `json:"email,omitempty"`
	Password        string                 `json:"password,omitempty"`
	Role            string                 `json:"role,omitempty"`
	DisplayName     string                 `json:"display_name,omitempty"`
	FullName        string                 `json:"full_name,omitempty"`
	Phone           string                 `json:"phone,omitempty"`
	Address         string                 `json:"address,omitempty"`
	BusinessName    string                 `json:"business_name,omitempty"`
	BusinessPhone   string                 `json:"business_phone,omitempty"`
	BusinessAddress string                 `json:"business_address,omitempty"`
	WATarget1       string                 `json:"wa_target_1,omitempty"`
	WATarget2       string                 `json:"wa_target_2,omitempty"`
	WATarget3       string                 `json:"wa_target_3,omitempty"`
	IsLocked        *bool                  `json:"is_locked,omitempty"`
	UserMetadata    map[string]interface{} `json:"user_metadata,omitempty"`
}

// AdminTopUpRequest represents payload for Super Admin to top up user balance
type AdminTopUpRequest struct {
	TargetUserID string  `json:"target_user_id"`
	Amount       float64 `json:"amount"`
	Channel      string  `json:"channel,omitempty"`
	Title        string  `json:"title,omitempty"`
	Description  string  `json:"description,omitempty"`
	Notes        string  `json:"notes,omitempty"`
}

// RoleAssignRequest represents payload to assign/remove a role
type RoleAssignRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// AdjustPointsRequest represents payload to adjust user loyalty points
type AdjustPointsRequest struct {
	TargetUserID string `json:"target_user_id"`
	Type         string `json:"type"`   // CREDIT or DEBIT
	Amount       int64  `json:"amount"` // Jumlah poin
	Reason       string `json:"reason"` // Alasan
}
