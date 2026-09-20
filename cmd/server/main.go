package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/app"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/migrations"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/config"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/database"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/logger"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/storage"
)

// @title XenoChoice Sandbox API («Машина выбора»)
// @version 2.0
// @description Авторитетный детерминированный движок симуляции небиологических сообществ на реальных планетах (Земля, Марс, Венера) по ТЗ v2 и ТЗ v1
// @termsOfService http://swagger.io/terms/
// @contact.name XenoChoice Team
// @contact.email support@xenochoice.io
// @license.name MIT
// @host localhost:8080
// @BasePath /
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logr := logger.NewLogger(cfg.App.Env)

	db, err := database.NewPostgres(cfg, logr)
	if err != nil {
		logr.Fatal().Err(err).Msg("Database initialization failed")
	}
	defer db.Close()

	if err := database.RunMigrations(db.DB, migrations.EmbedMigrations, logr); err != nil {
		logr.Fatal().Err(err).Msg("Failed to run database migrations")
	}

	rdb, err := database.NewRedis(cfg, logr)
	if err != nil {
		logr.Fatal().Err(err).Msg("Redis initialization failed")
	}
	defer rdb.Close()

	s3, err := storage.NewS3Client(cfg, logr)
	if err != nil {
		logr.Fatal().Err(err).Msg("S3 initialization failed")
	}

	application := app.NewApp(cfg, db, rdb, s3, logr)
	app.SetupRoutes(application)

	go func() {
		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		logr.Info().Msgf("Starting server on %s", addr)
		if err := application.Fiber.Listen(addr); err != nil {
			logr.Fatal().Err(err).Msg("Server forced to shutdown")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logr.Info().Msg("Shutting down server...")

	if err := application.Fiber.ShutdownWithContext(context.Background()); err != nil {
		logr.Fatal().Err(err).Msg("Server shutdown error")
	}

	logr.Info().Msg("Server exiting")
}
