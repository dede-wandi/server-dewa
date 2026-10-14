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

// NewCachedRepository initializes the product repository with an initial catalog
func NewCachedRepository(ttl time.Duration) *CachedRepository {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	repo := &CachedRepository{
		products:   make(map[string]*Product, 64),
		byCategory: make(map[string][]*Product, 16),
		byBrand:    make(map[string][]*Product, 32),
		ttl:        ttl,
	}

	repo.seedInitialCatalog()
	return repo
}

func (r *CachedRepository) GetAll() []*Product {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]*Product, len(r.allCache))
	copy(res, r.allCache)
	return res
}

func (r *CachedRepository) GetByCode(code string) (*Product, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.products[strings.ToUpper(strings.TrimSpace(code))]
	return p, ok
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

	// 1. Fetch live products from Supabase database (public.products)
	dbBytes, err := sbClient.GetDB("products", "select=*&is_active=eq.true", "")
	if err == nil && len(dbBytes) > 0 {
		var dbProducts []struct {
			ID            string  `json:"id"`
			Code          string  `json:"code"`
			Name          string  `json:"name"`
			Category      string  `json:"category"`
			Brand         string  `json:"brand"`
			Price         float64 `json:"price"`
			CostPrice     float64 `json:"cost_price"`
			PointsReward  int64   `json:"points_reward"`
			Stock         int     `json:"stock"`
			Description   string  `json:"description"`
			ImageURL      string  `json:"image_url"`
			IsActive      bool    `json:"is_active"`
		}
		if err := json.Unmarshal(dbBytes, &dbProducts); err == nil && len(dbProducts) > 0 {
			r.mu.Lock()
			for _, dp := range dbProducts {
				code := dp.Code
				if code == "" {
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
				p := &Product{
					ID:            dp.ID,
					Code:          code,
					Name:          dp.Name,
					Category:      ProductCategory(strings.ToLower(dp.Category)),
					Brand:         brand,
					Price:         dp.Price,
					OriginalPrice: dp.CostPrice,
					PointsReward:  ptsReward,
					Status:        "ACTIVE",
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
	initial := []*Product{
		// Pulsa
		{ID: "p-01", Code: "TSEL5", Name: "Telkomsel 5.000", Category: CategoryPulsa, Brand: "Telkomsel", Price: 6250, OriginalPrice: 5800, PointsReward: 50, Status: "ACTIVE", Description: "Pulsa Reguler Telkomsel masa aktif 7 hari"},
		{ID: "p-02", Code: "TSEL10", Name: "Telkomsel 10.000", Category: CategoryPulsa, Brand: "Telkomsel", Price: 11250, OriginalPrice: 10600, PointsReward: 100, Status: "ACTIVE", Description: "Pulsa Reguler Telkomsel masa aktif 15 hari"},
		{ID: "p-03", Code: "TSEL25", Name: "Telkomsel 25.000", Category: CategoryPulsa, Brand: "Telkomsel", Price: 26000, OriginalPrice: 25100, PointsReward: 250, Status: "ACTIVE", Description: "Pulsa Reguler Telkomsel masa aktif 30 hari"},
		{ID: "p-04", Code: "ISAT10", Name: "Indosat Ooredoo 10.000", Category: CategoryPulsa, Brand: "Indosat", Price: 11000, OriginalPrice: 10400, PointsReward: 100, Status: "ACTIVE", Description: "Pulsa Reguler Indosat masa aktif 15 hari"},
		{ID: "p-05", Code: "XL10", Name: "XL Axiata 10.000", Category: CategoryPulsa, Brand: "XL", Price: 11100, OriginalPrice: 10500, PointsReward: 100, Status: "ACTIVE", Description: "Pulsa Reguler XL masa aktif 15 hari"},

		// Paket Data
		{ID: "p-06", Code: "TSEL-DATA-10GB", Name: "Telkomsel InternetMax 10GB", Category: CategoryData, Brand: "Telkomsel", Price: 35000, OriginalPrice: 32500, PointsReward: 350, Status: "ACTIVE", Description: "Kuota Nasional 10GB (30 Hari)"},
		{ID: "p-07", Code: "TSEL-DATA-25GB", Name: "Telkomsel InternetMax 25GB", Category: CategoryData, Brand: "Telkomsel", Price: 65000, OriginalPrice: 61000, PointsReward: 650, Status: "ACTIVE", Description: "Kuota Nasional 25GB (30 Hari)"},
		{ID: "p-08", Code: "TSEL-DATA-50GB", Name: "Telkomsel InternetMax 50GB", Category: CategoryData, Brand: "Telkomsel", Price: 110000, OriginalPrice: 103000, PointsReward: 1100, Status: "ACTIVE", Description: "Kuota Nasional 50GB (30 Hari)"},

		// Token PLN
		{ID: "p-09", Code: "PLN20", Name: "Token Listrik PLN 20.000", Category: CategoryPLN, Brand: "PLN", Price: 21500, OriginalPrice: 20200, PointsReward: 200, Status: "ACTIVE", Description: "Stroom Token PLN Prabayar Rp 20.000"},
		{ID: "p-10", Code: "PLN50", Name: "Token Listrik PLN 50.000", Category: CategoryPLN, Brand: "PLN", Price: 51500, OriginalPrice: 50200, PointsReward: 500, Status: "ACTIVE", Description: "Stroom Token PLN Prabayar Rp 50.000"},
		{ID: "p-11", Code: "PLN100", Name: "Token Listrik PLN 100.000", Category: CategoryPLN, Brand: "PLN", Price: 101500, OriginalPrice: 100200, PointsReward: 1000, Status: "ACTIVE", Description: "Stroom Token PLN Prabayar Rp 100.000"},

		// E-Money
		{ID: "p-12", Code: "DANA25", Name: "Top Up DANA 25.000", Category: CategoryEMoney, Brand: "DANA", Price: 26000, OriginalPrice: 25200, PointsReward: 250, Status: "ACTIVE", Description: "Isi saldo dompet digital DANA"},
		{ID: "p-13", Code: "DANA50", Name: "Top Up DANA 50.000", Category: CategoryEMoney, Brand: "DANA", Price: 51000, OriginalPrice: 50200, PointsReward: 500, Status: "ACTIVE", Description: "Isi saldo dompet digital DANA"},
		{ID: "p-14", Code: "GOPAY50", Name: "Top Up GoPay 50.000", Category: CategoryEMoney, Brand: "GoPay", Price: 51200, OriginalPrice: 50300, PointsReward: 500, Status: "ACTIVE", Description: "Isi saldo GoPay Customer"},

		// Game
		{ID: "p-15", Code: "ML86", Name: "Mobile Legends 86 Diamonds", Category: CategoryGame, Brand: "Mobile Legends", Price: 20500, OriginalPrice: 19300, PointsReward: 200, Status: "ACTIVE", Description: "Top Up Instan Mobile Legends via User ID & Zone ID"},
		{ID: "p-16", Code: "FF140", Name: "Free Fire 140 Diamonds", Category: CategoryGame, Brand: "Free Fire", Price: 20000, OriginalPrice: 18900, PointsReward: 200, Status: "ACTIVE", Description: "Top Up Instan Free Fire via Player ID"},
	}

	for _, p := range initial {
		p.UpdatedAt = time.Now()
		r.products[p.Code] = p
	}

	r.rebuildIndexLocked()
}
