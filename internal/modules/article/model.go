package article

import "time"

// Article represents a news, blog post, or mitra education tutorial
type Article struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Slug         string    `json:"slug"`
	Category     string    `json:"category"` // 'TIPS_BISNIS', 'UPDATE_SISTEM', 'PANDUAN_KASIR', 'PROMO'
	Excerpt      string    `json:"excerpt"`
	Content      string    `json:"content"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Author       string    `json:"author"`
	ReadingTime  int       `json:"reading_time"` // in minutes
	IsPublished  bool      `json:"is_published"`
	ViewCount    int       `json:"view_count"`
	Tags         []string  `json:"tags,omitempty"`
	PublishedAt  time.Time `json:"published_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateArticleRequest payload for creating a new article
type CreateArticleRequest struct {
	Title        string   `json:"title"`
	Slug         string   `json:"slug,omitempty"`
	Category     string   `json:"category"`
	Excerpt      string   `json:"excerpt"`
	Content      string   `json:"content"`
	ThumbnailURL string   `json:"thumbnail_url,omitempty"`
	Author       string   `json:"author,omitempty"`
	ReadingTime  int      `json:"reading_time,omitempty"`
	IsPublished  *bool    `json:"is_published,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// UpdateArticleRequest payload for editing an existing article
type UpdateArticleRequest struct {
	Title        *string   `json:"title,omitempty"`
	Slug         *string   `json:"slug,omitempty"`
	Category     *string   `json:"category,omitempty"`
	Excerpt      *string   `json:"excerpt,omitempty"`
	Content      *string   `json:"content,omitempty"`
	ThumbnailURL *string   `json:"thumbnail_url,omitempty"`
	Author       *string   `json:"author,omitempty"`
	ReadingTime  *int      `json:"reading_time,omitempty"`
	IsPublished  *bool     `json:"is_published,omitempty"`
	Tags         *[]string `json:"tags,omitempty"`
}
