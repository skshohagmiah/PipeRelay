package models

import "time"

type DeliveryStatus string

const (
	DeliveryPending  DeliveryStatus = "pending"
	DeliverySuccess  DeliveryStatus = "success"
	DeliveryRetrying DeliveryStatus = "retrying"
	DeliveryFailed   DeliveryStatus = "failed"
)

type Delivery struct {
	ID           string         `json:"id"`
	MessageID    string         `json:"message_id"`
	EndpointID   string         `json:"endpoint_id"`
	Status       DeliveryStatus `json:"status"`
	AttemptCount int            `json:"attempt_count"`
	NextRetryAt  *time.Time     `json:"next_retry_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type Attempt struct {
	ID           string  `json:"id"`
	DeliveryID   string  `json:"delivery_id"`
	AttemptNumber int    `json:"attempt_number"`
	StatusCode   int     `json:"status_code"`
	ResponseBody string  `json:"response_body"`
	LatencyMs    int64   `json:"latency_ms"`
	Error        string  `json:"error,omitempty"`
	CreatedAt    string  `json:"created_at"`
}
