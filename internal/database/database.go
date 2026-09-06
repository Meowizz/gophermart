package database

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var db *pgxpool.Pool

func InitDB(ctx context.Context) error {
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URI")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URI environment variable is not set")
	}

	var err error

	db, err = pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("Unable to connect to the Database: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("%v\n Unable To PING DB:", dsn)
	}

	fmt.Println("Successfully connected to DB PG")
	return nil
}

func CloseDB() {
	if db != nil {
		db.Close()
	}
}
