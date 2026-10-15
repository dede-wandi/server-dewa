package app_menu

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dede-wandi/server-dewa/internal/modules/app_category"
	"github.com/dede-wandi/server-dewa/internal/supabase"
)

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func ptrString(s string) *string {
	return &s
}

type Service interface {
	GetActiveMenus(placement string) ([]AppMenu, error)
	GetAllMenus(placement string) ([]AppMenu, error)
	GetMenuByID(id string) (*AppMenu, error)
	CreateMenu(req CreateMenuRequest) (*AppMenu, error)
	UpdateMenu(id string, req UpdateMenuRequest) (*AppMenu, error)
	DeleteMenu(id string) error
}

type menuService struct {
	sbClient    *supabase.Client
	catService  app_category.Service
	mu          sync.RWMutex
	fallback    []AppMenu
}

func NewService(sbClient *supabase.Client, catService app_category.Service) Service {
	now := time.Now()
	// Fallback seed data matching migration 17
	defaultMenus := []AppMenu{
		{
			ID:           "m1000000-0000-0000-0000-000000000001",
			Code:         "menu_pulsa",
			Name:         "Pulsa",
			Subtitle:     "Isi ulang pulsa murah",
			CategoryID:   ptrString("c1000000-0000-0000-0000-000000000001"),
			ActionType:   "CATEGORY",
			TargetURL:    "/service/pulsa",
			IconType:     "IONICONS",
			IconName:     "phone-portrait-outline",
			BgColor:      "#EFF6FF",
			IconColor:    "#2563EB",
			Badge:        "",
			BadgeColor:   "#EF4444",
			Placement:    "HOME_GRID",
			SortOrder:    1,
			IsActive:     true,
			IsComingSoon: false,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "m1000000-0000-0000-0000-000000000002",
			Code:         "menu_paket_data",
			Name:         "Paket Data",
			Subtitle:     "Kuota internet cepat",
			CategoryID:   ptrString("c1000000-0000-0000-0000-000000000001"),
			ActionType:   "CATEGORY",
			TargetURL:    "/service/data",
			IconType:     "IONICONS",
			IconName:     "globe-outline",
			BgColor:      "#F0FDF4",
			IconColor:    "#16A34A",
			Badge:        "PROMO",
			BadgeColor:   "#EF4444",
			Placement:    "HOME_GRID",
			SortOrder:    2,
			IsActive:     true,
			IsComingSoon: false,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "m1000000-0000-0000-0000-000000000003",
			Code:         "menu_pln_token",
			Name:         "Listrik PLN",
			Subtitle:     "Token & tagihan listrik",
			CategoryID:   ptrString("c1000000-0000-0000-0000-000000000002"),
			ActionType:   "CATEGORY",
			TargetURL:    "/service/pln",
			IconType:     "IONICONS",
			IconName:     "flash-outline",
			BgColor:      "#FEF3C7",
			IconColor:    "#D97706",
			Badge:        "",
			BadgeColor:   "#EF4444",
			Placement:    "HOME_GRID",
			SortOrder:    3,
			IsActive:     true,
			IsComingSoon: false,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "m1000000-0000-0000-0000-000000000004",
			Code:         "menu_emoney",
			Name:         "E-Money",
			Subtitle:     "DANA, GoPay, OVO, Shopee",
			CategoryID:   ptrString("c1000000-0000-0000-0000-000000000003"),
			ActionType:   "CATEGORY",
			TargetURL:    "/service/emoney",
			IconType:     "IONICONS",
			IconName:     "wallet-outline",
			BgColor:      "#F3E8FF",
			IconColor:    "#9333EA",
			Badge:        "INSTAN",
			BadgeColor:   "#16A34A",
			Placement:    "HOME_GRID",
			SortOrder:    4,
			IsActive:     true,
			IsComingSoon: false,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "m1000000-0000-0000-0000-000000000005",
			Code:         "menu_game_voucher",
			Name:         "Voucher Game",
			Subtitle:     "Diamond ML, FF, PUBG",
			CategoryID:   ptrString("c1000000-0000-0000-0000-000000000004"),
			ActionType:   "CATEGORY",
			TargetURL:    "/service/games",
			IconType:     "IONICONS",
			IconName:     "game-controller-outline",
			BgColor:      "#FFE4E6",
			IconColor:    "#E11D48",
			Badge:        "HOT",
			BadgeColor:   "#E11D48",
			Placement:    "HOME_GRID",
			SortOrder:    5,
			IsActive:     true,
			IsComingSoon: false,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "m1000000-0000-0000-0000-000000000006",
			Code:         "menu_bpjs",
			Name:         "BPJS Kesehatan",
			Subtitle:     "Bayar premi BPJS bulanan",
			CategoryID:   ptrString("c1000000-0000-0000-0000-000000000005"),
			ActionType:   "CATEGORY",
			TargetURL:    "/service/bpjs",
			IconType:     "IONICONS",
			IconName:     "shield-checkmark-outline",
			BgColor:      "#ECFDF5",
			IconColor:    "#059669",
			Badge:        "",
			BadgeColor:   "#EF4444",
			Placement:    "HOME_GRID",
			SortOrder:    6,
			IsActive:     true,
			IsComingSoon: false,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "m1000000-0000-0000-0000-000000000007",
			Code:         "menu_pdam",
			Name:         "Air PDAM",
			Subtitle:     "Tagihan PDAM seluruh daerah",
			CategoryID:   ptrString("c1000000-0000-0000-0000-000000000005"),
			ActionType:   "CATEGORY",
			TargetURL:    "/service/pdam",
			IconType:     "IONICONS",
			IconName:     "water-outline",
			BgColor:      "#E0F2FE",
			IconColor:    "#0284C7",
			Badge:        "",
			BadgeColor:   "#EF4444",
			Placement:    "HOME_GRID",
			SortOrder:    7,
			IsActive:     true,
			IsComingSoon: false,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "m1000000-0000-0000-0000-000000000008",
			Code:         "menu_semua",
			Name:         "Lainnya",
			Subtitle:     "Semua produk & tagihan",
			CategoryID:   nil,
			ActionType:   "ROUTE",
			TargetURL:    "/service/more",
			IconType:     "IONICONS",
			IconName:     "grid-outline",
			BgColor:      "#F1F5F9",
			IconColor:    "#475569",
			Badge:        "",
			BadgeColor:   "#EF4444",
			Placement:    "HOME_GRID",
			SortOrder:    8,
			IsActive:     true,
			IsComingSoon: false,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	return &menuService{
		sbClient:   sbClient,
		catService: catService,
		fallback:   defaultMenus,
	}
}

// populateCategory attaches the linked category object to the menu
func (s *menuService) populateCategory(menu *AppMenu) {
	if menu.CategoryID != nil && *menu.CategoryID != "" && s.catService != nil {
		cat, err := s.catService.GetCategoryByID(*menu.CategoryID)
		if err == nil && cat != nil {
			menu.Category = cat
		}
	}
}

func (s *menuService) filterFallback(activeOnly bool, placement string) []AppMenu {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []AppMenu
	for _, m := range s.fallback {
		if activeOnly && !m.IsActive {
			continue
		}
		if placement != "" && !strings.EqualFold(m.Placement, placement) {
			continue
		}
		item := m
		s.populateCategory(&item)
		result = append(result, item)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].SortOrder < result[j].SortOrder
	})

	return result
}

// GetActiveMenus returns active menus for mobile app (default placement HOME_GRID)
func (s *menuService) GetActiveMenus(placement string) ([]AppMenu, error) {
	if placement == "" {
		placement = "HOME_GRID"
	}

	queryParams := fmt.Sprintf("is_active=eq.true&placement=eq.%s&order=sort_order.asc,name.asc", url.QueryEscape(strings.ToUpper(placement)))
	respBytes, err := s.sbClient.GetDB("app_menus", queryParams, "")
	if err != nil {
		log.Printf("[MenuService] GetActiveMenus DB query failed: %v. Using in-memory fallback.", err)
		return s.filterFallback(true, placement), nil
	}

	var menus []AppMenu
	if err := json.Unmarshal(respBytes, &menus); err != nil || len(menus) == 0 {
		return s.filterFallback(true, placement), nil
	}

	for i := range menus {
		s.populateCategory(&menus[i])
	}

	return menus, nil
}

// GetAllMenus returns all menus for admin management
func (s *menuService) GetAllMenus(placement string) ([]AppMenu, error) {
	var parts []string
	if placement != "" {
		parts = append(parts, fmt.Sprintf("placement=eq.%s", url.QueryEscape(strings.ToUpper(placement))))
	}
	parts = append(parts, "order=sort_order.asc,name.asc")
	queryParams := strings.Join(parts, "&")

	respBytes, err := s.sbClient.GetDB("app_menus", queryParams, "")
	if err != nil {
		log.Printf("[MenuService] GetAllMenus DB query failed: %v. Using in-memory fallback.", err)
		return s.filterFallback(false, placement), nil
	}

	var menus []AppMenu
	if err := json.Unmarshal(respBytes, &menus); err != nil || len(menus) == 0 {
		return s.filterFallback(false, placement), nil
	}

	for i := range menus {
		s.populateCategory(&menus[i])
	}

	return menus, nil
}

// GetMenuByID returns a single menu by ID or code
func (s *menuService) GetMenuByID(id string) (*AppMenu, error) {
	queryParams := fmt.Sprintf("or=(id.eq.%s,code.eq.%s)&limit=1", url.QueryEscape(id), url.QueryEscape(id))
	respBytes, err := s.sbClient.GetDB("app_menus", queryParams, "")
	if err == nil {
		var list []AppMenu
		if err := json.Unmarshal(respBytes, &list); err == nil && len(list) > 0 {
			menu := list[0]
			s.populateCategory(&menu)
			return &menu, nil
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.fallback {
		if m.ID == id || m.Code == id {
			item := m
			s.populateCategory(&item)
			return &item, nil
		}
	}

	return nil, errors.New("menu tidak ditemukan")
}

// CreateMenu inserts a new menu
func (s *menuService) CreateMenu(req CreateMenuRequest) (*AppMenu, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("nama menu wajib diisi")
	}

	now := time.Now()
	newID := newUUID()

	code := req.Code
	if code == "" {
		code = "menu_" + strings.ToLower(strings.ReplaceAll(req.Name, " ", "_"))
	}

	actionType := req.ActionType
	if actionType == "" {
		actionType = "CATEGORY"
	}

	iconType := req.IconType
	if iconType == "" {
		iconType = "IONICONS"
	}

	iconName := req.IconName
	if iconName == "" {
		iconName = "grid-outline"
	}

	bgColor := req.BgColor
	if bgColor == "" {
		bgColor = "#EFF6FF"
	}

	iconColor := req.IconColor
	if iconColor == "" {
		iconColor = "#2563EB"
	}

	placement := req.Placement
	if placement == "" {
		placement = "HOME_GRID"
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	isComingSoon := false
	if req.IsComingSoon != nil {
		isComingSoon = *req.IsComingSoon
	}

	newMenu := AppMenu{
		ID:           newID,
		Code:         code,
		Name:         req.Name,
		Subtitle:     req.Subtitle,
		CategoryID:   req.CategoryID,
		ActionType:   actionType,
		TargetURL:    req.TargetURL,
		IconType:     iconType,
		IconName:     iconName,
		IconURL:      req.IconURL,
		BgColor:      bgColor,
		IconColor:    iconColor,
		Badge:        req.Badge,
		BadgeColor:   req.BadgeColor,
		Placement:    placement,
		SortOrder:    req.SortOrder,
		IsActive:     isActive,
		IsComingSoon: isComingSoon,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	payload := map[string]interface{}{
		"id":             newMenu.ID,
		"code":           newMenu.Code,
		"name":           newMenu.Name,
		"subtitle":       newMenu.Subtitle,
		"category_id":    newMenu.CategoryID,
		"action_type":    newMenu.ActionType,
		"target_url":     newMenu.TargetURL,
		"icon_type":      newMenu.IconType,
		"icon_name":      newMenu.IconName,
		"icon_url":       newMenu.IconURL,
		"bg_color":       newMenu.BgColor,
		"icon_color":     newMenu.IconColor,
		"badge":          newMenu.Badge,
		"badge_color":    newMenu.BadgeColor,
		"placement":      newMenu.Placement,
		"sort_order":     newMenu.SortOrder,
		"is_active":      newMenu.IsActive,
		"is_coming_soon": newMenu.IsComingSoon,
	}

	respBytes, err := s.sbClient.PostDB("app_menus", payload, "return=representation", "")
	if err == nil {
		var created []AppMenu
		if err := json.Unmarshal(respBytes, &created); err == nil && len(created) > 0 {
			newMenu = created[0]
		}
	} else {
		log.Printf("[MenuService] PostDB failed: %v. Storing in-memory fallback.", err)
	}

	s.populateCategory(&newMenu)

	s.mu.Lock()
	s.fallback = append(s.fallback, newMenu)
	s.mu.Unlock()

	return &newMenu, nil
}

// UpdateMenu modifies an existing menu
func (s *menuService) UpdateMenu(id string, req UpdateMenuRequest) (*AppMenu, error) {
	current, err := s.GetMenuByID(id)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"updated_at": time.Now(),
	}

	if req.Code != "" {
		current.Code = req.Code
		payload["code"] = req.Code
	}
	if req.Name != "" {
		current.Name = req.Name
		payload["name"] = req.Name
	}
	if req.Subtitle != "" {
		current.Subtitle = req.Subtitle
		payload["subtitle"] = req.Subtitle
	}
	if req.CategoryID != nil {
		current.CategoryID = req.CategoryID
		payload["category_id"] = req.CategoryID
	}
	if req.ActionType != "" {
		current.ActionType = req.ActionType
		payload["action_type"] = req.ActionType
	}
	if req.TargetURL != "" {
		current.TargetURL = req.TargetURL
		payload["target_url"] = req.TargetURL
	}
	if req.IconType != "" {
		current.IconType = req.IconType
		payload["icon_type"] = req.IconType
	}
	if req.IconName != "" {
		current.IconName = req.IconName
		payload["icon_name"] = req.IconName
	}
	if req.IconURL != "" {
		current.IconURL = req.IconURL
		payload["icon_url"] = req.IconURL
	}
	if req.BgColor != "" {
		current.BgColor = req.BgColor
		payload["bg_color"] = req.BgColor
	}
	if req.IconColor != "" {
		current.IconColor = req.IconColor
		payload["icon_color"] = req.IconColor
	}
	if req.Badge != "" {
		current.Badge = req.Badge
		payload["badge"] = req.Badge
	}
	if req.BadgeColor != "" {
		current.BadgeColor = req.BadgeColor
		payload["badge_color"] = req.BadgeColor
	}
	if req.Placement != "" {
		current.Placement = req.Placement
		payload["placement"] = req.Placement
	}
	if req.SortOrder != nil {
		current.SortOrder = *req.SortOrder
		payload["sort_order"] = *req.SortOrder
	}
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
		payload["is_active"] = *req.IsActive
	}
	if req.IsComingSoon != nil {
		current.IsComingSoon = *req.IsComingSoon
		payload["is_coming_soon"] = *req.IsComingSoon
	}

	queryParams := fmt.Sprintf("id=eq.%s", url.QueryEscape(id))
	respBytes, err := s.sbClient.PatchDB("app_menus", queryParams, payload, "")
	if err == nil {
		var updated []AppMenu
		if err := json.Unmarshal(respBytes, &updated); err == nil && len(updated) > 0 {
			current = &updated[0]
		}
	} else {
		log.Printf("[MenuService] PatchDB failed: %v. Updating in-memory fallback.", err)
	}

	s.populateCategory(current)

	s.mu.Lock()
	for i, m := range s.fallback {
		if m.ID == id {
			s.fallback[i] = *current
			break
		}
	}
	s.mu.Unlock()

	return current, nil
}

// DeleteMenu removes a menu
func (s *menuService) DeleteMenu(id string) error {
	queryParams := fmt.Sprintf("id=eq.%s", url.QueryEscape(id))
	err := s.sbClient.DeleteDB("app_menus", queryParams, "")
	if err != nil {
		log.Printf("[MenuService] DeleteDB failed: %v. Deleting from memory fallback.", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, m := range s.fallback {
		if m.ID == id {
			idx = i
			break
		}
	}
	if idx != -1 {
		s.fallback = append(s.fallback[:idx], s.fallback[idx+1:]...)
	}

	return nil
}
