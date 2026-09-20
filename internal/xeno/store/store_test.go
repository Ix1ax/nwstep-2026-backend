package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/store"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func setupTestDB(t *testing.T) *sqlx.DB {
	db, err := sqlx.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=nwstep sslmode=disable")
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("PostgreSQL ping failed: %v", err)
	}
	return db
}

func setupTestRedis(t *testing.T) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("Redis ping failed: %v", err)
	}
	return rdb
}

func TestPostgresRepository_CRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := store.NewPostgresRepository(db, zerolog.Nop())
	ctx := context.Background()

	testExp := &store.PersistedExperiment{
		ID:         "test-exp-pg-1",
		Name:       "Test Postgres Exp",
		WorldID:    "earth",
		Mode:       model.ModeEvolutionary,
		Status:     model.StatusReady,
		Seed:       12345,
		Speed:      1,
		Parameters: model.DefaultParameters(),
		InitialSnapshot: &model.StateSnapshot{
			Tick: 0,
		},
		LatestSnapshot: &model.StateSnapshot{
			Tick: 0,
		},
		MetricsHistory: []*model.MetricsSnapshot{
			{Tick: 0, Population: 12, Efficiency: 0.5},
		},
		Interventions: []*model.Intervention{},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// 1. Save
	if err := repo.SaveExperiment(ctx, testExp); err != nil {
		t.Fatalf("SaveExperiment failed: %v", err)
	}

	// 2. Get
	fetched, err := repo.GetExperiment(ctx, testExp.ID)
	if err != nil {
		t.Fatalf("GetExperiment failed: %v", err)
	}
	if fetched.Name != testExp.Name {
		t.Errorf("expected name %s, got %s", testExp.Name, fetched.Name)
	}

	// 3. Save Intervention
	it := &model.Intervention{
		ID:       "test-event-1",
		Tick:     1,
		Sequence: 1,
		Type:     "set_flow",
		Value:    2.0,
		Params:   map[string]float64{"flow": 2.0},
	}
	if err := repo.SaveIntervention(ctx, testExp.ID, it); err != nil {
		t.Fatalf("SaveIntervention failed: %v", err)
	}

	// 4. Save Metrics
	mSnap := &model.MetricsSnapshot{
		Tick:            1,
		Population:      12,
		SurvivalRate:    100,
		Efficiency:      0.8,
		DecisionEntropy: 0.1,
	}
	if err := repo.SaveMetricsSnapshot(ctx, testExp.ID, mSnap); err != nil {
		t.Fatalf("SaveMetricsSnapshot failed: %v", err)
	}

	// 5. Delete
	if err := repo.DeleteExperiment(ctx, testExp.ID); err != nil {
		t.Fatalf("DeleteExperiment failed: %v", err)
	}

	// Verify deletion
	_, err = repo.GetExperiment(ctx, testExp.ID)
	if err == nil {
		t.Errorf("expected error after deletion, got nil")
	}
}

func TestRedisCache_Operations(t *testing.T) {
	rdb := setupTestRedis(t)
	defer rdb.Close()

	cache := store.NewRedisCache(rdb, zerolog.Nop())
	ctx := context.Background()

	expID := "test-exp-redis-1"
	snap := &model.StateSnapshot{
		Tick:     42,
		Revision: 5,
		Status:   model.StatusRunning,
	}

	// 1. Cache snapshot
	if err := cache.CacheSnapshot(ctx, expID, snap); err != nil {
		t.Fatalf("CacheSnapshot failed: %v", err)
	}

	// 2. Get cached snapshot
	cachedSnap, err := cache.GetCachedSnapshot(ctx, expID)
	if err != nil {
		t.Fatalf("GetCachedSnapshot failed: %v", err)
	}
	if cachedSnap.Tick != 42 {
		t.Errorf("expected tick 42, got %d", cachedSnap.Tick)
	}

	// 3. Track active
	if err := cache.TrackActive(ctx, expID, true); err != nil {
		t.Fatalf("TrackActive failed: %v", err)
	}
	activeList, err := cache.GetActiveExperiments(ctx)
	if err != nil {
		t.Fatalf("GetActiveExperiments failed: %v", err)
	}
	found := false
	for _, id := range activeList {
		if id == expID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected expID in active list")
	}

	// 4. Invalidate
	if err := cache.InvalidateExperiment(ctx, expID); err != nil {
		t.Fatalf("InvalidateExperiment failed: %v", err)
	}
}
