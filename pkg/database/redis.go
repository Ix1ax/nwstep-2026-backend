package database

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/config"
)

func NewRedis(cfg *config.Config, log zerolog.Logger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Error().Err(err).Msg("Failed to connect to Redis")
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	log.Info().Msg("Successfully connected to Redis")
	return client, nil
}
