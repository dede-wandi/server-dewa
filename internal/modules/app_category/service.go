package app_category

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dede-wandi/server-dewa/internal/supabase"
)

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func slugify(text string) string {
	lower := strings.ToLower(text)
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug := reg.ReplaceAllString(lower, "-")
	return strings.Trim(slug, "-")
}

func ptrString(s string) *string {
	return &s
}

type Service interface {
	GetActiveCategories(parentID string, level int, asTree bool) ([]AppCategory, error)
	GetAllCategories(parentID string, level int, asTree bool) ([]AppCategory, error)
	GetCategoryByID(id string) (*AppCategory, error)
	GetCategoryByCodeOrSlug(identifier string) (*AppCategory, error)
	CreateCategory(req CreateCategoryRequest) (*AppCategory, error)
	UpdateCategory(id string, req UpdateCategoryRequest) (*AppCategory, error)
	DeleteCategory(id string) error
}

type categoryService struct {
	sbClient *supabase.Client
	mu       sync.RWMutex
	fallback []AppCategory
}

func NewService(sbClient *supabase.Client) Service {
	now := time.Now()
	// Initial seed fallback categories matching migration 17
	defaultCategories := []AppCategory{
		// Level 1: Kategori Utama
		{
			ID:          "c1000000-0000-0000-0000-000000000001",
			ParentID:    nil,
			Name:        "Pulsa & Paket Data",
			Code:        "cat_pulsa_data",
			Slug:        "pulsa-dan-paket-data",
			Description: "Layanan pulsa reguler, transfer pulsa, paket kuota internet, telepon & SMS semua operator",
			Icon:        "phone-portrait-outline",
			Badge:       "PROMO",
			Level:       1,
			SortOrder:   1,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c1000000-0000-0000-0000-000000000002",
			ParentID:    nil,
			Name:        "Listrik & PLN",
			Code:        "cat_pln",
			Slug:        "listrik-pln",
			Description: "Token listrik prabayar, tagihan pascabayar, dan pembayaran non-taglis PLN",
			Icon:        "flash-outline",
			Badge:       "HEMAT",
			Level:       1,
			SortOrder:   2,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c1000000-0000-0000-0000-000000000003",
			ParentID:    nil,
			Name:        "Uang Elektronik",
			Code:        "cat_emoney",
			Slug:        "uang-elektronik",
			Description: "Top up saldo e-wallet DANA, GoPay, OVO, ShopeePay, LinkAja",
			Icon:        "wallet-outline",
			Badge:       "INSTAN",
			Level:       1,
			SortOrder:   3,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c1000000-0000-0000-0000-000000000004",
			ParentID:    nil,
			Name:        "Voucher Game",
			Code:        "cat_games",
			Slug:        "voucher-game",
			Description: "Top up diamond & kredit game Mobile Legends, Free Fire, PUBG, Genshin Impact, Valorant",
			Icon:        "game-controller-outline",
			Badge:       "HOT",
			Level:       1,
			SortOrder:   4,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c1000000-0000-0000-0000-000000000005",
			ParentID:    nil,
			Name:        "Tagihan PPOB & Air",
			Code:        "cat_tagihan",
			Slug:        "tagihan-ppob",
			Description: "Pembayaran tagihan air PDAM, BPJS Kesehatan, Telkom Indihome, Multifinance",
			Icon:        "receipt-outline",
			Level:       1,
			SortOrder:   5,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c1000000-0000-0000-0000-000000000006",
			ParentID:    nil,
			Name:        "Voucher & Streaming",
			Code:        "cat_streaming",
			Slug:        "voucher-dan-streaming",
			Description: "Langganan streaming Netflix, Spotify, Vidio, Viu, YouTube Premium",
			Icon:        "tv-outline",
			Badge:       "NEW",
			Level:       1,
			SortOrder:   6,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// Level 2: Sub-Kategori di bawah Pulsa & Paket Data
		{
			ID:          "c2000000-0000-0000-0000-000000000001",
			ParentID:    ptrString("c1000000-0000-0000-0000-000000000001"),
			Name:        "Telkomsel",
			Code:        "sub_tsel",
			Slug:        "telkomsel",
			Description: "Produk pulsa, paket data, dan masa aktif Telkomsel By.U",
			Icon:        "phone-portrait",
			Badge:       "TERLARIS",
			Level:       2,
			SortOrder:   1,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c2000000-0000-0000-0000-000000000002",
			ParentID:    ptrString("c1000000-0000-0000-0000-000000000001"),
			Name:        "Indosat Ooredoo",
			Code:        "sub_isat",
			Slug:        "indosat-ooredoo",
			Description: "Produk pulsa IM3, data Freedom, dan nelpon Indosat",
			Icon:        "phone-portrait",
			Level:       2,
			SortOrder:   2,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c2000000-0000-0000-0000-000000000003",
			ParentID:    ptrString("c1000000-0000-0000-0000-000000000001"),
			Name:        "XL Axiata",
			Code:        "sub_xl",
			Slug:        "xl-axiata",
			Description: "Produk pulsa XL, paket Akrab, dan Xtra Combo",
			Icon:        "phone-portrait",
			Level:       2,
			SortOrder:   3,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c2000000-0000-0000-0000-000000000004",
			ParentID:    ptrString("c1000000-0000-0000-0000-000000000001"),
			Name:        "Tri (3)",
			Code:        "sub_tri",
			Slug:        "tri-indonesia",
			Description: "Produk pulsa Tri, kuota AlwaysOn, dan Happy",
			Icon:        "phone-portrait",
			Level:       2,
			SortOrder:   4,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c2000000-0000-0000-0000-000000000005",
			ParentID:    ptrString("c1000000-0000-0000-0000-000000000001"),
			Name:        "Smartfren",
			Code:        "sub_smart",
			Slug:        "smartfren",
			Description: "Produk pulsa Smartfren dan paket Unlimited Nonstop",
			Icon:        "phone-portrait",
			Level:       2,
			SortOrder:   5,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// Level 2: Sub-Kategori di bawah Listrik & PLN
		{
			ID:          "c2000000-0000-0000-0000-000000000010",
			ParentID:    ptrString("c1000000-0000-0000-0000-000000000002"),
			Name:        "Token Listrik Prabayar",
			Code:        "sub_pln_token",
			Slug:        "token-listrik-prabayar",
			Description: "Beli 20 digit nomor stroom token PLN instan 24 jam",
			Icon:        "flash",
			Badge:       "INSTAN",
			Level:       2,
			SortOrder:   1,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c2000000-0000-0000-0000-000000000011",
			ParentID:    ptrString("c1000000-0000-0000-0000-000000000002"),
			Name:        "Tagihan Listrik Pascabayar",
			Code:        "sub_pln_pasca",
			Slug:        "tagihan-listrik-pascabayar",
			Description: "Cek dan bayar tagihan pemakaian listrik bulanan",
			Icon:        "receipt",
			Level:       2,
			SortOrder:   2,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// Level 2: Sub-Kategori di bawah Uang Elektronik
		{
			ID:          "c2000000-0000-0000-0000-000000000020",
			ParentID:    ptrString("c1000000-0000-0000-0000-000000000003"),
			Name:        "DANA",
			Code:        "sub_dana",
			Slug:        "dana",
			Description: "Top up saldo akun DANA instan",
			Icon:        "wallet",
			Badge:       "HEMAT",
			Level:       2,
			SortOrder:   1,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c2000000-0000-0000-0000-000000000021",
			ParentID:    ptrString("c1000000-0000-0000-0000-000000000003"),
			Name:        "GoPay",
			Code:        "sub_gopay",
			Slug:        "gopay",
			Description: "Top up saldo GoPay Customer & Driver",
			Icon:        "wallet",
			Level:       2,
			SortOrder:   2,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// Level 2: Sub-Kategori di bawah Voucher Game
		{
			ID:          "c2000000-0000-0000-0000-000000000030",
			ParentID:    ptrString("c1000000-0000-0000-0000-000000000004"),
			Name:        "Mobile Legends",
			Code:        "sub_mlbb",
			Slug:        "mobile-legends",
			Description: "Top up Diamond Mobile Legends Bang Bang",
			Icon:        "game-controller",
			Badge:       "HOT",
			Level:       2,
			SortOrder:   1,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// Level 3: Sub-Sub-Kategori di bawah Sub-Kategori Telkomsel (Bercabang ke dalam)
		{
			ID:          "c3000000-0000-0000-0000-000000000001",
			ParentID:    ptrString("c2000000-0000-0000-0000-000000000001"),
			Name:        "Pulsa Reguler Telkomsel",
			Code:        "sub3_tsel_pulsa",
			Slug:        "pulsa-reguler-telkomsel",
			Description: "Pulsa reguler menambah masa aktif",
			Icon:        "phone-portrait",
			Level:       3,
			SortOrder:   1,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c3000000-0000-0000-0000-000000000002",
			ParentID:    ptrString("c2000000-0000-0000-0000-000000000001"),
			Name:        "Kuota Internet Telkomsel",
			Code:        "sub3_tsel_data",
			Slug:        "kuota-internet-telkomsel",
			Description: "Paket internet MAXstream, OMG, Combo Sakti",
			Icon:        "globe-outline",
			Badge:       "PROMO",
			Level:       3,
			SortOrder:   2,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c3000000-0000-0000-0000-000000000003",
			ParentID:    ptrString("c2000000-0000-0000-0000-000000000001"),
			Name:        "Paket Nelpon & SMS",
			Code:        "sub3_tsel_telp",
			Slug:        "paket-nelpon-sms-telkomsel",
			Description: "Paket bicara hemat ke sesama dan operator lain",
			Icon:        "call-outline",
			Level:       3,
			SortOrder:   3,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "c3000000-0000-0000-0000-000000000004",
			ParentID:    ptrString("c2000000-0000-0000-0000-000000000001"),
			Name:        "Perpanjang Masa Aktif",
			Code:        "sub3_tsel_masa_aktif",
			Slug:        "perpanjang-masa-aktif-telkomsel",
			Description: "Beli masa aktif tanpa perlu isi pulsa besar",
			Icon:        "calendar-outline",
			Level:       3,
			SortOrder:   4,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	return &categoryService{
		sbClient: sbClient,
		fallback: defaultCategories,
	}
}

// buildTree creates a hierarchical tree from a flat list of AppCategory
func buildTree(flat []AppCategory, rootParentID string) []AppCategory {
	childrenMap := make(map[string][]AppCategory)
	var roots []AppCategory

	// Sort items by SortOrder ASC
	sort.Slice(flat, func(i, j int) bool {
		if flat[i].SortOrder == flat[j].SortOrder {
			return flat[i].Name < flat[j].Name
		}
		return flat[i].SortOrder < flat[j].SortOrder
	})

	for _, item := range flat {
		pID := ""
		if item.ParentID != nil {
			pID = *item.ParentID
		}

		if (rootParentID == "" && pID == "") || (rootParentID != "" && pID == rootParentID) {
			roots = append(roots, item)
		} else if pID != "" {
			childrenMap[pID] = append(childrenMap[pID], item)
		}
	}

	var attachChildren func(cat *AppCategory)
	attachChildren = func(cat *AppCategory) {
		if children, exists := childrenMap[cat.ID]; exists {
			for i := range children {
				attachChildren(&children[i])
			}
			cat.Children = children
		} else {
			cat.Children = []AppCategory{}
		}
	}

	for i := range roots {
		attachChildren(&roots[i])
	}

	return roots
}

func (s *categoryService) filterFallback(activeOnly bool, parentID string, level int) []AppCategory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []AppCategory
	for _, c := range s.fallback {
		if activeOnly && !c.IsActive {
			continue
		}
		if parentID != "" {
			if c.ParentID == nil || *c.ParentID != parentID {
				continue
			}
		}
		if level > 0 && c.Level != level {
			continue
		}
		result = append(result, c)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].SortOrder == result[j].SortOrder {
			return result[i].Level < result[j].Level
		}
		return result[i].SortOrder < result[j].SortOrder
	})

	return result
}

// GetActiveCategories returns public categories for users/mobile apps
func (s *categoryService) GetActiveCategories(parentID string, level int, asTree bool) ([]AppCategory, error) {
	// If tree is requested, fetch all active items and organize hierarchy
	var parts []string
	parts = append(parts, "is_active=eq.true")
	if !asTree {
		if parentID != "" {
			if parentID == "null" || parentID == "root" {
				parts = append(parts, "parent_id=is.null")
			} else {
				parts = append(parts, fmt.Sprintf("parent_id=eq.%s", url.QueryEscape(parentID)))
			}
		}
		if level > 0 {
			parts = append(parts, fmt.Sprintf("level=eq.%d", level))
		}
	}
	parts = append(parts, "order=sort_order.asc,name.asc")
	queryParams := strings.Join(parts, "&")

	respBytes, err := s.sbClient.GetDB("app_categories", queryParams, "")
	if err != nil {
		log.Printf("[CategoryService] GetActiveCategories DB query failed: %v. Using fallback in-memory.", err)
		list := s.filterFallback(true, parentID, level)
		if asTree {
			return buildTree(list, parentID), nil
		}
		return list, nil
	}

	var categories []AppCategory
	if err := json.Unmarshal(respBytes, &categories); err != nil {
		list := s.filterFallback(true, parentID, level)
		if asTree {
			return buildTree(list, parentID), nil
		}
		return list, nil
	}

	if len(categories) == 0 {
		list := s.filterFallback(true, parentID, level)
		if asTree {
			return buildTree(list, parentID), nil
		}
		return list, nil
	}

	if asTree {
		return buildTree(categories, parentID), nil
	}

	return categories, nil
}

// GetAllCategories returns all categories for Admin (active & inactive)
func (s *categoryService) GetAllCategories(parentID string, level int, asTree bool) ([]AppCategory, error) {
	var parts []string
	if !asTree {
		if parentID != "" {
			if parentID == "null" || parentID == "root" {
				parts = append(parts, "parent_id=is.null")
			} else {
				parts = append(parts, fmt.Sprintf("parent_id=eq.%s", url.QueryEscape(parentID)))
			}
		}
		if level > 0 {
			parts = append(parts, fmt.Sprintf("level=eq.%d", level))
		}
	}
	parts = append(parts, "order=sort_order.asc,level.asc,name.asc")
	queryParams := strings.Join(parts, "&")

	respBytes, err := s.sbClient.GetDB("app_categories", queryParams, "")
	if err != nil {
		log.Printf("[CategoryService] GetAllCategories DB query failed: %v. Using fallback in-memory.", err)
		list := s.filterFallback(false, parentID, level)
		if asTree {
			return buildTree(list, parentID), nil
		}
		return list, nil
	}

	var categories []AppCategory
	if err := json.Unmarshal(respBytes, &categories); err != nil {
		list := s.filterFallback(false, parentID, level)
		if asTree {
			return buildTree(list, parentID), nil
		}
		return list, nil
	}

	if len(categories) == 0 {
		list := s.filterFallback(false, parentID, level)
		if asTree {
			return buildTree(list, parentID), nil
		}
		return list, nil
	}

	if asTree {
		return buildTree(categories, parentID), nil
	}

	return categories, nil
}

// GetCategoryByID returns category by ID (with children)
func (s *categoryService) GetCategoryByID(id string) (*AppCategory, error) {
	queryParams := fmt.Sprintf("id=eq.%s", url.QueryEscape(id))
	respBytes, err := s.sbClient.GetDB("app_categories", queryParams, "")
	if err == nil {
		var list []AppCategory
		if err := json.Unmarshal(respBytes, &list); err == nil && len(list) > 0 {
			cat := list[0]
			// Fetch immediate children
			childrenBytes, errCh := s.sbClient.GetDB("app_categories", fmt.Sprintf("parent_id=eq.%s&order=sort_order.asc", url.QueryEscape(id)), "")
			if errCh == nil {
				var children []AppCategory
				if err := json.Unmarshal(childrenBytes, &children); err == nil {
					cat.Children = children
				}
			}
			return &cat, nil
		}
	}

	// Fallback lookup
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.fallback {
		if c.ID == id {
			// Find children in fallback
			var children []AppCategory
			for _, sub := range s.fallback {
				if sub.ParentID != nil && *sub.ParentID == id {
					children = append(children, sub)
				}
			}
			c.Children = children
			return &c, nil
		}
	}

	return nil, errors.New("kategori tidak ditemukan")
}

// GetCategoryByCodeOrSlug finds a category by its unique code or slug
func (s *categoryService) GetCategoryByCodeOrSlug(identifier string) (*AppCategory, error) {
	queryParams := fmt.Sprintf("or=(code.eq.%s,slug.eq.%s,id.eq.%s)&limit=1",
		url.QueryEscape(identifier),
		url.QueryEscape(identifier),
		url.QueryEscape(identifier),
	)

	respBytes, err := s.sbClient.GetDB("app_categories", queryParams, "")
	if err == nil {
		var list []AppCategory
		if err := json.Unmarshal(respBytes, &list); err == nil && len(list) > 0 {
			cat := list[0]
			// Fetch immediate children
			childrenBytes, errCh := s.sbClient.GetDB("app_categories", fmt.Sprintf("parent_id=eq.%s&order=sort_order.asc", url.QueryEscape(cat.ID)), "")
			if errCh == nil {
				var children []AppCategory
				if err := json.Unmarshal(childrenBytes, &children); err == nil {
					cat.Children = children
				}
			}
			return &cat, nil
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.fallback {
		if c.Code == identifier || c.Slug == identifier || c.ID == identifier {
			var children []AppCategory
			for _, sub := range s.fallback {
				if sub.ParentID != nil && *sub.ParentID == c.ID {
					children = append(children, sub)
				}
			}
			c.Children = children
			return &c, nil
		}
	}

	return nil, errors.New("kategori tidak ditemukan")
}

// CreateCategory adds a new category
func (s *categoryService) CreateCategory(req CreateCategoryRequest) (*AppCategory, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("nama kategori wajib diisi")
	}

	now := time.Now()
	newID := newUUID()

	slug := req.Slug
	if slug == "" {
		slug = slugify(req.Name)
	}

	code := req.Code
	if code == "" {
		code = "cat_" + strings.ReplaceAll(slug, "-", "_")
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	level := req.Level
	if level <= 0 {
		if req.ParentID == nil || *req.ParentID == "" {
			level = 1
		} else {
			// Find parent level
			parent, err := s.GetCategoryByID(*req.ParentID)
			if err == nil && parent != nil {
				level = parent.Level + 1
			} else {
				level = 2
			}
		}
	}

	newCat := AppCategory{
		ID:          newID,
		ParentID:    req.ParentID,
		Name:        req.Name,
		Code:        code,
		Slug:        slug,
		Description: req.Description,
		Icon:        req.Icon,
		IconURL:     req.IconURL,
		BannerURL:   req.BannerURL,
		Badge:       req.Badge,
		Level:       level,
		SortOrder:   req.SortOrder,
		IsActive:    isActive,
		Meta:        req.Meta,
		Children:    []AppCategory{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	payload := map[string]interface{}{
		"id":          newCat.ID,
		"parent_id":   newCat.ParentID,
		"name":        newCat.Name,
		"code":        newCat.Code,
		"slug":        newCat.Slug,
		"description": newCat.Description,
		"icon":        newCat.Icon,
		"icon_url":    newCat.IconURL,
		"banner_url":  newCat.BannerURL,
		"badge":       newCat.Badge,
		"level":       newCat.Level,
		"sort_order":  newCat.SortOrder,
		"is_active":   newCat.IsActive,
		"meta":        newCat.Meta,
	}

	respBytes, err := s.sbClient.PostDB("app_categories", payload, "return=representation", "")
	if err == nil {
		var created []AppCategory
		if err := json.Unmarshal(respBytes, &created); err == nil && len(created) > 0 {
			newCat = created[0]
		}
	} else {
		log.Printf("[CategoryService] PostDB failed: %v. Storing in memory fallback.", err)
	}

	s.mu.Lock()
	s.fallback = append(s.fallback, newCat)
	s.mu.Unlock()

	return &newCat, nil
}

// UpdateCategory edits an existing category
func (s *categoryService) UpdateCategory(id string, req UpdateCategoryRequest) (*AppCategory, error) {
	current, err := s.GetCategoryByID(id)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"updated_at": time.Now(),
	}

	if req.ParentID != nil {
		current.ParentID = req.ParentID
		payload["parent_id"] = req.ParentID
	}
	if req.Name != "" {
		current.Name = req.Name
		payload["name"] = req.Name
	}
	if req.Code != "" {
		current.Code = req.Code
		payload["code"] = req.Code
	}
	if req.Slug != "" {
		current.Slug = req.Slug
		payload["slug"] = req.Slug
	}
	if req.Description != "" {
		current.Description = req.Description
		payload["description"] = req.Description
	}
	if req.Icon != "" {
		current.Icon = req.Icon
		payload["icon"] = req.Icon
	}
	if req.IconURL != "" {
		current.IconURL = req.IconURL
		payload["icon_url"] = req.IconURL
	}
	if req.BannerURL != "" {
		current.BannerURL = req.BannerURL
		payload["banner_url"] = req.BannerURL
	}
	if req.Badge != "" {
		current.Badge = req.Badge
		payload["badge"] = req.Badge
	}
	if req.Level != nil {
		current.Level = *req.Level
		payload["level"] = *req.Level
	}
	if req.SortOrder != nil {
		current.SortOrder = *req.SortOrder
		payload["sort_order"] = *req.SortOrder
	}
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
		payload["is_active"] = *req.IsActive
	}
	if req.Meta != nil {
		current.Meta = req.Meta
		payload["meta"] = req.Meta
	}

	queryParams := fmt.Sprintf("id=eq.%s", url.QueryEscape(id))
	respBytes, err := s.sbClient.PatchDB("app_categories", queryParams, payload, "")
	if err == nil {
		var updated []AppCategory
		if err := json.Unmarshal(respBytes, &updated); err == nil && len(updated) > 0 {
			current = &updated[0]
		}
	} else {
		log.Printf("[CategoryService] PatchDB failed: %v. Updating in memory fallback.", err)
	}

	s.mu.Lock()
	for i, c := range s.fallback {
		if c.ID == id {
			s.fallback[i] = *current
			break
		}
	}
	s.mu.Unlock()

	return current, nil
}

// DeleteCategory removes a category
func (s *categoryService) DeleteCategory(id string) error {
	queryParams := fmt.Sprintf("id=eq.%s", url.QueryEscape(id))
	err := s.sbClient.DeleteDB("app_categories", queryParams, "")
	if err != nil {
		log.Printf("[CategoryService] DeleteDB failed: %v. Deleting from memory fallback.", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, c := range s.fallback {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx != -1 {
		s.fallback = append(s.fallback[:idx], s.fallback[idx+1:]...)
	}

	return nil
}
