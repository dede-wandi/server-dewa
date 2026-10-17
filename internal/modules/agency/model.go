package agency

import (
	"time"

	"github.com/dede-wandi/server-dewa/internal/modules/wallet"
)

// AgencyProfile represents an Agent / Upline profile and referral settings
type AgencyProfile struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	ReferralCode string    `json:"referral_code"`
	BusinessName string    `json:"business_name"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AgencyMember represents a downline user registered under an Agent
type AgencyMember struct {
	ID                string    `json:"id"`
	AgentID           string    `json:"agent_id"`
	MemberID          string    `json:"member_id"`
	MemberName        string    `json:"member_name"`
	MemberPhone       string    `json:"member_phone"`
	Status            string    `json:"status"` // ACTIVE, INACTIVE, SUSPENDED
	JoinedAt          time.Time `json:"joined_at"`
	TotalTransactions int       `json:"total_transactions,omitempty"`
	TotalCommission   float64   `json:"total_commission,omitempty"`
}

// AgentCategoryMarkup defines a price markup for a specific product category
// If MemberID is nil or empty, this markup is the agent's default markup template
type AgentCategoryMarkup struct {
	ID           string    `json:"id"`
	AgentID      string    `json:"agent_id"`
	MemberID     *string   `json:"member_id,omitempty"`
	CategoryID   string    `json:"category_id"`
	CategoryName string    `json:"category_name,omitempty"`
	MarkupAmount float64   `json:"markup_amount"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CategoryMarkupInput holds input data when setting a category markup
type CategoryMarkupInput struct {
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name,omitempty"`
	MarkupAmount float64 `json:"markup_amount"`
}

// RegisterMemberRequest payload for an agent directly registering a new member
type RegisterMemberRequest struct {
	Name         string                `json:"name"`
	Phone        string                `json:"phone"`
	Email        string                `json:"email,omitempty"`
	Password     string                `json:"password"`
	BusinessName string                `json:"business_name,omitempty"`
	Markups      []CategoryMarkupInput `json:"markups,omitempty"`
}

// CopyMarkupsRequest payload to clone markups from one member to others
type CopyMarkupsRequest struct {
	FromMemberID string   `json:"from_member_id"`
	ToMemberIDs  []string `json:"to_member_ids"`
}

// UpdateProfileRequest payload to update agent profile
type UpdateProfileRequest struct {
	BusinessName string `json:"business_name"`
}

// UpdateMarkupsRequest payload to update markups list
type UpdateMarkupsRequest struct {
	Markups []CategoryMarkupInput `json:"markups"`
}

// AgencyDashboardResponse returns summary analytics and lists for the agency dashboard
type AgencyDashboardResponse struct {
	Profile           *AgencyProfile              `json:"profile"`
	TotalMembers      int                         `json:"total_members"`
	TotalCommission   float64                     `json:"total_commission"`
	TodayCommission   float64                     `json:"today_commission"`
	TotalOrders       int                         `json:"total_orders"`
	RecentMembers     []*AgencyMember             `json:"recent_members"`
	RecentCommissions []*wallet.WalletTransaction `json:"recent_commissions"`
}
