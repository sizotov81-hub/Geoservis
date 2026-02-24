package user

import (
	"context"
	"errors"
	"fmt"

	"gitlab.com/s.izotov81/hugoproxy/internal/domain/entity"
	"gitlab.com/s.izotov81/hugoproxy/internal/domain/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrUserNotFound ошибка при отсутствии пользователя
	ErrUserNotFound = repository.ErrUserNotFound
	// ErrUserAlreadyExists ошибка при попытке создания существующего пользователя
	ErrUserAlreadyExists = repository.ErrUserAlreadyExists
	// ErrInvalidCredentials ошибка при неверных учетных данных
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// UserService предоставляет сервис для управления пользователями
type UserService struct {
	repo repository.UserRepository
}

// NewUserService создает новый экземпляр UserService
func NewUserService(repo repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// Register регистрирует нового пользователя
func (s *UserService) Register(ctx context.Context, email, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := entity.User{
		Email:        email,
		PasswordHash: string(hashedPassword),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			return ErrUserAlreadyExists
		}
		return fmt.Errorf("failed to create user in repository: %w", err)
	}

	return nil
}

// Login аутентифицирует пользователя
func (s *UserService) Login(ctx context.Context, email, password string) (entity.User, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return entity.User{}, ErrInvalidCredentials
		}
		return entity.User{}, fmt.Errorf("failed to get user by email: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return entity.User{}, ErrInvalidCredentials
	}

	return user, nil
}

// GetUser получает пользователя по ID
func (s *UserService) GetUser(ctx context.Context, id int) (entity.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return entity.User{}, ErrUserNotFound
		}
		return entity.User{}, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return user, nil
}

// UpdateUser обновляет данные пользователя
func (s *UserService) UpdateUser(ctx context.Context, user entity.User) error {
	if err := s.repo.Update(ctx, user); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to update user in repository: %w", err)
	}

	return nil
}

// DeleteUser удаляет пользователя
func (s *UserService) DeleteUser(ctx context.Context, id int) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to delete user in repository: %w", err)
	}

	return nil
}

// ListUsers возвращает список пользователей с пагинацией
func (s *UserService) ListUsers(ctx context.Context, limit, offset int) ([]entity.User, error) {
	users, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return users, nil
}

// GetUserByEmail получает пользователя по email
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (entity.User, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return entity.User{}, ErrUserNotFound
		}
		return entity.User{}, fmt.Errorf("failed to get user by email: %w", err)
	}

	return user, nil
}
