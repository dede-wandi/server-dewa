package transaction

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/dede-wandi/server-dewa/internal/modules/product"
	"github.com/dede-wandi/server-dewa/internal/modules/vendor"
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

	// 5. Eksekusi Pengiriman ke Provider / Switching PPOB (DFLASH / Vendor Resmi)
	vMgr := vendor.GetGlobalManager()
	vendorCode := strings.ToUpper(strings.TrimSpace(prod.VendorCode))
	if vendorCode == "" || vendorCode == "MANUAL" {
		vendorCode = "DFLASH" // Defaultkan otomatis ke server vendor DFLASH
	}

	sku := strings.TrimSpace(prod.BuyerSKUCode)
	if sku == "" {
		sku = strings.TrimSpace(prod.Code)
	}

	vReq := &vendor.VendorOrderRequest{
		OrderID:      order.ID,
		VendorCode:   vendorCode,
		BuyerSKUCode: sku,
		CustomerNo:   target,
		RefID:        orderCode,
		MaxPrice:     prod.CostPrice,
		Testing:      false,
	}

	log.Printf("🚀 [DISPATCH] Meneruskan pesanan %s ke provider (SKU=%s, Tujuan=%s)...", orderCode, sku, target)
	vResp, err := vMgr.ProcessOrder(context.Background(), vReq)
	if err != nil {
		log.Printf("❌ [FAILED] Transaksi %s gagal: %v", orderCode, err)
		order.Status = StatusFailed
		order.ErrorMessage = sanitizeErrorMessage(err.Error())

		// Auto-refund ke saldo dompet jika provider menolak / gagal
		refundReq := wallet.RefundRequest{
			TransactionCode: wTrx.TransactionCode,
			Amount:          prod.Price,
			TargetUserID:    user.ID,
			Reason:          fmt.Sprintf("Auto-refund otomatis atas kegagalan order %s (%s)", orderCode, prod.Name),
		}
		refTrx, _, errRefund := s.walletService.Refund(user, token, refundReq)
		if errRefund != nil {
			log.Printf("⚠️ [AUTO-REFUND WARNING] Gagal auto-refund untuk %s: %v", orderCode, errRefund)
		} else if refTrx != nil {
			log.Printf("💰 [AUTO-REFUND SUCCESS] Saldo berhasil direfund otomatis! TrxCode=%s, Amount=Rp %.0f", refTrx.TransactionCode, refTrx.Amount)
		}
		order.Status = StatusRefunded
		s.repo.CreateOrder(order)
		return order, fmt.Errorf("%s. Saldo Anda telah otomatis dikembalikan (Auto-Refund)!", order.ErrorMessage)
	}

	log.Printf("📥 [RESPONSE] Status=%s, SN=%s, Msg=%s", vResp.Status, vResp.SerialNumber, vResp.Message)

	if vResp.Status == "FAILED" {
		order.Status = StatusFailed
		order.ErrorMessage = sanitizeErrorMessage(vResp.Message)

		// Auto-refund otomatis jika provider menyatakan gagal
		refundReq := wallet.RefundRequest{
			TransactionCode: wTrx.TransactionCode,
			Amount:          prod.Price,
			TargetUserID:    user.ID,
			Reason:          fmt.Sprintf("Auto-refund atas penolakan order %s (%s)", orderCode, prod.Name),
		}
		refTrx, _, errRefund := s.walletService.Refund(user, token, refundReq)
		if errRefund != nil {
			log.Printf("⚠️ [AUTO-REFUND WARNING] Gagal auto-refund untuk %s: %v", orderCode, errRefund)
		} else if refTrx != nil {
			log.Printf("💰 [AUTO-REFUND SUCCESS] Saldo berhasil direfund otomatis! TrxCode=%s, Amount=Rp %.0f", refTrx.TransactionCode, refTrx.Amount)
		}
		order.Status = StatusRefunded
		s.repo.CreateOrder(order)
		return order, fmt.Errorf("%s. Saldo Anda telah otomatis dikembalikan (Auto-Refund)!", order.ErrorMessage)
	}

	if vResp.Status == "SUCCESS" {
		order.Status = StatusSuccess
		order.SerialNumber = vResp.SerialNumber
		completedAt := time.Now()
		order.CompletedAt = &completedAt
	} else {
		// PENDING / PROCESSING (Menunggu Serial Number dari switching atau webhook)
		order.Status = StatusProcessing
		if vResp.SerialNumber != "" {
			order.SerialNumber = vResp.SerialNumber
		}
	}

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

// sanitizeErrorMessage membersihkan dan menyamarkan seluruh respons error server / provider
// agar nama vendor, IP server, atau endpoint teknis tidak bocor ke client pengguna
func sanitizeErrorMessage(raw string) string {
	upper := strings.ToUpper(raw)

	if strings.Contains(upper, "SALDO") || strings.Contains(upper, "BALANCE") || strings.Contains(upper, "INSUFFICIENT") {
		return "Saldo server tidak mencukupi"
	}
	if strings.Contains(upper, "STOK") || strings.Contains(upper, "KOSONG") || strings.Contains(upper, "HABIS") {
		return "Stok produk sedang habis di server"
	}
	if strings.Contains(upper, "CUT OFF") || strings.Contains(upper, "CUTOFF") || strings.Contains(upper, "MAINTENANCE") || strings.Contains(upper, "GANGGUAN") {
		return "Layanan produk sedang dalam pemeliharaan sistem"
	}
	if strings.Contains(upper, "INVALID") || strings.Contains(upper, "SALAH") || strings.Contains(upper, "TIDAK TERDAFTAR") || strings.Contains(upper, "BELUM TERDAFTAR") {
		return "Nomor tujuan tidak terdaftar atau tidak valid"
	}
	return "Pesanan sedang tidak dapat diproses oleh server"
}
