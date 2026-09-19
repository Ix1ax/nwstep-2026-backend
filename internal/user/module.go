package user

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

type Module struct {
	handler *UserHandler
}

func NewModule(db *sqlx.DB, log zerolog.Logger) *Module {
	repo := NewUserRepository(db)
	service := NewUserService(repo)
	handler := NewUserHandler(service)
	return &Module{handler: handler}
}

func (m *Module) Register(router fiber.Router, authMiddleware fiber.Handler) {
	group := router.Group("/users")
	
	// Public routes
	group.Get("/", m.handler.ListUsers)
	group.Get("/:id", m.handler.GetUser)
	
	// Protected routes
	protected := group.Group("", authMiddleware)
	protected.Put("/:id", m.handler.UpdateUser)
}
