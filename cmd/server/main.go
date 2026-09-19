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

// @title NWSTEP Hackathon 2026 API
// @version 1.0
// @description Hackathon boilerplate API documentation
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.email support@nwstep.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
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
