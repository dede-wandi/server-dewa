package banner

import "time"

// Banner represents a promotional banner or slider shown on home and catalog pages
type Banner struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Subtitle    string     `json:"subtitle"`
	Tag         string     `json:"tag"`
	ImageURL    string     `json:"image_url,omitempty"`
	CtaText     string     `json:"cta_text"`
	TargetURL   string     `json:"target_url"`
	ActionType  string     `json:"action_type"` // 'navigate', 'link', 'none'
	BgColor     string     `json:"bg_color"`
	AccentColor string     `json:"accent_color"`
	Icon        string     `json:"icon"`
	Emoji       string     `json:"emoji"`
	SortOrder   int        `json:"sort_order"`
	IsActive    bool       `json:"is_active"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CreateBannerRequest represents the payload for creating a new banner
type CreateBannerRequest struct {
	Title       string     `json:"title"`
	Subtitle    string     `json:"subtitle"`
	Tag         string     `json:"tag"`
	ImageURL    string     `json:"image_url,omitempty"`
	CtaText     string     `json:"cta_text"`
	TargetURL   string     `json:"target_url"`
	ActionType  string     `json:"action_type"`
	BgColor     string     `json:"bg_color"`
	AccentColor string     `json:"accent_color"`
	Icon        string     `json:"icon"`
	Emoji       string     `json:"emoji"`
	SortOrder   int        `json:"sort_order"`
	IsActive    *bool      `json:"is_active,omitempty"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
}

// UpdateBannerRequest represents the payload for updating an existing banner
type UpdateBannerRequest struct {
	Title       *string    `json:"title,omitempty"`
	Subtitle    *string    `json:"subtitle,omitempty"`
	Tag         *string    `json:"tag,omitempty"`
	ImageURL    *string    `json:"image_url,omitempty"`
	CtaText     *string    `json:"cta_text,omitempty"`
	TargetURL   *string    `json:"target_url,omitempty"`
	ActionType  *string    `json:"action_type,omitempty"`
	BgColor     *string    `json:"bg_color,omitempty"`
	AccentColor *string    `json:"accent_color,omitempty"`
	Icon        *string    `json:"icon,omitempty"`
	Emoji       *string    `json:"emoji,omitempty"`
	SortOrder   *int       `json:"sort_order,omitempty"`
	IsActive    *bool      `json:"is_active,omitempty"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
}
