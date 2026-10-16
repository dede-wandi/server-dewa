package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dede-wandi/server-dewa/internal/modules/vendor"
	"github.com/dede-wandi/server-dewa/internal/supabase"
)

// Service defines domain business operations for digital product management
type Service interface {
	ListProducts(category, brand, search string, activeOnly bool) []*Product
	GetProduct(code string) (*Product, error)
	GetCategories(activeOnly bool) []string
	GetBrands(category string, activeOnly bool) []string
	CreateProduct(req CreateProductRequest) (*Product, error)
	UpdateProduct(id string, req UpdateProductRequest) (*Product, error)
	DeleteProduct(id string) error
	SyncProducts(adminID string) (*SyncResult, error)
	StartDFlashCron(ctx context.Context, interval time.Duration)
	GetCronConfig() CronConfig
	UpdateCronConfig(intervalMinutes int, isEnabled bool) (*CronConfig, error)
	GetSyncLogs(limit int) []SyncLogEntry
}

type service struct {
	repo      Repository
	sbClient  *supabase.Client
	vendorMgr *vendor.VendorManager
	syncer    *DFlashSyncer
}

// NewService constructs a new product service instance
func NewService(repo Repository, sbClient *supabase.Client) Service {
	return &service{
		repo:      repo,
		sbClient:  sbClient,
		vendorMgr: vendor.GetGlobalManager(),
		syncer:    NewDFlashSyncer(""),
	}
}

func (s *service) ListProducts(category, brand, search string, activeOnly bool) []*Product {
	category = strings.ToLower(strings.TrimSpace(category))
	brand = strings.ToLower(strings.TrimSpace(brand))
	search = strings.TrimSpace(search)

	var prods []*Product

	// If search query provided, search across all fields
	if search != "" {
		prods = s.repo.Search(search)
	} else if category != "" {
		// Filter by category if specified
		prods = s.repo.GetByCategory(category)
		if brand != "" {
			var filtered []*Product
			for _, p := range prods {
				if strings.ToLower(p.Brand) == brand {
					filtered = append(filtered, p)
				}
			}
			prods = filtered
		}
	} else if brand != "" {
		// Filter by brand if specified
		prods = s.repo.GetByBrand(brand)
	} else {
		// Return all cached products
		prods = s.repo.GetAll()
	}

	if !activeOnly {
		if prods == nil {
			return []*Product{}
		}
		return prods
	}

	// Filter products to display to users (p.IsActive && status != INACTIVE)
	// Produk dengan status GANGGUAN tetap tampil di user (kecuali status INACTIVE)
	// Serta sembunyikan SKU inquiry-only untuk produk pascabayar agar hanya tampil 1 produk bayar ke user
	activeProds := make([]*Product, 0)
	for _, p := range prods {
		if p.IsActive && !strings.EqualFold(p.Status, "INACTIVE") {
			if !IsPostpaidInquiryOnly(p, prods) {
				activeProds = append(activeProds, p)
			}
		}
	}
	return activeProds
}

// IsPostpaidInquiryOnly mengecek apakah produk pascabayar ini murni SKU pengecekan (inquiry-only)
// yang tidak boleh ditampilkan sebagai item terpisah di katalog pengguna.
func IsPostpaidInquiryOnly(p *Product, allProducts []*Product) bool {
	if !strings.EqualFold(p.Type, "POSTPAID") {
		return false
	}

	pCode := strings.ToUpper(strings.TrimSpace(p.Code))
	pBuyer := strings.ToUpper(strings.TrimSpace(p.BuyerSKUCode))

	for _, other := range allProducts {
		if other.ID == p.ID {
			continue
		}
		if !strings.EqualFold(other.Type, "POSTPAID") {
			continue
		}
		otherInq := strings.ToUpper(strings.TrimSpace(other.InquirySKUCode))
		if (pBuyer != "" && otherInq == pBuyer) || (pCode != "" && otherInq == pCode) {
			return true
		}
		if strings.HasPrefix(pBuyer, "C") && strings.HasPrefix(strings.ToUpper(other.BuyerSKUCode), "B") {
			if len(pBuyer) > 1 && pBuyer[1:] == strings.ToUpper(other.BuyerSKUCode)[1:] {
				return true
			}
		}
	}

	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(p.Name)), "CEK ") && p.Price <= 0 {
		for _, other := range allProducts {
			if other.ID != p.ID && strings.EqualFold(other.Type, "POSTPAID") && other.Price > 0 {
				return true
			}
		}
	}

	return false
}

func (s *service) GetProduct(code string) (*Product, error) {
	p, ok := s.repo.GetByCode(code)
	if !ok {
		return nil, errors.New("Produk tidak ditemukan atau kode produk tidak valid")
	}
	return p, nil
}

func (s *service) GetCategories(activeOnly bool) []string {
	var prods []*Product
	if activeOnly {
		prods = s.ListProducts("", "", "", true)
	} else {
		prods = s.repo.GetAll()
	}

	catMap := make(map[string]bool)
	for _, p := range prods {
		catMap[string(p.Category)] = true
	}

	categories := make([]string, 0)
	for c := range catMap {
		categories = append(categories, c)
	}
	sort.Strings(categories)
	return categories
}

func (s *service) GetBrands(category string, activeOnly bool) []string {
	category = strings.ToLower(strings.TrimSpace(category))
	var prods []*Product
	if activeOnly {
		prods = s.ListProducts(category, "", "", true)
	} else if category != "" {
		prods = s.repo.GetByCategory(category)
	} else {
		prods = s.repo.GetAll()
	}

	brandMap := make(map[string]bool)
	for _, p := range prods {
		if p.Brand != "" {
			brandMap[p.Brand] = true
		}
	}

	brands := make([]string, 0)
	for b := range brandMap {
		brands = append(brands, b)
	}
	sort.Strings(brands)
	return brands
}

// CreateProduct membuat produk baru dengan validasi kategori wajib & OOP vendor check
func (s *service) CreateProduct(req CreateProductRequest) (*Product, error) {
	// 1. Validasi Kategori Wajib
	categoryID := strings.TrimSpace(req.CategoryID)
	if categoryID == "" {
		return nil, errors.New("Kategori produk wajib diisi (category_id tidak boleh kosong). Produk belum bisa dibuat jika kategori tidak ada.")
	}

	// 2. Verifikasi keberadaan kategori di tabel app_categories Supabase
	var categoryName string
	var categorySlug string
	if s.sbClient != nil {
		catQuery := fmt.Sprintf("id=eq.%s&select=id,name,code,slug", categoryID)
		catBytes, err := s.sbClient.GetDB("app_categories", catQuery, "")
		if err != nil {
			return nil, fmt.Errorf("gagal memverifikasi kategori: %v", err)
		}

		var matchedCategories []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Code string `json:"code"`
			Slug string `json:"slug"`
		}
		if err := json.Unmarshal(catBytes, &matchedCategories); err != nil || len(matchedCategories) == 0 {
			return nil, fmt.Errorf("kategori dengan ID '%s' tidak ditemukan di database. Produk belum bisa dibuat jika belum ada kategori.", categoryID)
		}
		categoryName = matchedCategories[0].Name
		categorySlug = matchedCategories[0].Slug
	}

	// 3. Validasi Kode & Vendor SKU
	buyerSKU := strings.TrimSpace(req.BuyerSKUCode)
	code := strings.TrimSpace(req.Code)
	if code == "" && buyerSKU != "" {
		code = buyerSKU
	} else if buyerSKU == "" && code != "" {
		buyerSKU = code
	}
	if code == "" {
		return nil, errors.New("kode produk (code / buyer_sku_code) wajib diisi")
	}

	vendorCode := strings.ToUpper(strings.TrimSpace(req.VendorCode))
	if vendorCode == "" && strings.TrimSpace(req.Vendor) != "" {
		vendorCode = strings.ToUpper(strings.TrimSpace(req.Vendor))
	}
	if vendorCode == "" {
		vendorCode = "DFLASH"
	}

	// 4. Validasi OOP Vendor SKU:
	// Jika kode tidak sesuai dengan aturan nama vendor, jangan diteruskan / gagalkan!
	if err := s.vendorMgr.ValidateProductVendor(vendorCode, buyerSKU); err != nil {
		return nil, err
	}

	// 5. Cek duplikasi SKU di database khusus untuk vendor yang sama
	if s.sbClient != nil {
		checkQuery := fmt.Sprintf("vendor_code=eq.%s&or=(code.eq.%s,buyer_sku_code.eq.%s)&select=id,code,buyer_sku_code,name,vendor_code", vendorCode, strings.ToUpper(code), strings.ToUpper(buyerSKU))
		existingBytes, err := s.sbClient.GetDB("ppob_products", checkQuery, "")
		if err == nil && len(existingBytes) > 0 {
			var dupList []struct {
				ID         string `json:"id"`
				Code       string `json:"code"`
				BuyerSKU   string `json:"buyer_sku_code"`
				Name       string `json:"name"`
				VendorCode string `json:"vendor_code"`
			}
			if err := json.Unmarshal(existingBytes, &dupList); err == nil && len(dupList) > 0 {
				return nil, fmt.Errorf("kode SKU '%s' sudah terdaftar pada vendor %s (produk '%s')", buyerSKU, vendorCode, dupList[0].Name)
			}
		}
	}

	// Pastikan kolom DB 'code' unik jika kode dasar sudah dipakai oleh vendor berbeda
	finalCode := strings.ToUpper(code)
	if s.sbClient != nil {
		codeCheckQuery := fmt.Sprintf("code=eq.%s&select=id,vendor_code", finalCode)
		existingCodeBytes, err := s.sbClient.GetDB("ppob_products", codeCheckQuery, "")
		if err == nil && len(existingCodeBytes) > 0 {
			var codeDupList []struct {
				ID         string `json:"id"`
				VendorCode string `json:"vendor_code"`
			}
			if err := json.Unmarshal(existingCodeBytes, &codeDupList); err == nil && len(codeDupList) > 0 {
				finalCode = fmt.Sprintf("%s-%s", vendorCode, strings.ToUpper(buyerSKU))
			}
		}
	}

	if req.Name == "" {
		return nil, errors.New("nama produk (name) wajib diisi")
	}
	if req.Price < 0 {
		return nil, errors.New("harga jual (price) tidak boleh negatif")
	}

	brand := strings.TrimSpace(req.Brand)
	if brand == "" {
		brand = extractBrand(req.Name)
	}

	catTag := req.Category
	if catTag == "" {
		if categorySlug != "" {
			catTag = categorySlug
		} else {
			catTag = "pulsa"
		}
	}

	status := strings.ToUpper(strings.TrimSpace(req.Status))
	if status == "" {
		status = "ACTIVE"
	}

	prodType := strings.ToUpper(strings.TrimSpace(req.Type))
	if prodType == "" {
		prodType = "PREPAID"
	}

	ptsReward := int64(0)
	if req.PointsReward != nil {
		ptsReward = *req.PointsReward
	} else if req.Price > 0 {
		ptsReward = int64(req.Price * 0.01) // 1% cashback default
	}

	inqSKU := strings.TrimSpace(req.InquirySKUCode)
	if inqSKU == "" && prodType == "POSTPAID" {
		if strings.HasPrefix(strings.ToUpper(buyerSKU), "B") {
			inqSKU = "C" + buyerSKU[1:]
		} else {
			inqSKU = buyerSKU
		}
	}

	// 6. Simpan ke database Supabase (public.ppob_products)
	newProd := &Product{
		CategoryID:     categoryID,
		CategoryName:   categoryName,
		Category:       ProductCategory(strings.ToLower(catTag)),
		VendorCode:     vendorCode,
		BuyerSKUCode:   buyerSKU,
		InquirySKUCode: inqSKU,
		Code:           finalCode,
		Name:           req.Name,
		Brand:          brand,
		Type:           prodType,
		CostPrice:      req.CostPrice,
		OriginalPrice:  req.CostPrice,
		Price:          req.Price,
		PointsReward:   ptsReward,
		Status:         status,
		Description:    req.Description,
		Stock:          req.Stock,
		ImageURL:       req.ImageURL,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if s.sbClient != nil {
		insertPayload := map[string]interface{}{
			"category_id":      categoryID,
			"vendor_code":      vendorCode,
			"buyer_sku_code":   buyerSKU,
			"inquiry_sku_code": inqSKU,
			"code":             newProd.Code,
			"name":             newProd.Name,
			"type":             prodType,
			"category":         string(newProd.Category),
			"brand":          newProd.Brand,
			"cost_price":     newProd.CostPrice,
			"price":          newProd.Price,
			"points_reward":  newProd.PointsReward,
			"status":         newProd.Status,
			"description":    newProd.Description,
			"stock":          newProd.Stock,
			"image_url":      newProd.ImageURL,
			"is_active":      status == "ACTIVE",
			"created_at":     newProd.CreatedAt,
			"updated_at":     newProd.UpdatedAt,
		}

		respBytes, err := s.sbClient.PostDB("ppob_products", insertPayload, "", "")
		if err != nil {
			return nil, fmt.Errorf("gagal menyimpan produk ke database: %v", err)
		}

		var inserted []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(respBytes, &inserted); err == nil && len(inserted) > 0 {
			newProd.ID = inserted[0].ID
		}
	}

	// 6. Simpan ke memori cache agar instan dapat diakses API & Mobile
	s.repo.Save(newProd)
	if s.sbClient != nil {
		_, _ = s.repo.SyncFromSupabase(s.sbClient)
	}

	return newProd, nil
}

// UpdateProduct memperbarui data produk
func (s *service) UpdateProduct(id string, req UpdateProductRequest) (*Product, error) {
	if s.sbClient == nil {
		return nil, errors.New("database client tidak tersedia")
	}

	updatePayload := make(map[string]interface{})
	if req.CategoryID != nil {
		// Validasi kategori baru jika diubah
		catID := strings.TrimSpace(*req.CategoryID)
		if catID != "" {
			catQuery := fmt.Sprintf("id=eq.%s&select=id", catID)
			catBytes, err := s.sbClient.GetDB("app_categories", catQuery, "")
			if err != nil || len(catBytes) == 0 {
				return nil, fmt.Errorf("kategori baru ID '%s' tidak ditemukan", catID)
			}
		}
		updatePayload["category_id"] = catID
	}
	if req.VendorCode != nil && strings.TrimSpace(*req.VendorCode) != "" {
		updatePayload["vendor_code"] = strings.ToUpper(strings.TrimSpace(*req.VendorCode))
	} else if req.Vendor != nil && strings.TrimSpace(*req.Vendor) != "" {
		updatePayload["vendor_code"] = strings.ToUpper(strings.TrimSpace(*req.Vendor))
	}
	if req.BuyerSKUCode != nil {
		updatePayload["buyer_sku_code"] = *req.BuyerSKUCode
	}
	if req.InquirySKUCode != nil {
		updatePayload["inquiry_sku_code"] = strings.TrimSpace(*req.InquirySKUCode)
	}
	if req.Code != nil {
		updatePayload["code"] = strings.ToUpper(*req.Code)
	}
	if req.Name != nil {
		updatePayload["name"] = *req.Name
	}
	if req.Brand != nil {
		updatePayload["brand"] = *req.Brand
	}
	if req.Type != nil {
		prodType := strings.ToUpper(strings.TrimSpace(*req.Type))
		if prodType == "" {
			prodType = "PREPAID"
		}
		updatePayload["type"] = prodType
	}
	if req.CostPrice != nil {
		updatePayload["cost_price"] = *req.CostPrice
	}
	if req.Price != nil {
		updatePayload["price"] = *req.Price
	}
	if req.PointsReward != nil {
		updatePayload["points_reward"] = *req.PointsReward
	}
	if req.Status != nil {
		status := strings.ToUpper(strings.TrimSpace(*req.Status))
		updatePayload["status"] = status
		updatePayload["is_active"] = status != "INACTIVE"
	}
	if req.Description != nil {
		updatePayload["description"] = *req.Description
	}
	if req.Stock != nil {
		updatePayload["stock"] = *req.Stock
	}
	if req.ImageURL != nil {
		updatePayload["image_url"] = *req.ImageURL
	}
	updatePayload["updated_at"] = time.Now()

	// Cari produk yang ada untuk menentukan ID dan Code
	existing, _ := s.GetProduct(id)
	targetID := id
	targetCode := id
	if existing != nil {
		if existing.ID != "" {
			targetID = existing.ID
		}
		if existing.Code != "" {
			targetCode = existing.Code
		}
	}

	filter := fmt.Sprintf("id=eq.%s", targetID)
	if existing != nil && existing.ID != "" {
		filter = fmt.Sprintf("id=eq.%s", existing.ID)
	} else if !strings.Contains(id, "-") {
		filter = fmt.Sprintf("code=eq.%s", strings.ToUpper(id))
	}

	_, err := s.sbClient.PatchDB("ppob_products", filter, updatePayload, "")
	if err != nil {
		return nil, fmt.Errorf("gagal mengupdate produk: %v", err)
	}

	// Resync cache
	_, _ = s.repo.SyncFromSupabase(s.sbClient)

	// Ambil detail produk terbaru
	updated, err := s.GetProduct(targetCode)
	if err != nil {
		updated, err = s.GetProduct(targetID)
	}
	if err != nil && existing != nil {
		if req.Price != nil {
			existing.Price = *req.Price
		}
		if req.Status != nil {
			existing.Status = strings.ToUpper(*req.Status)
		}
		if req.Name != nil {
			existing.Name = *req.Name
		}
		if req.Type != nil {
			existing.Type = strings.ToUpper(strings.TrimSpace(*req.Type))
		}
		if req.InquirySKUCode != nil {
			existing.InquirySKUCode = strings.TrimSpace(*req.InquirySKUCode)
		}
		s.repo.Save(existing)
		return existing, nil
	}
	return updated, err
}

// DeleteProduct menghapus produk
func (s *service) DeleteProduct(id string) error {
	if s.sbClient == nil {
		return errors.New("database client tidak tersedia")
	}

	existing, _ := s.GetProduct(id)
	filter := fmt.Sprintf("id=eq.%s", id)
	if existing != nil && existing.ID != "" {
		filter = fmt.Sprintf("id=eq.%s", existing.ID)
	} else if !strings.Contains(id, "-") {
		filter = fmt.Sprintf("code=eq.%s", strings.ToUpper(id))
	}

	err := s.sbClient.DeleteDB("ppob_products", filter, "")
	if err != nil {
		return fmt.Errorf("gagal menghapus produk: %v", err)
	}

	// Resync cache
	_, _ = s.repo.SyncFromSupabase(s.sbClient)
	return nil
}

func (s *service) SyncProducts(adminID string) (*SyncResult, error) {
	return s.syncer.SyncCatalog(s.sbClient, s.repo)
}

func (s *service) StartDFlashCron(ctx context.Context, interval time.Duration) {
	s.syncer.StartAutoCronJob(ctx, s.sbClient, s.repo, interval)
}

func (s *service) GetCronConfig() CronConfig {
	return s.syncer.GetCronConfig()
}

func (s *service) UpdateCronConfig(intervalMinutes int, isEnabled bool) (*CronConfig, error) {
	return s.syncer.UpdateCronSettings(s.sbClient, intervalMinutes, isEnabled)
}

func (s *service) GetSyncLogs(limit int) []SyncLogEntry {
	return s.syncer.GetSyncLogs(s.sbClient, limit)
}
