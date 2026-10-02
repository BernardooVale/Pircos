package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pircos/api/internal/domain"
)

// UserRepo handles user persistence.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo creates a new UserRepo instance.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// GetOrCreateDefault retrieves or creates the default single user for local operation.
func (r *UserRepo) GetOrCreateDefault(ctx context.Context) (*domain.User, error) {
	defaultEmail := "user@pircos.local"

	var user domain.User
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email)
		VALUES ($1)
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id, email, created_at
	`, defaultEmail).Scan(&user.ID, &user.Email, &user.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("getting or creating default user: %w", err)
	}

	return &user, nil
}

// GetByID retrieves a user by ID.
func (r *UserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	var createdAt time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, created_at
		FROM users
		WHERE id = $1
	`, id).Scan(&user.ID, &user.Email, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("getting user by id %s: %w", id, err)
	}
	user.CreatedAt = createdAt
	return &user, nil
}
