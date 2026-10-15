package router

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dede-wandi/server-dewa/internal/config"
	"github.com/dede-wandi/server-dewa/internal/middleware"
	"github.com/dede-wandi/server-dewa/internal/modules/admin"
	"github.com/dede-wandi/server-dewa/internal/modules/article"
	"github.com/dede-wandi/server-dewa/internal/modules/auth"
	"github.com/dede-wandi/server-dewa/internal/modules/banner"
	"github.com/dede-wandi/server-dewa/internal/modules/notification"
	"github.com/dede-wandi/server-dewa/internal/modules/payment_channel"
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
	r.Use(chiMiddleware.Compress(5))
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
	productHandler := product.NewHandler(sbClient)
	walletHandler := wallet.NewHandler(sbClient)
	transactionHandler := transaction.NewHandler(walletHandler.GetWalletService(), productHandler.GetService(), sbClient)
	adminHandler := admin.NewHandler(sbClient, walletHandler)
	bannerHandler := banner.NewHandler(sbClient)
	articleHandler := article.NewHandler(sbClient)
	notificationHandler := notification.NewHandler(sbClient)
	paymentChannelHandler := payment_channel.NewHandler(sbClient)

	r.Route("/api/v1", func(api chi.Router) {
		// Auth endpoints (public login/register/refresh, protected /me)
		api.Mount("/auth", authHandler.Routes())

		// Public catalog browsing (fast cached responses)
		api.Mount("/products", productHandler.Routes())

		// Public banners & promos (for mobile home screen & app)
		api.Mount("/banners", bannerHandler.Routes())
		api.Mount("/promos", bannerHandler.Routes())

		// Public articles & news (edukasi, panduan kasir, tips bisnis mitra)
		api.Mount("/articles", articleHandler.Routes())
		api.Mount("/news", articleHandler.Routes())

		// User notifications (dengan bearer token atau demo fallback, limit max 20)
		api.Mount("/notifications", notificationHandler.Routes())

		// Dynamic payment channels & vendors (active & coming soon PG)
		api.Mount("/payment-channels", paymentChannelHandler.PublicRoutes())

		// DEDICATED SUPER ADMIN & ADMIN ENDPOINTS: Wajib login & role 'admin' / 'superadmin'
		api.Route("/admin", func(adminRouter chi.Router) {
			adminRouter.Use(middleware.RequireAuth(sbClient))
			adminRouter.Use(middleware.RequireRoles("admin", "superadmin"))
			adminRouter.Mount("/", adminHandler.Routes())
			adminRouter.Post("/products/sync", productHandler.SyncProducts)
			adminRouter.Mount("/banners", bannerHandler.AdminRoutes())
			adminRouter.Mount("/promos", bannerHandler.AdminRoutes())
			adminRouter.Mount("/articles", articleHandler.AdminRoutes())
			adminRouter.Mount("/news", articleHandler.AdminRoutes())
			adminRouter.Mount("/notifications", notificationHandler.AdminRoutes())
			adminRouter.Mount("/payment-channels", paymentChannelHandler.AdminRoutes())
		})

		// PROTECTED MICROSERVICES: Semua modul transaksi & dompet wajib melalui autentikasi login Supabase
		api.Group(func(protected chi.Router) {
			protected.Use(middleware.RequireAuth(sbClient))

			protected.Mount("/transactions", transactionHandler.Routes())
			protected.Mount("/wallet", walletHandler.Routes())
		})
	})

	return r
}
