package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Meowizz/gophermart/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var ErrInsufficientFunds = errors.New("insufficient funds")

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
	query := `SELECT number, user_id, status, accrual, uploaded_at FROM orders WHERE number = $1`
	var order models.Order

	err := s.db.QueryRow(ctx, query, orderNumber).Scan(&order.Number, &order.UserID, &order.Status, &order.Accrual, &order.UploadedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("order not found")
		}
		return nil, fmt.Errorf("query error: %w", err)
	}

	return &order, nil
}

func (s *Store) CreateOrder(ctx context.Context, userID int, orderNumber string) error {
	query := `INSERT INTO orders (user_id, number,status) VALUES ($1, $2, 'NEW')`

	_, err := s.db.Exec(ctx, query, userID, orderNumber)
	if err != nil {
		return fmt.Errorf("query error: %w", err)
	}

	return nil
}

func (s *Store) GetOrderByUserID(ctx context.Context, userID int) ([]models.Order, error) {
	query := `SELECT number, user_id, status, accrual, uploaded_at FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	var orders []models.Order

	for rows.Next() {
		var order models.Order
		if err := rows.Scan(&order.Number, &order.UserID, &order.Status, &order.Accrual, &order.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return orders, nil
}

func (s *Store) GetBalance(ctx context.Context, userID int) (*models.Balance, error) {
	query := `SELECT user_id, current, withdrawn FROM balances WHERE user_id = $1`

	var balance models.Balance
	err := s.db.QueryRow(ctx, query, userID).Scan(&balance.UserID, &balance.Current, &balance.Withdrawn)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &models.Balance{
				UserID:    userID,
				Current:   decimal.Zero,
				Withdrawn: decimal.Zero,
			}, nil
		}
		return nil, fmt.Errorf("query error: %w", err)
	}
	return &balance, nil
}

func (s *Store) WithdrawBalance(ctx context.Context, userID int, amount decimal.Decimal, orderNumber string) error {

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction error: %w", err)
	}
	defer tx.Rollback(ctx)

	updateQuery := `UPDATE balances SET current = current - $1, withdrawn = withdrawn + $1 WHERE user_id = $2 and current >= $1`
	res, err := tx.Exec(ctx, updateQuery, amount, userID)
	if err != nil {
		return fmt.Errorf("update balance error: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrInsufficientFunds
	}

	insertQuery := `INSERT INTO withdrawals (user_id, order_number, sum, processed_at) VALUES ($1, $2, $3, CURRENT_TIMESTAMP)`
	_, err = tx.Exec(ctx, insertQuery, userID, orderNumber, amount)
	if err != nil {
		return fmt.Errorf("insert order error: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *Store) GetWithdrawals(ctx context.Context, userID int) ([]models.Withdrawal, error) {
	query := `SELECT order_number, sum, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC`
	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	var withdrawals []models.Withdrawal
	for rows.Next() {
		var w models.Withdrawal
		if err := rows.Scan(&w.OrderNumber, &w.Sum, &w.ProcessedAt); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		withdrawals = append(withdrawals, w)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return withdrawals, nil
}
