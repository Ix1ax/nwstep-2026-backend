package colony

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

type Module struct {
	Service *Service
	Handler *Handler
}

func NewModule(log zerolog.Logger) *Module {
	service := NewService(log)
	handler := NewHandler(service)
	return &Module{
		Service: service,
		Handler: handler,
	}
}

func (m *Module) Register(router fiber.Router) {
	// Colonies
	colonies := router.Group("/colonies")
	colonies.Get("/", m.Handler.ListColonies)
	colonies.Get("/:id", m.Handler.GetColony)

	// Environment
	env := router.Group("/environment")
	env.Get("/", m.Handler.GetEnvironment)
	env.Post("/trigger", m.Handler.TriggerEvent)

	// Simulation
	sim := router.Group("/simulation")
	sim.Post("/reset", m.Handler.ResetSimulation)
	sim.Post("/step", m.Handler.StepSimulation)
}
