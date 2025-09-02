package service

import (
	"context"
	"errors"

	"gitlab.com/s.izotov81/hugoproxy/internal/core/entity"
	"gitlab.com/s.izotov81/hugoproxy/internal/core/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound      = repository.ErrUserNotFound
	ErrUserAlreadyExists = repository.ErrUserAlreadyExists
)

type UserService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Register(ctx context.Context, email, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := entity.User{
		Email:        email,
		PasswordHash: string(hashedPassword),
	}

	return s.repo.Create(ctx, user)
}

func (s *UserService) Login(ctx context.Context, email, password string) (entity.User, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return entity.User{}, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return entity.User{}, errors.New("invalid credentials")
	}

	return user, nil
}

func (s *UserService) GetUser(ctx context.Context, id int) (entity.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) UpdateUser(ctx context.Context, user entity.User) error {
	return s.repo.Update(ctx, user)
}

func (s *UserService) DeleteUser(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

func (s *UserService) ListUsers(ctx context.Context, limit, offset int) ([]entity.User, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (entity.User, error) {
	return s.repo.GetByEmail(ctx, email)
}
