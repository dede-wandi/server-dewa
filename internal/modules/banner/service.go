package banner

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/dede-wandi/server-dewa/internal/supabase"
)

// newUUID generates a standard RFC4122 UUID v4 using standard crypto/rand
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type Service interface {
	GetActiveBanners() ([]Banner, error)
	GetAllBanners() ([]Banner, error)
	GetBannerByID(id string) (*Banner, error)
	CreateBanner(req CreateBannerRequest) (*Banner, error)
	UpdateBanner(id string, req UpdateBannerRequest) (*Banner, error)
	DeleteBanner(id string) error
}

type bannerService struct {
	sbClient *supabase.Client
	mu       sync.RWMutex
	fallback []Banner
}

func NewService(sbClient *supabase.Client) Service {
	now := time.Now()
	defaultBanners := []Banner{
		{
			ID:          "promo-1",
			Title:       "Pulsa & Data Murah Banget!",
			Subtitle:    "Hemat hingga 40% semua operator",
			Tag:         "PROMO SPESIAL",
			CtaText:     "Beli Sekarang 🔥",
			TargetURL:   "/service/pulsa",
			ActionType:  "navigate",
			BgColor:     "#FFA000",
			AccentColor: "#FF8F00",
			Icon:        "phone-portrait",
			Emoji:       "📱",
			SortOrder:   1,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "promo-2",
			Title:       "Token PLN Bonus 5%!",
			Subtitle:    "Transaksi instan 24 jam, tanpa antre",
			Tag:         "TOKEN LISTRIK",
			CtaText:     "Beli Token ⚡",
			TargetURL:   "/service/pln",
			ActionType:  "navigate",
			BgColor:     "#1E3A5F",
			AccentColor: "#2563EB",
			Icon:        "flash",
			Emoji:       "⚡",
			SortOrder:   2,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "promo-3",
			Title:       "Sayur & Buah Antar ke Pintu!",
			Subtitle:    "Order sekarang, tiba dalam 2 jam",
			Tag:         "GROCERIES SEGAR",
			CtaText:     "Order Sekarang 🛒",
			TargetURL:   "/service/groceries",
			ActionType:  "navigate",
			BgColor:     "#065F46",
			AccentColor: "#10B981",
			Icon:        "leaf",
			Emoji:       "🥦",
			SortOrder:   3,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "promo-4",
			Title:       "Bayar Semua Tagihan di Sini!",
			Subtitle:    "PDAM, BPJS, Internet, TV Kabel & lebih",
			Tag:         "TAGIHAN PPOB",
			CtaText:     "Bayar Tagihan 💳",
			TargetURL:   "/service/bills",
			ActionType:  "navigate",
			BgColor:     "#4C1D95",
			AccentColor: "#7C3AED",
			Icon:        "receipt",
			Emoji:       "🧾",
			SortOrder:   4,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	return &bannerService{
		sbClient: sbClient,
		fallback: defaultBanners,
	}
}

// GetActiveBanners returns only active banners ordered by sort_order ASC
func (s *bannerService) GetActiveBanners() ([]Banner, error) {
	// PostgREST query: is_active=eq.true&order=sort_order.asc,created_at.desc
	queryParams := "is_active=eq.true&order=sort_order.asc,created_at.desc"
	respBytes, err := s.sbClient.GetDB("banners", queryParams, "")
	if err != nil {
		log.Printf("[BannerService] DB query failed or table not ready, using fallback banners: %v", err)
		s.mu.RLock()
		defer s.mu.RUnlock()
		active := make([]Banner, 0, len(s.fallback))
		for _, b := range s.fallback {
			if b.IsActive {
				active = append(active, b)
			}
		}
		return active, nil
	}

	var banners []Banner
	if err := json.Unmarshal(respBytes, &banners); err != nil {
		log.Printf("[BannerService] Failed to unmarshal banners: %v", err)
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.fallback, nil
	}

	// Filter by current date if start_date/end_date set
	now := time.Now()
	filtered := make([]Banner, 0, len(banners))
	for _, b := range banners {
		if !b.IsActive {
			continue
		}
		if b.StartDate != nil && now.Before(*b.StartDate) {
			continue
		}
		if b.EndDate != nil && now.After(*b.EndDate) {
			continue
		}
		filtered = append(filtered, b)
	}

	if len(filtered) == 0 {
		return s.fallback, nil
	}

	return filtered, nil
}

// GetAllBanners returns all banners for admin management
func (s *bannerService) GetAllBanners() ([]Banner, error) {
	queryParams := "order=sort_order.asc,created_at.desc"
	respBytes, err := s.sbClient.GetDB("banners", queryParams, "")
	if err != nil {
		log.Printf("[BannerService] Admin DB query failed: %v, returning in-memory banners", err)
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.fallback, nil
	}

	var banners []Banner
	if err := json.Unmarshal(respBytes, &banners); err != nil {
		return nil, fmt.Errorf("gagal parsing data banner: %w", err)
	}

	if len(banners) == 0 {
		return s.fallback, nil
	}

	return banners, nil
}

// GetBannerByID returns detail of a single banner
func (s *bannerService) GetBannerByID(id string) (*Banner, error) {
	queryParams := fmt.Sprintf("id=eq.%s", url.QueryEscape(id))
	respBytes, err := s.sbClient.GetDB("banners", queryParams, "")
	if err != nil {
		// Check fallback
		s.mu.RLock()
		defer s.mu.RUnlock()
		for _, b := range s.fallback {
			if b.ID == id {
				return &b, nil
			}
		}
		return nil, fmt.Errorf("banner tidak ditemukan: %w", err)
	}

	var list []Banner
	if err := json.Unmarshal(respBytes, &list); err != nil || len(list) == 0 {
		s.mu.RLock()
		defer s.mu.RUnlock()
		for _, b := range s.fallback {
			if b.ID == id {
				return &b, nil
			}
		}
		return nil, errors.New("banner tidak ditemukan")
	}

	return &list[0], nil
}

// CreateBanner inserts a new banner into Supabase
func (s *bannerService) CreateBanner(req CreateBannerRequest) (*Banner, error) {
	if req.Title == "" {
		return nil, errors.New("judul banner wajib diisi")
	}

	now := time.Now()
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	actionType := req.ActionType
	if actionType == "" {
		actionType = "navigate"
	}

	tag := req.Tag
	if tag == "" {
		tag = "PROMO"
	}

	ctaText := req.CtaText
	if ctaText == "" {
		ctaText = "Lihat Promo"
	}

	bgColor := req.BgColor
	if bgColor == "" {
		bgColor = "#FFA000"
	}

	accentColor := req.AccentColor
	if accentColor == "" {
		accentColor = "#FF8F00"
	}

	newID := newUUID()

	payload := map[string]interface{}{
		"id":           newID,
		"title":        req.Title,
		"subtitle":     req.Subtitle,
		"tag":          tag,
		"image_url":    req.ImageURL,
		"cta_text":     ctaText,
		"target_url":   req.TargetURL,
		"action_type":  actionType,
		"bg_color":     bgColor,
		"accent_color": accentColor,
		"icon":         req.Icon,
		"emoji":        req.Emoji,
		"sort_order":   req.SortOrder,
		"is_active":    isActive,
		"start_date":   req.StartDate,
		"end_date":     req.EndDate,
		"created_at":   now,
		"updated_at":   now,
	}

	respBytes, err := s.sbClient.PostDB("banners", payload, "return=representation", "")
	if err != nil {
		log.Printf("[BannerService] DB insert failed: %v. Simulating locally in fallback.", err)
		// Fallback local memory
		created := Banner{
			ID:          newID,
			Title:       req.Title,
			Subtitle:    req.Subtitle,
			Tag:         tag,
			ImageURL:    req.ImageURL,
			CtaText:     ctaText,
			TargetURL:   req.TargetURL,
			ActionType:  actionType,
			BgColor:     bgColor,
			AccentColor: accentColor,
			Icon:        req.Icon,
			Emoji:       req.Emoji,
			SortOrder:   req.SortOrder,
			IsActive:    isActive,
			StartDate:   req.StartDate,
			EndDate:     req.EndDate,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		s.mu.Lock()
		s.fallback = append(s.fallback, created)
		s.mu.Unlock()
		return &created, nil
	}

	var inserted []Banner
	if err := json.Unmarshal(respBytes, &inserted); err == nil && len(inserted) > 0 {
		return &inserted[0], nil
	}

	return s.GetBannerByID(newID)
}

// UpdateBanner updates an existing banner in Supabase
func (s *bannerService) UpdateBanner(id string, req UpdateBannerRequest) (*Banner, error) {
	payload := make(map[string]interface{})
	now := time.Now()
	payload["updated_at"] = now

	if req.Title != nil {
		payload["title"] = *req.Title
	}
	if req.Subtitle != nil {
		payload["subtitle"] = *req.Subtitle
	}
	if req.Tag != nil {
		payload["tag"] = *req.Tag
	}
	if req.ImageURL != nil {
		payload["image_url"] = *req.ImageURL
	}
	if req.CtaText != nil {
		payload["cta_text"] = *req.CtaText
	}
	if req.TargetURL != nil {
		payload["target_url"] = *req.TargetURL
	}
	if req.ActionType != nil {
		payload["action_type"] = *req.ActionType
	}
	if req.BgColor != nil {
		payload["bg_color"] = *req.BgColor
	}
	if req.AccentColor != nil {
		payload["accent_color"] = *req.AccentColor
	}
	if req.Icon != nil {
		payload["icon"] = *req.Icon
	}
	if req.Emoji != nil {
		payload["emoji"] = *req.Emoji
	}
	if req.SortOrder != nil {
		payload["sort_order"] = *req.SortOrder
	}
	if req.IsActive != nil {
		payload["is_active"] = *req.IsActive
	}
	if req.StartDate != nil {
		payload["start_date"] = req.StartDate
	}
	if req.EndDate != nil {
		payload["end_date"] = req.EndDate
	}

	queryParams := fmt.Sprintf("id=eq.%s", url.QueryEscape(id))
	respBytes, err := s.sbClient.PatchDB("banners", queryParams, payload, "")
	if err != nil {
		log.Printf("[BannerService] DB update failed: %v. Updating fallback locally.", err)
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, b := range s.fallback {
			if b.ID == id {
				if req.Title != nil {
					s.fallback[i].Title = *req.Title
				}
				if req.Subtitle != nil {
					s.fallback[i].Subtitle = *req.Subtitle
				}
				if req.Tag != nil {
					s.fallback[i].Tag = *req.Tag
				}
				if req.ImageURL != nil {
					s.fallback[i].ImageURL = *req.ImageURL
				}
				if req.CtaText != nil {
					s.fallback[i].CtaText = *req.CtaText
				}
				if req.TargetURL != nil {
					s.fallback[i].TargetURL = *req.TargetURL
				}
				if req.ActionType != nil {
					s.fallback[i].ActionType = *req.ActionType
				}
				if req.BgColor != nil {
					s.fallback[i].BgColor = *req.BgColor
				}
				if req.AccentColor != nil {
					s.fallback[i].AccentColor = *req.AccentColor
				}
				if req.Icon != nil {
					s.fallback[i].Icon = *req.Icon
				}
				if req.Emoji != nil {
					s.fallback[i].Emoji = *req.Emoji
				}
				if req.SortOrder != nil {
					s.fallback[i].SortOrder = *req.SortOrder
				}
				if req.IsActive != nil {
					s.fallback[i].IsActive = *req.IsActive
				}
				s.fallback[i].UpdatedAt = now
				return &s.fallback[i], nil
			}
		}
		return nil, fmt.Errorf("banner tidak ditemukan: %w", err)
	}

	var updated []Banner
	if err := json.Unmarshal(respBytes, &updated); err == nil && len(updated) > 0 {
		return &updated[0], nil
	}

	return s.GetBannerByID(id)
}

// DeleteBanner removes a banner from Supabase
func (s *bannerService) DeleteBanner(id string) error {
	queryParams := fmt.Sprintf("id=eq.%s", url.QueryEscape(id))
	err := s.sbClient.DeleteDB("banners", queryParams, "")
	if err != nil {
		log.Printf("[BannerService] DB delete failed: %v. Deleting from fallback locally.", err)
		s.mu.Lock()
		defer s.mu.Unlock()
		newFallback := make([]Banner, 0, len(s.fallback))
		found := false
		for _, b := range s.fallback {
			if b.ID == id {
				found = true
				continue
			}
			newFallback = append(newFallback, b)
		}
		s.fallback = newFallback
		if !found {
			return errors.New("banner tidak ditemukan")
		}
		return nil
	}

	// Also remove from local fallback if present
	s.mu.Lock()
	defer s.mu.Unlock()
	newFallback := make([]Banner, 0, len(s.fallback))
	for _, b := range s.fallback {
		if b.ID != id {
			newFallback = append(newFallback, b)
		}
	}
	s.fallback = newFallback

	return nil
}

func sortBanners(b []Banner) {
	sort.Slice(b, func(i, j int) bool {
		if b[i].SortOrder != b[j].SortOrder {
			return b[i].SortOrder < b[j].SortOrder
		}
		return b[i].CreatedAt.After(b[j].CreatedAt)
	})
}
