package database

import (
	"context"
	"fmt"

	"github.com/Meowizz/gophermart/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	Store  *repository.Store
	dbPool *pgxpool.Pool
)

func GetStore() *repository.Store {
	return Store
}

func InitDB(ctx context.Context, dsn string) error {
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	var err error

	dbPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("Unable to connect to the Database: %w", err)
	}

	if err := dbPool.Ping(ctx); err != nil {
		dbPool.Close()
		return fmt.Errorf("%v\n Unable To PING DB: %w", err)
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
