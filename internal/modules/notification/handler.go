package notification

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/dede-wandi/server-dewa/internal/middleware"
	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	sbClient *supabase.Client
	service  Service
}

func NewHandler(sbClient *supabase.Client) *Handler {
	return &Handler{
		sbClient: sbClient,
		service:  NewService(sbClient),
	}
}

func (h *Handler) GetService() Service {
	return h.service
}

// Routes mounts endpoints for authenticated mobile & web users
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.GetNotifications)

	// Mark as read endpoints (mendukung bahasa inggris & indonesia)
	r.Put("/{id}/read", h.MarkRead)
	r.Put("/{id}/baca", h.MarkRead)

	// Mark as unread endpoints (mendukung bahasa inggris & indonesia)
	r.Put("/{id}/unread", h.MarkUnread)
	r.Put("/{id}/belum-baca", h.MarkUnread)

	// Mark all as read
	r.Put("/read-all", h.MarkAllRead)
	r.Put("/baca-semua", h.MarkAllRead)

	// Delete single notification
	r.Delete("/{id}", h.DeleteNotification)
	r.Delete("/{id}/hapus", h.DeleteNotification)

	// Delete all notifications
	r.Delete("/", h.DeleteAllNotifications)
	r.Delete("/hapus-semua", h.DeleteAllNotifications)

	return r
}

// AdminRoutes mounts endpoints for Admin & Superadmin
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.AdminGetAll)
	r.Post("/broadcast", h.AdminBroadcast)
	r.Post("/send", h.AdminSend)
	r.Delete("/{id}", h.AdminDelete)

	return r
}

// parseJWTSub extracts the subject (user UUID) directly from a JWT access token without network latency
func parseJWTSub(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	payloadSegment := parts[1]
	switch len(payloadSegment) % 4 {
	case 2:
		payloadSegment += "=="
	case 3:
		payloadSegment += "="
	}
	bytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		bytes, err = base64.URLEncoding.DecodeString(payloadSegment)
		if err != nil {
			bytes, err = base64.StdEncoding.DecodeString(payloadSegment)
			if err != nil {
				return ""
			}
		}
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(bytes, &claims); err == nil && claims.Sub != "" {
		return claims.Sub
	}
	return ""
}

// getUserID helper to extract authenticated user ID with multiple fallback layers
func (h *Handler) getUserID(r *http.Request) string {
	// 1. Context user (from middleware if present)
	user := middleware.GetUserFromContext(r.Context())
	if user != nil && user.ID != "" {
		return user.ID
	}

	// 2. Explicit header X-User-ID (passed by mobile app or frontend)
	if uid := strings.TrimSpace(r.Header.Get("X-User-ID")); uid != "" {
		return uid
	}

	// 3. Explicit query parameter user_id
	if uid := strings.TrimSpace(r.URL.Query().Get("user_id")); uid != "" {
		return uid
	}

	// 4. Extract subject claim directly from Bearer JWT
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		token := strings.TrimSpace(authHeader[7:])
		if sub := parseJWTSub(token); sub != "" {
			return sub
		}
		if token != "" && h.sbClient != nil {
			if u, err := h.sbClient.GetUser(token); err == nil && u != nil && u.ID != "" {
				return u.ID
			}
		}
	}

	// 5. Device ID isolation fallback so unauthenticated clients don't collide
	if devID := strings.TrimSpace(r.Header.Get("X-Device-ID")); devID != "" {
		return "device-" + devID
	}

	// 6. Remote IP isolation fallback
	remoteAddr := r.RemoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		remoteAddr = remoteAddr[:idx]
	}
	if remoteAddr != "" && remoteAddr != "127.0.0.1" && remoteAddr != "::1" {
		return "guest-" + remoteAddr
	}

	return "guest-user"
}

// GetNotifications returns user's notifications (maksimal 20 item)
func (h *Handler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := h.getUserID(r)

	filterType := r.URL.Query().Get("type")
	filterStatus := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 20 {
			limit = l
		}
	}

	notifs, unreadCount, err := h.service.GetUserNotifications(userID, filterType, filterStatus, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(NotificationListResponse{
		Status:      "success",
		TotalUnread: unreadCount,
		Count:       len(notifs),
		Limit:       limit,
		Data:        notifs,
	})
}

// MarkRead marks single notification as read ("Sudah Dibaca")
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := h.getUserID(r)
	id := chi.URLParam(r, "id")

	notif, err := h.service.MarkAsRead(userID, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"message":     "Notifikasi berhasil ditandai sudah dibaca",
		"status_baca": StatusSudahDibaca,
		"data":        notif,
	})
}

// MarkUnread marks single notification as unread ("Belum Dibaca")
func (h *Handler) MarkUnread(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := h.getUserID(r)
	id := chi.URLParam(r, "id")

	notif, err := h.service.MarkAsUnread(userID, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"message":     "Notifikasi berhasil ditandai belum dibaca",
		"status_baca": StatusBelumDibaca,
		"data":        notif,
	})
}

// MarkAllRead marks all notifications as read
func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := h.getUserID(r)

	if err := h.service.MarkAllAsRead(userID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Semua notifikasi berhasil ditandai sudah dibaca",
	})
}

// DeleteNotification removes a single notification
func (h *Handler) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := h.getUserID(r)
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteNotification(userID, id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Notifikasi berhasil dihapus",
	})
}

// DeleteAllNotifications deletes all notifications for user
func (h *Handler) DeleteAllNotifications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := h.getUserID(r)

	if err := h.service.DeleteAllNotifications(userID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Semua notifikasi berhasil dibersihkan",
	})
}

// AdminGetAll returns all notifications for admin dashboard
func (h *Handler) AdminGetAll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	notifs, count, err := h.service.GetAllNotificationsAdmin(limit, offset)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  count,
		"data":   notifs,
	})
}

// AdminBroadcast sends notification to all users
func (h *Handler) AdminBroadcast(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req BroadcastNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Format request broadcast tidak valid: " + err.Error(),
		})
		return
	}

	sentCount, err := h.service.BroadcastNotification(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"message":    "Pengumuman berhasil disiarkan ke semua pengguna",
		"sent_count": sentCount,
		"type":       strings.ToUpper(req.Type),
	})
}

// AdminSend sends notification to specific user
func (h *Handler) AdminSend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req CreateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Format payload tidak valid: " + err.Error(),
		})
		return
	}

	notif, err := h.service.CreateNotification(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Notifikasi berhasil dikirim ke pengguna",
		"data":    notif,
	})
}

// AdminDelete deletes notification by ID
func (h *Handler) AdminDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(r, "id")

	// Delete directly from DB
	err := h.service.DeleteNotification("admin", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Notifikasi berhasil dihapus oleh admin",
	})
}
