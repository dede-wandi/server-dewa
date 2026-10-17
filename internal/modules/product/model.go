package product

import "time"

// ProductCategory represents a high-level PPOB / digital product category
type ProductCategory string

const (
	CategoryPulsa   ProductCategory = "pulsa"
	CategoryData    ProductCategory = "data"
	CategoryPLN     ProductCategory = "pln"
	CategoryEMoney  ProductCategory = "emoney"
	CategoryGame    ProductCategory = "game"
	CategoryVoucher ProductCategory = "voucher"
)

// Product represents a digital product item in DewaCell PPOB catalog
type Product struct {
	ID            string          `json:"id"`
	CategoryID    string          `json:"category_id"`              // Wajib berelasi dengan app_categories
	CategoryName  string          `json:"category_name,omitempty"`   // Nama kategori (join/lookup)
	CategoryType  string          `json:"category_type,omitempty"`   // Tipe kategori: pulsa, data, pln, emoney, game, pdam
	Category      ProductCategory `json:"category"`                  // pulsa, data, pln, emoney, game
	VendorCode    string          `json:"vendor_code"`               // DIGIFLAZZ, IAK, SERPUL, MANUAL
	Vendor        string          `json:"vendor"`                    // Alias vendor untuk kompatibilitas frontend
	BuyerSKUCode  string          `json:"buyer_sku_code"`            // Kode produk bayar dari vendor (e.g. BPDAM177 / FIBP14)
	InquirySKUCode string         `json:"inquiry_sku_code,omitempty"`// Kode produk cek/inquiry (e.g. CPDAM177) jika pascabayar
	Code          string          `json:"code"`                      // Kode unik sistem DewaCell (e.g. FIBP14)
	Name          string          `json:"name"`                      // e.g. Promo Indosat Data 14GB 28 Hari
	Brand         string          `json:"brand"`                     // Telkomsel, Indosat, XL, PLN, DANA
	CostPrice     float64         `json:"cost_price"`                // Harga modal dari vendor (e.g. Rp 44.640)
	OriginalPrice float64         `json:"original_price"`            // Alias harga modal untuk backward compatibility
	Price         float64         `json:"price"`                     // Harga jual ke pelanggan/mitra
	PointsReward  int64           `json:"points_reward"`             // Cashback DewaPoin
	Status        string          `json:"status"`                    // ACTIVE, INACTIVE, GANGGUAN, EMPTY
	Type          string          `json:"type"`                      // PREPAID, POSTPAID (default: PREPAID)
	IsActive      bool            `json:"is_active"`                 // true = Aktif, false = Non-aktif
	Description   string          `json:"description"`
	Stock         int             `json:"stock"`
	ImageURL      string          `json:"image_url,omitempty"`
	CreatedAt     time.Time       `json:"created_at,omitempty"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// CreateProductRequest payload pembuatan produk baru
type CreateProductRequest struct {
	CategoryID     string  `json:"category_id"`              // WAJIB: Produk belum bisa dibuat jika kategori tidak ada
	VendorCode     string  `json:"vendor_code,omitempty"`    // e.g. DFLASH, MANUAL
	Vendor         string  `json:"vendor,omitempty"`         // Fallback alias dari frontend
	BuyerSKUCode   string  `json:"buyer_sku_code,omitempty"` // e.g. FIBP14 / BPDAM177
	InquirySKUCode string  `json:"inquiry_sku_code,omitempty"`// e.g. CPDAM177 (khusus pascabayar jika beda dari buyer_sku_code)
	Code           string  `json:"code,omitempty"`           // Kode unik (opsional, jika kosong diisi buyer_sku_code)
	Name           string  `json:"name"`                     // e.g. Promo Indosat Data 14GB 28 Hari
	Category       string  `json:"category,omitempty"`       // pulsa, data, pln, dll (opsional, diambil dari kategori)
	Brand          string  `json:"brand,omitempty"`          // e.g. Indosat
	Type           string  `json:"type,omitempty"`           // PREPAID / POSTPAID (default: PREPAID)
	CostPrice      float64 `json:"cost_price"`               // e.g. 44640 (Harga modal vendor)
	Price          float64 `json:"price"`                    // e.g. 46000 (Harga jual)
	PointsReward   *int64  `json:"points_reward,omitempty"`  // Poin reward transaksi
	Status         string  `json:"status,omitempty"`         // ACTIVE / INACTIVE (default: ACTIVE)
	Description    string  `json:"description,omitempty"`
	Stock          int     `json:"stock,omitempty"`
	ImageURL       string  `json:"image_url,omitempty"`
}

// UpdateProductRequest payload update produk
type UpdateProductRequest struct {
	CategoryID     *string  `json:"category_id,omitempty"`
	VendorCode     *string  `json:"vendor_code,omitempty"`
	Vendor         *string  `json:"vendor,omitempty"`
	BuyerSKUCode   *string  `json:"buyer_sku_code,omitempty"`
	InquirySKUCode *string  `json:"inquiry_sku_code,omitempty"`
	Code           *string  `json:"code,omitempty"`
	Name           *string  `json:"name,omitempty"`
	Brand          *string  `json:"brand,omitempty"`
	Type           *string  `json:"type,omitempty"`
	CostPrice      *float64 `json:"cost_price,omitempty"`
	Price          *float64 `json:"price,omitempty"`
	PointsReward   *int64   `json:"points_reward,omitempty"`
	Status         *string  `json:"status,omitempty"`
	Description    *string  `json:"description,omitempty"`
	Stock          *int     `json:"stock,omitempty"`
	ImageURL     *string  `json:"image_url,omitempty"`
}
