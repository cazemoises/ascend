package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateUser always creates students; roles are promoted out-of-band
// (SQL/admin tooling), never via the public register endpoint.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES (LOWER($1), $2)
		 RETURNING id, email, role, created_at, updated_at`,
		email, passwordHash)

	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return User{}, ErrConflict
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, string, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, role, password_hash, created_at, updated_at
		 FROM users WHERE email = LOWER($1)`,
		email)

	var u User
	var passwordHash string
	if err := row.Scan(&u.ID, &u.Email, &u.Role, &passwordHash, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, "", ErrNotFound
		}
		return User{}, "", fmt.Errorf("get user by email: %w", err)
	}
	return u, passwordHash, nil
}
