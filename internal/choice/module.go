package choice

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/colony"
)

type Module struct {
	Service *Service
	Handler *Handler
}

func NewModule(colonyService *colony.Service, log zerolog.Logger) *Module {
	service := NewService(colonyService, log)
	handler := NewHandler(service)
	return &Module{
		Service: service,
		Handler: handler,
	}
}

func (m *Module) Register(router fiber.Router) {
	group := router.Group("/choice")
	group.Post("/allocate", m.Handler.Allocate)
	group.Get("/dilemmas", m.Handler.ListDilemmas)
	group.Post("/dilemmas/resolve", m.Handler.ResolveDilemma)
}
