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

	"github.com/dede-wandi/server-dewa/internal/modules/product"
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
	sbClient    *supabase.Client
	prodService product.Service
	mu          sync.RWMutex
	fallback    []AppCategory
}

func NewService(sbClient *supabase.Client, prodService product.Service) Service {
	return &categoryService{
		sbClient:    sbClient,
		prodService: prodService,
		fallback:    []AppCategory{},
	}
}

func (s *categoryService) fetchProductsForCategory(categoryID, slug string) []*product.Product {
	var results []*product.Product
	seen := make(map[string]bool)

	// 1. In-memory product service query (instant cache)
	if s.prodService != nil {
		if categoryID != "" {
			for _, p := range s.prodService.ListProducts(categoryID, "", "", true) {
				if !seen[p.ID] {
					seen[p.ID] = true
					results = append(results, p)
				}
			}
		}
		if slug != "" {
			for _, p := range s.prodService.ListProducts(slug, "", "", true) {
				if !seen[p.ID] {
					seen[p.ID] = true
					results = append(results, p)
				}
			}
		}
	}

	// 2. Query Supabase DB directly if results are empty
	if len(results) == 0 && s.sbClient != nil {
		var orConditions []string
		if categoryID != "" {
			orConditions = append(orConditions, fmt.Sprintf("category_id.eq.%s", url.QueryEscape(categoryID)))
		}
		if slug != "" {
			orConditions = append(orConditions, fmt.Sprintf("category.eq.%s", url.QueryEscape(slug)))
		}

		if len(orConditions) > 0 {
			queryParams := fmt.Sprintf("or=(%s)&is_active=eq.true&status=neq.INACTIVE&order=price.asc,name.asc", strings.Join(orConditions, ","))
			dbBytes, err := s.sbClient.GetDB("ppob_products", queryParams, "")
			if err == nil && len(dbBytes) > 0 {
				var dbProds []*product.Product
				if err := json.Unmarshal(dbBytes, &dbProds); err == nil {
					for _, p := range dbProds {
						if !seen[p.ID] {
							seen[p.ID] = true
							results = append(results, p)
						}
					}
				}
			}
		}
	}

	// Filter out postpaid inquiry-only products (hanya tampilkan produk bayar yang valid ke user)
	var filteredResults []*product.Product
	for _, p := range results {
		if !product.IsPostpaidInquiryOnly(p, results) {
			filteredResults = append(filteredResults, p)
		}
	}
	results = filteredResults

	if results == nil {
		results = []*product.Product{}
	}
	return results
}

func (s *categoryService) populateProductsForCategory(cat *AppCategory) {
	if cat == nil {
		return
	}
	// Fetch products directly for this category
	cat.Products = s.fetchProductsForCategory(cat.ID, cat.Slug)

	// Fetch products for all children
	for i := range cat.Children {
		child := &cat.Children[i]
		child.Products = s.fetchProductsForCategory(child.ID, child.Slug)
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
		log.Printf("[CategoryService] GetActiveCategories DB query failed: %v", err)
		return []AppCategory{}, nil
	}

	var categories []AppCategory
	if err := json.Unmarshal(respBytes, &categories); err != nil {
		return []AppCategory{}, nil
	}

	if categories == nil {
		categories = []AppCategory{}
	}

	if asTree {
		tree := buildTree(categories, parentID)
		for i := range tree {
			s.populateProductsForCategory(&tree[i])
		}
		return tree, nil
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
		log.Printf("[CategoryService] GetAllCategories DB query failed: %v", err)
		return []AppCategory{}, nil
	}

	var categories []AppCategory
	if err := json.Unmarshal(respBytes, &categories); err != nil {
		return []AppCategory{}, nil
	}

	if categories == nil {
		categories = []AppCategory{}
	}

	if asTree {
		tree := buildTree(categories, parentID)
		for i := range tree {
			s.populateProductsForCategory(&tree[i])
		}
		return tree, nil
	}

	return categories, nil
}

// GetCategoryByID returns category by ID (with children and products)
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
			s.populateProductsForCategory(&cat)
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
			s.populateProductsForCategory(&c)
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
			s.populateProductsForCategory(&cat)
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
			s.populateProductsForCategory(&c)
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

	var parentID *string
	if req.ParentID != nil && strings.TrimSpace(*req.ParentID) != "" {
		clean := strings.TrimSpace(*req.ParentID)
		parentID = &clean
	}

	slug := req.Slug
	if slug == "" {
		slug = slugify(req.Name)
	}

	code := req.Code
	if code == "" {
		if parentID != nil {
			parent, err := s.GetCategoryByID(*parentID)
			if err == nil && parent != nil {
				code = parent.Code + "_" + strings.ReplaceAll(slug, "-", "_")
			} else {
				code = "cat_" + strings.ReplaceAll(slug, "-", "_")
			}
		} else {
			code = "cat_" + strings.ReplaceAll(slug, "-", "_")
		}
	}

	// Clamp lengths to match DB VARCHAR limits (code <= 50, slug <= 120)
	if len(code) > 50 {
		code = code[:50]
	}
	if len(slug) > 120 {
		slug = slug[:120]
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	level := req.Level
	if level <= 0 {
		if parentID == nil {
			level = 1
		} else {
			// Find parent level
			parent, err := s.GetCategoryByID(*parentID)
			if err == nil && parent != nil {
				level = parent.Level + 1
			} else {
				level = 2
			}
		}
	}

	categoryType := strings.ToLower(strings.TrimSpace(req.Type))

	newCat := AppCategory{
		ID:          newID,
		ParentID:    parentID,
		Name:        req.Name,
		Code:        code,
		Slug:        slug,
		Description: req.Description,
		Icon:        req.Icon,
		IconURL:     req.IconURL,
		BannerURL:   req.BannerURL,
		Badge:       req.Badge,
		Type:        categoryType,
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
		"name":        newCat.Name,
		"code":        newCat.Code,
		"slug":        newCat.Slug,
		"description": newCat.Description,
		"icon":        newCat.Icon,
		"icon_url":    newCat.IconURL,
		"banner_url":  newCat.BannerURL,
		"badge":       newCat.Badge,
		"type":        newCat.Type,
		"level":       newCat.Level,
		"sort_order":  newCat.SortOrder,
		"is_active":   newCat.IsActive,
		"meta":        newCat.Meta,
	}
	if parentID != nil {
		payload["parent_id"] = *parentID
	} else {
		payload["parent_id"] = nil
	}

	respBytes, err := s.sbClient.PostDB("app_categories", payload, "return=representation", "")
	if err != nil {
		log.Printf("[CategoryService] PostDB failed: %v", err)
		errStr := err.Error()
		if strings.Contains(errStr, "23505") || strings.Contains(errStr, "unique constraint") {
			if strings.Contains(errStr, "app_categories_code_key") {
				return nil, fmt.Errorf("kode kategori '%s' sudah digunakan oleh kategori lain di database. Silakan gunakan kode unik yang berbeda", newCat.Code)
			}
			if strings.Contains(errStr, "app_categories_slug_key") {
				return nil, fmt.Errorf("slug kategori '%s' sudah digunakan di database. Silakan gunakan nama atau kode kategori yang berbeda", newCat.Slug)
			}
		}
		return nil, fmt.Errorf("gagal menyimpan kategori ke database: %w", err)
	}

	var created []AppCategory
	if err := json.Unmarshal(respBytes, &created); err == nil && len(created) > 0 {
		newCat = created[0]
	}

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
		if strings.TrimSpace(*req.ParentID) == "" {
			current.ParentID = nil
			payload["parent_id"] = nil
		} else {
			clean := strings.TrimSpace(*req.ParentID)
			current.ParentID = &clean
			payload["parent_id"] = clean
		}
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
	if req.Type != nil {
		cleanType := strings.ToLower(strings.TrimSpace(*req.Type))
		current.Type = cleanType
		payload["type"] = cleanType
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
	if err != nil {
		log.Printf("[CategoryService] PatchDB failed: %v", err)
		errStr := err.Error()
		if strings.Contains(errStr, "23505") || strings.Contains(errStr, "unique constraint") {
			if strings.Contains(errStr, "app_categories_code_key") {
				return nil, fmt.Errorf("kode kategori sudah digunakan oleh kategori lain di database. Silakan gunakan kode unik yang berbeda")
			}
			if strings.Contains(errStr, "app_categories_slug_key") {
				return nil, fmt.Errorf("slug kategori sudah digunakan di database. Silakan gunakan nama atau kode lain")
			}
		}
		return nil, fmt.Errorf("gagal mengupdate kategori di database: %w", err)
	}

	var updated []AppCategory
	if err := json.Unmarshal(respBytes, &updated); err == nil && len(updated) > 0 {
		current = &updated[0]
	}

	return current, nil
}

// DeleteCategory removes a category
func (s *categoryService) DeleteCategory(id string) error {
	queryParams := fmt.Sprintf("id=eq.%s", url.QueryEscape(id))
	err := s.sbClient.DeleteDB("app_categories", queryParams, "")
	if err != nil {
		log.Printf("[CategoryService] DeleteDB failed: %v", err)
		return fmt.Errorf("gagal menghapus kategori dari database: %w", err)
	}

	return nil
}
