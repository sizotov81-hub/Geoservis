package responder

import (
	"encoding/json"
	"net/http"
)

// Responder определяет интерфейс для отправки ответов
type Responder interface {
	Respond(w http.ResponseWriter, status int, data interface{})
	Error(w http.ResponseWriter, status int, message string)
	Decode(r *http.Request, v interface{}) error
}

// ErrorResponse представляет стандартный ответ об ошибке
type ErrorResponse struct {
	Error string `json:"error"`
}

// JSONResponder реализует Responder для JSON ответов
type JSONResponder struct{}

// NewJSONResponder создает новый JSONResponder
func NewJSONResponder() *JSONResponder {
	return &JSONResponder{}
}

// Respond отправляет успешный JSON ответ
func (j *JSONResponder) Respond(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Error отправляет JSON ответ с ошибкой
func (j *JSONResponder) Error(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// Decode декодирует тело запроса в структуру
func (j *JSONResponder) Decode(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
