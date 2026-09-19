package ws

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
)

type WsHandler struct {
	hub       *Hub
	log       zerolog.Logger
	jwtSecret string
}

func NewWsHandler(hub *Hub, log zerolog.Logger, jwtSecret string) *WsHandler {
	return &WsHandler{
		hub:       hub,
		log:       log,
		jwtSecret: jwtSecret,
	}
}

func (h *WsHandler) Upgrade() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			tokenString := c.Query("token")
			userID := "anonymous"
			
			if tokenString != "" {
				token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
					return []byte(h.jwtSecret), nil
				})
				if err == nil && token.Valid {
					if claims, ok := token.Claims.(jwt.MapClaims); ok {
						userID = claims["user_id"].(string)
					}
				}
			}
			
			c.Locals("userID", userID)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	}
}

func (h *WsHandler) Handle(c *websocket.Conn) {
	userID := c.Locals("userID").(string)
	
	client := &Client{
		hub:      h.hub,
		conn:     c,
		send:     make(chan []byte, 256),
		userID:   userID,
		username: "User-" + userID[:8], // Simplified for hackathon
		log:      h.log,
	}
	
	client.hub.register <- client
	
	go client.writePump()
	client.readPump()
}
