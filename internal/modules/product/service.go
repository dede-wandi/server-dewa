package product

import (
	"errors"
	"sort"
	"strings"

	"github.com/dede-wandi/server-dewa/internal/supabase"
)

// Service defines domain business operations for digital product management
type Service interface {
	ListProducts(category, brand, search string) []*Product
	GetProduct(code string) (*Product, error)
	GetCategories() []string
	GetBrands(category string) []string
	SyncProducts(adminID string) (int, error)
}

type service struct {
	repo     Repository
	sbClient *supabase.Client
}

// NewService constructs a new product service instance
func NewService(repo Repository, sbClient *supabase.Client) Service {
	return &service{
		repo:     repo,
		sbClient: sbClient,
	}
}

func (s *service) ListProducts(category, brand, search string) []*Product {
	category = strings.ToLower(strings.TrimSpace(category))
	brand = strings.ToLower(strings.TrimSpace(brand))
	search = strings.TrimSpace(search)

	// If search query provided, search across all fields
	if search != "" {
		return s.repo.Search(search)
	}

	// Filter by category if specified
	if category != "" {
		prods := s.repo.GetByCategory(category)
		if brand != "" {
			var filtered []*Product
			for _, p := range prods {
				if strings.ToLower(p.Brand) == brand {
					filtered = append(filtered, p)
				}
			}
			return filtered
		}
		return prods
	}

	// Filter by brand if specified
	if brand != "" {
		return s.repo.GetByBrand(brand)
	}

	// Return all cached products
	return s.repo.GetAll()
}

func (s *service) GetProduct(code string) (*Product, error) {
	p, ok := s.repo.GetByCode(code)
	if !ok {
		return nil, errors.New("Produk tidak ditemukan atau kode produk tidak valid")
	}
	return p, nil
}

func (s *service) GetCategories() []string {
	all := s.repo.GetAll()
	catMap := make(map[string]bool)
	for _, p := range all {
		catMap[string(p.Category)] = true
	}

	var categories []string
	for c := range catMap {
		categories = append(categories, c)
	}
	sort.Strings(categories)
	return categories
}

func (s *service) GetBrands(category string) []string {
	category = strings.ToLower(strings.TrimSpace(category))
	var prods []*Product
	if category != "" {
		prods = s.repo.GetByCategory(category)
	} else {
		prods = s.repo.GetAll()
	}

	brandMap := make(map[string]bool)
	for _, p := range prods {
		if p.Brand != "" {
			brandMap[p.Brand] = true
		}
	}

	var brands []string
	for b := range brandMap {
		brands = append(brands, b)
	}
	sort.Strings(brands)
	return brands
}

func (s *service) SyncProducts(adminID string) (int, error) {
	return s.repo.SyncFromSupabase(s.sbClient)
}
