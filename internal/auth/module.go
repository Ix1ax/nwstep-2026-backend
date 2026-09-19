package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/config"
)

type Module struct {
	handler *AuthHandler
}

func NewModule(db *sqlx.DB, rdb *redis.Client, cfg *config.Config, log zerolog.Logger) *Module {
	repo := NewUserRepository(db)
	service := NewAuthService(repo, rdb, cfg)
	handler := NewAuthHandler(service)
	return &Module{handler: handler}
}

func (m *Module) Register(router fiber.Router, authMiddleware fiber.Handler) {
	group := router.Group("/auth")
	group.Post("/register", m.handler.Register)
	group.Post("/login", m.handler.Login)
	group.Post("/refresh", m.handler.RefreshToken)
	
	// Protected routes
	protected := group.Group("", authMiddleware)
	protected.Get("/me", m.handler.Me)
}
