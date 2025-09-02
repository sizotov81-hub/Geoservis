package entity

import "time"

// User представляет модель пользователя системы
type User struct {
	ID           int        `json:"id" db:"id" example:"1"`                      // Пример ID пользователя
	Email        string     `json:"email" db:"email" example:"user@example.com"` // Пример email пользователя
	PasswordHash string     `json:"-" db:"password_hash"`                        // Это поле не будет включено в JSON
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`                  // Время создания
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`                  // Время обновления
	DeletedAt    *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`        // Время удаления
}

type UpdateUserRequest struct {
	Email    string `json:"email" example:"newemail@example.com"`
	Password string `json:"password" example:"newpassword123"`
}

type CreateUserRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"password123"`
}
