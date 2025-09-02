package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gitlab.com/s.izotov81/hugoproxy/internal/core/entity"
	"gitlab.com/s.izotov81/hugoproxy/internal/core/service"
	"gitlab.com/s.izotov81/hugoproxy/pkg/responder"
)

type UserController struct {
	userService *service.UserService
	responder   responder.Responder
}

func NewUserController(userService *service.UserService, responder responder.Responder) *UserController {
	return &UserController{
		userService: userService,
		responder:   responder,
	}
}

// RegisterUser godoc
// @Summary Register a new user
// @Description Register a new user with email and password
// @Tags users
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param Authorization header string true "Токен авторизации" default(Bearer <ТОКЕН>)
// @Param request body entity.CreateUserRequest true "User registration data"
// @Success 201 {object} entity.User
// @Failure 400 {object} responder.ErrorResponse
// @Failure 409 {object} responder.ErrorResponse
// @Failure 500 {object} responder.ErrorResponse
// @Router /api/users [post]
func (c *UserController) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var user entity.User
	if err := c.responder.Decode(r, &user); err != nil {
		c.responder.Error(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	err := c.userService.Register(r.Context(), user.Email, user.PasswordHash)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrUserAlreadyExists) {
			status = http.StatusConflict
		}
		c.responder.Error(w, status, err.Error())
		return
	}

	c.responder.Respond(w, http.StatusCreated, nil)
}

// GetUser godoc
// @Summary Get user by ID
// @Description Get user details by ID
// @Tags users
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "User ID"
// @Success 200 {object} entity.User
// @Failure 400 {object} responder.ErrorResponse
// @Failure 404 {object} responder.ErrorResponse
// @Failure 500 {object} responder.ErrorResponse
// @Router /api/users/{id} [get]
func (c *UserController) GetUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.responder.Error(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	user, err := c.userService.GetUser(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.responder.Error(w, status, err.Error())
		return
	}

	c.responder.Respond(w, http.StatusOK, user)
}

// ListUsers godoc
// @Summary List users
// @Description Get list of users with pagination
// @Tags users
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param limit query int false "Limit" default(10)
// @Param offset query int false "Offset" default(0)
// @Success 200 {array} entity.User
// @Failure 400 {object} responder.ErrorResponse
// @Failure 500 {object} responder.ErrorResponse
// @Router /api/users [get]
func (c *UserController) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	users, err := c.userService.ListUsers(r.Context(), limit, offset)
	if err != nil {
		c.responder.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	c.responder.Respond(w, http.StatusOK, users)
}

// UpdateUser godoc
// @Summary Update user
// @Description Update user details
// @Tags users
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "User ID"
// @Param request body entity.UpdateUserRequest true "User data"
// @Success 200 {object} entity.User
// @Failure 400 {object} responder.ErrorResponse
// @Failure 404 {object} responder.ErrorResponse
// @Failure 500 {object} responder.ErrorResponse
// @Router /api/users/{id} [put]
func (c *UserController) UpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.responder.Error(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var user entity.User
	if err := c.responder.Decode(r, &user); err != nil {
		c.responder.Error(w, http.StatusBadRequest, "Invalid request format")
		return
	}
	user.ID = id

	err = c.userService.UpdateUser(r.Context(), user)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.responder.Error(w, status, err.Error())
		return
	}

	c.responder.Respond(w, http.StatusOK, user)
}

// DeleteUser godoc
// @Summary Delete user
// @Description Soft delete a user
// @Tags users
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "User ID"
// @Success 204 "No Content"
// @Failure 400 {object} responder.ErrorResponse
// @Failure 404 {object} responder.ErrorResponse
// @Failure 500 {object} responder.ErrorResponse
// @Router /api/users/{id} [delete]
func (c *UserController) DeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.responder.Error(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	err = c.userService.DeleteUser(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.responder.Error(w, status, err.Error())
		return
	}

	c.responder.Respond(w, http.StatusNoContent, nil)
}

// GetUserByEmail godoc
// @Summary Get user by email
// @Description Get user details by email address
// @Tags users
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param email query string true "User email"
// @Success 200 {object} entity.User
// @Failure 400 {object} responder.ErrorResponse
// @Failure 404 {object} responder.ErrorResponse
// @Failure 500 {object} responder.ErrorResponse
// @Router /api/users/email [get]
func (c *UserController) GetUserByEmail(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		c.responder.Error(w, http.StatusBadRequest, "Email parameter is required")
		return
	}

	user, err := c.userService.GetUserByEmail(r.Context(), email)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.responder.Error(w, status, err.Error())
		return
	}

	c.responder.Respond(w, http.StatusOK, user)
}
