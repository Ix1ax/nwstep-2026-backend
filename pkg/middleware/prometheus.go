package middleware

import (
	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
)

func NewPrometheusMiddleware(app *fiber.App, serviceName string) fiber.Handler {
	prometheus := fiberprometheus.New(serviceName)
	prometheus.RegisterAt(app, "/metrics")
	return prometheus.Middleware
}
