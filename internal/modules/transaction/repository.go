package transaction

import (
	"strings"
	"sync"
)

// Repository defines the contract for storing and querying PPOB orders
type Repository interface {
	CreateOrder(order *Order)
	GetOrderByCode(code string) (*Order, bool)
	GetOrdersByUser(userID string) []*Order
	GetAllOrders() []*Order
	UpdateOrder(order *Order)
}

// MemoryRepository provides thread-safe order storage with O(1) indexed lookups
type MemoryRepository struct {
	mu           sync.RWMutex
	orders       map[string]*Order   // Key: OrderCode
	ordersByUser map[string][]*Order // Key: UserID
	allOrders    []*Order
}

// NewMemoryRepository initializes an empty, clean repository
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		orders:       make(map[string]*Order, 64),
		ordersByUser: make(map[string][]*Order, 16),
		allOrders:    make([]*Order, 0, 64),
	}
}

func (r *MemoryRepository) CreateOrder(order *Order) {
	r.mu.Lock()
	defer r.mu.Unlock()

	code := strings.ToUpper(strings.TrimSpace(order.OrderCode))
	order.OrderCode = code

	r.orders[code] = order
	r.ordersByUser[order.UserID] = append([]*Order{order}, r.ordersByUser[order.UserID]...)
	r.allOrders = append([]*Order{order}, r.allOrders...)
}

func (r *MemoryRepository) GetOrderByCode(code string) (*Order, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	o, ok := r.orders[strings.ToUpper(strings.TrimSpace(code))]
	return o, ok
}

func (r *MemoryRepository) GetOrdersByUser(userID string) []*Order {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list, ok := r.ordersByUser[userID]
	if !ok {
		return []*Order{}
	}

	res := make([]*Order, len(list))
	copy(res, list)
	return res
}

func (r *MemoryRepository) GetAllOrders() []*Order {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]*Order, len(r.allOrders))
	copy(res, r.allOrders)
	return res
}

func (r *MemoryRepository) UpdateOrder(order *Order) {
	r.mu.Lock()
	defer r.mu.Unlock()

	code := strings.ToUpper(strings.TrimSpace(order.OrderCode))
	r.orders[code] = order
}
