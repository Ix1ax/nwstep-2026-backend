package app

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
	_ "github.com/ix1ax/nwstep-hackaton-2026/golang/docs" // Ignore if not generated yet

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/auth"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/choice"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/colony"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/upload"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/user"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/ws"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/response"
)

func SetupRoutes(app *App) {
	// Swagger
	app.Fiber.Get("/swagger/*", swagger.HandlerDefault)

	// Health and Ready
	app.Fiber.Get("/health", func(c *fiber.Ctx) error {
		return response.OK(c, fiber.Map{"status": "ok"})
	})

	app.Fiber.Get("/ready", func(c *fiber.Ctx) error {
		if err := app.DB.Ping(); err != nil {
			return response.InternalError(c, "database not ready")
		}
		if err := app.RDB.Ping(c.Context()).Err(); err != nil {
			return response.InternalError(c, "redis not ready")
		}
		return response.OK(c, fiber.Map{"status": "ready"})
	})

	// API v1
	v1 := app.Fiber.Group("/api/v1")

	// Middleware
	authMiddleware := auth.JWTMiddleware(app.Cfg.JWT.Secret)

	// Boilerplate Modules
	authModule := auth.NewModule(app.DB, app.RDB, app.Cfg, app.Log)
	authModule.Register(v1, authMiddleware)

	userModule := user.NewModule(app.DB, app.Log)
	userModule.Register(v1, authMiddleware)

	wsHub := ws.NewHub()
	wsModule := ws.NewModule(wsHub, app.Log)
	wsModule.Register(v1, app.Cfg.JWT.Secret)

	uploadModule := upload.NewModule(app.DB, app.S3, app.Log)
	uploadModule.Register(v1, authMiddleware)

	// 🪐 XenoChoice Domain Modules
	colonyModule := colony.NewModule(app.Log)
	colonyModule.Register(v1)

	choiceModule := choice.NewModule(colonyModule.Service, app.Log)
	choiceModule.Register(v1)

	// 🔬 XenoChoice Sandbox Engine («Машина выбора» по ТЗ v1 и v2)
	xenoModule := xeno.NewModule(app.Log)
	xenoModule.Register(v1)

	// API v2 (ТЗ v2: миры, особи, колонии, replay)
	v2 := app.Fiber.Group("/api/v2")
	xenoModule.RegisterV2(v2)

	// Background simulation ticker: advances physical state and broadcasts telemetry to WebSockets
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			colonyModule.Service.StepSimulation()

			// Broadcast real-time telemetry to all connected research clients
			telemetry := fiber.Map{
				"type":        "TELEMETRY_UPDATE",
				"colonies":    colonyModule.Service.GetColonies(),
				"environment": colonyModule.Service.GetEnvironment(),
			}

			if b, err := json.Marshal(telemetry); err == nil {
				wsHub.Broadcast(b)
			}
		}
	}()
}
