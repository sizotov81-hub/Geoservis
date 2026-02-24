package main

// User представляет модель пользователя системы
// @Description Информация о пользователе системы
type User struct {
	Email        string `json:"email" example:"user@example.com"` // Email пользователя
	PasswordHash string `json:"-"`                                // Хэш пароля (не возвращается в ответах)
}

// RegisterRequest представляет данные для регистрации пользователя
// @Description Данные для регистрации нового пользователя
type RegisterRequest struct {
	Email    string `json:"email" example:"user@example.com"`    // Email пользователя
	Password string `json:"password" example:"securepassword123"` // Пароль пользователя
}

// LoginRequest представляет данные для входа пользователя
// @Description Данные для входа пользователя
type LoginRequest struct {
	Email    string `json:"email" example:"user@example.com"`    // Email пользователя
	Password string `json:"password" example:"securepassword123"` // Пароль пользователя
}

// LoginResponse представляет ответ с JWT токеном
// @Description Ответ сервера с JWT токеном после успешной аутентификации
type LoginResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."` // JWT токен
}

// ErrorResponse представляет стандартный ответ об ошибке
// @Description Стандартный формат ответа при возникновении ошибки
type ErrorResponse struct {
	Error string `json:"error" example:"error message"` // Описание ошибки
}
