package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Meowizz/gophermart/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) GetUserByLogin(ctx context.Context, login string) (*models.User, error) {

	query := `SELECT id, login, password FROM users WHERE login = $1`
	var user models.User

	err := s.db.QueryRow(ctx, query, login).Scan(&user.ID, &user.Login, &user.Password)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("query error: %w", err)
	}

	return &user, nil
}

func (s *Store) CreateUser(ctx context.Context, login, passwordHash string) (*models.User, error) {
	query := `INSERT INTO users (login, password) VALUES ($1, $2) RETURNING id, login, password`

	var user models.User

	err := s.db.QueryRow(ctx, query, login, passwordHash).Scan(&user.ID, &user.Login, &user.Password)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}

	return &user, nil
}

func (s *Store) GetOrderByNumber(ctx context.Context, orderNumber string) (*models.Order, error) {
	query := `SELECT  FROM orders WHERE order_number = $1`
	var order models.Order

	err := s.db.QueryRow(ctx, query, orderNumber).Scan(&order.ID, &order.UserID, &order.Status, &order.Accrual, &order.UploadedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("order not found")
		}
		return nil, fmt.Errorf("query error: %w", err)
	}

	return &order, nil
}

func (s *Store) CreateOrder(ctx context.Context, userID int, orderNumber string) error {
	query := `INSERT INTO orders (user_id, order_number, status) VALUES ($1, $2, 'NEW')`

	_, err := s.db.Exec(ctx, query, userID, orderNumber)
	if err != nil {
		return fmt.Errorf("query error: %w", err)
	}

	return nil
}

func (s *Store) GetOrderByUserID(ctx context.Context, userID int) ([]models.Order, error) {
	query := `SELECT * FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	var orders []models.Order

	for rows.Next() {
		var order models.Order
		if err := rows.Scan(&order.ID, &order.UserID, &order.Status, &order.Accrual, &order.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return orders, nil
}
