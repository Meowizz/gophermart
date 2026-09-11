package database

import (
	"context"
	"fmt"
	"os"

	"github.com/Meowizz/gophermart/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var (
	Store  *repository.Store
	dbPool *pgxpool.Pool
)

func GetStore() *repository.Store {
	return Store
}

func InitDB(ctx context.Context) error {
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	var err error

	dbPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("Unable to connect to the Database: %w", err)
	}

	if err := dbPool.Ping(ctx); err != nil {
		return fmt.Errorf("%v\n Unable To PING DB:", dsn)
	}

	Store = repository.NewStore(dbPool)

	fmt.Println("Successfully connected to DB PG")
	return nil
}

func CloseDB() {
	if dbPool != nil {
		dbPool.Close()
		fmt.Println("Successfully closed DB connection")
	}
}
