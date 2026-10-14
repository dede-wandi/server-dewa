package role

import (
	"strings"
	"sync"
)

// Registry manages all registered roles and business types in the system
type Registry struct {
	mu          sync.RWMutex
	definitions map[Role]BusinessDefinition
}

// GlobalRegistry is the singleton default registry for the application
var GlobalRegistry = NewRegistry()

// NewRegistry initializes a clean OOP registry with pre-configured DewaCell domain roles
func NewRegistry() *Registry {
	r := &Registry{
		definitions: make(map[Role]BusinessDefinition),
	}

	// 1. Core Platform Roles
	r.Register(BusinessDefinition{
		Role:        RoleSuperAdmin,
		Code:        string(RoleSuperAdmin),
		Name:        "Super Admin",
		Category:    CategoryPlatform,
		BadgeColor:  "purple",
		Description: "Akses penuh platform dan seluruh kontrol sistem",
		Keywords:    []string{"superadmin", "owner", "admin"},
	})
	r.Register(BusinessDefinition{
		Role:        RoleAdmin,
		Code:        string(RoleAdmin),
		Name:        "Administrator",
		Category:    CategoryPlatform,
		BadgeColor:  "indigo",
		Description: "Pengelola operasional dan penyesuaian transaksi",
		Keywords:    []string{"admin"},
	})
	r.Register(BusinessDefinition{
		Role:        RoleKasir,
		Code:        string(RoleKasir),
		Name:        "Kasir Toko",
		Category:    CategoryStaff,
		BadgeColor:  "blue",
		Description: "Staff kasir operasional transaksi harian toko",
		CanHavePOS:  true,
		Keywords:    []string{"kasir", "cashier"},
	})
	r.Register(BusinessDefinition{
		Role:        RoleUser,
		Code:        string(RoleUser),
		Name:        "Pelanggan",
		Category:    CategoryCustomer,
		BadgeColor:  "gray",
		Description: "Pengguna reguler aplikasi DewaCell",
		Keywords:    []string{"user", "pelanggan", "customer"},
	})

	// 2. Merchant & Business Models (Tambahkan 1 definisi di sini jika ingin ekspansi bisnis baru!)
	r.Register(BusinessDefinition{
		Role:        RoleFNB,
		Code:        string(RoleFNB),
		Name:        "Merchant F&B (Kuliner)",
		Category:    CategoryMerchant,
		BadgeColor:  "orange",
		Description: "Restoran, Cafe, Rumah Makan, Warung Kuliner",
		Keywords:    []string{"fnb", "resto", "kuliner", "cafe", "makan", "warung"},
		CanHavePOS:  true,
		Features:    []string{"table_management", "kitchen_order", "split_bill"},
	})
	r.Register(BusinessDefinition{
		Role:        RoleRetail,
		Code:        string(RoleRetail),
		Name:        "Merchant Retail / Toko",
		Category:    CategoryMerchant,
		BadgeColor:  "emerald",
		Description: "Toko Kelontong, Minimarket, Fashion, Aksesoris",
		Keywords:    []string{"retail", "kelontong", "toko", "mart", "sembako"},
		CanHavePOS:  true,
		Features:    []string{"barcode_scanning", "stock_alert", "wholesale_pricing"},
	})
	r.Register(BusinessDefinition{
		Role:        RoleJasa,
		Code:        string(RoleJasa),
		Name:        "Merchant Jasa & Servis",
		Category:    CategoryMerchant,
		BadgeColor:  "cyan",
		Description: "Laundry, Barbershop, Cuci Kendaraan, Servis Elektronik",
		Keywords:    []string{"jasa", "laundry", "barber", "servis", "salon"},
		CanHavePOS:  true,
		Features:    []string{"queue_order", "status_tracking"},
	})
	r.Register(BusinessDefinition{
		Role:        RoleApotek,
		Code:        string(RoleApotek),
		Name:        "Merchant Farmasi / Apotek",
		Category:    CategoryMerchant,
		BadgeColor:  "teal",
		Description: "Apotek, Toko Obat, Herbal, Alat Kesehatan",
		Keywords:    []string{"apotek", "farmasi", "obat", "medis"},
		CanHavePOS:  true,
		Features:    []string{"expiry_tracking", "batch_number"},
	})

	return r
}

// Register registers a new business definition into the registry
func (r *Registry) Register(def BusinessDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.definitions[def.Role] = def
}

// Get retrieves a business definition by its role
func (r *Registry) Get(target Role) (BusinessDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.definitions[target]
	return def, ok
}

// GetAll returns all registered roles and business definitions
func (r *Registry) GetAll() []BusinessDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]BusinessDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		res = append(res, def)
	}
	return res
}

// GetMerchants returns only business definitions categorized as merchants
func (r *Registry) GetMerchants() []BusinessDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]BusinessDefinition, 0, 8)
	for _, def := range r.definitions {
		if def.Category == CategoryMerchant {
			res = append(res, def)
		}
	}
	return res
}

// Resolve resolves a given role string to a concrete BusinessDefinition
func (r *Registry) Resolve(explicitRole, _ string) BusinessDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. If explicit role matches a registered definition from database, return it
	if explicitRole != "" {
		targetRole := Role(strings.ToLower(strings.TrimSpace(explicitRole)))
		if def, ok := r.definitions[targetRole]; ok {
			return def
		}
	}

	// 2. Default fallback to regular user/customer
	return r.definitions[RoleUser]
}
