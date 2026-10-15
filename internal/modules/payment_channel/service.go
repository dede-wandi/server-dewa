package payment_channel

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dede-wandi/server-dewa/internal/supabase"
)

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Service defines payment channel operations
type Service interface {
	GetActiveChannels() ([]PaymentChannel, error)
	GetAllChannelsAdmin() ([]PaymentChannel, error)
	GetChannelByID(id string) (*PaymentChannel, error)
	CreateChannel(req CreatePaymentChannelRequest, token string) (*PaymentChannel, error)
	UpdateChannel(id string, req UpdatePaymentChannelRequest, token string) (*PaymentChannel, error)
	DeleteChannel(id string, token string) error
}

type paymentChannelService struct {
	sbClient *supabase.Client
	mu       sync.RWMutex
	store    []PaymentChannel
}

func defaultChannels() []PaymentChannel {
	now := time.Now()
	return []PaymentChannel{
		{
			ID:            "c0000000-0000-0000-0000-000000000001",
			Code:          "fund_credit",
			Name:          "Fund Credit (Transfer Manual Admin)",
			ChannelType:   TypeManual,
			Vendor:        "manual",
			AccountNumber: "8801 2345 6789",
			AccountName:   "PT DEWA CELLULER INDONESIA",
			IconName:      "wallet-outline",
			AdminFee:      0,
			MinAmount:     10000,
			MaxAmount:     10000000,
			Status:        StatusActive,
			IsPG:          false,
			Badge:         "AKTIF",
			Instructions:  "Lakukan transfer sesuai nominal ke rekening resmi BCA PT DEWA CELL, lalu klik tombol konfirmasi. Admin akan langsung memproses saldo via fund_credit.",
			SortOrder:     1,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			ID:           "c0000000-0000-0000-0000-000000000002",
			Code:         "qris_duitku",
			Name:         "QRIS Duitku",
			ChannelType:  TypeQRIS,
			Vendor:       "duitku",
			IconName:     "qr-code-outline",
			AdminFee:     0,
			MinAmount:    10000,
			MaxAmount:    5000000,
			Status:       StatusComingSoon,
			IsPG:         true,
			Badge:        "Coming Soon",
			Instructions: "Scan QRIS Duitku otomatis menggunakan GoPay, OVO, Dana, LinkAja, ShopeePay atau Mobile Banking.",
			SortOrder:    2,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "c0000000-0000-0000-0000-000000000003",
			Code:         "qris_dana",
			Name:         "QRIS DANA Bisnis",
			ChannelType:  TypeQRIS,
			Vendor:       "dana",
			IconName:     "qr-code-outline",
			AdminFee:     0,
			MinAmount:    10000,
			MaxAmount:    5000000,
			Status:       StatusComingSoon,
			IsPG:         true,
			Badge:        "Coming Soon",
			Instructions: "Scan QRIS langsung terhubung dengan merchant resmi DANA Bisnis.",
			SortOrder:    3,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "c0000000-0000-0000-0000-000000000004",
			Code:         "bca_va_duitku",
			Name:         "BCA Virtual Account (Duitku)",
			ChannelType:  TypeVirtualAccount,
			Vendor:       "duitku",
			IconName:     "card-outline",
			AdminFee:     0,
			MinAmount:    10000,
			MaxAmount:    10000000,
			Status:       StatusComingSoon,
			IsPG:         true,
			Badge:        "Coming Soon",
			Instructions: "Transfer melalui BCA Virtual Account otomatis tanpa perlu konfirmasi manual.",
			SortOrder:    4,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "c0000000-0000-0000-0000-000000000005",
			Code:         "bni_va_duitku",
			Name:         "BNI Virtual Account (Duitku)",
			ChannelType:  TypeVirtualAccount,
			Vendor:       "duitku",
			IconName:     "card-outline",
			AdminFee:     0,
			MinAmount:    10000,
			MaxAmount:    10000000,
			Status:       StatusComingSoon,
			IsPG:         true,
			Badge:        "Coming Soon",
			Instructions: "Transfer melalui BNI Virtual Account otomatis tanpa konfirmasi.",
			SortOrder:    5,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "c0000000-0000-0000-0000-000000000006",
			Code:         "mandiri_va_duitku",
			Name:         "Mandiri Livin VA (Duitku)",
			ChannelType:  TypeVirtualAccount,
			Vendor:       "duitku",
			IconName:     "card-outline",
			AdminFee:     0,
			MinAmount:    10000,
			MaxAmount:    10000000,
			Status:       StatusComingSoon,
			IsPG:         true,
			Badge:        "Coming Soon",
			Instructions: "Transfer melalui Mandiri Virtual Account otomatis.",
			SortOrder:    6,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "c0000000-0000-0000-0000-000000000007",
			Code:         "bri_va_duitku",
			Name:         "BRI BRIVA (Duitku)",
			ChannelType:  TypeVirtualAccount,
			Vendor:       "duitku",
			IconName:     "card-outline",
			AdminFee:     0,
			MinAmount:    10000,
			MaxAmount:    10000000,
			Status:       StatusComingSoon,
			IsPG:         true,
			Badge:        "Coming Soon",
			Instructions: "Transfer melalui BRI Virtual Account (BRIVA) otomatis.",
			SortOrder:    7,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
}

// NewService creates a new Payment Channel Service
func NewService(sbClient *supabase.Client) Service {
	return &paymentChannelService{
		sbClient: sbClient,
		store:    defaultChannels(),
	}
}

func (s *paymentChannelService) GetActiveChannels() ([]PaymentChannel, error) {
	// Try fetching from Supabase DB
	if s.sbClient != nil {
		query := "status=in.(ACTIVE,COMING_SOON)&order=sort_order.asc"
		if respBytes, err := s.sbClient.GetDB("payment_channels", query, ""); err == nil {
			var list []PaymentChannel
			if err := json.Unmarshal(respBytes, &list); err == nil && len(list) > 0 {
				return list, nil
			}
		}
	}

	// Fallback in-memory
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]PaymentChannel, 0)
	for _, ch := range s.store {
		if ch.Status == StatusActive || ch.Status == StatusComingSoon {
			res = append(res, ch)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].SortOrder < res[j].SortOrder
	})
	return res, nil
}

func (s *paymentChannelService) GetAllChannelsAdmin() ([]PaymentChannel, error) {
	// Try fetching from Supabase DB
	if s.sbClient != nil {
		query := "order=sort_order.asc"
		if respBytes, err := s.sbClient.GetDB("payment_channels", query, ""); err == nil {
			var list []PaymentChannel
			if err := json.Unmarshal(respBytes, &list); err == nil && len(list) > 0 {
				return list, nil
			}
		}
	}

	// Fallback in-memory
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]PaymentChannel, len(s.store))
	copy(res, s.store)
	sort.Slice(res, func(i, j int) bool {
		return res[i].SortOrder < res[j].SortOrder
	})
	return res, nil
}

func (s *paymentChannelService) GetChannelByID(id string) (*PaymentChannel, error) {
	if s.sbClient != nil {
		query := "id=eq." + id
		if respBytes, err := s.sbClient.GetDB("payment_channels", query, ""); err == nil {
			var list []PaymentChannel
			if err := json.Unmarshal(respBytes, &list); err == nil && len(list) > 0 {
				return &list[0], nil
			}
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ch := range s.store {
		if ch.ID == id || ch.Code == id {
			return &ch, nil
		}
	}
	return nil, errors.New("Channel pembayaran tidak ditemukan")
}

func (s *paymentChannelService) CreateChannel(req CreatePaymentChannelRequest, token string) (*PaymentChannel, error) {
	req.Code = strings.TrimSpace(strings.ToLower(req.Code))
	if req.Code == "" {
		return nil, errors.New("Kode channel wajib diisi (contoh: qris_duitku)")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, errors.New("Nama channel pembayaran wajib diisi")
	}
	if req.ChannelType == "" {
		req.ChannelType = TypeManual
	}
	if req.Vendor == "" {
		req.Vendor = "manual"
	}
	if req.Status == "" {
		req.Status = StatusComingSoon
	}
	if req.MinAmount <= 0 {
		req.MinAmount = 10000
	}
	if req.MaxAmount <= 0 {
		req.MaxAmount = 10000000
	}

	isPG := false
	if req.IsPG != nil {
		isPG = *req.IsPG
	} else if req.Vendor != "manual" && req.ChannelType != TypeManual {
		isPG = true
	}

	badge := req.Badge
	if badge == "" {
		if req.Status == StatusActive {
			badge = "AKTIF"
		} else if req.Status == StatusComingSoon {
			badge = "Coming Soon"
		}
	}

	iconName := req.IconName
	if iconName == "" {
		if req.ChannelType == TypeQRIS {
			iconName = "qr-code-outline"
		} else if req.ChannelType == TypeVirtualAccount {
			iconName = "card-outline"
		} else {
			iconName = "wallet-outline"
		}
	}

	now := time.Now()
	newCh := PaymentChannel{
		ID:              newUUID(),
		Code:            req.Code,
		Name:            req.Name,
		ChannelType:     req.ChannelType,
		Vendor:          req.Vendor,
		AccountNumber:   req.AccountNumber,
		AccountName:     req.AccountName,
		IconName:        iconName,
		IconURL:         req.IconURL,
		AdminFee:        req.AdminFee,
		AdminFeePercent: req.AdminFeePercent,
		MinAmount:       req.MinAmount,
		MaxAmount:       req.MaxAmount,
		Status:          req.Status,
		IsPG:            isPG,
		Badge:           badge,
		Instructions:    req.Instructions,
		SortOrder:       req.SortOrder,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// Persist to Supabase DB
	if s.sbClient != nil {
		payload := map[string]interface{}{
			"id":                newCh.ID,
			"code":              newCh.Code,
			"name":              newCh.Name,
			"channel_type":      newCh.ChannelType,
			"vendor":            newCh.Vendor,
			"account_number":    newCh.AccountNumber,
			"account_name":      newCh.AccountName,
			"icon_name":         newCh.IconName,
			"icon_url":          newCh.IconURL,
			"admin_fee":         newCh.AdminFee,
			"admin_fee_percent": newCh.AdminFeePercent,
			"min_amount":        newCh.MinAmount,
			"max_amount":        newCh.MaxAmount,
			"status":            newCh.Status,
			"is_pg":             newCh.IsPG,
			"badge":             newCh.Badge,
			"instructions":      newCh.Instructions,
			"sort_order":        newCh.SortOrder,
		}
		_, _ = s.sbClient.PostDB("payment_channels", payload, "", token)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = append(s.store, newCh)

	return &newCh, nil
}

func (s *paymentChannelService) UpdateChannel(id string, req UpdatePaymentChannelRequest, token string) (*PaymentChannel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var target *PaymentChannel
	for i := range s.store {
		if s.store[i].ID == id || s.store[i].Code == id {
			target = &s.store[i]
			break
		}
	}

	if target == nil {
		return nil, errors.New("Channel pembayaran tidak ditemukan")
	}

	patchPayload := make(map[string]interface{})

	if req.Name != nil {
		target.Name = *req.Name
		patchPayload["name"] = target.Name
	}
	if req.ChannelType != nil {
		target.ChannelType = *req.ChannelType
		patchPayload["channel_type"] = target.ChannelType
	}
	if req.Vendor != nil {
		target.Vendor = *req.Vendor
		patchPayload["vendor"] = target.Vendor
	}
	if req.AccountNumber != nil {
		target.AccountNumber = *req.AccountNumber
		patchPayload["account_number"] = target.AccountNumber
	}
	if req.AccountName != nil {
		target.AccountName = *req.AccountName
		patchPayload["account_name"] = target.AccountName
	}
	if req.IconName != nil {
		target.IconName = *req.IconName
		patchPayload["icon_name"] = target.IconName
	}
	if req.IconURL != nil {
		target.IconURL = *req.IconURL
		patchPayload["icon_url"] = target.IconURL
	}
	if req.AdminFee != nil {
		target.AdminFee = *req.AdminFee
		patchPayload["admin_fee"] = target.AdminFee
	}
	if req.AdminFeePercent != nil {
		target.AdminFeePercent = *req.AdminFeePercent
		patchPayload["admin_fee_percent"] = target.AdminFeePercent
	}
	if req.MinAmount != nil {
		target.MinAmount = *req.MinAmount
		patchPayload["min_amount"] = target.MinAmount
	}
	if req.MaxAmount != nil {
		target.MaxAmount = *req.MaxAmount
		patchPayload["max_amount"] = target.MaxAmount
	}
	if req.Status != nil {
		target.Status = *req.Status
		patchPayload["status"] = target.Status
	}
	if req.IsPG != nil {
		target.IsPG = *req.IsPG
		patchPayload["is_pg"] = target.IsPG
	}
	if req.Badge != nil {
		target.Badge = *req.Badge
		patchPayload["badge"] = target.Badge
	}
	if req.Instructions != nil {
		target.Instructions = *req.Instructions
		patchPayload["instructions"] = target.Instructions
	}
	if req.SortOrder != nil {
		target.SortOrder = *req.SortOrder
		patchPayload["sort_order"] = target.SortOrder
	}

	target.UpdatedAt = time.Now()
	patchPayload["updated_at"] = target.UpdatedAt

	if s.sbClient != nil && len(patchPayload) > 0 {
		query := "id=eq." + target.ID
		_, _ = s.sbClient.PatchDB("payment_channels", query, patchPayload, token)
	}

	res := *target
	return &res, nil
}

func (s *paymentChannelService) DeleteChannel(id string, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	foundIndex := -1
	for i := range s.store {
		if s.store[i].ID == id || s.store[i].Code == id {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		return errors.New("Channel pembayaran tidak ditemukan")
	}

	targetID := s.store[foundIndex].ID
	s.store = append(s.store[:foundIndex], s.store[foundIndex+1:]...)

	if s.sbClient != nil {
		query := "id=eq." + targetID
		_ = s.sbClient.DeleteDB("payment_channels", query, token)
	}

	return nil
}
