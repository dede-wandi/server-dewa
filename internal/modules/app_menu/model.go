package app_menu

import (
	"time"

	"github.com/dede-wandi/server-dewa/internal/modules/app_category"
)

// AppMenu model menu layanan aplikasi (Home Screen & Quick Action)
type AppMenu struct {
	ID           string                    `json:"id"`
	Code         string                    `json:"code"`
	Name         string                    `json:"name"`
	Subtitle     string                    `json:"subtitle,omitempty"`
	CategoryID   *string                   `json:"category_id,omitempty"`
	Category     *app_category.AppCategory `json:"category,omitempty"`
	ActionType   string                    `json:"action_type"` // CATEGORY, ROUTE, WEBVIEW, DIALOG
	TargetURL    string                    `json:"target_url"`
	IconType     string                    `json:"icon_type"` // IONICONS, URL, SVG, EMOJI
	IconName     string                    `json:"icon_name"`
	IconURL      string                    `json:"icon_url,omitempty"`
	BgColor      string                    `json:"bg_color"`
	IconColor    string                    `json:"icon_color"`
	Badge        string                    `json:"badge,omitempty"`
	BadgeColor   string                    `json:"badge_color,omitempty"`
	Placement    string                    `json:"placement"` // HOME_GRID, QUICK_ACTION, DRAWER, FOOTER
	SortOrder    int                       `json:"sort_order"`
	IsActive     bool                      `json:"is_active"`
	IsComingSoon bool                      `json:"is_coming_soon"`
	CreatedAt    time.Time                 `json:"created_at"`
	UpdatedAt    time.Time                 `json:"updated_at"`
}

type CreateMenuRequest struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Subtitle     string  `json:"subtitle,omitempty"`
	CategoryID   *string `json:"category_id,omitempty"`
	ActionType   string  `json:"action_type,omitempty"`
	TargetURL    string  `json:"target_url,omitempty"`
	IconType     string  `json:"icon_type,omitempty"`
	IconName     string  `json:"icon_name,omitempty"`
	IconURL      string  `json:"icon_url,omitempty"`
	BgColor      string  `json:"bg_color,omitempty"`
	IconColor    string  `json:"icon_color,omitempty"`
	Badge        string  `json:"badge,omitempty"`
	BadgeColor   string  `json:"badge_color,omitempty"`
	Placement    string  `json:"placement,omitempty"`
	SortOrder    int     `json:"sort_order,omitempty"`
	IsActive     *bool   `json:"is_active,omitempty"`
	IsComingSoon *bool   `json:"is_coming_soon,omitempty"`
}

type UpdateMenuRequest struct {
	Code         string  `json:"code,omitempty"`
	Name         string  `json:"name,omitempty"`
	Subtitle     string  `json:"subtitle,omitempty"`
	CategoryID   *string `json:"category_id,omitempty"`
	ActionType   string  `json:"action_type,omitempty"`
	TargetURL    string  `json:"target_url,omitempty"`
	IconType     string  `json:"icon_type,omitempty"`
	IconName     string  `json:"icon_name,omitempty"`
	IconURL      string  `json:"icon_url,omitempty"`
	BgColor      string  `json:"bg_color,omitempty"`
	IconColor    string  `json:"icon_color,omitempty"`
	Badge        string  `json:"badge,omitempty"`
	BadgeColor   string  `json:"badge_color,omitempty"`
	Placement    string  `json:"placement,omitempty"`
	SortOrder    *int    `json:"sort_order,omitempty"`
	IsActive     *bool   `json:"is_active,omitempty"`
	IsComingSoon *bool   `json:"is_coming_soon,omitempty"`
}
