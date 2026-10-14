package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dede-wandi/server-dewa/internal/config"
	"github.com/dede-wandi/server-dewa/internal/router"
)

func main() {
	cfg := config.LoadConfig()

	r := router.New(cfg)

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server run context
	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("🚀 Starting %s on port %s (env: %s)", cfg.ServiceName, cfg.Port, cfg.Environment)
		log.Printf("📦 Microservices running in 1 container: Auth, Products, Transactions, Wallet")
		serverErrors <- srv.ListenAndServe()
	}()

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("Error starting server: %v", err)

	case sig := <-shutdown:
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Graceful shutdown failed, forcing close: %v", err)
			srv.Close()
		}
		log.Println("Server stopped cleanly")
	}
}
