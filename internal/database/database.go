package database

import (
	"context"
	"embed"
	"fmt"

	"github.com/Meowizz/gophermart/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var (
	Store  *repository.Store
	dbPool *pgxpool.Pool
)

func GetStore() *repository.Store {
	return Store
}

func InitDB(ctx context.Context, dsn string) error {
	if dsn == "" {
		return fmt.Errorf("database connection string (dsn) is empty")
	}

	var err error

	dbPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("unable to connect to the database: %w", err)
	}

	if err := dbPool.Ping(ctx); err != nil {
		dbPool.Close()
		return fmt.Errorf("unable to ping database: %w", err)
	}

	if err := runMigrations(ctx, dbPool); err != nil {
		dbPool.Close()
		return fmt.Errorf("migration failed: %w", err)
	}

	Store = repository.NewStore(dbPool)

	fmt.Println("successfully connected to DB PG and applied migrations")
	return nil
}

func CloseDB() {
	if dbPool != nil {
		dbPool.Close()
		fmt.Println("successfully closed DB connection")
	}
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	files := []string{
		"migrations/000001_create_users.up.sql",
		"migrations/000002_create_orders.up.sql",
		"migrations/000003_create_balances.up.sql",
		"migrations/000004_create_withdrawals.up.sql",
	}

	for _, file := range files {
		content, err := migrationsFS.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}
		_, err = pool.Exec(ctx, string(content))
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file, err)
		}
	}

	return nil
}
