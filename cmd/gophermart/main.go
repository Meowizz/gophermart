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

	if err := database.InitDB(context.Background()); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	h := handler.NewHandler(database.GetStore(), []byte(cfg.JWTSecret))

	http.HandleFunc("/api/user/register", h.RegisterHandler)
	http.HandleFunc("/api/user/login", h.LoginHandler)
	http.Handle("/api/user/orders",
		middleware.AuthTokenMiddleware([]byte(cfg.JWTSecret))(
			http.HandlerFunc(h.CreateOrder),
		),
	)
	srv := &http.Server{
		Addr:         cfg.ServerPort,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Server is running on port %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	log.Println("Shutting down...")

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Failed to shutdown server: %v", err)
	}
	log.Println("Server stopped")
}
