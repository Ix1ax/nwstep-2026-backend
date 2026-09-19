package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/response"
)

type AuthHandler struct {
	service *AuthService
}

func NewAuthHandler(service *AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

// Register godoc
// @Summary Register a new user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Register details"
// @Success 201 {object} response.Response{data=TokenResponse}
// @Failure 400 {object} response.Response
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	
	tokens, err := h.service.Register(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	
	return response.Created(c, tokens)
}

// Login godoc
// @Summary Login
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login details"
// @Success 200 {object} response.Response{data=TokenResponse}
// @Failure 401 {object} response.Response
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	
	tokens, err := h.service.Login(c.Context(), req)
	if err != nil {
		return response.Unauthorized(c, err.Error())
	}
	
	return response.OK(c, tokens)
}

// RefreshToken godoc
// @Summary Refresh access token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh token"
// @Success 200 {object} response.Response{data=TokenResponse}
// @Failure 401 {object} response.Response
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	
	tokens, err := h.service.RefreshToken(c.Context(), req)
	if err != nil {
		return response.Unauthorized(c, err.Error())
	}
	
	return response.OK(c, tokens)
}

// Me godoc
// @Summary Get current user
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=UserResponse}
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userID := GetUserID(c)
	user, err := h.service.GetCurrentUser(c.Context(), userID)
	if err != nil {
		return response.NotFound(c, "user not found")
	}
	return response.OK(c, user)
}
