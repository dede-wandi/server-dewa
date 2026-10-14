package router

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dede-wandi/server-dewa/internal/config"
	"github.com/dede-wandi/server-dewa/internal/middleware"
	"github.com/dede-wandi/server-dewa/internal/modules/admin"
	"github.com/dede-wandi/server-dewa/internal/modules/auth"
	"github.com/dede-wandi/server-dewa/internal/modules/product"
	"github.com/dede-wandi/server-dewa/internal/modules/transaction"
	"github.com/dede-wandi/server-dewa/internal/modules/wallet"
	"github.com/dede-wandi/server-dewa/internal/supabase"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

var startTime = time.Now()

func New(cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	// Initialize Supabase Client
	sbClient := supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseAnonKey, cfg.SupabaseSecretKey)

	// Global Middlewares
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(30 * time.Second))

	// CORS Configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "apikey"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Custom 405 Method Not Allowed handler to always return clean JSON
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"code":    401,
			"message": "No auth credentials provided. Gunakan method yang sesuai (POST untuk login/register) dengan payload JSON.",
		})
	})

	// Root Route - Clean, secure status
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "Hello World! server OK",
		})
	})

	// Healthcheck for Docker & monitoring
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "UP",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Initialize Handlers
	authHandler := auth.NewHandler(sbClient)
	productHandler := product.NewHandler()
	transactionHandler := transaction.NewHandler()
	walletHandler := wallet.NewHandler(sbClient)
	adminHandler := admin.NewHandler(sbClient, walletHandler)

	r.Route("/api/v1", func(api chi.Router) {
		// Auth endpoints (public login/register/refresh, protected /me)
		api.Mount("/auth", authHandler.Routes())

		// DEDICATED SUPER ADMIN & ADMIN ENDPOINTS: Wajib login & role 'admin' / 'superadmin'
		api.Route("/admin", func(adminRouter chi.Router) {
			adminRouter.Use(middleware.RequireAuth(sbClient))
			adminRouter.Use(middleware.RequireRoles("admin", "superadmin"))
			adminRouter.Mount("/", adminHandler.Routes())
		})

		// PROTECTED MICROSERVICES: Semua modul wajib melalui autentikasi login Supabase
		api.Group(func(protected chi.Router) {
			protected.Use(middleware.RequireAuth(sbClient))

			protected.Mount("/products", productHandler.Routes())
			protected.Mount("/transactions", transactionHandler.Routes())
			protected.Mount("/wallet", walletHandler.Routes())
		})
	})

	return r
}
