package models

import (
	"encoding/json"
	"time"
)

type Message struct {
	ID        string          `json:"id"`
	AppID     string          `json:"app_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}
