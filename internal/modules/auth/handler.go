package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dede-wandi/server-dewa/internal/middleware"
	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	sb *supabase.Client
}

func NewHandler(sb *supabase.Client) *Handler {
	return &Handler{sb: sb}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public routes
	r.Get("/ping", h.Ping)
	r.Get("/info", h.Info)
	r.Post("/login", h.Login)
	r.Post("/register", h.Register)
	r.Post("/refresh", h.Refresh)

	// Protected route (requires valid token)
	r.Group(func(protected chi.Router) {
		protected.Use(middleware.RequireAuth(h.sb))
		protected.Get("/me", h.Me)
	})

	return r
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
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

	if req.Email == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "Email dan password wajib diisi",
		})
		return
	}

	authResp, err := h.sb.SignIn(req.Email, req.Password)
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

	if req.Email == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    http.StatusBadRequest,
			"message": "Email dan password wajib diisi",
		})
		return
	}

	authResp, err := h.sb.SignUp(req.Email, req.Password, req.Data)
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

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"user":   user,
		"roles":  user.GetRoles(),
	})
}

