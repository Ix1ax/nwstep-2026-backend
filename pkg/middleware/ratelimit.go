package middleware

import (
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

func NewRateLimitMiddleware(max int, expiration time.Duration) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        max,
		Expiration: expiration,
		Next: func(c *fiber.Ctx) bool {
			// Don't rate limit healthchecks, metrics, swagger, and websocket upgrades
			path := c.Path()
			if path == "/health" || path == "/ready" || path == "/metrics" || path == "/swagger" {
				return true
			}
			return websocket.IsWebSocketUpgrade(c)
		},
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP() // Per-IP rate limiting
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "Too many requests, please try again later.",
				},
			})
		},
	})
}
