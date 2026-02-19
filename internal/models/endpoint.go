package models

import "time"

type Endpoint struct {
	ID          string            `json:"id"`
	AppID       string            `json:"app_id"`
	URL         string            `json:"url"`
	Description string            `json:"description"`
	Secret      string            `json:"secret,omitempty"`
	EventTypes  []string          `json:"event_types"`
	RateLimit   int               `json:"rate_limit,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Active      bool              `json:"active"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
