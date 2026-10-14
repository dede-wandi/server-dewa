package transaction

import "time"

// OrderStatus represents the lifecycle of a PPOB order
type OrderStatus string

const (
	StatusPending    OrderStatus = "PENDING"
	StatusProcessing OrderStatus = "PROCESSING"
	StatusSuccess    OrderStatus = "SUCCESS"
	StatusFailed     OrderStatus = "FAILED"
	StatusRefunded   OrderStatus = "REFUNDED"
)

// Order represents a PPOB transaction order
type Order struct {
	ID              string      `json:"id"`
	OrderCode       string      `json:"order_code"`       // e.g. ORD-20261014-998811
	UserID          string      `json:"user_id"`
	UserEmail       string      `json:"user_email"`
	ProductCode     string      `json:"product_code"`
	ProductName     string      `json:"product_name"`
	ProductCategory string      `json:"product_category"` // pulsa, data, pln, emoney, game
	TargetAccount   string      `json:"target_account"`   // No HP, No Meter, Player ID
	Price           float64     `json:"price"`
	PointsEarned    int64       `json:"points_earned"`
	Status          OrderStatus `json:"status"`
	SerialNumber    string      `json:"serial_number,omitempty"` // Stroom token PLN / SN Pulsa
	WalletTrxCode   string      `json:"wallet_trx_code,omitempty"`
	ErrorMessage    string      `json:"error_message,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	CompletedAt     *time.Time  `json:"completed_at,omitempty"`
}

// CreateOrderRequest represents client payload to purchase a digital product
type CreateOrderRequest struct {
	ProductCode   string `json:"product_code"`
	TargetAccount string `json:"target_account"` // No HP / Meteran / ID
}
