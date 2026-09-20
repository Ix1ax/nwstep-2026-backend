package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const (
	defaultCacheTTL  = 10 * time.Minute
	worldsCacheTTL   = 1 * time.Hour
	keyPrefix        = "xeno:"
	activeSetKey     = "xeno:active_experiments"
	telemetryChannel = "xeno:telemetry"
)

type RedisCache struct {
	client *redis.Client
	log    zerolog.Logger
}

func NewRedisCache(client *redis.Client, log zerolog.Logger) *RedisCache {
	return &RedisCache{
		client: client,
		log:    log.With().Str("component", "redis_cache").Logger(),
	}
}

// Client returns the underlying redis.Client
func (c *RedisCache) Client() *redis.Client {
	return c.client
}

// CacheSnapshot stores the latest state snapshot in Redis
func (c *RedisCache) CacheSnapshot(ctx context.Context, expID string, snap *model.StateSnapshot) error {
	if c.client == nil || snap == nil {
		return nil
	}

	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("%sexp:%s:snapshot", keyPrefix, expID)
	return c.client.Set(ctx, key, data, defaultCacheTTL).Err()
}

// GetCachedSnapshot retrieves the cached state snapshot from Redis
func (c *RedisCache) GetCachedSnapshot(ctx context.Context, expID string) (*model.StateSnapshot, error) {
	if c.client == nil {
		return nil, errors.New("redis client unavailable")
	}

	key := fmt.Sprintf("%sexp:%s:snapshot", keyPrefix, expID)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var snap model.StateSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}

	return &snap, nil
}

// CacheMetrics stores the metrics history slice in Redis
func (c *RedisCache) CacheMetrics(ctx context.Context, expID string, metrics []*model.MetricsSnapshot) error {
	if c.client == nil || len(metrics) == 0 {
		return nil
	}

	data, err := json.Marshal(metrics)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("%sexp:%s:metrics", keyPrefix, expID)
	return c.client.Set(ctx, key, data, defaultCacheTTL).Err()
}

// GetCachedMetrics retrieves the cached metrics history slice from Redis
func (c *RedisCache) GetCachedMetrics(ctx context.Context, expID string) ([]*model.MetricsSnapshot, error) {
	if c.client == nil {
		return nil, errors.New("redis client unavailable")
	}

	key := fmt.Sprintf("%sexp:%s:metrics", keyPrefix, expID)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var metrics []*model.MetricsSnapshot
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, err
	}

	return metrics, nil
}

// CacheWorlds caches the preset worlds list
func (c *RedisCache) CacheWorlds(ctx context.Context, worlds []model.World) error {
	if c.client == nil || len(worlds) == 0 {
		return nil
	}

	data, err := json.Marshal(worlds)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("%sworlds", keyPrefix)
	return c.client.Set(ctx, key, data, worldsCacheTTL).Err()
}

// GetCachedWorlds retrieves the preset worlds list from Redis
func (c *RedisCache) GetCachedWorlds(ctx context.Context) ([]model.World, error) {
	if c.client == nil {
		return nil, errors.New("redis client unavailable")
	}

	key := fmt.Sprintf("%sworlds", keyPrefix)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var worlds []model.World
	if err := json.Unmarshal(data, &worlds); err != nil {
		return nil, err
	}

	return worlds, nil
}

// PublishSnapshot broadcasts a simulation snapshot over Redis Pub/Sub
func (c *RedisCache) PublishSnapshot(ctx context.Context, expID string, payload []byte) error {
	if c.client == nil {
		return nil
	}

	channel := fmt.Sprintf("%sstream:%s", keyPrefix, expID)
	if err := c.client.Publish(ctx, channel, payload).Err(); err != nil {
		c.log.Debug().Err(err).Str("channel", channel).Msg("Failed to publish snapshot to Redis")
		return err
	}

	// Also publish to global channel for cluster listeners
	_ = c.client.Publish(ctx, fmt.Sprintf("%sstream:all", keyPrefix), payload).Err()
	return nil
}

// SubscribeSnapshot creates a PubSub subscription to an experiment stream
func (c *RedisCache) SubscribeSnapshot(ctx context.Context, expID string) *redis.PubSub {
	if c.client == nil {
		return nil
	}

	channel := fmt.Sprintf("%sstream:%s", keyPrefix, expID)
	return c.client.Subscribe(ctx, channel)
}

// TrackActive tracks which experiments are currently running
func (c *RedisCache) TrackActive(ctx context.Context, expID string, active bool) error {
	if c.client == nil {
		return nil
	}

	if active {
		return c.client.SAdd(ctx, activeSetKey, expID).Err()
	}
	return c.client.SRem(ctx, activeSetKey, expID).Err()
}

// GetActiveExperiments returns the list of currently active running experiment IDs
func (c *RedisCache) GetActiveExperiments(ctx context.Context) ([]string, error) {
	if c.client == nil {
		return nil, nil
	}

	return c.client.SMembers(ctx, activeSetKey).Result()
}

// PublishTelemetry broadcasts v1 colony telemetry over Redis
func (c *RedisCache) PublishTelemetry(ctx context.Context, payload []byte) error {
	if c.client == nil {
		return nil
	}

	return c.client.Publish(ctx, telemetryChannel, payload).Err()
}

// InvalidateExperiment removes cached keys for an experiment
func (c *RedisCache) InvalidateExperiment(ctx context.Context, expID string) error {
	if c.client == nil {
		return nil
	}

	snapKey := fmt.Sprintf("%sexp:%s:snapshot", keyPrefix, expID)
	metricsKey := fmt.Sprintf("%sexp:%s:metrics", keyPrefix, expID)
	c.client.Del(ctx, snapKey, metricsKey)
	c.client.SRem(ctx, activeSetKey, expID)
	return nil
}
