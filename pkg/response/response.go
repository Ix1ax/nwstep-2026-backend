package response

import (
	"github.com/gofiber/fiber/v2"
)

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Meta struct {
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

func OK(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Success: true,
		Data:    data,
	})
}

func Created(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(Response{
		Success: true,
		Data:    data,
	})
}

func Paginated(c *fiber.Ctx, data interface{}, total, page, limit int) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Success: true,
		Data:    data,
		Meta: &Meta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

func Error(c *fiber.Ctx, status int, code, msg string) error {
	return c.Status(status).JSON(Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: msg,
		},
	})
}

func BadRequest(c *fiber.Ctx, msg string) error {
	return Error(c, fiber.StatusBadRequest, "BAD_REQUEST", msg)
}

func Unauthorized(c *fiber.Ctx, msg string) error {
	return Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", msg)
}

func NotFound(c *fiber.Ctx, msg string) error {
	return Error(c, fiber.StatusNotFound, "NOT_FOUND", msg)
}

func InternalError(c *fiber.Ctx, msg string) error {
	return Error(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", msg)
}
