package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/dede-wandi/server-dewa/internal/domain/role"
	"github.com/dede-wandi/server-dewa/internal/middleware"
	"github.com/dede-wandi/server-dewa/internal/modules/wallet"
	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

var adminStartTime = time.Now()

type Handler struct {
	sbClient      *supabase.Client
	walletHandler *wallet.Handler
}

func NewHandler(sbClient *supabase.Client, walletHandler *wallet.Handler) *Handler {
	return &Handler{
		sbClient:      sbClient,
		walletHandler: walletHandler,
	}
}

// Routes defines all /api/v1/admin endpoints
// All routes under this handler are strictly protected for roles: admin, superadmin
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Admin Ping / Info
	r.Get("/ping", h.Ping)

	// Dashboard Overview & Platform Stats
	r.Get("/dashboard", h.DashboardStats)
	r.Get("/stats", h.DashboardStats)

	// User & Wallet Management
	r.Get("/users", h.ListUsers)
	r.Get("/users/{id}", h.GetUserDetail)
	r.Put("/users/{id}/lock", h.LockUser)
	r.Put("/users/{id}/unlock", h.UnlockUser)

	// Platform-wide Ledger & Transactions (All users)
	r.Get("/transactions", h.ListAllTransactions)
	r.Get("/ledger", h.ListAllTransactions)

	// Balance Adjustment & Refund (Admin actions)
	r.Post("/adjust", h.walletHandler.AdjustBalance)
	r.Post("/wallets/adjust", h.walletHandler.AdjustBalance)
	r.Post("/refund", h.walletHandler.Refund)
	r.Post("/wallets/refund", h.walletHandler.Refund)

	// Role Assignment via Supabase RPC
	r.Post("/roles/assign", h.AssignRole)
	r.Post("/roles/remove", h.RemoveRole)

	// Points Ledger & Adjustment (Admin)
	r.Get("/points/transactions", h.ListAllPointTransactions)
	r.Get("/points/ledger", h.ListAllPointTransactions)
	r.Post("/points/adjust", h.AdjustUserPoints)

	// Business Types & Extensible Merchant Models Registry
	r.Get("/business-types", h.ListBusinessTypes)
	r.Get("/roles", h.ListBusinessTypes)

	return r
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	adminUser := middleware.GetUserFromContext(r.Context())
	adminEmail := "anonymous"
	if adminUser != nil {
		adminEmail = adminUser.Email
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":     "admin-service",
		"status":      "healthy",
		"admin_email": adminEmail,
		"uptime":      time.Since(adminStartTime).String(),
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

// DashboardStats returns aggregated platform statistics for Super Admin
func (h *Handler) DashboardStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := h.walletHandler.GetStats()

	sbStatus := "connected"
	if h.sbClient != nil {
		if err := h.sbClient.Ping(); err != nil {
			sbStatus = "error: " + err.Error()
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"overview": stats,
			"system": map[string]interface{}{
				"database_status": sbStatus,
				"uptime":          time.Since(adminStartTime).String(),
				"memory_alloc_mb": float64(m.Alloc) / 1024 / 1024,
				"goroutines":      runtime.NumGoroutine(),
			},
		},
	})
}

// ListUsers returns all platform users and their wallet balances, strictly excluding superadmin accounts
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	roleFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("role")))
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))

	isSuperAdminRole := func(rStr string) bool {
		clean := strings.ToLower(strings.TrimSpace(rStr))
		return clean == "superadmin" || strings.Contains(clean, "superadmin")
	}

	// 1. Try querying real users & wallets directly from database via Supabase RPC
	if h.sbClient != nil {
		if dbBytes, err := h.sbClient.CallRPC("get_all_registered_users_wallets", nil, ""); err == nil && len(dbBytes) > 0 {
			var dbUsers []map[string]interface{}
			if err := json.Unmarshal(dbBytes, &dbUsers); err == nil && len(dbUsers) > 0 {
				filtered := make([]map[string]interface{}, 0, len(dbUsers))
				for _, u := range dbUsers {
					userRole, _ := u["role"].(string)
					userEmail, _ := u["user_email"].(string)

					// Strictly exclude superadmin
					if isSuperAdminRole(userRole) {
						continue
					}

					def := role.GlobalRegistry.Resolve(userRole, userEmail)
					if def.Role == role.RoleSuperAdmin {
						continue
					}

					if roleFilter != "" && !strings.EqualFold(string(def.Role), roleFilter) {
						continue
					}
					if search != "" && !strings.Contains(strings.ToLower(userEmail), search) {
						continue
					}

					u["role"] = string(def.Role)
					u["role_name"] = def.Name
					u["badge_color"] = def.BadgeColor
					u["category"] = string(def.Category)
					if bal, ok := u["balance"].(float64); ok {
						u["formatted"] = fmt.Sprintf("Rp %s", wallet.FormatRupiah(int64(bal)))
					}
					filtered = append(filtered, u)
				}
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "success",
					"count":  len(filtered),
					"data":   filtered,
				})
				return
			}
		}
	}

	// 2. Query all registered users directly from Supabase Auth Admin API (100% real database users)
	if h.sbClient != nil {
		if authUsers, err := h.sbClient.AdminListUsers(1, 100); err == nil && len(authUsers) > 0 {
			list := make([]map[string]interface{}, 0, len(authUsers))
			for _, u := range authUsers {
				userRoles := u.GetRoles()
				isSuper := false
				for _, r := range userRoles {
					if isSuperAdminRole(r) {
						isSuper = true
						break
					}
				}
				if isSuper || isSuperAdminRole(u.Role) {
					continue // Strictly exclude superadmin
				}

				primaryRole := "user"
				if len(userRoles) > 0 {
					primaryRole = userRoles[0]
				}
				def := role.GlobalRegistry.Resolve(primaryRole, u.Email)
				if def.Role == role.RoleSuperAdmin {
					continue // Strictly exclude superadmin
				}

				if roleFilter != "" && !strings.EqualFold(string(def.Role), roleFilter) {
					continue
				}
				if search != "" && !strings.Contains(strings.ToLower(u.Email), search) {
					continue
				}

				wal, _ := h.walletHandler.GetWalletService().GetBalance(u.ID, u.Email, "")
				var balance float64
				var points int64
				var isLocked bool
				if wal != nil {
					balance = wal.Balance
					points = wal.Points
					isLocked = wal.IsLocked
				}

				list = append(list, map[string]interface{}{
					"user_id":     u.ID,
					"user_email":  u.Email,
					"role":        string(def.Role),
					"role_name":   def.Name,
					"badge_color": def.BadgeColor,
					"category":    string(def.Category),
					"balance":     balance,
					"formatted":   fmt.Sprintf("Rp %s", wallet.FormatRupiah(int64(balance))),
					"points":      points,
					"is_locked":   isLocked,
					"created_at":  u.CreatedAt,
				})
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"count":  len(list),
				"data":   list,
			})
			return
		}
	}

	// 3. Fallback to active runtime wallets (Clean OOP role resolution)
	wallets := h.walletHandler.GetAllWallets()
	userList := make([]map[string]interface{}, 0, len(wallets))

	for _, wal := range wallets {
		if isSuperAdminRole(wal.Role) {
			continue // Strictly exclude superadmin
		}
		def := role.GlobalRegistry.Resolve(wal.Role, wal.UserEmail)
		if def.Role == role.RoleSuperAdmin {
			continue // Strictly exclude superadmin
		}

		if roleFilter != "" && !strings.EqualFold(string(def.Role), roleFilter) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(wal.UserEmail), search) {
			continue
		}

		userList = append(userList, map[string]interface{}{
			"user_id":     wal.UserID,
			"user_email":  wal.UserEmail,
			"role":        string(def.Role),
			"role_name":   def.Name,
			"badge_color": def.BadgeColor,
			"category":    string(def.Category),
			"balance":     wal.Balance,
			"formatted":   fmt.Sprintf("Rp %s", wallet.FormatRupiah(int64(wal.Balance))),
			"points":      wal.Points,
			"is_locked":   wal.IsLocked,
			"updated_at":  wal.UpdatedAt.Format(time.RFC3339),
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(userList),
		"data":   userList,
	})
}

// GetUserDetail returns a single user's wallet and specific transaction history
func (h *Handler) GetUserDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := chi.URLParam(r, "id")

	wal, trxs, err := h.walletHandler.GetUserWallet(userID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"user_id":      wal.UserID,
			"user_email":   wal.UserEmail,
			"balance":      wal.Balance,
			"formatted":    fmt.Sprintf("Rp %s", wallet.FormatRupiah(int64(wal.Balance))),
			"points":       wal.Points,
			"is_locked":    wal.IsLocked,
			"updated_at":   wal.UpdatedAt.Format(time.RFC3339),
			"transactions": trxs,
		},
	})
}

// LockUser locks a user's wallet
func (h *Handler) LockUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := chi.URLParam(r, "id")

	wal, err := h.walletHandler.SetUserLock(userID, true)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"message":   fmt.Sprintf("Dompet pengguna %s berhasil dibekukan (LOCKED)", wal.UserEmail),
		"is_locked": wal.IsLocked,
	})
}

// UnlockUser unlocks a user's wallet
func (h *Handler) UnlockUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := chi.URLParam(r, "id")

	wal, err := h.walletHandler.SetUserLock(userID, false)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"message":   fmt.Sprintf("Dompet pengguna %s berhasil dibuka kembali (UNLOCKED)", wal.UserEmail),
		"is_locked": wal.IsLocked,
	})
}

// ListAllTransactions returns the full platform mutation ledger across all users
func (h *Handler) ListAllTransactions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 1. Try querying real ledger directly from database via Supabase RPC
	if h.sbClient != nil {
		if dbBytes, err := h.sbClient.CallRPC("get_all_app_transactions", map[string]interface{}{"p_limit": 200}, ""); err == nil && len(dbBytes) > 0 {
			var dbTrxs []map[string]interface{}
			if err := json.Unmarshal(dbBytes, &dbTrxs); err == nil && len(dbTrxs) > 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "success",
					"count":  len(dbTrxs),
					"data":   dbTrxs,
				})
				return
			}
		}
	}

	allTrxs := h.walletHandler.GetAllTransactions()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(allTrxs),
		"data":   allTrxs,
	})
}

type RoleAssignRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// AssignRole assigns a system role to a user via Supabase RPC
func (h *Handler) AssignRole(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req RoleAssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))

	if req.Email == "" || req.Role == "" {
		http.Error(w, `{"status":"error","message":"Email dan Role wajib diisi"}`, http.StatusBadRequest)
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	if h.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_email": req.Email,
			"p_role":  req.Role,
		}
		respBytes, err := h.sbClient.CallRPC("assign_user_role", rpcPayload, token)
		if err == nil {
			var rpcRes interface{}
			json.Unmarshal(respBytes, &rpcRes)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "success",
				"message": fmt.Sprintf("Role '%s' berhasil diberikan ke %s", req.Role, req.Email),
				"result":  rpcRes,
			})
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Role '%s' untuk %s dicatat di sistem (RPC fallback)", req.Role, req.Email),
	})
}

// RemoveRole removes a system role from a user via Supabase RPC
func (h *Handler) RemoveRole(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req RoleAssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	if h.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_email": req.Email,
			"p_role":  req.Role,
		}
		respBytes, err := h.sbClient.CallRPC("remove_user_role", rpcPayload, token)
		if err == nil {
			var rpcRes interface{}
			json.Unmarshal(respBytes, &rpcRes)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "success",
				"message": fmt.Sprintf("Role '%s' berhasil dicabut dari %s", req.Role, req.Email),
				"result":  rpcRes,
			})
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Role '%s' untuk %s dihapus dari sistem (RPC fallback)", req.Role, req.Email),
	})
}

// ListAllPointTransactions returns all point transactions across all users
func (h *Handler) ListAllPointTransactions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 1. Try querying real point ledger directly from database via Supabase RPC
	if h.sbClient != nil {
		if dbBytes, err := h.sbClient.CallRPC("get_all_point_transactions", map[string]interface{}{"p_limit": 200}, ""); err == nil && len(dbBytes) > 0 {
			var dbPts []map[string]interface{}
			if err := json.Unmarshal(dbBytes, &dbPts); err == nil && len(dbPts) > 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "success",
					"count":  len(dbPts),
					"data":   dbPts,
				})
				return
			}
		}
	}

	allPts := h.walletHandler.GetAllPointTransactions()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(allPts),
		"data":   allPts,
	})
}

type AdjustPointsRequest struct {
	TargetUserID string `json:"target_user_id"`
	Type         string `json:"type"`   // CREDIT or DEBIT
	Amount       int64  `json:"amount"` // Jumlah poin
	Reason       string `json:"reason"` // Alasan
}

// AdjustUserPoints allows Super Admin to adjust user points with an audit trail
func (h *Handler) AdjustUserPoints(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	adminUser := middleware.GetUserFromContext(r.Context())
	if adminUser == nil || !adminUser.HasAnyRole("admin", "superadmin") {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    403,
			"message": "Akses ditolak. Fitur penyesuaian poin hanya dapat dilakukan oleh Super Admin.",
		})
		return
	}

	var req AdjustPointsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.TargetUserID == "" || req.Amount <= 0 || strings.TrimSpace(req.Reason) == "" {
		http.Error(w, `{"status":"error","message":"target_user_id, amount > 0, dan reason wajib diisi"}`, http.StatusBadRequest)
		return
	}

	ptTrx, err := h.walletHandler.AdjustPoints(req.TargetUserID, req.Type, req.Amount, req.Reason, adminUser.Email)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"message":     fmt.Sprintf("Penyesuaian %d Poin untuk user %s berhasil diproses", req.Amount, req.TargetUserID),
		"admin_by":    adminUser.Email,
		"transaction": ptTrx,
	})
}

// ListBusinessTypes returns all registered business models and roles from the OOP registry
func (h *Handler) ListBusinessTypes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	types := role.GlobalRegistry.GetAll()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(types),
		"data":   types,
	})
}
