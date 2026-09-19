// Standalone research server for local development and demonstrations.
package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno"
	"github.com/rs/zerolog"
	"os"
)

func main() {
	app := fiber.New(fiber.Config{BodyLimit: 20 * 1024 * 1024})
	app.Use(recover.New())
	module := xeno.NewModule(zerolog.New(os.Stdout))
	module.RegisterV2(app.Group("/api/v2"))
	app.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })
	address := os.Getenv("LAB_ADDR")
	if address == "" {
		address = "127.0.0.1:8081"
	}
	if err := app.Listen(address); err != nil {
		panic(err)
	}
}
