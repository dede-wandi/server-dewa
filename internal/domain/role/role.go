package role

// Role represents a system, merchant, or customer role in DewaCell
type Role string

const (
	// Platform Core Roles
	RoleSuperAdmin Role = "superadmin"
	RoleAdmin      Role = "admin"
	RoleKasir      Role = "kasir"
	RoleUser       Role = "user" // Default pelanggan

	// Merchant / Business Model Roles (Easily extensible by adding 1 line)
	RoleFNB    Role = "fnb"    // Kuliner, Resto, Cafe, Warung Makan
	RoleRetail Role = "retail" // Toko Kelontong, Minimarket, Retail
	RoleJasa   Role = "jasa"   // Laundry, Barbershop, Cuci Kendaraan, Salon
	RoleApotek Role = "apotek" // Apotek, Toko Obat, Herbal
)

// Category represents the high-level grouping of a role
type Category string

const (
	CategoryPlatform Category = "PLATFORM" // Superadmin, Admin
	CategoryMerchant Category = "MERCHANT" // F&B, Retail, Jasa, Apotek, etc.
	CategoryStaff    Category = "STAFF"    // Kasir, Supervisor
	CategoryCustomer Category = "CUSTOMER" // Pelanggan umum
)

// BusinessDefinition represents the domain model and metadata of a business type or role
type BusinessDefinition struct {
	Role        Role     `json:"role"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`        // Human-friendly title
	Category    Category `json:"category"`    // PLATFORM, MERCHANT, STAFF, CUSTOMER
	BadgeColor  string   `json:"badge_color"` // UI badge color token
	Description string   `json:"description"`
	Keywords    []string `json:"keywords,omitempty"` // Automatic detection keywords
	CanHavePOS  bool     `json:"can_have_pos"`       // Can access DewaCell POS
	Features    []string `json:"features,omitempty"` // Feature flags: "tables", "barcode_scan", "booking", "inventory"
}
