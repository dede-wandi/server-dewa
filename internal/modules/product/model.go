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
	Code          string          `json:"code"`           // e.g. TSEL10, PLN50, DANA25
	Name          string          `json:"name"`           // e.g. Telkomsel 10.000 Reguler
	Category      ProductCategory `json:"category"`       // pulsa, data, pln, emoney, game
	Brand         string          `json:"brand"`          // Telkomsel, Indosat, PLN, DANA, GoPay
	Price         float64         `json:"price"`          // Harga jual ke pelanggan
	OriginalPrice float64         `json:"original_price"` // Harga modal dari provider
	PointsReward  int64           `json:"points_reward"`  // Cashback DewaPoin otomatis
	Status        string          `json:"status"`         // ACTIVE, GANGGUAN, EMPTY
	Description   string          `json:"description"`
	Stock         int             `json:"stock,omitempty"`
	ImageURL      string          `json:"image_url,omitempty"`
	UpdatedAt     time.Time       `json:"updated_at"`
}
