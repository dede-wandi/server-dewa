package app_menu

import (
	"time"

	"github.com/dede-wandi/server-dewa/internal/modules/app_category"
	"github.com/dede-wandi/server-dewa/internal/modules/product"
)

// AppMenu model menu layanan aplikasi (Home Screen, Quick Action, Sub-Menus)
type AppMenu struct {
	ID           string                    `json:"id"`
	ParentID     *string                   `json:"parent_id,omitempty"`
	Level        int                       `json:"level"`
	Code         string                    `json:"code"`
	Name         string                    `json:"name"`
	Subtitle     string                    `json:"subtitle,omitempty"`
	CategoryID   *string                   `json:"category_id,omitempty"`
	Category     *app_category.AppCategory `json:"category,omitempty"`
	Products     []*product.Product        `json:"products,omitempty"`
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
	Children     []AppMenu                 `json:"children,omitempty"`
	CreatedAt    time.Time                 `json:"created_at"`
	UpdatedAt    time.Time                 `json:"updated_at"`
}

type CreateMenuRequest struct {
	ParentID     *string `json:"parent_id,omitempty"`
	Level        *int    `json:"level,omitempty"`
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
	ParentID     *string `json:"parent_id,omitempty"`
	Level        *int    `json:"level,omitempty"`
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
