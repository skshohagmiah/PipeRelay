package storage

import (
	"context"

	"github.com/shohag/piperelay/internal/models"
)

type Storage interface {
	// Applications
	CreateApplication(ctx context.Context, app *models.Application) error
	GetApplication(ctx context.Context, id string) (*models.Application, error)
	GetApplicationByAPIKey(ctx context.Context, apiKey string) (*models.Application, error)
	ListApplications(ctx context.Context) ([]models.Application, error)
	DeleteApplication(ctx context.Context, id string) error
	UpdateApplicationAPIKey(ctx context.Context, id, newKey string) error

	// Endpoints
	CreateEndpoint(ctx context.Context, ep *models.Endpoint) error
	GetEndpoint(ctx context.Context, id string) (*models.Endpoint, error)
	ListEndpoints(ctx context.Context, appID string) ([]models.Endpoint, error)
	UpdateEndpoint(ctx context.Context, ep *models.Endpoint) error
	DeleteEndpoint(ctx context.Context, id string) error
	ToggleEndpoint(ctx context.Context, id string, active bool) error
	GetEndpointsByEventType(ctx context.Context, appID, eventType string) ([]models.Endpoint, error)

	// Messages
	CreateMessage(ctx context.Context, msg *models.Message) error
	GetMessage(ctx context.Context, id string) (*models.Message, error)
	ListMessages(ctx context.Context, appID string, limit, offset int) ([]models.Message, error)

	// Deliveries
	CreateDelivery(ctx context.Context, d *models.Delivery) error
	GetDelivery(ctx context.Context, id string) (*models.Delivery, error)
	GetDeliveriesByMessage(ctx context.Context, messageID string) ([]models.Delivery, error)
	UpdateDeliveryStatus(ctx context.Context, id string, status models.DeliveryStatus, nextRetryAt *interface{}) error
	UpdateDelivery(ctx context.Context, d *models.Delivery) error
	GetPendingDeliveries(ctx context.Context, limit int) ([]models.Delivery, error)

	// Attempts
	CreateAttempt(ctx context.Context, a *models.Attempt) error
	GetAttemptsByDelivery(ctx context.Context, deliveryID string) ([]models.Attempt, error)

	// Stats
	GetStats(ctx context.Context, appID string) (*Stats, error)

	// Lifecycle
	Migrate(ctx context.Context) error
	Close() error
}

type Stats struct {
	TotalMessages    int64   `json:"total_messages"`
	TotalDeliveries  int64   `json:"total_deliveries"`
	SuccessCount     int64   `json:"success_count"`
	FailedCount      int64   `json:"failed_count"`
	PendingCount     int64   `json:"pending_count"`
	SuccessRate      float64 `json:"success_rate"`
	TotalEndpoints   int64   `json:"total_endpoints"`
	ActiveEndpoints  int64   `json:"active_endpoints"`
}
