package transaction

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strconv"
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
	InquiryBill(user *supabase.User, req BillInquiryRequest) (*BillInquiryResponse, error)
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
		if strings.EqualFold(prod.Status, "GANGGUAN") {
			return nil, fmt.Errorf("Produk %s saat ini sedang GANGGUAN dari provider. Silakan coba kembali beberapa saat lagi.", prod.Name)
		}
		return nil, fmt.Errorf("Produk %s saat ini sedang %s. Silakan coba produk lain.", prod.Name, prod.Status)
	}

	// 2. Generate Order ID Numerik Bersih (Standar Gojek: YYMMDDHHMMSS + 4 digit acak)
	var orderCode string
	for attempt := 0; attempt < 10; attempt++ {
		now := time.Now()
		candidate := fmt.Sprintf("%s%04d", now.Format("060102150405"), rand.Intn(9000)+1000)
		if _, exists := s.repo.GetOrderByCode(candidate); !exists {
			orderCode = candidate
			break
		}
		time.Sleep(1 * time.Millisecond)
	}
	if orderCode == "" {
		now := time.Now()
		orderCode = fmt.Sprintf("%s%04d", now.Format("060102150405"), (now.Nanosecond()/100000)%9000+1000)
	}

	// 3. Tentukan Nominal Bayar & SKU Dispatch (Prepaid vs Postpaid)
	chargedAmount := prod.Price
	sku := strings.TrimSpace(prod.BuyerSKUCode)
	if sku == "" {
		sku = strings.TrimSpace(prod.Code)
	}

	if strings.TrimSpace(req.InquiryID) != "" {
		inqID := strings.TrimSpace(req.InquiryID)
		inq, exists := s.repo.GetInquiry(inqID)
		if !exists || inq == nil {
			return nil, errors.New("Sesi inquiry tagihan tidak ditemukan atau sudah pernah dibayar. Harap lakukan cek tagihan ulang.")
		}
		if time.Now().After(inq.ExpiresAt) {
			s.repo.DeleteInquiry(inqID)
			return nil, errors.New("Sesi inquiry tagihan telah kedaluwarsa (berlaku 15 menit). Harap lakukan cek tagihan ulang.")
		}
		if inq.CustomerNo != "" && inq.CustomerNo != target {
			return nil, fmt.Errorf("Nomor pelanggan (%s) tidak cocok dengan nomor pada sesi inquiry (%s)", target, inq.CustomerNo)
		}
		chargedAmount = inq.TotalAmount
		if inq.PaymentSKU != "" {
			sku = inq.PaymentSKU
		}
		// Hapus sesi inquiry agar tidak terjadi pembayaran ganda (double-payment prevention)
		defer s.repo.DeleteInquiry(inqID)
	} else if prod.Type == "POSTPAID" {
		return nil, errors.New("Produk pascabayar wajib melakukan cek tagihan (inquiry) terlebih dahulu sebelum pembayaran. Silakan lakukan cek tagihan dan sertakan inquiry_id.")
	}

	// 4. Potong Saldo Atomik via WalletService
	payReq := wallet.PayRequest{
		Amount:       chargedAmount,
		Category:     wallet.CategoryPayment,
		Title:        prod.Name,
		Description:  target,
		ReferenceID:  orderCode,
		PIN:          req.PIN,
		PointsReward: prod.PointsReward,
	}

	wTrx, _, err := s.walletService.Pay(user.ID, user.Email, token, payReq)
	if err != nil {
		return nil, fmt.Errorf("Gagal memproses pembayaran saldo: %w", err)
	}

	// 5. Buat Record Order
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
		Price:           chargedAmount,
		PointsEarned:    prod.PointsReward,
		Status:          StatusProcessing,
		WalletTrxCode:   wTrx.TransactionCode,
		CreatedAt:       now,
	}

	// 6. Eksekusi Pengiriman ke Provider / Switching PPOB
	vMgr := vendor.GetGlobalManager()
	vendorCode := strings.ToUpper(strings.TrimSpace(prod.VendorCode))
	if vendorCode == "" {
		vendorCode = "MANUAL"
	}

	vReq := &vendor.VendorOrderRequest{
		OrderID:      order.ID,
		VendorCode:   vendorCode,
		BuyerSKUCode: sku,
		CustomerNo:   target,
		RefID:        orderCode,
		MaxPrice:     chargedAmount,
		Testing:      false,
	}

	log.Printf("🚀 [DISPATCH] Meneruskan pesanan %s ke provider (SKU=%s, Tujuan=%s)...", orderCode, sku, target)
	vResp, err := vMgr.ProcessOrder(context.Background(), vReq)
	if err != nil {
		log.Printf("❌ [FAILED] Transaksi %s gagal: %v", orderCode, err)
		order.Status = StatusFailed
		order.ErrorMessage = sanitizeErrorMessage(err.Error())

		// Auto-refund ke saldo dompet jika provider menolak / gagal
		refundReason := order.ErrorMessage
		if refundReason == "" {
			refundReason = "Pesanan ditolak server provider"
		}
		refundReq := wallet.RefundRequest{
			TransactionCode: wTrx.TransactionCode,
			Amount:          chargedAmount,
			TargetUserID:    user.ID,
			Reason:          refundReason,
		}
		refTrx, _, errRefund := s.walletService.Refund(user, token, refundReq)
		if errRefund != nil {
			log.Printf("Gagal auto-refund untuk %s: %v", orderCode, errRefund)
		} else if refTrx != nil {
			log.Printf("Saldo berhasil direfund TrxCode=%s, Amount=Rp %.0f", refTrx.TransactionCode, refTrx.Amount)
		}
		order.Status = StatusRefunded
		s.repo.CreateOrder(order)
		return order, fmt.Errorf("%s. Saldo Anda telah otomatis dikembalikan", order.ErrorMessage)
	}

	log.Printf("📥 [RESPONSE] Status=%s, SN=%s, Msg=%s", vResp.Status, vResp.SerialNumber, vResp.Message)

	if vResp.Status == "FAILED" {
		order.Status = StatusFailed
		order.ErrorMessage = sanitizeErrorMessage(vResp.Message)

		// Auto-refund otomatis jika provider menyatakan gagal
		refundReason := order.ErrorMessage
		if refundReason == "" {
			refundReason = "Pesanan ditolak server provider"
		}
		refundReq := wallet.RefundRequest{
			TransactionCode: wTrx.TransactionCode,
			Amount:          chargedAmount,
			TargetUserID:    user.ID,
			Reason:          refundReason,
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
	clean := strings.ToUpper(strings.TrimSpace(orderCode))
	if o, ok := s.repo.GetOrderByCode(clean); ok {
		return o, nil
	}

	if s.walletService != nil {
		allTrx := s.walletService.GetAllTransactions()
		for _, trx := range allTrx {
			if trx != nil && (strings.EqualFold(trx.TransactionCode, clean) || strings.EqualFold(trx.ReferenceID, clean) || strings.EqualFold(trx.ID, clean)) {
				if o := mapWalletTrxToOrder(trx); o != nil {
					return o, nil
				}
			}
		}
	}

	return nil, errors.New("Order transaksi tidak ditemukan")
}

func deduplicateOrders(orders []*Order) []*Order {
	seenCodes := make(map[string]bool)
	seenIDs := make(map[string]bool)
	seenTrxCodes := make(map[string]bool)
	result := make([]*Order, 0, len(orders))

	for _, o := range orders {
		if o == nil {
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(o.OrderCode))
		id := strings.ToUpper(strings.TrimSpace(o.ID))
		trxCode := strings.ToUpper(strings.TrimSpace(o.WalletTrxCode))

		// Abaikan jika kode pesanan atau ID sudah pernah dimasukkan
		if code != "" && seenCodes[code] {
			continue
		}
		if id != "" && seenIDs[id] {
			continue
		}
		if trxCode != "" && seenTrxCodes[trxCode] {
			continue
		}

		if code != "" {
			seenCodes[code] = true
		}
		if id != "" {
			seenIDs[id] = true
		}
		if trxCode != "" {
			seenTrxCodes[trxCode] = true
		}

		result = append(result, o)
	}
	return result
}

func (s *service) GetUserOrders(userID string) []*Order {
	memOrders := s.repo.GetOrdersByUser(userID)
	var combined []*Order
	combined = append(combined, memOrders...)

	// Gabungkan secara cerdas dengan mutasi transaksi yang tercatat di Supabase / WalletService
	if s.walletService != nil {
		walletTrxs := s.walletService.GetHistory(userID)
		for _, trx := range walletTrxs {
			if trx == nil {
				continue
			}
			mapped := mapWalletTrxToOrder(trx)
			if mapped != nil {
				combined = append(combined, mapped)
			}
		}
	}

	return deduplicateOrders(combined)
}

func (s *service) GetAllOrders() []*Order {
	memOrders := s.repo.GetAllOrders()
	var combined []*Order
	combined = append(combined, memOrders...)

	if s.walletService != nil {
		walletTrxs := s.walletService.GetAllTransactions()
		for _, trx := range walletTrxs {
			if trx == nil {
				continue
			}
			mapped := mapWalletTrxToOrder(trx)
			if mapped != nil {
				combined = append(combined, mapped)
			}
		}
	}

	return deduplicateOrders(combined)
}

func mapWalletTrxToOrder(trx *wallet.WalletTransaction) *Order {
	if trx == nil {
		return nil
	}

	orderCode := strings.TrimSpace(trx.ReferenceID)
	isUUID := func(s string) bool {
		return len(s) == 36 && strings.Count(s, "-") == 4
	}

	if orderCode == "" || isUUID(orderCode) {
		if trx.TransactionCode != "" && !isUUID(trx.TransactionCode) {
			orderCode = trx.TransactionCode
		} else {
			dateStr := trx.CreatedAt.Format("060102150405")
			if dateStr == "010101000000" || strings.HasPrefix(dateStr, "010101") {
				dateStr = time.Now().Format("060102150405")
			}
			// Gunakan hash deterministik dari ID agar stabil numerik 16 digit
			hash := uint32(0)
			for _, c := range trx.ID {
				hash = hash*31 + uint32(c)
			}
			suffix := 1000 + (hash % 9000)
			orderCode = fmt.Sprintf("%s%04d", dateStr, suffix)
		}
	}

	// Hilangkan prefix legacy seperti ORD-, TRX-, REF- agar seragam numerik murni
	for _, pfx := range []string{"ORD-", "TRX-", "TOP-", "REF-", "ADJ-", "TFO-", "TFI-"} {
		if strings.HasPrefix(strings.ToUpper(orderCode), pfx) {
			orderCode = strings.ReplaceAll(orderCode[len(pfx):], "-", "")
			break
		}
	}

	if strings.ToUpper(trx.Category) == "REFUND" || strings.HasPrefix(strings.ToUpper(trx.Title), "REFUND") {
		// Refund adalah mutasi saldo pengembalian dana, bukan transaksi pesanan baru
		return nil
	}

	prodName := strings.TrimPrefix(trx.Title, "Pembelian ")

	target := "-"
	desc := strings.TrimSpace(trx.Description)
	if strings.HasPrefix(desc, "Tujuan: ") {
		targetPart := strings.TrimPrefix(desc, "Tujuan: ")
		if idx := strings.Index(targetPart, " ("); idx != -1 {
			target = targetPart[:idx]
		} else {
			target = targetPart
		}
	} else if desc != "" {
		target = desc
	}

	status := StatusSuccess
	switch strings.ToUpper(trx.Status) {
	case "PENDING":
		status = StatusProcessing
	case "FAILED":
		status = StatusFailed
	case "REFUND", "REFUNDED", "REVERSED":
		status = StatusRefunded
	}

	if strings.ToUpper(trx.Category) == "REFUND" {
		status = StatusRefunded
	}

	sn := ""
	if strings.Contains(trx.AdminNotes, "SN: ") {
		parts := strings.Split(trx.AdminNotes, "SN: ")
		if len(parts) > 1 {
			sn = strings.TrimSpace(parts[1])
		}
	}

	return &Order{
		ID:            trx.ID,
		OrderCode:     orderCode,
		UserID:        trx.UserID,
		ProductName:   prodName,
		TargetAccount: target,
		Price:         trx.Amount,
		Status:        status,
		SerialNumber:  sn,
		WalletTrxCode: trx.TransactionCode,
		CreatedAt:     trx.CreatedAt,
	}
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

// InquiryBill mengecek tagihan pascabayar ke server provider/switching PPOB
func (s *service) InquiryBill(user *supabase.User, req BillInquiryRequest) (*BillInquiryResponse, error) {
	prodCode := strings.ToUpper(strings.TrimSpace(req.ProductCode))
	if prodCode == "" {
		return nil, errors.New("kode produk (product_code) wajib diisi")
	}

	customerNo := strings.TrimSpace(req.CustomerNo)
	if customerNo == "" {
		return nil, errors.New("nomor pelanggan / ID pelanggan (customer_no) wajib diisi")
	}

	prod, err := s.productService.GetProduct(prodCode)
	if err != nil {
		return nil, err
	}

	if prod.Status != "ACTIVE" {
		if strings.EqualFold(prod.Status, "GANGGUAN") {
			return nil, fmt.Errorf("Produk %s saat ini sedang GANGGUAN dari provider. Silakan coba beberapa saat lagi.", prod.Name)
		}
		return nil, fmt.Errorf("Produk %s saat ini sedang %s.", prod.Name, prod.Status)
	}

	// Tentukan Inquiry SKU (kode cek)
	inquirySKU := strings.TrimSpace(prod.InquirySKUCode)
	paymentSKU := strings.TrimSpace(prod.BuyerSKUCode)
	if paymentSKU == "" {
		paymentSKU = prod.Code
	}
	if inquirySKU == "" {
		if prod.Type == "POSTPAID" && strings.HasPrefix(strings.ToUpper(paymentSKU), "B") {
			inquirySKU = "C" + paymentSKU[1:]
		} else {
			inquirySKU = paymentSKU
		}
	}

	vendorCode := strings.ToUpper(strings.TrimSpace(prod.VendorCode))
	if vendorCode == "" {
		vendorCode = "DFLASH"
	}

	// Generate refID untuk inquiry
	inqID := fmt.Sprintf("INQ-%s%04d", time.Now().Format("060102150405"), rand.Intn(9000)+1000)

	vMgr := vendor.GetGlobalManager()
	vReq := &vendor.VendorOrderRequest{
		OrderID:      inqID,
		VendorCode:   vendorCode,
		BuyerSKUCode: inquirySKU,
		CustomerNo:   customerNo,
		RefID:        inqID,
		MaxPrice:     0,
		Testing:      false,
	}

	inqChan := vendor.RegisterInquiryWaiter(inqID)
	defer vendor.RemoveInquiryWaiter(inqID)

	log.Printf("🔍 [INQUIRY DISPATCH] Mengirim Cek Tagihan %s (SKU=%s, Tujuan=%s)...", inqID, inquirySKU, customerNo)
	vResp, err := vMgr.ProcessOrder(context.Background(), vReq)
	if err != nil {
		log.Printf("⚠️ [INQUIRY INITIAL DISPATCH] %s: %v. Menunggu callback vendor...", inqID, err)
	}

	var finalStatus string
	var finalMessage string
	var finalRaw map[string]interface{}

	if vResp != nil && vResp.Status == "SUCCESS" {
		finalStatus = vResp.Status
		finalMessage = vResp.Message
		finalRaw = vResp.RawResponse
	} else {
		// Tunggu callback webhook vendor hingga 4 detik (DFlash rata-rata 1-1.5 detik)
		select {
		case ev := <-inqChan:
			log.Printf("⚡ [INQUIRY CALLBACK RECEIVED] %s: Status=%s, Msg=%s", inqID, ev.Status, ev.Message)
			finalStatus = ev.Status
			finalMessage = ev.Message
			finalRaw = map[string]interface{}{"message": ev.Message, "sn": ev.SN}
		case <-time.After(4000 * time.Millisecond):
			if vResp != nil {
				finalStatus = vResp.Status
				finalMessage = vResp.Message
				finalRaw = vResp.RawResponse
			} else {
				finalStatus = "FAILED"
				finalMessage = "Waktu pengecekan tagihan ke vendor habis (timeout)"
			}
		}
	}

	if finalStatus == "FAILED" {
		msg := finalMessage
		if msg == "" {
			msg = "Tagihan tidak ditemukan atau sudah lunas"
		}
		return nil, errors.New(msg)
	}

	adminFee := prod.Price
	if adminFee <= 0 {
		adminFee = 2500 // Default admin fee dari panel superadmin
	}

	// Parse teks atau map hasil respon inquiry
	customerName, period, periodCount, meterInfo, billAmount, desc, details := parseInquiryDetails(finalMessage, finalRaw, customerNo, adminFee)
	totalAmount := billAmount + adminFee

	inquiryResp := &BillInquiryResponse{
		InquiryID:    inqID,
		ProductCode:  prod.Code,
		ProductName:  prod.Name,
		VendorCode:   vendorCode,
		InquirySKU:   inquirySKU,
		PaymentSKU:   paymentSKU,
		CustomerNo:   customerNo,
		CustomerName: customerName,
		Period:       period,
		PeriodCount:  periodCount,
		MeterInfo:    meterInfo,
		BillAmount:   billAmount,
		AdminFee:     adminFee,
		TotalAmount:  totalAmount,
		Description:  desc,
		Details:      details,
		RawResponse:  finalRaw,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}

	s.repo.SaveInquiry(inquiryResp)
	return inquiryResp, nil
}

func parseInquiryDetails(msg string, raw map[string]interface{}, customerNo string, adminFee float64) (customerName string, period string, periodCount int, meterInfo string, billAmount float64, desc string, details []InquiryDetailItem) {
	desc = msg
	customerName = "PELANGGAN"
	periodCount = 1

	// Cek dari map jika ada
	if raw != nil {
		if n, ok := raw["nama"].(string); ok && n != "" {
			customerName = n
		} else if n, ok := raw["customer_name"].(string); ok && n != "" {
			customerName = n
		}
		if p, ok := raw["periode"].(string); ok && p != "" {
			period = p
		}
		if t, ok := raw["total"].(float64); ok && t > 0 {
			billAmount = t
		} else if t, ok := raw["tagihan"].(float64); ok && t > 0 {
			billAmount = t
		} else if t, ok := raw["nominal"].(float64); ok && t > 0 {
			billAmount = t
		}
	}

	// Parsing regex dari pesan teks DFLASH / OtomaX
	// Contoh: CPDAM71 ke 01030040238 BERHASIL @16/10 18:03 SN: Dewi Lestari/BLN:APR 26,MEI 26,SEP 26/JBLN:1/SM:1075-1318/TAG:1344540/ADM:9000/TOTAL:1353540.
	if msg != "" {
		// 1. Regex Nama: dari SN: [Nama]/ atau NAMA: [Nama]
		reNama := regexp.MustCompile(`(?i)(?:NAMA\s*(?:PELANGGAN)?|SN)\s*[:=]\s*([^,;.\n/]+)`)
		if matches := reNama.FindStringSubmatch(msg); len(matches) > 1 {
			val := strings.TrimSpace(matches[1])
			if !strings.EqualFold(val, "BERHASIL") && !strings.EqualFold(val, "SUKSES") && len(val) > 1 {
				customerName = val
			}
		}

		// 2. Regex Periode Bulan: BLN:APR 26,MEI 26,SEP 26/ atau PERIODE: OKTOBER 2026
		rePeriode := regexp.MustCompile(`(?i)(?:PERIODE|BLN|BULAN)\s*[:=]\s*([^;.\n/]+)`)
		if matches := rePeriode.FindStringSubmatch(msg); len(matches) > 1 {
			period = strings.TrimSpace(matches[1])
		}

		// 3. Regex Jumlah Bulan (JBLN)
		reJbln := regexp.MustCompile(`(?i)(?:JBLN|JML\s*BLN|LEMBAR|LEMBER)\s*[:=]\s*([0-9]+)`)
		if matches := reJbln.FindStringSubmatch(msg); len(matches) > 1 {
			if cnt, err := strconv.Atoi(matches[1]); err == nil && cnt > 0 {
				periodCount = cnt
			}
		}
		// Jika di teks JBLN tertulis 1 tapi di Periode ada koma (misal "APR 26,MEI 26,SEP 26" = 3 bulan)
		if strings.Contains(period, ",") {
			commaCount := strings.Count(period, ",") + 1
			if commaCount > periodCount {
				periodCount = commaCount
			}
		}

		// 4. Regex Stand Meter (SM)
		reSM := regexp.MustCompile(`(?i)(?:SM|METER|STAND)\s*[:=]\s*([^/;\n]+)`)
		if matches := reSM.FindStringSubmatch(msg); len(matches) > 1 {
			meterInfo = strings.TrimSpace(matches[1])
		}

		// 5. Regex Tagihan: PRIORITASKAN NILAI "TOTAL" dari vendor
		reTotal := regexp.MustCompile(`(?i)(?:TOTAL|JUMLAH|JML\s*TAG)\s*[:=]?\s*(?:RP\.?\s*)?([0-9]{1,3}(?:\.[0-9]{3})+|[0-9]{3,})`)
		if matches := reTotal.FindStringSubmatch(msg); len(matches) > 1 {
			cleanNum := strings.ReplaceAll(matches[1], ".", "")
			cleanNum = strings.ReplaceAll(cleanNum, ",", "")
			if val, err := strconv.ParseFloat(cleanNum, 64); err == nil && val > 0 {
				billAmount = val
			}
		}

		// Fallback jika TOTAL tidak ada: coba dari TAGIHAN / TAG / POKOK
		if billAmount <= 0 {
			reTag := regexp.MustCompile(`(?i)(?:TAGIHAN|TAG|POKOK)\s*[:=]?\s*(?:RP\.?\s*)?([0-9]{1,3}(?:\.[0-9]{3})+|[0-9]{3,})`)
			if matches := reTag.FindStringSubmatch(msg); len(matches) > 1 {
				cleanNum := strings.ReplaceAll(matches[1], ".", "")
				cleanNum = strings.ReplaceAll(cleanNum, ",", "")
				if val, err := strconv.ParseFloat(cleanNum, 64); err == nil && val > 0 {
					billAmount = val
				}
			}
		}
	}

	if period == "" {
		period = time.Now().Format("January 2006")
	}

	// 6. Buat Array Terstruktur Details agar Frontend Mobile App Sangat Mudah Mengambilnya
	details = []InquiryDetailItem{
		{Label: "Nama Pelanggan", Value: customerName},
		{Label: "ID Pelanggan", Value: customerNo},
		{Label: "Bulan / Periode", Value: period},
		{Label: "Jumlah Bulan", Value: fmt.Sprintf("%d Bulan", periodCount)},
	}
	if meterInfo != "" {
		details = append(details, InquiryDetailItem{Label: "Stand Meter", Value: meterInfo})
	}
	details = append(details,
		InquiryDetailItem{Label: "Total Tagihan", Value: fmt.Sprintf("Rp %s", formatRupiah(billAmount))},
		InquiryDetailItem{Label: "Biaya Admin", Value: fmt.Sprintf("Rp %s", formatRupiah(adminFee))},
		InquiryDetailItem{Label: "Total Bayar", Value: fmt.Sprintf("Rp %s", formatRupiah(billAmount+adminFee))},
	)

	return customerName, period, periodCount, meterInfo, billAmount, desc, details
}

func formatRupiah(amount float64) string {
	s := fmt.Sprintf("%.0f", amount)
	n := len(s)
	if n <= 3 {
		return s
	}
	var res []byte
	rem := n % 3
	if rem > 0 {
		res = append(res, s[:rem]...)
		if n > rem {
			res = append(res, '.')
		}
	}
	for i := rem; i < n; i += 3 {
		res = append(res, s[i:i+3]...)
		if i+3 < n {
			res = append(res, '.')
		}
	}
	return string(res)
}
