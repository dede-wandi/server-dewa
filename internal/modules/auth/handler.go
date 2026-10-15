package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dede-wandi/server-dewa/internal/domain/role"
	"github.com/dede-wandi/server-dewa/internal/middleware"
	"github.com/dede-wandi/server-dewa/internal/modules/notification"
	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	sb           *supabase.Client
	notifService notification.Service
}

func NewHandler(sb *supabase.Client, notifService notification.Service) *Handler {
	return &Handler{
		sb:           sb,
		notifService: notifService,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public routes
	r.Get("/ping", h.Ping)
	r.Get("/info", h.Info)
	r.Post("/login", h.Login)
	r.Get("/login", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    401,
			"message": "No auth credentials provided. Endpoint ini memerlukan otentikasi method POST dengan body JSON: {\"email\": \"...\", \"password\": \"...\"}",
		})
	})
	r.Post("/register", h.Register)
	r.Get("/register", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    401,
			"message": "No auth credentials provided. Gunakan method POST untuk mendaftarkan akun.",
		})
	})
	r.Post("/refresh", h.Refresh)

	// Protected route (requires valid token)
	r.Group(func(protected chi.Router) {
		protected.Use(middleware.RequireAuth(h.sb))
		protected.Get("/me", h.Me)
		protected.Put("/profile", h.UpdateProfile)
		protected.Post("/profile", h.UpdateProfile)
		protected.Post("/change-password", h.ChangePassword)
	})

	return r
}

type LoginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	DeviceID   string `json:"device_id,omitempty"`
	DeviceName string `json:"device_name,omitempty"`
	DeviceOS   string `json:"device_os,omitempty"`
	Location   string `json:"location,omitempty"`
}

type RegisterRequest struct {
	Email    string                 `json:"email"`
	Password string                 `json:"password"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	sbErr := h.sb.Ping()
	sbStatus := "connected"
	if sbErr != nil {
		sbStatus = "error: " + sbErr.Error()
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":         "auth-service",
		"status":          "healthy",
		"supabase_status": sbStatus,
		"timestamp":       time.Now().Format(time.RFC3339),
		"message":         "Auth service is active and connected to Supabase",
	})
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": "auth-service",
		"version": "v1.1.0",
		"endpoints": []map[string]string{
			{"method": "POST", "path": "/api/v1/auth/login", "auth": "public", "desc": "Login with email & password"},
			{"method": "POST", "path": "/api/v1/auth/register", "auth": "public", "desc": "Register new account"},
			{"method": "POST", "path": "/api/v1/auth/refresh", "auth": "public", "desc": "Refresh access token"},
			{"method": "GET", "path": "/api/v1/auth/me", "auth": "Bearer Token required", "desc": "Get current user profile"},
			{"method": "GET", "path": "/api/v1/auth/ping", "auth": "public", "desc": "Health & Supabase status"},
		},
		"description": "User authentication, JWT issuance, and session management via Supabase",
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "Body request tidak valid: JSON diperlukan",
		})
		return
	}

	cleanEmail := strings.ToLower(strings.TrimSpace(req.Email))
	if cleanEmail == "" || !strings.Contains(cleanEmail, "@") {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "Format alamat email tidak valid",
		})
		return
	}

	if req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "Password wajib diisi",
		})
		return
	}

	authResp, err := h.sb.SignIn(cleanEmail, req.Password)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)

		// Evaluasi penyebab kegagalan: email belum terdaftar vs password keliru
		errMsg := "Email atau password yang Anda masukkan salah"
		if !h.sb.IsEmailRegistered(cleanEmail) {
			errMsg = "Email tidak terdaftar di sistem. Silakan periksa kembali email Anda atau lakukan pendaftaran."
		} else {
			errMsg = "Password yang Anda masukkan salah. Silakan coba lagi."
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusUnauthorized,
			"message": errMsg,
		})
		return
	}

	// Deteksi Device, Lokasi & IP untuk Audit Log Database & Notifikasi Keamanan Pengguna
	go func(userUUID string, userEmail string, loginReq LoginRequest, userAgent string, remoteAddr string, xForwardedFor string, xRealIP string, xDeviceID string, xDeviceName string, xDeviceOS string, xDeviceLocation string) {
		// 1. Ekstrak IP Address
		clientIP := strings.TrimSpace(xForwardedFor)
		if clientIP != "" {
			parts := strings.Split(clientIP, ",")
			clientIP = strings.TrimSpace(parts[0])
		}
		if clientIP == "" {
			clientIP = strings.TrimSpace(xRealIP)
		}
		if clientIP == "" {
			clientIP = remoteAddr
			if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
				clientIP = clientIP[:idx]
			}
		}
		if clientIP == "" || clientIP == "[" || clientIP == "127.0.0.1" || clientIP == "::1" {
			clientIP = "Local / VPN"
		}

		// 2. Ekstrak Device ID, Name & OS
		deviceID := strings.TrimSpace(loginReq.DeviceID)
		if deviceID == "" {
			deviceID = strings.TrimSpace(xDeviceID)
		}
		if deviceID == "" {
			deviceID = "unknown-device"
		}

		deviceName := strings.TrimSpace(loginReq.DeviceName)
		if deviceName == "" {
			deviceName = strings.TrimSpace(xDeviceName)
		}

		deviceOS := strings.TrimSpace(loginReq.DeviceOS)
		if deviceOS == "" {
			deviceOS = strings.TrimSpace(xDeviceOS)
		}

		location := strings.TrimSpace(loginReq.Location)
		if location == "" {
			location = strings.TrimSpace(xDeviceLocation)
		}

		if deviceName == "" {
			if strings.Contains(userAgent, "iPhone") {
				deviceName = "Apple iPhone"
			} else if strings.Contains(userAgent, "Android") {
				deviceName = "Perangkat Android"
			} else if strings.Contains(userAgent, "Macintosh") {
				deviceName = "Apple Mac"
			} else if strings.Contains(userAgent, "Windows") {
				deviceName = "Windows PC"
			} else if strings.Contains(userAgent, "Linux") {
				deviceName = "Linux Desktop"
			} else {
				deviceName = "Perangkat Pengguna"
			}
		}

		if deviceOS == "" {
			if strings.Contains(userAgent, "iPhone") {
				deviceOS = "iOS"
			} else if strings.Contains(userAgent, "Android") {
				deviceOS = "Android"
			} else if strings.Contains(userAgent, "Mac OS") {
				deviceOS = "macOS"
			} else if strings.Contains(userAgent, "Windows") {
				deviceOS = "Windows"
			} else {
				deviceOS = "Mobile Client"
			}
		}

		// 3. Format waktu WIB
		wib := time.FixedZone("WIB", 7*3600)
		loginTimeStr := time.Now().In(wib).Format("02 Jan 2006, 15:04 WIB")

		// 4. Catat ke tabel database public.user_login_logs
		loginLogRecord := map[string]interface{}{
			"user_id":     userUUID,
			"email":       userEmail,
			"device_id":   deviceID,
			"device_name": deviceName,
			"device_os":   deviceOS,
			"location":    location,
			"ip_address":  clientIP,
			"user_agent":  userAgent,
			"created_at":  time.Now(),
		}
		_, _ = h.sb.PostDB("user_login_logs", loginLogRecord, "", "")

		// 5. Catat notifikasi keamanan login
		locSuffix := ""
		if location != "" {
			locSuffix = fmt.Sprintf(" di %s", location)
		}

		title := fmt.Sprintf("🛡️ Aktivitas Login: %s", deviceName)
		msg := fmt.Sprintf("Akun Anda baru saja login dari perangkat %s (%s)%s pada %s (IP: %s). Jika Anda tidak merasa melakukan aktivitas ini atau perangkat tidak dikenali, segera amankan akun Anda dengan mengganti PIN atau Password.",
			deviceName, deviceOS, locSuffix, loginTimeStr, clientIP)

		isDeletable := true
		refID := fmt.Sprintf("LOG-%s", time.Now().Format("20060102150405"))

		if h.notifService != nil {
			_, _ = h.notifService.CreateNotification(notification.CreateNotificationRequest{
				UserID:      userUUID,
				Title:       title,
				Message:     msg,
				Type:        string(notification.TypeSecurity),
				IsDeletable: &isDeletable,
				ReferenceID: refID,
				ActionURL:   "/(tabs)/settings?action=change_pin",
			})
		}
	}(authResp.User.ID, cleanEmail, req, r.UserAgent(), r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Real-IP"), r.Header.Get("X-Device-ID"), r.Header.Get("X-Device-Name"), r.Header.Get("X-Device-OS"), r.Header.Get("X-Device-Location"))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Login berhasil! Gunakan access_token untuk mengakses API yang diproteksi",
		"data":    authResp,
	})
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "Body request tidak valid",
		})
		return
	}

	cleanEmail := strings.ToLower(strings.TrimSpace(req.Email))
	if cleanEmail == "" || !strings.Contains(cleanEmail, "@") {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "Format alamat email tidak valid",
		})
		return
	}

	if len(req.Password) < 6 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "Password minimal harus 6 karakter",
		})
		return
	}

	if h.sb.IsEmailRegistered(cleanEmail) {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusConflict,
			"message": "Alamat email sudah terdaftar di sistem. Silakan masuk dengan email ini atau gunakan email lain.",
		})
		return
	}

	authResp, err := h.sb.SignUp(cleanEmail, req.Password, req.Data)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Registrasi berhasil",
		"data":    authResp,
	})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "Body request tidak valid",
		})
		return
	}

	if req.RefreshToken == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "refresh_token wajib diisi",
		})
		return
	}

	authResp, err := h.sb.RefreshToken(req.RefreshToken)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusUnauthorized,
			"message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Token berhasil diperbarui",
		"data":    authResp,
	})
}

type UpdateProfileRequest struct {
	FullName        string `json:"full_name,omitempty"`
	Phone           string `json:"phone,omitempty"`
	Address         string `json:"address,omitempty"`
	BusinessName    string `json:"business_name,omitempty"`
	BusinessPhone   string `json:"business_phone,omitempty"`
	BusinessAddress string `json:"business_address,omitempty"`
	WATarget1       string `json:"wa_target_1,omitempty"`
	WATarget2       string `json:"wa_target_2,omitempty"`
	WATarget3       string `json:"wa_target_3,omitempty"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password,omitempty"`
	NewPassword string `json:"new_password"`
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusUnauthorized,
			"message": "User session not found",
		})
		return
	}

	userRoles := user.GetRoles()
	primaryRole := "user"
	if len(userRoles) > 0 {
		primaryRole = userRoles[0]
	}
	def := role.GlobalRegistry.Resolve(primaryRole, user.Email)

	var fullName, phone, avatarURL, address, businessName, businessPhone, businessAddress, waTarget1, waTarget2, waTarget3 string
	if user.UserMetadata != nil {
		if fn, ok := user.UserMetadata["full_name"].(string); ok && fn != "" {
			fullName = fn
		} else if dn, ok := user.UserMetadata["display_name"].(string); ok && dn != "" {
			fullName = dn
		}
		if p, ok := user.UserMetadata["phone"].(string); ok && p != "" {
			phone = p
		}
		if av, ok := user.UserMetadata["avatar_url"].(string); ok {
			avatarURL = av
		}
		address, _ = user.UserMetadata["address"].(string)
		businessName, _ = user.UserMetadata["business_name"].(string)
		businessPhone, _ = user.UserMetadata["business_phone"].(string)
		businessAddress, _ = user.UserMetadata["business_address"].(string)
		waTarget1, _ = user.UserMetadata["wa_target_1"].(string)
		waTarget2, _ = user.UserMetadata["wa_target_2"].(string)
		waTarget3, _ = user.UserMetadata["wa_target_3"].(string)
	}
	if phone == "" && user.Phone != "" {
		phone = user.Phone
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"user": map[string]interface{}{
			"id":               user.ID,
			"email":            user.Email,
			"role":             string(def.Role),
			"role_name":        def.Name,
			"full_name":        fullName,
			"phone":            phone,
			"avatar_url":       avatarURL,
			"address":          address,
			"business_name":    businessName,
			"business_phone":   businessPhone,
			"business_address": businessAddress,
			"wa_target_1":      waTarget1,
			"wa_target_2":      waTarget2,
			"wa_target_3":      waTarget3,
			"created_at":       user.CreatedAt,
		},
	})
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusUnauthorized,
			"message": "User session not found",
		})
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "Body request tidak valid",
		})
		return
	}

	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

	// Build metadata payload
	userMeta := make(map[string]interface{})
	if user.UserMetadata != nil {
		for k, v := range user.UserMetadata {
			userMeta[k] = v
		}
	}

	if req.FullName != "" {
		userMeta["full_name"] = strings.TrimSpace(req.FullName)
		userMeta["display_name"] = strings.TrimSpace(req.FullName)
	}
	if req.Phone != "" {
		userMeta["phone"] = strings.TrimSpace(req.Phone)
	}
	if req.Address != "" {
		userMeta["address"] = strings.TrimSpace(req.Address)
	}
	if req.BusinessName != "" {
		userMeta["business_name"] = strings.TrimSpace(req.BusinessName)
	}
	if req.BusinessPhone != "" {
		userMeta["business_phone"] = strings.TrimSpace(req.BusinessPhone)
	}
	if req.BusinessAddress != "" {
		userMeta["business_address"] = strings.TrimSpace(req.BusinessAddress)
	}
	if req.WATarget1 != "" {
		userMeta["wa_target_1"] = strings.TrimSpace(req.WATarget1)
	}
	if req.WATarget2 != "" {
		userMeta["wa_target_2"] = strings.TrimSpace(req.WATarget2)
	}
	if req.WATarget3 != "" {
		userMeta["wa_target_3"] = strings.TrimSpace(req.WATarget3)
	}

	payload := map[string]interface{}{
		"data": userMeta,
	}

	updatedUser, err := h.sb.UpdateUserWithToken(token, payload)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Profil berhasil diperbarui",
		"data":    updatedUser.UserMetadata,
	})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusUnauthorized,
			"message": "User session not found",
		})
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "Body request tidak valid",
		})
		return
	}

	newPassword := strings.TrimSpace(req.NewPassword)
	if len(newPassword) < 6 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "Password baru minimal 6 karakter",
		})
		return
	}

	// Verify old password if provided
	if req.OldPassword != "" {
		if _, err := h.sb.SignIn(user.Email, req.OldPassword); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"code":    http.StatusBadRequest,
				"message": "Password lama yang Anda masukkan salah. Silakan coba lagi.",
			})
			return
		}
	}

	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	payload := map[string]interface{}{
		"password": newPassword,
	}

	if _, err := h.sb.UpdateUserWithToken(token, payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Password berhasil diubah. Gunakan password baru Anda untuk login berikutnya.",
	})
}

