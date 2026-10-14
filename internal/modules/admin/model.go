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

// UserDetailResponse represents a single user's detail with fixed JSON key order
type UserDetailResponse struct {
	UserID       string      `json:"user_id"`
	UserEmail    string      `json:"user_email"`
	Balance      float64     `json:"balance"`
	Points       int64       `json:"points"`
	IsLocked     bool        `json:"is_locked"`
	UpdatedAt    string      `json:"updated_at"`
	Transactions interface{} `json:"transactions"`
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

// UpdateUserRequest represents payload to comprehensively edit user details
type UpdateUserRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
	IsLocked    *bool  `json:"is_locked"`
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
