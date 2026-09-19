package ws

import (
	"encoding/json"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/rs/zerolog"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024
)

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userID   string
	username string
	log      zerolog.Logger
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { _ = c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.log.Error().Err(err).Msg("error reading message")
			}
			break
		}

		var msg ChatMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		msg.UserID = c.userID
		msg.Username = c.username
		msg.Timestamp = time.Now()

		processedMsg, _ := json.Marshal(msg)

		switch msg.Type {
		case MessageTypeJoin:
			c.hub.JoinRoom(msg.Room, c)
		case MessageTypeLeave:
			c.hub.LeaveRoom(msg.Room, c)
		case MessageTypeBroadcast:
			if msg.Room != "" {
				c.hub.BroadcastToRoom(msg.Room, processedMsg)
			} else {
				c.hub.broadcast <- processedMsg
			}
		case MessageTypeDirect:
			// Expecting msg.Room to hold target userID for simplicity here
			c.hub.SendDirect(msg.Room, processedMsg)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
