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

type Service interface {
	GetActiveMenus(placement string, parentID string, level int, asTree bool) ([]AppMenu, error)
	GetAllMenus(placement string, parentID string, level int, asTree bool) ([]AppMenu, error)
	GetMenuByID(id string) (*AppMenu, error)
	GetMenuByCode(code string) (*AppMenu, error)
	CreateMenu(req CreateMenuRequest) (*AppMenu, error)
	UpdateMenu(id string, req UpdateMenuRequest) (*AppMenu, error)
	DeleteMenu(id string) error
}

type menuService struct {
	sbClient   *supabase.Client
	catService app_category.Service
	mu         sync.RWMutex
	fallback   []AppMenu
}

func NewService(sbClient *supabase.Client, catService app_category.Service) Service {
	return &menuService{
		sbClient:   sbClient,
		catService: catService,
		fallback:   []AppMenu{},
	}
}

// buildMenuTree creates a hierarchical tree from a flat list of AppMenu
func buildMenuTree(flat []AppMenu, rootParentID string) []AppMenu {
	childrenMap := make(map[string][]AppMenu)
	var roots []AppMenu

	// Sort items by SortOrder ASC, then Name ASC
	sort.Slice(flat, func(i, j int) bool {
		if flat[i].SortOrder == flat[j].SortOrder {
			return flat[i].Name < flat[j].Name
		}
		return flat[i].SortOrder < flat[j].SortOrder
	})

	for _, item := range flat {
		pID := ""
		if item.ParentID != nil {
			pID = strings.TrimSpace(*item.ParentID)
		}

		if (rootParentID == "" && pID == "") || (rootParentID != "" && pID == rootParentID) {
			roots = append(roots, item)
		} else if pID != "" {
			childrenMap[pID] = append(childrenMap[pID], item)
		}
	}

	var attachChildren func(m *AppMenu)
	attachChildren = func(m *AppMenu) {
		if children, exists := childrenMap[m.ID]; exists {
			for i := range children {
				attachChildren(&children[i])
			}
			m.Children = children
		} else {
			m.Children = []AppMenu{}
		}
	}

	for i := range roots {
		attachChildren(&roots[i])
	}

	if roots == nil {
		roots = []AppMenu{}
	}
	return roots
}

func (s *menuService) populateCategory(menu *AppMenu) {
	if menu == nil || menu.CategoryID == nil || *menu.CategoryID == "" || s.catService == nil {
		return
	}
	cat, err := s.catService.GetCategoryByID(*menu.CategoryID)
	if err == nil && cat != nil {
		menu.Category = cat
	}
}

// GetActiveMenus returns active menus for mobile app (supports tree, placement, and parent filtering)
func (s *menuService) GetActiveMenus(placement string, parentID string, level int, asTree bool) ([]AppMenu, error) {
	var parts []string
	parts = append(parts, "is_active=eq.true")

	if placement != "" && strings.ToUpper(placement) != "ALL" {
		parts = append(parts, fmt.Sprintf("placement=eq.%s", url.QueryEscape(strings.ToUpper(placement))))
	}

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

	respBytes, err := s.sbClient.GetDB("app_menus", queryParams, "")
	if err != nil {
		log.Printf("[MenuService] GetActiveMenus DB query failed: %v", err)
		return []AppMenu{}, nil
	}

	var menus []AppMenu
	if err := json.Unmarshal(respBytes, &menus); err != nil {
		return []AppMenu{}, nil
	}

	for i := range menus {
		s.populateCategory(&menus[i])
	}

	if asTree {
		rootParentID := ""
		if parentID != "" && parentID != "null" && parentID != "root" {
			rootParentID = parentID
		}
		return buildMenuTree(menus, rootParentID), nil
	}

	if menus == nil {
		menus = []AppMenu{}
	}
	return menus, nil
}

// GetAllMenus returns all menus for admin management (supports tree, placement, parent, and level filtering)
func (s *menuService) GetAllMenus(placement string, parentID string, level int, asTree bool) ([]AppMenu, error) {
	var parts []string
	if placement != "" && strings.ToUpper(placement) != "ALL" {
		parts = append(parts, fmt.Sprintf("placement=eq.%s", url.QueryEscape(strings.ToUpper(placement))))
	}

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

	respBytes, err := s.sbClient.GetDB("app_menus", queryParams, "")
	if err != nil {
		log.Printf("[MenuService] GetAllMenus DB query failed: %v", err)
		return []AppMenu{}, nil
	}

	var menus []AppMenu
	if err := json.Unmarshal(respBytes, &menus); err != nil {
		return []AppMenu{}, nil
	}

	for i := range menus {
		s.populateCategory(&menus[i])
	}

	if asTree {
		rootParentID := ""
		if parentID != "" && parentID != "null" && parentID != "root" {
			rootParentID = parentID
		}
		return buildMenuTree(menus, rootParentID), nil
	}

	if menus == nil {
		menus = []AppMenu{}
	}
	return menus, nil
}

// GetMenuByID retrieves a single menu by UUID
func (s *menuService) GetMenuByID(id string) (*AppMenu, error) {
	cleanID := strings.TrimSpace(id)
	if cleanID == "" {
		return nil, errors.New("id menu tidak boleh kosong")
	}

	queryParams := fmt.Sprintf("id=eq.%s&limit=1", url.QueryEscape(cleanID))
	respBytes, err := s.sbClient.GetDB("app_menus", queryParams, "")
	if err != nil {
		return nil, err
	}

	var items []AppMenu
	if err := json.Unmarshal(respBytes, &items); err != nil || len(items) == 0 {
		return nil, fmt.Errorf("menu dengan id '%s' tidak ditemukan", cleanID)
	}

	menu := items[0]
	s.populateCategory(&menu)
	return &menu, nil
}

// GetMenuByCode retrieves a single menu by its unique code
func (s *menuService) GetMenuByCode(code string) (*AppMenu, error) {
	cleanCode := strings.TrimSpace(code)
	if cleanCode == "" {
		return nil, errors.New("kode menu tidak boleh kosong")
	}

	queryParams := fmt.Sprintf("code=eq.%s&limit=1", url.QueryEscape(cleanCode))
	respBytes, err := s.sbClient.GetDB("app_menus", queryParams, "")
	if err != nil {
		return nil, err
	}

	var items []AppMenu
	if err := json.Unmarshal(respBytes, &items); err != nil || len(items) == 0 {
		return nil, fmt.Errorf("menu dengan kode '%s' tidak ditemukan", cleanCode)
	}

	menu := items[0]
	s.populateCategory(&menu)
	return &menu, nil
}

// CreateMenu inserts a new service menu or sub-menu
func (s *menuService) CreateMenu(req CreateMenuRequest) (*AppMenu, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, errors.New("nama menu wajib diisi")
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		code = strings.ToLower(strings.ReplaceAll(req.Name, " ", "_"))
	}

	// Cek apakah code sudah digunakan
	existing, _ := s.GetMenuByCode(code)
	if existing != nil {
		code = fmt.Sprintf("%s_%d", code, time.Now().Unix()%1000)
	}

	newID := newUUID()
	now := time.Now()

	actionType := strings.ToUpper(strings.TrimSpace(req.ActionType))
	if actionType == "" {
		actionType = "CATEGORY"
	}

	iconType := strings.ToUpper(strings.TrimSpace(req.IconType))
	if iconType == "" {
		iconType = "IONICONS"
	}

	iconName := strings.TrimSpace(req.IconName)
	if iconName == "" {
		iconName = "grid-outline"
	}

	bgColor := strings.TrimSpace(req.BgColor)
	if bgColor == "" {
		bgColor = "#EFF6FF"
	}

	iconColor := strings.TrimSpace(req.IconColor)
	if iconColor == "" {
		iconColor = "#2563EB"
	}

	placement := strings.ToUpper(strings.TrimSpace(req.Placement))
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

	var categoryID *string
	if req.CategoryID != nil && strings.TrimSpace(*req.CategoryID) != "" {
		clean := strings.TrimSpace(*req.CategoryID)
		categoryID = &clean
	}

	var parentID *string
	level := 1
	if req.ParentID != nil && strings.TrimSpace(*req.ParentID) != "" {
		cleanParent := strings.TrimSpace(*req.ParentID)
		parent, err := s.GetMenuByID(cleanParent)
		if err != nil {
			return nil, fmt.Errorf("parent menu dengan id '%s' tidak ditemukan: %w", cleanParent, err)
		}
		parentID = &cleanParent
		level = parent.Level + 1
	}
	if req.Level != nil && *req.Level > 0 {
		level = *req.Level
	}

	newMenu := AppMenu{
		ID:           newID,
		ParentID:     parentID,
		Level:        level,
		Code:         code,
		Name:         req.Name,
		Subtitle:     req.Subtitle,
		CategoryID:   categoryID,
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
		Children:     []AppMenu{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	payload := map[string]interface{}{
		"id":             newMenu.ID,
		"code":           newMenu.Code,
		"name":           newMenu.Name,
		"subtitle":       newMenu.Subtitle,
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
		"level":          newMenu.Level,
	}
	if categoryID != nil {
		payload["category_id"] = *categoryID
	} else {
		payload["category_id"] = nil
	}
	if parentID != nil {
		payload["parent_id"] = *parentID
	} else {
		payload["parent_id"] = nil
	}

	respBytes, err := s.sbClient.PostDB("app_menus", payload, "return=representation", "")
	if err != nil {
		log.Printf("[MenuService] PostDB failed: %v", err)
		return nil, fmt.Errorf("gagal menyimpan menu ke database: %w", err)
	}

	var created []AppMenu
	if err := json.Unmarshal(respBytes, &created); err == nil && len(created) > 0 {
		newMenu = created[0]
	}

	s.populateCategory(&newMenu)
	return &newMenu, nil
}

// UpdateMenu modifies an existing menu or its parent/sub-menu link
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
		if strings.TrimSpace(*req.CategoryID) == "" {
			current.CategoryID = nil
			payload["category_id"] = nil
		} else {
			clean := strings.TrimSpace(*req.CategoryID)
			current.CategoryID = &clean
			payload["category_id"] = clean
		}
	}
	if req.ParentID != nil {
		clean := strings.TrimSpace(*req.ParentID)
		if clean == "" || clean == "null" || clean == "root" {
			current.ParentID = nil
			current.Level = 1
			payload["parent_id"] = nil
			payload["level"] = 1
		} else {
			if clean == id {
				return nil, errors.New("menu tidak dapat menjadi parent dari dirinya sendiri")
			}
			parent, err := s.GetMenuByID(clean)
			if err != nil {
				return nil, fmt.Errorf("parent menu dengan id '%s' tidak ditemukan", clean)
			}
			current.ParentID = &clean
			current.Level = parent.Level + 1
			payload["parent_id"] = clean
			payload["level"] = current.Level
		}
	}
	if req.Level != nil && *req.Level > 0 {
		current.Level = *req.Level
		payload["level"] = *req.Level
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
		current.Placement = strings.ToUpper(req.Placement)
		payload["placement"] = current.Placement
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
	_, err = s.sbClient.PatchDB("app_menus", queryParams, payload, "")
	if err != nil {
		log.Printf("[MenuService] PatchDB failed: %v", err)
		return nil, fmt.Errorf("gagal mengupdate menu: %w", err)
	}

	s.populateCategory(current)
	return current, nil
}

// DeleteMenu removes a menu and cascade-deletes its sub-menus
func (s *menuService) DeleteMenu(id string) error {
	cleanID := strings.TrimSpace(id)
	if cleanID == "" {
		return errors.New("id menu tidak boleh kosong")
	}

	// Cek keberadaan menu
	_, err := s.GetMenuByID(cleanID)
	if err != nil {
		return err
	}

	queryParams := fmt.Sprintf("id=eq.%s", url.QueryEscape(cleanID))
	err = s.sbClient.DeleteDB("app_menus", queryParams, "")
	if err != nil {
		log.Printf("[MenuService] DeleteDB failed: %v", err)
		return fmt.Errorf("gagal menghapus menu: %w", err)
	}

	return nil
}
