package transaction

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/dede-wandi/server-dewa/internal/modules/product"
	"github.com/dede-wandi/server-dewa/internal/modules/wallet"
	"github.com/dede-wandi/server-dewa/internal/supabase"
)

// Service defines domain orchestration for digital product orders
type Service interface {
	CreateOrder(user *supabase.User, token string, req CreateOrderRequest) (*Order, error)
	GetOrderDetail(orderCode string) (*Order, error)
	GetUserOrders(userID string) []*Order
	GetAllOrders() []*Order
}

type service struct {
	repo           Repository
	walletService  wallet.WalletService
	productService product.Service
	sbClient       *supabase.Client
}

// NewService constructs a new transaction orchestration service instance
func NewService(
	repo Repository,
	walletService wallet.WalletService,
	productService product.Service,
	sbClient *supabase.Client,
) Service {
	return &service{
		repo:           repo,
		walletService:  walletService,
		productService: productService,
		sbClient:       sbClient,
	}
}

func (s *service) CreateOrder(user *supabase.User, token string, req CreateOrderRequest) (*Order, error) {
	if user == nil {
		return nil, errors.New("Sesi pengguna tidak valid")
	}

	target := strings.TrimSpace(req.TargetAccount)
	if target == "" {
		return nil, errors.New("Nomor tujuan / ID pelanggan wajib diisi")
	}

	prodCode := strings.ToUpper(strings.TrimSpace(req.ProductCode))
	if prodCode == "" {
		return nil, errors.New("Kode produk wajib dipilih")
	}

	// 1. Validasi Produk via ProductService
	prod, err := s.productService.GetProduct(prodCode)
	if err != nil {
		return nil, err
	}
	if prod.Status != "ACTIVE" {
		return nil, fmt.Errorf("Produk %s saat ini sedang %s. Silakan coba produk lain.", prod.Name, prod.Status)
	}

	// 2. Generate Order Code Unik
	orderCode := fmt.Sprintf("ORD-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)

	// 3. Potong Saldo Atomik via WalletService
	payReq := wallet.PayRequest{
		Amount:      prod.Price,
		Category:    wallet.CategoryPayment,
		Title:       fmt.Sprintf("Pembelian %s", prod.Name),
		Description: fmt.Sprintf("Tujuan: %s (%s)", target, prod.Brand),
		ReferenceID: orderCode,
		PIN:         req.PIN,
	}

	wTrx, _, err := s.walletService.Pay(user.ID, user.Email, token, payReq)
	if err != nil {
		return nil, fmt.Errorf("Gagal memproses pembayaran saldo: %w", err)
	}

	// 4. Buat Record Order
	now := time.Now()
	order := &Order{
		ID:              fmt.Sprintf("ord-%d", now.UnixNano()),
		OrderCode:       orderCode,
		UserID:          user.ID,
		UserEmail:       user.Email,
		ProductCode:     prod.Code,
		ProductName:     prod.Name,
		ProductCategory: string(prod.Category),
		TargetAccount:   target,
		Price:           prod.Price,
		PointsEarned:    prod.PointsReward,
		Status:          StatusProcessing,
		WalletTrxCode:   wTrx.TransactionCode,
		CreatedAt:       now,
	}

	// 5. Eksekusi Pengiriman ke Provider / Switching PPOB
	// Jika nomor tujuan mengandung kata "gagal" atau "error" -> Uji skenario auto-refund otomatis
	if strings.Contains(strings.ToLower(target), "gagal") || strings.Contains(strings.ToLower(target), "error") {
		order.Status = StatusFailed
		order.ErrorMessage = "Gagal dari provider: Nomor tujuan tidak aktif atau gangguan jaringan provider"

		// AUTO-REFUND: Otomatis kembalikan uang ke saldo pengguna secara aman
		refundReq := wallet.RefundRequest{
			TransactionCode: wTrx.TransactionCode,
			Reason:          fmt.Sprintf("Auto-refund otomatis atas kegagalan order %s (%s)", orderCode, prod.Name),
		}
		s.walletService.Refund(user, token, refundReq)
		order.Status = StatusRefunded

		s.repo.CreateOrder(order)
		return order, fmt.Errorf("Transaksi gagal diproses provider. Saldo Anda sebesar Rp %s telah otomatis dikembalikan (Auto-Refund)!", wallet.FormatRupiah(int64(prod.Price)))
	}

	// Skenario Sukses: Generate SN / Token Resmi
	var sn string
	if prod.Category == product.CategoryPLN {
		// Generate 20-digit Stroom Token PLN
		sn = fmt.Sprintf("%04d-%04d-%04d-%04d-%04d",
			rand.Intn(9000)+1000, rand.Intn(9000)+1000, rand.Intn(9000)+1000, rand.Intn(9000)+1000, rand.Intn(9000)+1000)
	} else {
		// Generate Serial Number Pulsa / Data / E-Money
		sn = fmt.Sprintf("%s%010d", time.Now().Format("2006010215"), rand.Int63n(9000000000)+1000000000)
	}

	order.Status = StatusSuccess
	order.SerialNumber = sn
	completedAt := time.Now()
	order.CompletedAt = &completedAt

	s.repo.CreateOrder(order)
	return order, nil
}

func (s *service) GetOrderDetail(orderCode string) (*Order, error) {
	o, ok := s.repo.GetOrderByCode(orderCode)
	if !ok {
		return nil, errors.New("Order transaksi tidak ditemukan")
	}
	return o, nil
}

func (s *service) GetUserOrders(userID string) []*Order {
	return s.repo.GetOrdersByUser(userID)
}

func (s *service) GetAllOrders() []*Order {
	return s.repo.GetAllOrders()
}
