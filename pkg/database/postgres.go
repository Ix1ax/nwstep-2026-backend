package database

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/config"
)

func NewPostgres(cfg *config.Config, log zerolog.Logger) (*sqlx.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode)

	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)

	// Retry mechanism for ping
	for i := 0; i < 3; i++ {
		if err = db.Ping(); err == nil {
			log.Info().Msg("Successfully connected to Postgres")
			return db, nil
		}
		log.Warn().Err(err).Msgf("Failed to ping database, retrying... (%d/3)", i+1)
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("could not connect to postgres after retries: %w", err)
}
