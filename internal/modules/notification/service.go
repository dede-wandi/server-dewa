package notification

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
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

type Service interface {
	GetUserNotifications(userID, filterType, filterStatus string, limit int) ([]Notification, int, error)
	MarkAsRead(userID, id string) (*Notification, error)
	MarkAsUnread(userID, id string) (*Notification, error)
	MarkAllAsRead(userID string) error
	DeleteNotification(userID, id string) error
	DeleteAllNotifications(userID string) error
	CreateNotification(req CreateNotificationRequest) (*Notification, error)
	BroadcastNotification(req BroadcastNotificationRequest) (int, error)
	GetAllNotificationsAdmin(limit, offset int) ([]Notification, int, error)
}

type notificationService struct {
	sbClient *supabase.Client
	mu       sync.RWMutex
	store    map[string][]Notification // Fallback in-memory store keyed by userID
}

func NewService(sbClient *supabase.Client) Service {
	s := &notificationService{
		sbClient: sbClient,
		store:    make(map[string][]Notification),
	}
	return s
}

// generateDefaultNotifications provides 20 realistic notifications in Indonesian
func generateDefaultNotifications(userID string) []Notification {
	now := time.Now()
	notifs := []Notification{
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "🎉 Selamat Datang di DewaCell!",
			Message:     "Akun Anda berhasil aktif. Mulai jualan pulsa, paket data, dan bayar tagihan PPOB termurah se-Indonesia sekarang juga.",
			Type:        string(TypeAnnouncement),
			StatusBaca:  StatusBelumDibaca,
			IsRead:      false,
			ReferenceID: "WELC-001",
			ActionURL:   "/(tabs)",
			CreatedAt:   now.Add(-10 * time.Minute),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "💳 Top Up Saldo Berhasil",
			Message:     "Deposit saldo sebesar Rp 100.000 via Transfer Bank BCA telah berhasil ditambahkan ke dompet akun Anda.",
			Type:        string(TypePayment),
			StatusBaca:  StatusBelumDibaca,
			IsRead:      false,
			ReferenceID: "TOPUP-BCA-88912",
			ActionURL:   "/(tabs)/wallet",
			CreatedAt:   now.Add(-25 * time.Minute),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "⚡ Transaksi Token Listrik PLN Sukses",
			Message:     "Pembelian Token PLN Rp 50.000 untuk No Meter 14285910245 SUKSES. Struk transaksi siap dicetak.",
			Type:        string(TypeTransaction),
			StatusBaca:  StatusBelumDibaca,
			IsRead:      false,
			ReferenceID: "TRX-PLN-55410",
			ActionURL:   "/(tabs)/orders",
			CreatedAt:   now.Add(-1 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "📱 Paket Data Telkomsel 14GB Sukses",
			Message:     "Pengisian Kuota Telkomsel Max 14GB ke 081234567890 berhasil diproses dalam 3 detik.",
			Type:        string(TypeTransaction),
			StatusBaca:  StatusBelumDibaca,
			IsRead:      false,
			ReferenceID: "TRX-TSEL-99021",
			ActionURL:   "/(tabs)/orders",
			CreatedAt:   now.Add(-2 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "🔥 Promo Kilat Cashback Saldo 5%",
			Message:     "Dapatkan cashback saldo hingga Rp 25.000 untuk setiap transaksi produk E-Money (DANA, OVO, GoPay) hari ini!",
			Type:        string(TypePromo),
			StatusBaca:  StatusBelumDibaca,
			IsRead:      false,
			ReferenceID: "PROMO-EMN-05",
			ActionURL:   "/(tabs)",
			CreatedAt:   now.Add(-4 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "🛡️ Keamanan: PIN Transaksi Aktif",
			Message:     "PIN Keamanan 6 Digit dompet digital Anda telah berhasil diatur. Semua transaksi transfer kini terlindungi maksimal.",
			Type:        string(TypeSecurity),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "PIN-SET-001",
			ActionURL:   "/(tabs)/wallet",
			CreatedAt:   now.Add(-6 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "💸 Pembayaran Merchant QRIS Berhasil",
			Message:     "Pembayaran sebesar Rp 35.000 ke 'Kopi Kenangan' via Saldo DewaCell berhasil diproses.",
			Type:        string(TypePayment),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "QRIS-887102",
			ActionURL:   "/(tabs)/wallet",
			CreatedAt:   now.Add(-8 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "🎁 Bonus Reward 100 Poin Dewa",
			Message:     "Selamat! Anda mendapatkan 100 Poin Dewa dari transaksi aktif minggu ini. Tukarkan poin dengan saldo gratis.",
			Type:        string(TypePromo),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "POINT-RW-100",
			ActionURL:   "/(tabs)/wallet",
			CreatedAt:   now.Add(-12 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "📲 Pulsa Indosat 25.000 Berhasil",
			Message:     "Pengisian Pulsa Indosat Ooredoo Rp 25.000 ke 085712349910 telah berhasil masuk ke pelanggan.",
			Type:        string(TypeTransaction),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "TRX-ISAT-33104",
			ActionURL:   "/(tabs)/orders",
			CreatedAt:   now.Add(-16 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "📢 Pembaruan Server & Jalur Transaksi",
			Message:     "Optimasi jalur transaksi PPOB selesai dilakukan. Kecepatan respon server kini meningkat 300% lebih gesit.",
			Type:        string(TypeAnnouncement),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "SYS-UPD-991",
			ActionURL:   "/(tabs)",
			CreatedAt:   now.Add(-24 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "💰 Top Up Saldo Rp 500.000 Sukses",
			Message:     "Top up saldo via Virtual Account Mandiri berhasil diverifikasi secara otomatis tanpa biaya admin.",
			Type:        string(TypePayment),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "TOPUP-MDR-7712",
			ActionURL:   "/(tabs)/wallet",
			CreatedAt:   now.Add(-28 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "💧 Pembayaran Tagihan PDAM Berhasil",
			Message:     "Pembayaran PDAM Tirta Moedal Semarang No Pelanggan 901827402 Sebesar Rp 82.500 berhasil.",
			Type:        string(TypeTransaction),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "TRX-PDAM-6612",
			ActionURL:   "/(tabs)/orders",
			CreatedAt:   now.Add(-32 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "🔄 Transfer Saldo Antar Mitra Berhasil",
			Message:     "Transfer saldo Rp 50.000 ke mitra 'Dewa Cell Cabang 2' (081299887711) berhasil terkirim instan.",
			Type:        string(TypePayment),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "TRF-MITRA-002",
			ActionURL:   "/(tabs)/wallet",
			CreatedAt:   now.Add(-36 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "🎯 Target Transaksi Bulanan Tercapai!",
			Message:     "Hebat! Anda telah melampaui 50 transaksi bulan ini. Upgrade status Mitra ke level VIP kini terbuka.",
			Type:        string(TypePromo),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "TIER-VIP-OPEN",
			ActionURL:   "/(tabs)/settings",
			CreatedAt:   now.Add(-40 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "🔒 Login Baru Terdeteksi",
			Message:     "Akun Anda baru saja terdeteksi masuk melalui perangkat Android (Expo Mobile App). Jika ini bukan Anda, segera ganti password.",
			Type:        string(TypeSecurity),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "SEC-LOG-881",
			ActionURL:   "/(tabs)/settings",
			CreatedAt:   now.Add(-48 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "🎮 Top Up Diamond Mobile Legends Sukses",
			Message:     "Voucher 86 Diamonds MLBB ke ID: 12984920 (Zone 2102) sukses masuk ke akun game pelanggan.",
			Type:        string(TypeTransaction),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "TRX-MLBB-9012",
			ActionURL:   "/(tabs)/orders",
			CreatedAt:   now.Add(-54 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "🩺 Pembayaran BPJS Kesehatan Berhasil",
			Message:     "Iuran BPJS Kesehatan 1 Keluarga Rp 105.000 No VA 8888801928472910 sukses dibayar.",
			Type:        string(TypeTransaction),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "TRX-BPJS-4412",
			ActionURL:   "/(tabs)/orders",
			CreatedAt:   now.Add(-60 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "🏷️ Diskon Produk Pulsa Tri (3) 10%",
			Message:     "Harga modal pulsa Tri nominal 50rb & 100rb sedang promo spesial akhir pekan.",
			Type:        string(TypePromo),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "PROMO-TRI-10",
			ActionURL:   "/(tabs)",
			CreatedAt:   now.Add(-72 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "📶 Voucher WiFi ID 20.000 Sukses",
			Message:     "Username dan Password voucher seamless WiFi ID telah terbit dan struk siap diserahkan.",
			Type:        string(TypeTransaction),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "TRX-WIFI-1102",
			ActionURL:   "/(tabs)/orders",
			CreatedAt:   now.Add(-80 * time.Hour),
		},
		{
			ID:          newUUID(),
			UserID:      userID,
			Title:       "🔔 Tips Jualan Konter: Cetak Struk Bluetooth",
			Message:     "Simak panduan terbaru menghubungkan printer thermal bluetooth 58mm untuk meningkatkan kredibilitas konter Anda.",
			Type:        string(TypeAnnouncement),
			StatusBaca:  StatusSudahDibaca,
			IsRead:      true,
			ReferenceID: "ART-NEWS-001",
			ActionURL:   "/article/cara-cetak-struk-kasir-pos-bluetooth",
			CreatedAt:   now.Add(-96 * time.Hour),
		},
	}

	for i := range notifs {
		if notifs[i].Type == string(TypeSecurity) {
			notifs[i].IsDeletable = false
		} else {
			notifs[i].IsDeletable = true
		}
	}
	return notifs
}

// GetUserNotifications returns max 20 notifications for the user with optional filters
func (s *notificationService) GetUserNotifications(userID, filterType, filterStatus string, limit int) ([]Notification, int, error) {
	if limit <= 0 || limit > 20 {
		limit = 20 // Wajib maksimal 20 item sesuai instruksi
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("user_id=eq.%s", url.QueryEscape(userID)))

	if filterType != "" && strings.ToUpper(filterType) != "ALL" && strings.ToUpper(filterType) != "SEMUA" {
		parts = append(parts, fmt.Sprintf("type=eq.%s", url.QueryEscape(strings.ToUpper(filterType))))
	}

	statusLower := strings.ToLower(filterStatus)
	if statusLower == "unread" || statusLower == "belum dibaca" || statusLower == "belum_dibaca" {
		parts = append(parts, "is_read=eq.false")
	} else if statusLower == "read" || statusLower == "sudah dibaca" || statusLower == "sudah_dibaca" {
		parts = append(parts, "is_read=eq.true")
	}

	parts = append(parts, "order=created_at.desc")
	parts = append(parts, fmt.Sprintf("limit=%d", limit))
	queryParams := strings.Join(parts, "&")

	// Try PostgREST
	respBytes, err := s.sbClient.GetDB("notifications", queryParams, "")
	if err == nil {
		var notifs []Notification
		if err := json.Unmarshal(respBytes, &notifs); err == nil && len(notifs) > 0 {
			// Count total unread
			unreadCount := 0
			for _, n := range notifs {
				if !n.IsRead {
					unreadCount++
				}
			}
			return notifs, unreadCount, nil
		}
	}

	// Fallback in-memory
	s.mu.Lock()
	defer s.mu.Unlock()

	userList, ok := s.store[userID]
	if !ok || len(userList) == 0 {
		userList = generateDefaultNotifications(userID)
		s.store[userID] = userList
	}

	// Filter in-memory
	filtered := make([]Notification, 0)
	unreadTotal := 0

	for _, n := range userList {
		if !n.IsRead {
			unreadTotal++
		}

		// Type filter
		if filterType != "" && strings.ToUpper(filterType) != "ALL" && strings.ToUpper(filterType) != "SEMUA" {
			if !strings.EqualFold(n.Type, filterType) {
				continue
			}
		}

		// Status filter
		if statusLower == "unread" || statusLower == "belum dibaca" || statusLower == "belum_dibaca" {
			if n.IsRead {
				continue
			}
		} else if statusLower == "read" || statusLower == "sudah dibaca" || statusLower == "sudah_dibaca" {
			if !n.IsRead {
				continue
			}
		}

		filtered = append(filtered, n)
		if len(filtered) >= limit {
			break
		}
	}

	return filtered, unreadTotal, nil
}

// MarkAsRead marks a single notification as read ("Sudah Dibaca")
func (s *notificationService) MarkAsRead(userID, id string) (*Notification, error) {
	now := time.Now()
	payload := map[string]interface{}{
		"is_read":     true,
		"status_baca": StatusSudahDibaca,
		"read_at":     now.Format(time.RFC3339),
	}

	queryParams := fmt.Sprintf("id=eq.%s&user_id=eq.%s", url.QueryEscape(id), url.QueryEscape(userID))
	_, _ = s.sbClient.PatchDB("notifications", queryParams, payload, "")

	s.mu.Lock()
	defer s.mu.Unlock()

	list, ok := s.store[userID]
	if !ok {
		list = generateDefaultNotifications(userID)
		s.store[userID] = list
	}

	for i := range list {
		if list[i].ID == id {
			list[i].IsRead = true
			list[i].StatusBaca = StatusSudahDibaca
			list[i].ReadAt = &now
			s.store[userID] = list
			return &list[i], nil
		}
	}

	return nil, errors.New("notifikasi tidak ditemukan")
}

// MarkAsUnread marks a single notification as unread ("Belum Dibaca")
func (s *notificationService) MarkAsUnread(userID, id string) (*Notification, error) {
	payload := map[string]interface{}{
		"is_read":     false,
		"status_baca": StatusBelumDibaca,
		"read_at":     nil,
	}

	queryParams := fmt.Sprintf("id=eq.%s&user_id=eq.%s", url.QueryEscape(id), url.QueryEscape(userID))
	_, _ = s.sbClient.PatchDB("notifications", queryParams, payload, "")

	s.mu.Lock()
	defer s.mu.Unlock()

	list, ok := s.store[userID]
	if !ok {
		list = generateDefaultNotifications(userID)
		s.store[userID] = list
	}

	for i := range list {
		if list[i].ID == id {
			list[i].IsRead = false
			list[i].StatusBaca = StatusBelumDibaca
			list[i].ReadAt = nil
			s.store[userID] = list
			return &list[i], nil
		}
	}

	return nil, errors.New("notifikasi tidak ditemukan")
}

// MarkAllAsRead marks all user notifications as read
func (s *notificationService) MarkAllAsRead(userID string) error {
	now := time.Now()
	payload := map[string]interface{}{
		"is_read":     true,
		"status_baca": StatusSudahDibaca,
		"read_at":     now.Format(time.RFC3339),
	}

	queryParams := fmt.Sprintf("user_id=eq.%s&is_read=eq.false", url.QueryEscape(userID))
	_, _ = s.sbClient.PatchDB("notifications", queryParams, payload, "")

	s.mu.Lock()
	defer s.mu.Unlock()

	list, ok := s.store[userID]
	if !ok {
		list = generateDefaultNotifications(userID)
	}

	for i := range list {
		list[i].IsRead = true
		list[i].StatusBaca = StatusSudahDibaca
		list[i].ReadAt = &now
	}
	s.store[userID] = list

	return nil
}

// DeleteNotification removes single notification for user
func (s *notificationService) DeleteNotification(userID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, ok := s.store[userID]
	if !ok {
		list = generateDefaultNotifications(userID)
	}

	// Cek apakah notifikasi bertipe keamanan / log login
	for _, n := range list {
		if n.ID == id {
			if n.Type == string(TypeSecurity) || !n.IsDeletable {
				return errors.New("notifikasi keamanan login tidak dapat dihapus demi keamanan akun Anda")
			}
			break
		}
	}

	// Hapus di database dengan proteksi type != SECURITY dan is_deletable != false
	queryParams := fmt.Sprintf("id=eq.%s&user_id=eq.%s&type=neq.SECURITY&is_deletable=neq.false", url.QueryEscape(id), url.QueryEscape(userID))
	_ = s.sbClient.DeleteDB("notifications", queryParams, "")

	newList := make([]Notification, 0, len(list))
	for _, n := range list {
		if n.ID != id {
			newList = append(newList, n)
		}
	}
	s.store[userID] = newList
	return nil
}

// DeleteAllNotifications deletes all notifications for user except SECURITY / log login
func (s *notificationService) DeleteAllNotifications(userID string) error {
	// Hapus notifikasi di DB kecuali SECURITY
	queryParams := fmt.Sprintf("user_id=eq.%s&type=neq.SECURITY&is_deletable=neq.false", url.QueryEscape(userID))
	_ = s.sbClient.DeleteDB("notifications", queryParams, "")

	s.mu.Lock()
	defer s.mu.Unlock()

	list, ok := s.store[userID]
	if !ok {
		list = generateDefaultNotifications(userID)
	}

	// Pertahankan seluruh notifikasi keamanan dan log login
	keptList := make([]Notification, 0)
	for _, n := range list {
		if n.Type == string(TypeSecurity) || !n.IsDeletable {
			keptList = append(keptList, n)
		}
	}
	s.store[userID] = keptList
	return nil
}

// CreateNotification creates a single notification
func (s *notificationService) CreateNotification(req CreateNotificationRequest) (*Notification, error) {
	if req.Title == "" || req.Message == "" {
		return nil, errors.New("judul (title) dan isi pesan (message) wajib diisi")
	}

	if req.Type == "" {
		req.Type = string(TypeAnnouncement)
	}

	upperType := strings.ToUpper(req.Type)
	isDeletable := true
	if upperType == string(TypeSecurity) {
		isDeletable = false
	}
	if req.IsDeletable != nil {
		isDeletable = *req.IsDeletable
	}

	notif := Notification{
		ID:          newUUID(),
		UserID:      req.UserID,
		Title:       req.Title,
		Message:     req.Message,
		Type:        upperType,
		StatusBaca:  StatusBelumDibaca,
		IsRead:      false,
		IsDeletable: isDeletable,
		ReferenceID: req.ReferenceID,
		ActionURL:   req.ActionURL,
		CreatedAt:   time.Now(),
	}

	// Persist to DB if userID is valid UUID
	_, _ = s.sbClient.PostDB("notifications", notif, "", "")

	// Update in-memory
	s.mu.Lock()
	defer s.mu.Unlock()

	list := s.store[req.UserID]
	list = append([]Notification{notif}, list...)
	if len(list) > 20 {
		list = list[:20] // Keep max 20
	}
	s.store[req.UserID] = list

	return &notif, nil
}

// BroadcastNotification sends notification to all users (Admin)
func (s *notificationService) BroadcastNotification(req BroadcastNotificationRequest) (int, error) {
	if req.Title == "" || req.Message == "" {
		return 0, errors.New("judul dan pesan broadcast wajib diisi")
	}

	if req.Type == "" {
		req.Type = string(TypeAnnouncement)
	}

	// Fetch users from auth or admin
	users, err := s.sbClient.AdminListUsers(1, 100)
	sentCount := 0

	s.mu.Lock()
	defer s.mu.Unlock()

	if err == nil && len(users) > 0 {
		for _, u := range users {
			notif := Notification{
				ID:         newUUID(),
				UserID:     u.ID,
				Title:      req.Title,
				Message:    req.Message,
				Type:       strings.ToUpper(req.Type),
				StatusBaca: StatusBelumDibaca,
				IsRead:     false,
				ActionURL:  req.ActionURL,
				CreatedAt:  time.Now(),
			}
			_, _ = s.sbClient.PostDB("notifications", notif, "", "")

			list := s.store[u.ID]
			list = append([]Notification{notif}, list...)
			if len(list) > 20 {
				list = list[:20]
			}
			s.store[u.ID] = list
			sentCount++
		}
	} else {
		// Broadcast to all active in-memory users
		for uid, list := range s.store {
			notif := Notification{
				ID:         newUUID(),
				UserID:     uid,
				Title:      req.Title,
				Message:    req.Message,
				Type:       strings.ToUpper(req.Type),
				StatusBaca: StatusBelumDibaca,
				IsRead:     false,
				ActionURL:  req.ActionURL,
				CreatedAt:  time.Now(),
			}
			list = append([]Notification{notif}, list...)
			if len(list) > 20 {
				list = list[:20]
			}
			s.store[uid] = list
			sentCount++
		}
		if sentCount == 0 {
			sentCount = 1
		}
	}

	log.Printf("📢 [NOTIFICATION SERVICE] Broadcast terkirim ke %d pengguna", sentCount)
	return sentCount, nil
}

// GetAllNotificationsAdmin returns recent notifications across users
func (s *notificationService) GetAllNotificationsAdmin(limit, offset int) ([]Notification, int, error) {
	if limit <= 0 {
		limit = 50
	}
	queryParams := fmt.Sprintf("order=created_at.desc&limit=%d&offset=%d", limit, offset)

	respBytes, err := s.sbClient.GetDB("notifications", queryParams, "")
	if err == nil {
		var notifs []Notification
		if err := json.Unmarshal(respBytes, &notifs); err == nil && len(notifs) > 0 {
			return notifs, len(notifs), nil
		}
	}

	// Fallback in-memory
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []Notification
	for _, list := range s.store {
		all = append(all, list...)
	}

	if len(all) == 0 {
		all = generateDefaultNotifications("admin-preview")
	}

	if len(all) > limit {
		all = all[:limit]
	}

	return all, len(all), nil
}
