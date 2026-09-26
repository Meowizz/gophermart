package config

import (
	"flag"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	RunAddr           string
	DatabaseURI       string
	AccrualSystemAddr string
	JWTSecret         string
}

func LoadConfig() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		RunAddr:           ":8080",
		DatabaseURI:       "postgres://postgres:postgres@localhost:5432/gophermart?sslmode=disable",
		AccrualSystemAddr: "http://localhost:8081",
		JWTSecret:         "default_insecure_secret_for_local_and_ci_testing",
	}

	flag.StringVar(&cfg.RunAddr, "a", cfg.RunAddr, "address and port to run server")
	flag.StringVar(&cfg.DatabaseURI, "d", cfg.DatabaseURI, "database connection string")
	flag.StringVar(&cfg.AccrualSystemAddr, "r", cfg.AccrualSystemAddr, "address of accrual system")
	flag.StringVar(&cfg.JWTSecret, "jwt-secret", cfg.JWTSecret, "JWT secret key")

	flag.Parse()

	if envAddr := os.Getenv("RUN_ADDR"); envAddr != "" {
		cfg.RunAddr = envAddr
	}
	if envDB := os.Getenv("DATABASE_URI"); envDB != "" {
		cfg.DatabaseURI = envDB
	}
	if envAccrual := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); envAccrual != "" {
		cfg.AccrualSystemAddr = envAccrual
	}
	if envJWT := os.Getenv("JWT_SECRET"); envJWT != "" {
		cfg.JWTSecret = envJWT
	}

	log.Printf("Starting with config: Addr=%s, Accrual=%s", cfg.RunAddr, cfg.AccrualSystemAddr)

	return cfg
}
