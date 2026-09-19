package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

func NewLoggerMiddleware(log zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		
		err := c.Next()
		
		latency := time.Since(start)
		
		event := log.Info()
		if err != nil {
			event = log.Error().Err(err)
		} else if c.Response().StatusCode() >= 400 {
			event = log.Warn()
		}

		event.
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", c.Response().StatusCode()).
			Dur("latency", latency).
			Str("ip", c.IP()).
			Msg("HTTP Request")
			
		return err
	}
}
