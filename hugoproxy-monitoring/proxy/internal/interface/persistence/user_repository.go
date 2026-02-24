package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"gitlab.com/s.izotov81/hugoproxy/internal/domain/entity"
	"gitlab.com/s.izotov81/hugoproxy/internal/domain/repository"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/db/adapter"
)

// UserRepositoryImpl реализует repository.UserRepository
type UserRepositoryImpl struct {
	adapter *adapter.SQLAdapter
	db      *sqlx.DB
}

// NewUserRepository создает новый экземпляр UserRepository
func NewUserRepository(adapter *adapter.SQLAdapter, db *sqlx.DB) repository.UserRepository {
	return &UserRepositoryImpl{
		adapter: adapter,
		db:      db,
	}
}

func (r *UserRepositoryImpl) Create(ctx context.Context, user entity.User) error {
	// Проверяем существование пользователя
	_, err := r.GetByEmail(ctx, user.Email)
	if err == nil {
		return repository.ErrUserAlreadyExists
	}
	if !errors.Is(err, repository.ErrUserNotFound) {
		return fmt.Errorf("failed to check user existence: %w", err)
	}

	query := `
		INSERT INTO users (email, password_hash, created_at, updated_at)
		VALUES (:email, :password_hash, :created_at, :updated_at)
	`

	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	_, err = r.db.NamedExecContext(ctx, query, user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (r *UserRepositoryImpl) GetByID(ctx context.Context, id int) (entity.User, error) {
	var user entity.User
	query := `SELECT * FROM users WHERE id = $1`

	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.User{}, repository.ErrUserNotFound
		}
		return entity.User{}, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return user, nil
}

func (r *UserRepositoryImpl) GetByEmail(ctx context.Context, email string) (entity.User, error) {
	var user entity.User
	query := `SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL`

	err := r.db.GetContext(ctx, &user, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.User{}, repository.ErrUserNotFound
		}
		return entity.User{}, fmt.Errorf("failed to get user by email: %w", err)
	}

	return user, nil
}

func (r *UserRepositoryImpl) Update(ctx context.Context, user entity.User) error {
	user.UpdatedAt = time.Now()
	query := `
		UPDATE users
		SET email = :email,
		    password_hash = :password_hash,
		    updated_at = NOW()
		WHERE id = :id AND deleted_at IS NULL
	`

	result, err := r.db.NamedExecContext(ctx, query, user)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return repository.ErrUserNotFound
	}

	return nil
}

func (r *UserRepositoryImpl) Delete(ctx context.Context, id int) error {
	query := `
		UPDATE users
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return repository.ErrUserNotFound
	}

	return nil
}

func (r *UserRepositoryImpl) List(ctx context.Context, limit, offset int) ([]entity.User, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	var users []entity.User
	query := `SELECT * FROM users WHERE deleted_at IS NULL ORDER BY id LIMIT $1 OFFSET $2`

	err := r.db.SelectContext(ctx, &users, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return users, nil
}
