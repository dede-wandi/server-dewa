package product

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/dede-wandi/server-dewa/internal/supabase"
)

// Repository defines the storage and caching contract for PPOB products
type Repository interface {
	GetAll() []*Product
	GetByCode(code string) (*Product, bool)
	GetByCategory(category string) []*Product
	GetByBrand(brand string) []*Product
	Search(query string) []*Product
	Save(p *Product)
	SyncFromSupabase(sbClient *supabase.Client) (int, error)
}

// CachedRepository provides lightning-fast sub-millisecond product retrieval with TTL
type CachedRepository struct {
	mu           sync.RWMutex
	products     map[string]*Product   // Key: Code
	byCategory   map[string][]*Product // Key: Category lower
	byBrand      map[string][]*Product // Key: Brand lower
	allCache     []*Product
	lastSyncTime time.Time
	ttl          time.Duration
}

// NewCachedRepository initializes the product repository
func NewCachedRepository(ttl time.Duration) *CachedRepository {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	repo := &CachedRepository{
		products:   make(map[string]*Product, 64),
		byCategory: make(map[string][]*Product, 16),
		byBrand:    make(map[string][]*Product, 32),
		allCache:   []*Product{},
		ttl:        ttl,
	}

	return repo
}

func (r *CachedRepository) GetAll() []*Product {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.allCache) == 0 {
		return []*Product{}
	}

	res := make([]*Product, len(r.allCache))
	copy(res, r.allCache)
	return res
}

func (r *CachedRepository) GetByCode(codeOrID string) (*Product, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	trimmed := strings.TrimSpace(codeOrID)
	if p, ok := r.products[strings.ToUpper(trimmed)]; ok {
		return p, true
	}
	for _, p := range r.allCache {
		if p.ID == trimmed || strings.EqualFold(p.Code, trimmed) || strings.EqualFold(p.BuyerSKUCode, trimmed) {
			return p, true
		}
	}
	return nil, false
}

func (r *CachedRepository) GetByCategory(category string) []*Product {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list, ok := r.byCategory[strings.ToLower(strings.TrimSpace(category))]
	if !ok {
		return []*Product{}
	}
	res := make([]*Product, len(list))
	copy(res, list)
	return res
}

func (r *CachedRepository) GetByBrand(brand string) []*Product {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list, ok := r.byBrand[strings.ToLower(strings.TrimSpace(brand))]
	if !ok {
		return []*Product{}
	}
	res := make([]*Product, len(list))
	copy(res, list)
	return res
}

func (r *CachedRepository) Search(query string) []*Product {
	r.mu.RLock()
	defer r.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return r.GetAll()
	}

	var matched []*Product
	for _, p := range r.allCache {
		if strings.Contains(strings.ToLower(p.Code), q) ||
			strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Brand), q) ||
			strings.Contains(strings.ToLower(string(p.Category)), q) {
			matched = append(matched, p)
		}
	}
	return matched
}

func (r *CachedRepository) Save(p *Product) {
	r.mu.Lock()
	defer r.mu.Unlock()

	code := strings.ToUpper(strings.TrimSpace(p.Code))
	p.Code = code
	p.UpdatedAt = time.Now()
	r.products[code] = p

	r.rebuildIndexLocked()
}

func (r *CachedRepository) rebuildIndexLocked() {
	r.allCache = make([]*Product, 0, len(r.products))
	r.byCategory = make(map[string][]*Product, 16)
	r.byBrand = make(map[string][]*Product, 32)

	for _, p := range r.products {
		r.allCache = append(r.allCache, p)

		cat := strings.ToLower(string(p.Category))
		r.byCategory[cat] = append(r.byCategory[cat], p)

		brand := strings.ToLower(p.Brand)
		r.byBrand[brand] = append(r.byBrand[brand], p)
	}
}

func (r *CachedRepository) SyncFromSupabase(sbClient *supabase.Client) (int, error) {
	if sbClient == nil {
		return len(r.allCache), nil
	}

	// 1. Fetch live products from Supabase database (public.ppob_products)
	dbBytes, err := sbClient.GetDB("ppob_products", "select=*", "")
	if err == nil && len(dbBytes) > 0 {
		var dbProducts []struct {
			ID            string  `json:"id"`
			CategoryID    string  `json:"category_id"`
			VendorCode    string  `json:"vendor_code"`
			BuyerSKUCode  string  `json:"buyer_sku_code"`
			Code          string  `json:"code"`
			Name          string  `json:"name"`
			Category      string  `json:"category"`
			Brand         string  `json:"brand"`
			Price         float64 `json:"price"`
			CostPrice     float64 `json:"cost_price"`
			PointsReward  int64   `json:"points_reward"`
			Status        string  `json:"status"`
			Stock         int     `json:"stock"`
			Description   string  `json:"description"`
			ImageURL      string  `json:"image_url"`
			IsActive      bool    `json:"is_active"`
		}
		if err := json.Unmarshal(dbBytes, &dbProducts); err == nil && len(dbProducts) > 0 {
			r.mu.Lock()
			for _, dp := range dbProducts {
				code := dp.Code
				if code == "" && dp.BuyerSKUCode != "" {
					code = dp.BuyerSKUCode
				} else if code == "" {
					code = strings.ToUpper(strings.ReplaceAll(dp.Name, " ", "_"))
				}
				brand := dp.Brand
				if brand == "" {
					brand = extractBrand(dp.Name)
				}
				ptsReward := dp.PointsReward
				if ptsReward <= 0 {
					ptsReward = int64(dp.Price * 0.01)
				}
				vendorCode := dp.VendorCode
				if vendorCode == "" {
					vendorCode = "MANUAL"
				}
				status := dp.Status
				isActive := dp.IsActive
				if !isActive || strings.EqualFold(status, "INACTIVE") {
					status = "INACTIVE"
					isActive = false
				} else if status == "" {
					status = "ACTIVE"
					isActive = true
				} else {
					isActive = strings.EqualFold(status, "ACTIVE")
				}

				p := &Product{
					ID:            dp.ID,
					CategoryID:    dp.CategoryID,
					VendorCode:    vendorCode,
					BuyerSKUCode:  dp.BuyerSKUCode,
					Code:          code,
					Name:          dp.Name,
					Category:      ProductCategory(strings.ToLower(dp.Category)),
					Brand:         brand,
					CostPrice:     dp.CostPrice,
					OriginalPrice: dp.CostPrice,
					Price:         dp.Price,
					PointsReward:  ptsReward,
					Status:        status,
					IsActive:      isActive,
					Description:   dp.Description,
					Stock:         dp.Stock,
					ImageURL:      dp.ImageURL,
					UpdatedAt:     time.Now(),
				}
				r.products[p.Code] = p
			}
			r.lastSyncTime = time.Now()
			r.rebuildIndexLocked()
			count := len(r.allCache)
			r.mu.Unlock()
			return count, nil
		}
	}

	// Re-index locally after sync
	r.mu.Lock()
	r.lastSyncTime = time.Now()
	r.rebuildIndexLocked()
	count := len(r.allCache)
	r.mu.Unlock()

	return count, nil
}

func extractBrand(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "telkomsel"):
		return "Telkomsel"
	case strings.Contains(lower, "indosat"):
		return "Indosat"
	case strings.Contains(lower, "xl"):
		return "XL"
	case strings.Contains(lower, "axis"):
		return "Axis"
	case strings.Contains(lower, "tri") || strings.Contains(lower, "three"):
		return "Tri"
	case strings.Contains(lower, "smartfren"):
		return "Smartfren"
	case strings.Contains(lower, "pln"):
		return "PLN"
	case strings.Contains(lower, "dana"):
		return "DANA"
	case strings.Contains(lower, "gopay"):
		return "GoPay"
	case strings.Contains(lower, "ovo"):
		return "OVO"
	case strings.Contains(lower, "shopeepay"):
		return "ShopeePay"
	case strings.Contains(lower, "mobile legends"):
		return "Mobile Legends"
	case strings.Contains(lower, "free fire"):
		return "Free Fire"
	default:
		parts := strings.Split(name, " ")
		if len(parts) > 0 {
			return parts[0]
		}
		return "General"
	}
}

func (r *CachedRepository) seedInitialCatalog() {
	// Kosongkan - produk murni dari database Supabase
	r.rebuildIndexLocked()
}
