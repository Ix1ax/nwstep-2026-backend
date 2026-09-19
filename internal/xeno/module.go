package xeno

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/transport"
	"github.com/rs/zerolog"
	"os"
)

// Module инкапсулирует подсистему XenoChoice Sandbox («Машина выбора» ТЗ v2)
// Обеспечивает детерминированную симуляцию небиологических сообществ на реальных планетах (Земля, Марс, Венера),
// управление жизненным циклом экспериментов, replay и WebSocket/REST интерфейсы.
type Module struct {
	Manager *experiments.Manager
	Handler *transport.Handler
}

// NewModule инициализирует модуль симуляции XenoChoice
func NewModule(log zerolog.Logger) *Module {
	manager := experiments.NewManager(log)
	handler := transport.NewHandler(manager, log)
	if dir := os.Getenv("XENO_DATA_DIR"); dir != "" {
		if err := manager.Restore(dir, handler.BroadcastSnapshot); err != nil {
			log.Error().Err(err).Msg("Cannot restore research checkpoint")
		}
		manager.StartCheckpoints(dir)
	}
	return &Module{
		Manager: manager,
		Handler: handler,
	}
}

// RegisterV2 регистрирует контракт API v2 согласно разделу 13 ТЗ v2
func (m *Module) RegisterV2(router fiber.Router) {
	// 1. Каталог миров со справочными данными NASA (GET /api/v2/worlds)
	router.Get("/worlds", m.Handler.GetWorlds)

	// 2. Управление экспериментами (/api/v2/experiments)
	expGroup := router.Group("/experiments")
	expGroup.Post("/", m.Handler.CreateExperiment)
	expGroup.Get("/", m.Handler.ListExperiments)
	expGroup.Post("/import", m.Handler.ImportExperiment)

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
