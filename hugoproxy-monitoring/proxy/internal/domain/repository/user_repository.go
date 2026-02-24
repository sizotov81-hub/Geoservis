package repository

import (
	"context"
	"errors"

	"gitlab.com/s.izotov81/hugoproxy/internal/domain/entity"
)

var (
	// ErrUserNotFound ошибка при отсутствии пользователя
	ErrUserNotFound = errors.New("user not found")
	// ErrUserAlreadyExists ошибка при попытке создания существующего пользователя
	ErrUserAlreadyExists = errors.New("user already exists")
)

// UserRepository определяет интерфейс для работы с пользователями
type UserRepository interface {
	Create(ctx context.Context, user entity.User) error
	GetByID(ctx context.Context, id int) (entity.User, error)
	GetByEmail(ctx context.Context, email string) (entity.User, error)
	Update(ctx context.Context, user entity.User) error
	Delete(ctx context.Context, id int) error
	List(ctx context.Context, limit, offset int) ([]entity.User, error)
}
