package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

func NewRecoverMiddleware(log zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Msgf("Panic recovered: %v", r)
				
				err := c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"error": fiber.Map{
						"code":    "INTERNAL_ERROR",
						"message": fmt.Sprintf("Internal Server Error: %v", r),
					},
				})
				if err != nil {
					log.Error().Err(err).Msg("Failed to write recovery response")
				}
			}
		}()
		return c.Next()
	}
}
