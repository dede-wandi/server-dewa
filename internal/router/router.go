package router

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/dede-wandi/server-dewa/internal/config"
	"github.com/dede-wandi/server-dewa/internal/modules/auth"
	"github.com/dede-wandi/server-dewa/internal/modules/product"
	"github.com/dede-wandi/server-dewa/internal/modules/transaction"
	"github.com/dede-wandi/server-dewa/internal/modules/wallet"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

var startTime = time.Now()

func New(cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	// Global Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS Configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Root Route - Hello World & System Status
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":     "Hello World! DewaCell API is running successfully in 1 Docker Container 🚀",
			"service":     cfg.ServiceName,
			"environment": cfg.Environment,
			"version":     cfg.AppVersion,
			"uptime":      time.Since(startTime).String(),
			"timestamp":   time.Now().Format(time.RFC3339),
			"memory_usage": map[string]interface{}{
				"alloc_mb":        float64(m.Alloc) / 1024 / 1024,
				"total_alloc_mb":  float64(m.TotalAlloc) / 1024 / 1024,
				"sys_mb":          float64(m.Sys) / 1024 / 1024,
				"num_gc":          m.NumGC,
				"goroutines":      runtime.NumGoroutine(),
			},
			"architecture": "Single-Container Modular Microservices",
			"services": map[string]string{
				"auth":        "/api/v1/auth/ping",
				"products":    "/api/v1/products/ping",
				"transaction": "/api/v1/transactions/ping",
				"wallet":      "/api/v1/wallet/ping",
			},
		})
	})

	// Healthcheck
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "UP",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Mount Microservices Modules
	authHandler := auth.NewHandler()
	productHandler := product.NewHandler()
	transactionHandler := transaction.NewHandler()
	walletHandler := wallet.NewHandler()

	r.Route("/api/v1", func(api chi.Router) {
		api.Mount("/auth", authHandler.Routes())
		api.Mount("/products", productHandler.Routes())
		api.Mount("/transactions", transactionHandler.Routes())
		api.Mount("/wallet", walletHandler.Routes())
	})

	return r
}
