package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/shohag/piperelay/internal/models"
)

type SQLiteStorage struct {
	db *sql.DB
}

func NewSQLite(path string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &SQLiteStorage{db: db}, nil
}

func (s *SQLiteStorage) Migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS applications (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			api_key TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS endpoints (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			url TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			secret TEXT NOT NULL,
			event_types TEXT NOT NULL DEFAULT '[]',
			rate_limit INTEGER NOT NULL DEFAULT 0,
			metadata TEXT NOT NULL DEFAULT '{}',
			active INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			event_type TEXT NOT NULL,
			payload TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS deliveries (
			id TEXT PRIMARY KEY,
			message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
			endpoint_id TEXT NOT NULL REFERENCES endpoints(id) ON DELETE CASCADE,
			status TEXT NOT NULL DEFAULT 'pending',
			attempt_count INTEGER NOT NULL DEFAULT 0,
			next_retry_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS attempts (
			id TEXT PRIMARY KEY,
			delivery_id TEXT NOT NULL REFERENCES deliveries(id) ON DELETE CASCADE,
			attempt_number INTEGER NOT NULL,
			status_code INTEGER NOT NULL DEFAULT 0,
			response_body TEXT NOT NULL DEFAULT '',
			latency_ms INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_applications_api_key ON applications(api_key)`,
		`CREATE INDEX IF NOT EXISTS idx_endpoints_app ON endpoints(app_id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_app ON messages(app_id)`,
		`CREATE INDEX IF NOT EXISTS idx_deliveries_message ON deliveries(message_id)`,
		`CREATE INDEX IF NOT EXISTS idx_deliveries_endpoint ON deliveries(endpoint_id)`,
		`CREATE INDEX IF NOT EXISTS idx_deliveries_pending ON deliveries(status, next_retry_at) WHERE status IN ('pending', 'retrying')`,
		`CREATE INDEX IF NOT EXISTS idx_attempts_delivery ON attempts(delivery_id)`,
	}

	for _, q := range queries {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// --- Applications ---

func (s *SQLiteStorage) CreateApplication(ctx context.Context, app *models.Application) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO applications (id, name, api_key, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		app.ID, app.Name, app.APIKey, app.CreatedAt, app.UpdatedAt,
	)
	return err
}

func (s *SQLiteStorage) GetApplication(ctx context.Context, id string) (*models.Application, error) {
	var app models.Application
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, api_key, created_at, updated_at FROM applications WHERE id = ?`, id,
	).Scan(&app.ID, &app.Name, &app.APIKey, &app.CreatedAt, &app.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &app, err
}

func (s *SQLiteStorage) GetApplicationByAPIKey(ctx context.Context, apiKey string) (*models.Application, error) {
	var app models.Application
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, api_key, created_at, updated_at FROM applications WHERE api_key = ?`, apiKey,
	).Scan(&app.ID, &app.Name, &app.APIKey, &app.CreatedAt, &app.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &app, err
}

func (s *SQLiteStorage) ListApplications(ctx context.Context) ([]models.Application, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, api_key, created_at, updated_at FROM applications ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []models.Application
	for rows.Next() {
		var app models.Application
		if err := rows.Scan(&app.ID, &app.Name, &app.APIKey, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, rows.Err()
}

func (s *SQLiteStorage) DeleteApplication(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM applications WHERE id = ?`, id)
	return err
}

func (s *SQLiteStorage) UpdateApplicationAPIKey(ctx context.Context, id, newKey string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE applications SET api_key = ?, updated_at = ? WHERE id = ?`,
		newKey, time.Now().UTC(), id,
	)
	return err
}

// --- Endpoints ---

func (s *SQLiteStorage) CreateEndpoint(ctx context.Context, ep *models.Endpoint) error {
	eventTypes, _ := json.Marshal(ep.EventTypes)
	metadata, _ := json.Marshal(ep.Metadata)
	active := 0
	if ep.Active {
		active = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO endpoints (id, app_id, url, description, secret, event_types, rate_limit, metadata, active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ep.ID, ep.AppID, ep.URL, ep.Description, ep.Secret, string(eventTypes), ep.RateLimit, string(metadata), active, ep.CreatedAt, ep.UpdatedAt,
	)
	return err
}

func (s *SQLiteStorage) scanEndpoint(row interface{ Scan(...interface{}) error }) (*models.Endpoint, error) {
	var ep models.Endpoint
	var eventTypes, metadata string
	var active int
	err := row.Scan(&ep.ID, &ep.AppID, &ep.URL, &ep.Description, &ep.Secret, &eventTypes, &ep.RateLimit, &metadata, &active, &ep.CreatedAt, &ep.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(eventTypes), &ep.EventTypes)
	json.Unmarshal([]byte(metadata), &ep.Metadata)
	ep.Active = active == 1
	return &ep, nil
}

func (s *SQLiteStorage) GetEndpoint(ctx context.Context, id string) (*models.Endpoint, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, app_id, url, description, secret, event_types, rate_limit, metadata, active, created_at, updated_at FROM endpoints WHERE id = ?`, id)
	ep, err := s.scanEndpoint(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return ep, err
}

func (s *SQLiteStorage) ListEndpoints(ctx context.Context, appID string) ([]models.Endpoint, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, app_id, url, description, secret, event_types, rate_limit, metadata, active, created_at, updated_at FROM endpoints WHERE app_id = ? ORDER BY created_at DESC`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []models.Endpoint
	for rows.Next() {
		ep, err := s.scanEndpoint(rows)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, *ep)
	}
	return endpoints, rows.Err()
}

func (s *SQLiteStorage) UpdateEndpoint(ctx context.Context, ep *models.Endpoint) error {
	eventTypes, _ := json.Marshal(ep.EventTypes)
	metadata, _ := json.Marshal(ep.Metadata)
	active := 0
	if ep.Active {
		active = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE endpoints SET url = ?, description = ?, event_types = ?, rate_limit = ?, metadata = ?, active = ?, updated_at = ? WHERE id = ?`,
		ep.URL, ep.Description, string(eventTypes), ep.RateLimit, string(metadata), active, time.Now().UTC(), ep.ID,
	)
	return err
}

func (s *SQLiteStorage) DeleteEndpoint(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM endpoints WHERE id = ?`, id)
	return err
}

func (s *SQLiteStorage) ToggleEndpoint(ctx context.Context, id string, active bool) error {
	a := 0
	if active {
		a = 1
	}
	_, err := s.db.ExecContext(ctx, `UPDATE endpoints SET active = ?, updated_at = ? WHERE id = ?`, a, time.Now().UTC(), id)
	return err
}

func (s *SQLiteStorage) GetEndpointsByEventType(ctx context.Context, appID, eventType string) ([]models.Endpoint, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, app_id, url, description, secret, event_types, rate_limit, metadata, active, created_at, updated_at
		 FROM endpoints WHERE app_id = ? AND active = 1 ORDER BY created_at DESC`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []models.Endpoint
	for rows.Next() {
		ep, err := s.scanEndpoint(rows)
		if err != nil {
			return nil, err
		}
		if matchesEventType(ep.EventTypes, eventType) {
			endpoints = append(endpoints, *ep)
		}
	}
	return endpoints, rows.Err()
}

func matchesEventType(subscribed []string, eventType string) bool {
	if len(subscribed) == 0 {
		return true // no filter means all events
	}
	for _, sub := range subscribed {
		if sub == eventType {
			return true
		}
		// wildcard matching: "alert.*" matches "alert.created"
		if strings.HasSuffix(sub, ".*") {
			prefix := strings.TrimSuffix(sub, ".*")
			if strings.HasPrefix(eventType, prefix+".") {
				return true
			}
		}
		if sub == "*" {
			return true
		}
	}
	return false
}

// --- Messages ---

func (s *SQLiteStorage) CreateMessage(ctx context.Context, msg *models.Message) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO messages (id, app_id, event_type, payload, created_at) VALUES (?, ?, ?, ?, ?)`,
		msg.ID, msg.AppID, msg.EventType, string(msg.Payload), msg.CreatedAt,
	)
	return err
}

func (s *SQLiteStorage) GetMessage(ctx context.Context, id string) (*models.Message, error) {
	var msg models.Message
	var payload string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, app_id, event_type, payload, created_at FROM messages WHERE id = ?`, id,
	).Scan(&msg.ID, &msg.AppID, &msg.EventType, &payload, &msg.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	msg.Payload = json.RawMessage(payload)
	return &msg, err
}

func (s *SQLiteStorage) ListMessages(ctx context.Context, appID string, limit, offset int) ([]models.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, app_id, event_type, payload, created_at FROM messages WHERE app_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		appID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []models.Message
	for rows.Next() {
		var msg models.Message
		var payload string
		if err := rows.Scan(&msg.ID, &msg.AppID, &msg.EventType, &payload, &msg.CreatedAt); err != nil {
			return nil, err
		}
		msg.Payload = json.RawMessage(payload)
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

// --- Deliveries ---

func (s *SQLiteStorage) CreateDelivery(ctx context.Context, d *models.Delivery) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO deliveries (id, message_id, endpoint_id, status, attempt_count, next_retry_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.MessageID, d.EndpointID, d.Status, d.AttemptCount, d.NextRetryAt, d.CreatedAt, d.UpdatedAt,
	)
	return err
}

func (s *SQLiteStorage) GetDelivery(ctx context.Context, id string) (*models.Delivery, error) {
	var d models.Delivery
	err := s.db.QueryRowContext(ctx,
		`SELECT id, message_id, endpoint_id, status, attempt_count, next_retry_at, created_at, updated_at FROM deliveries WHERE id = ?`, id,
	).Scan(&d.ID, &d.MessageID, &d.EndpointID, &d.Status, &d.AttemptCount, &d.NextRetryAt, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &d, err
}

func (s *SQLiteStorage) GetDeliveriesByMessage(ctx context.Context, messageID string) ([]models.Delivery, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, message_id, endpoint_id, status, attempt_count, next_retry_at, created_at, updated_at FROM deliveries WHERE message_id = ? ORDER BY created_at`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []models.Delivery
	for rows.Next() {
		var d models.Delivery
		if err := rows.Scan(&d.ID, &d.MessageID, &d.EndpointID, &d.Status, &d.AttemptCount, &d.NextRetryAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

func (s *SQLiteStorage) UpdateDeliveryStatus(ctx context.Context, id string, status models.DeliveryStatus, nextRetryAt *interface{}) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE deliveries SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().UTC(), id,
	)
	return err
}

func (s *SQLiteStorage) UpdateDelivery(ctx context.Context, d *models.Delivery) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE deliveries SET status = ?, attempt_count = ?, next_retry_at = ?, updated_at = ? WHERE id = ?`,
		d.Status, d.AttemptCount, d.NextRetryAt, time.Now().UTC(), d.ID,
	)
	return err
}

func (s *SQLiteStorage) GetPendingDeliveries(ctx context.Context, limit int) ([]models.Delivery, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, message_id, endpoint_id, status, attempt_count, next_retry_at, created_at, updated_at
		 FROM deliveries
		 WHERE status IN ('pending', 'retrying') AND (next_retry_at IS NULL OR next_retry_at <= ?)
		 ORDER BY created_at ASC LIMIT ?`,
		time.Now().UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []models.Delivery
	for rows.Next() {
		var d models.Delivery
		if err := rows.Scan(&d.ID, &d.MessageID, &d.EndpointID, &d.Status, &d.AttemptCount, &d.NextRetryAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

// --- Attempts ---

func (s *SQLiteStorage) CreateAttempt(ctx context.Context, a *models.Attempt) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO attempts (id, delivery_id, attempt_number, status_code, response_body, latency_ms, error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.DeliveryID, a.AttemptNumber, a.StatusCode, a.ResponseBody, a.LatencyMs, a.Error, a.CreatedAt,
	)
	return err
}

func (s *SQLiteStorage) GetAttemptsByDelivery(ctx context.Context, deliveryID string) ([]models.Attempt, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, delivery_id, attempt_number, status_code, response_body, latency_ms, error, created_at FROM attempts WHERE delivery_id = ? ORDER BY attempt_number`, deliveryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attempts []models.Attempt
	for rows.Next() {
		var a models.Attempt
		if err := rows.Scan(&a.ID, &a.DeliveryID, &a.AttemptNumber, &a.StatusCode, &a.ResponseBody, &a.LatencyMs, &a.Error, &a.CreatedAt); err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}

// --- Stats ---

func (s *SQLiteStorage) GetStats(ctx context.Context, appID string) (*Stats, error) {
	stats := &Stats{}

	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE app_id = ?`, appID).Scan(&stats.TotalMessages)
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM deliveries d JOIN messages m ON d.message_id = m.id WHERE m.app_id = ?`, appID).Scan(&stats.TotalDeliveries)
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM deliveries d JOIN messages m ON d.message_id = m.id WHERE m.app_id = ? AND d.status = 'success'`, appID).Scan(&stats.SuccessCount)
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM deliveries d JOIN messages m ON d.message_id = m.id WHERE m.app_id = ? AND d.status = 'failed'`, appID).Scan(&stats.FailedCount)
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM deliveries d JOIN messages m ON d.message_id = m.id WHERE m.app_id = ? AND d.status IN ('pending', 'retrying')`, appID).Scan(&stats.PendingCount)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM endpoints WHERE app_id = ?`, appID).Scan(&stats.TotalEndpoints)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM endpoints WHERE app_id = ? AND active = 1`, appID).Scan(&stats.ActiveEndpoints)

	if stats.TotalDeliveries > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalDeliveries) * 100
	}

	return stats, nil
}
