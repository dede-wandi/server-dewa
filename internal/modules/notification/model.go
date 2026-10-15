package notification

import "time"

// NotificationType defines the classification of a notification
type NotificationType string

const (
	TypeTransaction  NotificationType = "TRANSACTION"  // Transaksi produk PPOB & Kasir
	TypePayment      NotificationType = "PAYMENT"      // Top up saldo, transfer, mutasi
	TypePromo        NotificationType = "PROMO"        // Promo cashback & diskon
	TypeAnnouncement NotificationType = "ANNOUNCEMENT" // Pengumuman resmi & info server
	TypeSecurity     NotificationType = "SECURITY"     // Keamanan akun, PIN, login baru
)

// ReadStatus represents the Indonesian status of notification
const (
	StatusBelumDibaca = "Belum Dibaca"
	StatusSudahDibaca = "Sudah Dibaca"
)

// Notification entity model
type Notification struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Title       string     `json:"title"`
	Message     string     `json:"message"`
	Type        string     `json:"type"`        // 'TRANSACTION', 'PAYMENT', 'PROMO', 'ANNOUNCEMENT', 'SECURITY'
	StatusBaca  string     `json:"status_baca"` // 'Belum Dibaca', 'Sudah Dibaca'
	IsRead      bool       `json:"is_read"`
	IsDeletable bool       `json:"is_deletable"` // false jika log login / keamanan permanen
	ReferenceID string     `json:"reference_id,omitempty"`
	ActionURL   string     `json:"action_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
}

// CreateNotificationRequest payload for creating a notification
type CreateNotificationRequest struct {
	UserID      string `json:"user_id"` // Spesifik user UUID atau 'all' untuk broadcast
	Title       string `json:"title"`
	Message     string `json:"message"`
	Type        string `json:"type"` // TRANSACTION, PAYMENT, PROMO, ANNOUNCEMENT, SECURITY
	IsDeletable *bool  `json:"is_deletable,omitempty"`
	ReferenceID string `json:"reference_id,omitempty"`
	ActionURL   string `json:"action_url,omitempty"`
}

// BroadcastNotificationRequest payload for admin broadcast announcement
type BroadcastNotificationRequest struct {
	Title      string `json:"title"`
	Message    string `json:"message"`
	Type       string `json:"type,omitempty"`        // Default: ANNOUNCEMENT
	TargetRole string `json:"target_role,omitempty"` // 'all', 'user', 'mitra'
	ActionURL  string `json:"action_url,omitempty"`
}

// NotificationResponse wrapper for single notification
type NotificationResponse struct {
	Status  string        `json:"status"`
	Message string        `json:"message,omitempty"`
	Data    *Notification `json:"data,omitempty"`
}

// NotificationListResponse wrapper for notification collection
type NotificationListResponse struct {
	Status      string         `json:"status"`
	TotalUnread int            `json:"total_unread"`
	Count       int            `json:"count"`
	Limit       int            `json:"limit"`
	Data        []Notification `json:"data"`
}
