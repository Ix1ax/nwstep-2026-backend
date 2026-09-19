package user

import (
	"github.com/gofiber/fiber/v2"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/auth"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/response"
)

type UserHandler struct {
	service *UserService
}

func NewUserHandler(service *UserService) *UserHandler {
	return &UserHandler{service: service}
}

// GetUser godoc
// @Summary Get user by ID
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} response.Response{data=User}
// @Failure 404 {object} response.Response
// @Router /users/{id} [get]
func (h *UserHandler) GetUser(c *fiber.Ctx) error {
	id := c.Params("id")
	user, err := h.service.GetUser(c.Context(), id)
	if err != nil {
		return response.NotFound(c, "user not found")
	}

	return response.OK(c, user)
}

// UpdateUser godoc
// @Summary Update user
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param request body UpdateUserRequest true "Update details"
// @Success 200 {object} response.Response{data=User}
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /users/{id} [put]
func (h *UserHandler) UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := auth.GetUserID(c)

	if id != userID {
		return response.Error(c, fiber.StatusForbidden, "FORBIDDEN", "you can only update your own profile")
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	user, err := h.service.UpdateUser(c.Context(), id, req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.OK(c, user)
}

// ListUsers godoc
// @Summary List users
// @Tags users
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} response.Response{data=[]User,meta=response.Meta}
// @Router /users [get]
func (h *UserHandler) ListUsers(c *fiber.Ctx) error {
	var query PaginationQuery
	if err := c.QueryParser(&query); err != nil {
		query = PaginationQuery{Page: 1, Limit: 10}
	}

	users, count, err := h.service.ListUsers(c.Context(), query)
	if err != nil {
		return response.InternalError(c, "failed to list users")
	}

	return response.Paginated(c, users, count, query.Page, query.Limit)
}

