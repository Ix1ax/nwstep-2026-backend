package database

import (
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
)

func RunMigrations(db *sql.DB, fsys fs.FS, log zerolog.Logger) error {
	goose.SetBaseFS(fsys)
	defer goose.SetBaseFS(nil)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	log.Info().Msg("Applying database migrations...")
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Info().Msg("Database migrations applied successfully")
	return nil
}
