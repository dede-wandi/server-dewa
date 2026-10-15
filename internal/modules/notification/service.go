package notification

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
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
	sbClient        *supabase.Client
	mu              sync.RWMutex
	store           map[string][]Notification // Fallback in-memory store keyed by userID
	userInitialized map[string]bool           // Tracks if defaults were initialized once per user
	broadcasts      []Notification            // Global broadcasts
}

func NewService(sbClient *supabase.Client) Service {
	s := &notificationService{
		sbClient:        sbClient,
		store:           make(map[string][]Notification),
		userInitialized: make(map[string]bool),
		broadcasts:      make([]Notification, 0),
	}
	return s
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
		if err := json.Unmarshal(respBytes, &notifs); err == nil {
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
	if !s.userInitialized[userID] && !ok {
		userList = make([]Notification, 0)
		// Tambahkan broadcast global yang belum diterima user
		for _, b := range s.broadcasts {
			userNotif := Notification{
				ID:          newUUID(),
				UserID:      userID,
				Title:       b.Title,
				Message:     b.Message,
				Type:        b.Type,
				StatusBaca:  StatusBelumDibaca,
				IsRead:      false,
				IsDeletable: true,
				ActionURL:   b.ActionURL,
				CreatedAt:   b.CreatedAt,
			}
			userList = append([]Notification{userNotif}, userList...)
		}
		if len(userList) > 20 {
			userList = userList[:20]
		}
		s.store[userID] = userList
		s.userInitialized[userID] = true
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
		list = make([]Notification, 0)
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
		list = make([]Notification, 0)
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
		list = make([]Notification, 0)
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
		list = make([]Notification, 0)
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
	s.userInitialized[userID] = true
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
		list = []Notification{}
	}

	// Pertahankan seluruh notifikasi keamanan dan log login
	keptList := make([]Notification, 0)
	for _, n := range list {
		if n.Type == string(TypeSecurity) || !n.IsDeletable {
			keptList = append(keptList, n)
		}
	}
	s.store[userID] = keptList
	s.userInitialized[userID] = true
	return nil
}

// CreateNotification creates a single notification (or broadcasts if user_id is 'all')
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

	// Jika ditujukan untuk 'all', delegasikan ke BroadcastNotification agar setiap user punya salinan mandiri
	if req.UserID == "all" || req.UserID == "" {
		_, err := s.BroadcastNotification(BroadcastNotificationRequest{
			Title:     req.Title,
			Message:   req.Message,
			Type:      req.Type,
			ActionURL: req.ActionURL,
		})
		if err != nil {
			return nil, err
		}
		return &Notification{
			ID:          newUUID(),
			UserID:      "all",
			Title:       req.Title,
			Message:     req.Message,
			Type:        upperType,
			StatusBaca:  StatusBelumDibaca,
			IsRead:      false,
			IsDeletable: isDeletable,
			ReferenceID: req.ReferenceID,
			ActionURL:   req.ActionURL,
			CreatedAt:   time.Now(),
		}, nil
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
	s.userInitialized[req.UserID] = true

	return &notif, nil
}

// BroadcastNotification sends notification to all users with independent read & delete state per user
func (s *notificationService) BroadcastNotification(req BroadcastNotificationRequest) (int, error) {
	if req.Title == "" || req.Message == "" {
		return 0, errors.New("judul dan pesan broadcast wajib diisi")
	}

	if req.Type == "" {
		req.Type = string(TypeAnnouncement)
	}

	upperType := strings.ToUpper(req.Type)

	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Simpan template broadcast global untuk user yang baru mendaftar / baru buka aplikasi
	bTemplate := Notification{
		ID:          newUUID(),
		Title:       req.Title,
		Message:     req.Message,
		Type:        upperType,
		StatusBaca:  StatusBelumDibaca,
		IsRead:      false,
		IsDeletable: true,
		ActionURL:   req.ActionURL,
		CreatedAt:   time.Now(),
	}
	s.broadcasts = append([]Notification{bTemplate}, s.broadcasts...)
	if len(s.broadcasts) > 20 {
		s.broadcasts = s.broadcasts[:20]
	}

	sentCount := 0

	// 2. Buat notifikasi mandiri untuk setiap user yang ada di memory (agar read/delete masing-masing)
	for uid, list := range s.store {
		userNotif := Notification{
			ID:          newUUID(),
			UserID:      uid,
			Title:       req.Title,
			Message:     req.Message,
			Type:        upperType,
			StatusBaca:  StatusBelumDibaca,
			IsRead:      false,
			IsDeletable: true,
			ActionURL:   req.ActionURL,
			CreatedAt:   time.Now(),
		}
		list = append([]Notification{userNotif}, list...)
		if len(list) > 20 {
			list = list[:20]
		}
		s.store[uid] = list
		s.userInitialized[uid] = true
		sentCount++
	}

	// 3. Simpan baris mandiri ke database Supabase untuk user terdaftar
	go func(bReq BroadcastNotificationRequest) {
		users, err := s.sbClient.AdminListUsers(1, 100)
		if err == nil && len(users) > 0 {
			for _, u := range users {
				userNotif := Notification{
					ID:          newUUID(),
					UserID:      u.ID,
					Title:       bReq.Title,
					Message:     bReq.Message,
					Type:        upperType,
					StatusBaca:  StatusBelumDibaca,
					IsRead:      false,
					IsDeletable: true,
					ActionURL:   bReq.ActionURL,
					CreatedAt:   time.Now(),
				}
				_, _ = s.sbClient.PostDB("notifications", userNotif, "", "")
			}
		}
	}(req)

	if sentCount == 0 {
		sentCount = 1
	}

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
		if err := json.Unmarshal(respBytes, &notifs); err == nil {
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
		all = make([]Notification, 0)
	}

	if len(all) > limit {
		all = all[:limit]
	}

	return all, len(all), nil
}
