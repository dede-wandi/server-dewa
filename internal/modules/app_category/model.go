package app_category

import (
	"time"

	"github.com/dede-wandi/server-dewa/internal/modules/product"
)

// AppCategory model kategori bertingkat (Tree Hierarchy)
type AppCategory struct {
	ID          string                 `json:"id"`
	ParentID    *string                `json:"parent_id,omitempty"`
	Name        string                 `json:"name"`
	Code        string                 `json:"code"`
	Slug        string                 `json:"slug"`
	Description string                 `json:"description"`
	Icon        string                 `json:"icon,omitempty"`
	IconURL     string                 `json:"icon_url,omitempty"`
	BannerURL   string                 `json:"banner_url,omitempty"`
	Badge       string                 `json:"badge,omitempty"`
	Type        string                 `json:"type"` // pulsa, data, voucher, tlp, sms, pln, emoney, game, pdam, etc.
	Level       int                    `json:"level"`
	SortOrder   int                    `json:"sort_order"`
	IsActive    bool                   `json:"is_active"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
	Children    []AppCategory          `json:"children,omitempty"`
	Products    []*product.Product     `json:"products,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type CreateCategoryRequest struct {
	ParentID    *string                `json:"parent_id,omitempty"`
	Name        string                 `json:"name"`
	Code        string                 `json:"code"`
	Slug        string                 `json:"slug,omitempty"`
	Description string                 `json:"description,omitempty"`
	Icon        string                 `json:"icon,omitempty"`
	IconURL     string                 `json:"icon_url,omitempty"`
	BannerURL   string                 `json:"banner_url,omitempty"`
	Badge       string                 `json:"badge,omitempty"`
	Type        string                 `json:"type,omitempty"` // pulsa, data, voucher, tlp, sms, etc.
	Level       int                    `json:"level,omitempty"`
	SortOrder   int                    `json:"sort_order,omitempty"`
	IsActive    *bool                  `json:"is_active,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

type UpdateCategoryRequest struct {
	ParentID    *string                `json:"parent_id,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Code        string                 `json:"code,omitempty"`
	Slug        string                 `json:"slug,omitempty"`
	Description string                 `json:"description,omitempty"`
	Icon        string                 `json:"icon,omitempty"`
	IconURL     string                 `json:"icon_url,omitempty"`
	BannerURL   string                 `json:"banner_url,omitempty"`
	Badge       string                 `json:"badge,omitempty"`
	Type        *string                `json:"type,omitempty"`
	Level       *int                   `json:"level,omitempty"`
	SortOrder   *int                   `json:"sort_order,omitempty"`
	IsActive    *bool                  `json:"is_active,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}
