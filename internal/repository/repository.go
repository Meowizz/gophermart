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
