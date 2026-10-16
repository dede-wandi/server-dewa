package transaction

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

const ordersStorageFile = "data/orders.json"

// Repository defines the contract for storing and querying PPOB orders
type Repository interface {
	CreateOrder(order *Order)
	GetOrderByCode(code string) (*Order, bool)
	GetOrdersByUser(userID string) []*Order
	GetAllOrders() []*Order
	UpdateOrder(order *Order)
	SaveInquiry(inq *BillInquiryResponse)
	GetInquiry(id string) (*BillInquiryResponse, bool)
	DeleteInquiry(id string)
}

// MemoryRepository provides thread-safe order storage with O(1) indexed lookups and disk persistence
type MemoryRepository struct {
	mu           sync.RWMutex
	orders       map[string]*Order   // Key: OrderCode
	ordersByUser map[string][]*Order // Key: UserID
	allOrders    []*Order
	inquiries    map[string]*BillInquiryResponse // Key: InquiryID
}

// NewMemoryRepository initializes repository and reloads persisted orders if available
func NewMemoryRepository() *MemoryRepository {
	repo := &MemoryRepository{
		orders:       make(map[string]*Order, 64),
		ordersByUser: make(map[string][]*Order, 16),
		allOrders:    make([]*Order, 0, 64),
		inquiries:    make(map[string]*BillInquiryResponse, 32),
	}
	repo.loadFromFile()
	return repo
}

func (r *MemoryRepository) loadFromFile() {
	data, err := os.ReadFile(ordersStorageFile)
	if err != nil {
		return
	}
	var list []*Order
	if err := json.Unmarshal(data, &list); err != nil {
		return
	}
	for _, o := range list {
		if o == nil {
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(o.OrderCode))
		if code == "" {
			continue
		}
		if _, exists := r.orders[code]; exists {
			continue // Skip duplicates from file
		}
		o.OrderCode = code
		r.orders[code] = o
		r.ordersByUser[o.UserID] = append(r.ordersByUser[o.UserID], o)
		r.allOrders = append(r.allOrders, o)
	}
}

func (r *MemoryRepository) saveToFile() {
	_ = os.MkdirAll("data", 0755)
	data, err := json.MarshalIndent(r.allOrders, "", "  ")
	if err == nil {
		_ = os.WriteFile(ordersStorageFile, data, 0644)
	}
}

func (r *MemoryRepository) CreateOrder(order *Order) {
	r.mu.Lock()
	defer r.mu.Unlock()

	code := strings.ToUpper(strings.TrimSpace(order.OrderCode))
	order.OrderCode = code

	// If order already exists with this code, update in-place to prevent duplicates
	if existing, exists := r.orders[code]; exists {
		*existing = *order
		r.saveToFile()
		return
	}

	r.orders[code] = order
	r.ordersByUser[order.UserID] = append([]*Order{order}, r.ordersByUser[order.UserID]...)
	r.allOrders = append([]*Order{order}, r.allOrders...)
	r.saveToFile()
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
	r.saveToFile()
}

func (r *MemoryRepository) SaveInquiry(inq *BillInquiryResponse) {
	if inq == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inquiries[inq.InquiryID] = inq
}

func (r *MemoryRepository) GetInquiry(id string) (*BillInquiryResponse, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inq, ok := r.inquiries[id]
	return inq, ok
}

func (r *MemoryRepository) DeleteInquiry(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.inquiries, id)
}
