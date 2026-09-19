package ws

import "time"

const (
	MessageTypeJoin      = "join"
	MessageTypeLeave     = "leave"
	MessageTypeBroadcast = "broadcast"
	MessageTypeDirect    = "direct"
	MessageTypeSystem    = "system"
)

type ChatMessage struct {
	Type      string    `json:"type"`
	Room      string    `json:"room,omitempty"`
	Content   string    `json:"content"`
	UserID    string    `json:"user_id,omitempty"`
	Username  string    `json:"username,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
