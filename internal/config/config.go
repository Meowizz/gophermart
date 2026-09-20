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

	cfg := &Config{}

	flag.StringVar(&cfg.RunAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&cfg.DatabaseURI, "d", "", "database connection string (required)")
	flag.StringVar(&cfg.AccrualSystemAddr, "r", "http://localhost:8081", "address of accrual system")
	flag.StringVar(&cfg.JWTSecret, "jwt-secret", "", "JWT secret key (required)")

	setDefaultsFromEnv(cfg)

	flag.Parse()

	if cfg.DatabaseURI == "" {
		log.Fatal("DATABASE_URI is required (use -d flag or DATABASE_URI env)")
	}
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required (use -jwt-secret flag or JWT_SECRET env)")
	}

	return cfg
}

func setDefaultsFromEnv(cfg *Config) {
	if val := os.Getenv("RUN_ADDR"); val != "" && !isFlagPassed("a") {
		cfg.RunAddr = val
	}
	if val := os.Getenv("DATABASE_URI"); val != "" && !isFlagPassed("d") {
		cfg.DatabaseURI = val
	}
	if val := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); val != "" && !isFlagPassed("r") {
		cfg.AccrualSystemAddr = val
	}
	if val := os.Getenv("JWT_SECRET"); val != "" && !isFlagPassed("jwt-secret") {
		cfg.JWTSecret = val
	}
}

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
