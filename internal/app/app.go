package app

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/config"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/middleware"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/response"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/storage"
)

type App struct {
	Fiber *fiber.App
	DB    *sqlx.DB
	RDB   *redis.Client
	S3    *storage.S3Storage
	Log   zerolog.Logger
	Cfg   *config.Config
}

func NewApp(cfg *config.Config, db *sqlx.DB, rdb *redis.Client, s3 *storage.S3Storage, log zerolog.Logger) *App {
	fiberApp := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return response.Error(c, code, "HTTP_ERROR", err.Error())
		},
		BodyLimit:    10 * 1024 * 1024, // 10MB
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	})

	fiberApp.Use(middleware.NewRecoverMiddleware(log))
	fiberApp.Use(middleware.NewCorsMiddleware())
	fiberApp.Use(middleware.NewLoggerMiddleware(log))
	fiberApp.Use(middleware.NewRateLimitMiddleware(100, 1*time.Minute))
	fiberApp.Use(middleware.NewPrometheusMiddleware(fiberApp, cfg.App.Name))

	return &App{
		Fiber: fiberApp,
		DB:    db,
		RDB:   rdb,
		S3:    s3,
		Log:   log,
		Cfg:   cfg,
	}
}
