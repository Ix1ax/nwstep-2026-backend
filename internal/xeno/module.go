package xeno

import (
	"context"
	"os"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/store"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/transport"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// Module инкапсулирует подсистему XenoChoice Sandbox («Машина выбора» ТЗ v2)
// Обеспечивает детерминированную симуляцию небиологических сообществ на реальных планетах (Земля, Марс, Венера),
// управление жизненным циклом экспериментов, replay и WebSocket/REST интерфейсы.
type Module struct {
	Manager    *experiments.Manager
	Handler    *transport.Handler
	PGRepo     *store.PostgresRepository
	RedisCache *store.RedisCache
}

// NewModule инициализирует модуль симуляции XenoChoice без внешних сервисов
func NewModule(log zerolog.Logger) *Module {
	return NewModuleWithDB(log, nil, nil)
}

// NewModuleWithDB инициализирует модуль симуляции XenoChoice с подключением к PostgreSQL и Redis
func NewModuleWithDB(log zerolog.Logger, db *sqlx.DB, rdb *redis.Client) *Module {
	manager := experiments.NewManager(log)
	handler := transport.NewHandler(manager, log)

	var pgRepo *store.PostgresRepository
	var redisCache *store.RedisCache

	if db != nil {
		pgRepo = store.NewPostgresRepository(db, log)
	}
	if rdb != nil {
		redisCache = store.NewRedisCache(rdb, log)
	}

	if pgRepo != nil || redisCache != nil {
		manager.SetStore(pgRepo, redisCache)
		handler.SetStore(pgRepo, redisCache)
	}

	// Restore previous experiments from PostgreSQL if DB is available
	if pgRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := manager.RestoreFromDB(ctx, handler.BroadcastSnapshot); err != nil {
			log.Error().Err(err).Msg("Failed to restore experiments from PostgreSQL")
		} else {
			log.Info().Msg("PostgreSQL persistence enabled and verified")
		}
		cancel()
		manager.StartDBSync(5 * time.Second)
	} else if dir := os.Getenv("XENO_DATA_DIR"); dir != "" {
		if err := manager.Restore(dir, handler.BroadcastSnapshot); err != nil {
			log.Error().Err(err).Msg("Cannot restore research checkpoint; automatic saves disabled to preserve recovery data")
		} else {
			manager.StartCheckpoints(dir)
		}
	}

	if redisCache != nil {
		log.Info().Msg("Redis caching and PubSub streaming enabled")
	}

	return &Module{
		Manager:    manager,
		Handler:    handler,
		PGRepo:     pgRepo,
		RedisCache: redisCache,
	}
}

// RegisterV2 регистрирует контракт API v2 согласно разделу 13 ТЗ v2
func (m *Module) RegisterV2(router fiber.Router) {
	// 1. Каталог миров со справочными данными NASA (GET /api/v2/worlds)
	router.Get("/worlds", m.Handler.GetWorlds)
	router.Get("/demo", m.Handler.GetDemo)

	// 2. Управление экспериментами (/api/v2/experiments)
	expGroup := router.Group("/experiments")
	expGroup.Post("/", m.Handler.CreateExperiment)
	expGroup.Get("/", m.Handler.ListExperiments)
	expGroup.Post("/import", m.Handler.ImportExperiment)
	router.Get("/imports/:id", m.Handler.GetImport)
	router.Delete("/imports/:id", m.Handler.CancelImport)

	expGroup.Get("/:id", m.Handler.GetExperiment)
	expGroup.Get("/:id/compare", m.Handler.Compare)
	expGroup.Get("/:id/preview", m.Handler.Preview)
	expGroup.Delete("/:id", m.Handler.DeleteExperiment)
	expGroup.Get("/:id/state", m.Handler.GetStateSnapshot)
	expGroup.Post("/:id/commands", m.Handler.ExecuteCommand)
	expGroup.Post("/:id/interventions", m.Handler.AddIntervention)
	expGroup.Get("/:id/colonies/:colonyId", m.Handler.GetColony)
	expGroup.Get("/:id/individuals/:individualId", m.Handler.GetIndividual)
	expGroup.Get("/:id/metrics", m.Handler.GetMetrics)
	expGroup.Get("/:id/export", m.Handler.ExportExperiment)
	expGroup.Post("/:id/replay", m.Handler.ReplayExperiment)

	// WebSocket стриминг состояния в реальном времени (/api/v2/experiments/:id/stream)
	expGroup.Get("/:id/stream", m.Handler.UpgradeWS(), websocket.New(m.Handler.HandleWebSocketStream))
}

// Register регистрирует маршруты API симуляции (поддерживает как /api/v1 так и v2)
func (m *Module) Register(router fiber.Router) {
	m.RegisterV2(router)
}
