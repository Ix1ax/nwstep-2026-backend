package ws

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

type Module struct {
	handler *WsHandler
	hub     *Hub
}

func NewModule(hub *Hub, log zerolog.Logger) *Module {
	return &Module{
		hub: hub,
	}
}

func (m *Module) Register(router fiber.Router, jwtSecret string) {
	m.handler = NewWsHandler(m.hub, zerolog.Nop(), jwtSecret)
	
	go m.hub.Run()
	
	group := router.Group("/ws")
	group.Use(m.handler.Upgrade())
	group.Get("/", websocket.New(m.handler.Handle))
}
