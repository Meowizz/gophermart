package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Meowizz/gophermart/internal/config"
	"github.com/Meowizz/gophermart/internal/database"
	"github.com/Meowizz/gophermart/internal/handler"
	"github.com/Meowizz/gophermart/internal/middleware"
)

func main() {
	cfg := config.LoadConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := database.InitDB(ctx, cfg.DatabaseURI); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	mux := http.NewServeMux()
	h := handler.NewHandler(database.GetStore(), []byte(cfg.JWTSecret))

	mux.HandleFunc("POST /api/user/register", h.RegisterHandler)
	mux.HandleFunc("POST /api/user/login", h.LoginHandler)

	authMW := middleware.AuthTokenMiddleware([]byte(cfg.JWTSecret))

	mux.Handle("POST /api/user/orders", authMW(http.HandlerFunc(h.CreateOrder)))
	mux.Handle("GET /api/user/orders", authMW(http.HandlerFunc(h.GetOrders)))

	mux.Handle("GET /api/user/balance", authMW(http.HandlerFunc(h.GetBalance)))
	mux.Handle("POST /api/user/balance/withdraw", authMW(http.HandlerFunc(h.WithdrawBalance)))
	mux.Handle("GET /api/user/withdrawals", authMW(http.HandlerFunc(h.GetWithdrawals)))
	srv := &http.Server{
		Addr:         cfg.RunAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server is running on port %s", cfg.RunAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	log.Println("Shutting down...")

	ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Failed to shutdown server: %v", err)
	}
	log.Println("Server stopped")
}
