# PipeRelay — Reliable Webhook Delivery System

### Open Source | Built with Go | Self-Hosted

---

## What is PipeRelay?

PipeRelay is a standalone, self-hosted webhook delivery service. You send it events via API, it guarantees delivery to subscriber endpoints with automatic retries, exponential backoff, delivery tracking, and a real-time dashboard.

Think of it as **Twilio SendGrid but for webhooks** — you don't build email delivery yourself, so why build webhook delivery yourself?

---

## The Problem

Every SaaS app eventually needs webhooks:
- "Notify the customer when a payment succeeds"
- "Send data to Zapier when a form is submitted"
- "Alert Slack when a deploy finishes"

What teams actually do today:
1. Fire-and-forget HTTP call (unreliable, events get lost)
2. Hack together a retry system with Redis + cron (fragile, no visibility)
3. Use a full message broker like Kafka (massive overkill for webhooks)
4. Pay for Svix ($500+/mo for serious usage)

PipeRelay fills the gap: **reliable webhook delivery that runs on a single binary.**

---

## Core Concepts

```
┌─────────────┐         ┌──────────────┐         ┌─────────────────┐
│  Your App   │ ──API──▶│  PipeRelay   │ ──HTTP──▶│  Customer's     │
│  (Seentics) │         │  (queue +    │         │  Endpoint       │
│             │         │   delivery)  │         │  (webhook URL)  │
└─────────────┘         └──────────────┘         └─────────────────┘
                              │
                              ▼
                        ┌──────────┐
                        │ Dashboard│
                        │ (Web UI) │
                        └──────────┘
```

**Application** — Your app (e.g., Seentics). You create it in PipeRelay to get an API key.

**Endpoint** — A customer's webhook URL where events should be delivered.

**Event Type** — A named category like `session.completed`, `alert.triggered`, `payment.succeeded`.

**Message** — A specific event payload to be delivered. PipeRelay queues it and handles delivery.

**Delivery Attempt** — Each try to deliver a message. Tracks status code, latency, and response body.

---

## Architecture

```
                    ┌─────────────────────────────────────────┐
                    │              PipeRelay                   │
                    │                                         │
  HTTP API ────────▶│  ┌─────────┐    ┌──────────────────┐   │
  (ingest events)   │  │   API   │───▶│   Message Queue   │   │
                    │  │ Server  │    │   (embedded)      │   │
                    │  └─────────┘    └────────┬─────────┘   │
                    │                          │              │
                    │                          ▼              │
                    │                 ┌──────────────────┐    │
                    │                 │  Delivery Workers │    │
                    │                 │  (goroutine pool) │    │
                    │                 └────────┬─────────┘    │
                    │                          │              │
                    │                          ▼              │
                    │                 ┌──────────────────┐    │
                    │                 │   HTTP Client     │───────▶ Customer endpoints
                    │                 │   (with retry)    │    │
                    │                 └──────────────────┘    │
                    │                                         │
                    │  ┌─────────┐    ┌──────────────────┐    │
                    │  │Dashboard│◀───│    SQLite/PG      │    │
                    │  │ (embed) │    │   (storage)       │    │
                    │  └─────────┘    └──────────────────┘    │
                    └─────────────────────────────────────────┘
```

### Key Design Decisions

**Embedded queue (not Kafka/Redis):**
PipeRelay uses an internal persistent queue backed by SQLite or BadgerDB. Zero external dependencies. One binary does everything.

**Goroutine worker pool:**
A configurable pool of workers (default: 50) picks messages off the queue and delivers them concurrently. Each worker handles one delivery attempt at a time.

**Pluggable storage:**
SQLite for small-medium (default, zero config). PostgreSQL for high-scale. Swap with a config flag.

**Embedded dashboard:**
The web UI is compiled into the binary using Go's `embed` package. No separate frontend deploy needed.

---

## Data Models

### Application

```go
type Application struct {
    ID        string    `json:"id"`         // app_xxxxxxxxxxxx
    Name      string    `json:"name"`       // "Seentics"
    APIKey    string    `json:"api_key"`    // pk_live_xxxxxxxxxxxx
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### Endpoint

```go
type Endpoint struct {
    ID            string            `json:"id"`             // ep_xxxxxxxxxxxx
    ApplicationID string            `json:"application_id"`
    URL           string            `json:"url"`            // https://customer.com/webhooks
    Description   string            `json:"description"`    // "Acme Corp production"
    Secret        string            `json:"secret"`         // whsec_xxxxxxxxxxxx (for HMAC signing)
    EventTypes    []string          `json:"event_types"`    // ["session.completed", "alert.*"]
    IsActive      bool              `json:"is_active"`
    RateLimit     *int              `json:"rate_limit"`     // max deliveries per second (nil = unlimited)
    Metadata      map[string]string `json:"metadata"`       // custom key-value pairs
    CreatedAt     time.Time         `json:"created_at"`
    UpdatedAt     time.Time         `json:"updated_at"`
}
```

### Message

```go
type Message struct {
    ID            string          `json:"id"`             // msg_xxxxxxxxxxxx
    ApplicationID string          `json:"application_id"`
    EventType     string          `json:"event_type"`     // "session.completed"
    Payload       json.RawMessage `json:"payload"`        // arbitrary JSON body
    Timestamp     time.Time       `json:"timestamp"`
    
    // Delivery tracking (per endpoint)
    Deliveries    []Delivery      `json:"deliveries,omitempty"`
}

type MessageStatus string

const (
    MessageStatusPending   MessageStatus = "pending"
    MessageStatusDelivered MessageStatus = "delivered"
    MessageStatusFailed    MessageStatus = "failed"
)
```

### Delivery

```go
type Delivery struct {
    ID            string        `json:"id"`             // dlv_xxxxxxxxxxxx
    MessageID     string        `json:"message_id"`
    EndpointID    string        `json:"endpoint_id"`
    Status        DeliveryStatus `json:"status"`
    Attempts      []Attempt     `json:"attempts"`
    NextRetryAt   *time.Time    `json:"next_retry_at"`
    CompletedAt   *time.Time    `json:"completed_at"`
}

type DeliveryStatus string

const (
    DeliveryPending  DeliveryStatus = "pending"
    DeliverySuccess  DeliveryStatus = "success"
    DeliveryRetrying DeliveryStatus = "retrying"
    DeliveryFailed   DeliveryStatus = "failed"  // exhausted all retries
)

type Attempt struct {
    AttemptNumber  int           `json:"attempt_number"`
    StatusCode     int           `json:"status_code"`     // HTTP response code
    ResponseBody   string        `json:"response_body"`   // first 1KB of response
    Latency        time.Duration `json:"latency_ms"`
    Error          string        `json:"error,omitempty"` // connection error, timeout, etc.
    Timestamp      time.Time     `json:"timestamp"`
}
```

---

## API Design

Base URL: `http://localhost:8080/api/v1`

All requests require `Authorization: Bearer <api_key>` header.

### Applications

```
POST   /api/v1/applications              Create application (returns API key)
GET    /api/v1/applications              List applications
GET    /api/v1/applications/:id          Get application
DELETE /api/v1/applications/:id          Delete application
POST   /api/v1/applications/:id/rotate-key   Rotate API key
```

### Endpoints

```
POST   /api/v1/endpoints                 Create endpoint
GET    /api/v1/endpoints                 List endpoints
GET    /api/v1/endpoints/:id             Get endpoint
PUT    /api/v1/endpoints/:id             Update endpoint
DELETE /api/v1/endpoints/:id             Delete endpoint
PATCH  /api/v1/endpoints/:id/toggle      Enable/disable endpoint
GET    /api/v1/endpoints/:id/stats       Get delivery stats
```

### Messages (This is the main one your app calls)

```
POST   /api/v1/messages                  Send a message (event)
GET    /api/v1/messages                  List messages (with filters)
GET    /api/v1/messages/:id              Get message with delivery status
POST   /api/v1/messages/:id/retry        Manually retry a failed message
```

### Delivery Attempts

```
GET    /api/v1/deliveries/:id            Get delivery details
GET    /api/v1/deliveries/:id/attempts   List all attempts for a delivery
```

### Health & Stats

```
GET    /api/v1/health                    Health check
GET    /api/v1/stats                     Dashboard stats (counts, rates, latency)
```

---

## API Usage Examples

### 1. Create an Application

```bash
curl -X POST http://localhost:8080/api/v1/applications \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Seentics"
  }'
```

Response:
```json
{
  "id": "app_2x7kM9pQrV",
  "name": "Seentics",
  "api_key": "pk_live_a8f3kd9x2m4n7b1c",
  "created_at": "2026-02-19T10:30:00Z"
}
```

### 2. Register a Customer's Webhook Endpoint

```bash
curl -X POST http://localhost:8080/api/v1/endpoints \
  -H "Authorization: Bearer pk_live_a8f3kd9x2m4n7b1c" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://acme-corp.com/webhooks/seentics",
    "description": "Acme Corp production webhook",
    "event_types": ["session.completed", "alert.triggered"],
    "rate_limit": 10
  }'
```

Response:
```json
{
  "id": "ep_9k3mX7pNqR",
  "url": "https://acme-corp.com/webhooks/seentics",
  "secret": "whsec_v3r1fy_m3_pl34s3",
  "event_types": ["session.completed", "alert.triggered"],
  "is_active": true,
  "rate_limit": 10,
  "created_at": "2026-02-19T10:31:00Z"
}
```

### 3. Send a Webhook Event (Your App Calls This)

```bash
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Authorization: Bearer pk_live_a8f3kd9x2m4n7b1c" \
  -H "Content-Type: application/json" \
  -d '{
    "event_type": "session.completed",
    "payload": {
      "session_id": "sess_abc123",
      "user_id": "user_456",
      "duration": 342,
      "pages_visited": 12,
      "recording_url": "https://app.seentics.com/replay/sess_abc123"
    }
  }'
```

Response:
```json
{
  "id": "msg_4n8mK2xPqR",
  "event_type": "session.completed",
  "status": "pending",
  "endpoints_targeted": 3,
  "timestamp": "2026-02-19T10:35:00Z"
}
```

PipeRelay now queues this message and delivers it to all endpoints subscribed to `session.completed`.

### 4. Check Delivery Status

```bash
curl http://localhost:8080/api/v1/messages/msg_4n8mK2xPqR \
  -H "Authorization: Bearer pk_live_a8f3kd9x2m4n7b1c"
```

Response:
```json
{
  "id": "msg_4n8mK2xPqR",
  "event_type": "session.completed",
  "payload": { "session_id": "sess_abc123", "..." : "..." },
  "deliveries": [
    {
      "id": "dlv_x1y2z3",
      "endpoint_id": "ep_9k3mX7pNqR",
      "endpoint_url": "https://acme-corp.com/webhooks/seentics",
      "status": "success",
      "attempts": [
        {
          "attempt_number": 1,
          "status_code": 200,
          "latency_ms": 145,
          "timestamp": "2026-02-19T10:35:01Z"
        }
      ]
    },
    {
      "id": "dlv_a4b5c6",
      "endpoint_id": "ep_7h2jL9mKpQ",
      "endpoint_url": "https://other-customer.io/hooks",
      "status": "retrying",
      "attempts": [
        {
          "attempt_number": 1,
          "status_code": 500,
          "latency_ms": 2300,
          "error": "Internal Server Error",
          "timestamp": "2026-02-19T10:35:01Z"
        }
      ],
      "next_retry_at": "2026-02-19T10:40:01Z"
    }
  ]
}
```

---

## Webhook Signature Verification

Every delivery includes an HMAC-SHA256 signature so customers can verify the webhook came from your app.

### Headers Sent to Customer Endpoint

```
POST /webhooks/seentics HTTP/1.1
Host: acme-corp.com
Content-Type: application/json
User-Agent: PipeRelay/1.0
X-PipeRelay-ID: msg_4n8mK2xPqR
X-PipeRelay-Timestamp: 1708340101
X-PipeRelay-Signature: v1=a3f2b8c9d1e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9
```

### How Signature is Generated

```go
// PipeRelay signs like this internally:
func Sign(payload []byte, timestamp int64, secret string) string {
    toSign := fmt.Sprintf("%d.%s", timestamp, string(payload))
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(toSign))
    return "v1=" + hex.EncodeToString(mac.Sum(nil))
}
```

### Customer Verification Code (Include in Docs)

```go
// Go
func VerifyWebhook(payload []byte, header http.Header, secret string) bool {
    timestamp := header.Get("X-PipeRelay-Timestamp")
    signature := header.Get("X-PipeRelay-Signature")
    
    toSign := fmt.Sprintf("%s.%s", timestamp, string(payload))
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(toSign))
    expected := "v1=" + hex.EncodeToString(mac.Sum(nil))
    
    return hmac.Equal([]byte(signature), []byte(expected))
}
```

```javascript
// Node.js
const crypto = require('crypto');

function verifyWebhook(payload, headers, secret) {
  const timestamp = headers['x-piperelay-timestamp'];
  const signature = headers['x-piperelay-signature'];
  
  const toSign = `${timestamp}.${payload}`;
  const expected = 'v1=' + crypto
    .createHmac('sha256', secret)
    .update(toSign)
    .digest('hex');
  
  return crypto.timingSafeEqual(
    Buffer.from(signature),
    Buffer.from(expected)
  );
}
```

```python
# Python
import hmac, hashlib

def verify_webhook(payload: bytes, headers: dict, secret: str) -> bool:
    timestamp = headers['X-PipeRelay-Timestamp']
    signature = headers['X-PipeRelay-Signature']
    
    to_sign = f"{timestamp}.{payload.decode()}"
    expected = "v1=" + hmac.new(
        secret.encode(), to_sign.encode(), hashlib.sha256
    ).hexdigest()
    
    return hmac.compare_digest(signature, expected)
```

---

## Retry Strategy

When a delivery fails, PipeRelay retries with exponential backoff:

```
Attempt 1: Immediate
Attempt 2: 30 seconds later
Attempt 3: 2 minutes later
Attempt 4: 10 minutes later
Attempt 5: 30 minutes later
Attempt 6: 2 hours later
Attempt 7: 8 hours later
Attempt 8: 24 hours later (final attempt)
```

**What counts as failure:**
- HTTP status code >= 400 (client/server error)
- Connection timeout (default: 30 seconds)
- Connection refused / DNS failure
- Response timeout (default: 60 seconds)

**What counts as success:**
- HTTP status code 2xx (200, 201, 202, etc.)

**After all retries exhausted:**
- Delivery marked as `failed`
- Message appears in Dead Letter Queue on dashboard
- Optional webhook/notification to app owner

The retry schedule is configurable per-application:

```yaml
# piperelay.yaml
retry:
  max_attempts: 8
  schedule: [30s, 2m, 10m, 30m, 2h, 8h, 24h]
  timeout: 30s
```

---

## Project Structure

```
piperelay/
├── cmd/
│   └── piperelay/
│       └── main.go                 # CLI entry point (cobra)
│
├── internal/
│   ├── config/
│   │   └── config.go               # YAML config loading
│   │
│   ├── models/
│   │   ├── application.go          # Application model
│   │   ├── endpoint.go             # Endpoint model
│   │   ├── message.go              # Message model
│   │   └── delivery.go             # Delivery + Attempt models
│   │
│   ├── api/
│   │   ├── server.go               # HTTP server setup
│   │   ├── middleware.go            # Auth, logging, rate limit
│   │   ├── handlers_app.go         # Application CRUD handlers
│   │   ├── handlers_endpoint.go    # Endpoint CRUD handlers
│   │   ├── handlers_message.go     # Message send + query handlers
│   │   ├── handlers_delivery.go    # Delivery status handlers
│   │   └── handlers_stats.go       # Dashboard stats handlers
│   │
│   ├── queue/
│   │   ├── queue.go                # Queue interface
│   │   ├── sqlite_queue.go         # SQLite-backed queue implementation
│   │   └── pg_queue.go             # PostgreSQL queue (optional)
│   │
│   ├── delivery/
│   │   ├── worker.go               # Delivery worker (picks from queue)
│   │   ├── pool.go                 # Worker pool management
│   │   ├── sender.go               # HTTP client + signing logic
│   │   └── retry.go                # Retry schedule + backoff logic
│   │
│   ├── storage/
│   │   ├── storage.go              # Storage interface
│   │   ├── sqlite.go               # SQLite implementation
│   │   └── postgres.go             # PostgreSQL implementation
│   │
│   ├── signing/
│   │   └── hmac.go                 # HMAC signature generation
│   │
│   └── dashboard/
│       ├── dashboard.go            # Embedded web UI server
│       └── static/                 # React dashboard (embedded)
│           ├── index.html
│           ├── app.js
│           └── style.css
│
├── migrations/
│   ├── 001_initial.sql
│   └── 002_add_indexes.sql
│
├── docs/
│   ├── api.md                      # API documentation
│   ├── verification.md             # Webhook signature verification guide
│   └── self-hosting.md             # Deployment guide
│
├── examples/
│   ├── go-receiver/                # Example webhook receiver in Go
│   ├── node-receiver/              # Example in Node.js
│   └── python-receiver/            # Example in Python
│
├── piperelay.yaml                  # Default config file
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── go.mod
├── go.sum
├── LICENSE                         # MIT
└── README.md
```

---

## Config File

```yaml
# piperelay.yaml

server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

storage:
  driver: "sqlite"                  # "sqlite" or "postgres"
  sqlite:
    path: "./data/piperelay.db"
  postgres:
    url: "postgres://user:pass@localhost:5432/piperelay?sslmode=disable"

queue:
  driver: "sqlite"                  # uses same DB by default
  poll_interval: 100ms              # how often workers check for new messages

delivery:
  workers: 50                       # concurrent delivery goroutines
  timeout: 30s                      # HTTP request timeout
  max_attempts: 8
  retry_schedule:                   # delay before each retry
    - 30s
    - 2m
    - 10m
    - 30m
    - 2h
    - 8h
    - 24h

dashboard:
  enabled: true
  path: "/dashboard"                # accessible at localhost:8080/dashboard
  admin_password: ""                # empty = no auth (set in production!)

logging:
  level: "info"                     # debug, info, warn, error
  format: "json"                    # json or text

retention:
  message_ttl: 30d                  # auto-delete messages older than 30 days
  attempt_ttl: 7d                   # auto-delete attempt logs older than 7 days
```

---

## Database Schema

```sql
-- 001_initial.sql

CREATE TABLE applications (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    api_key     TEXT NOT NULL UNIQUE,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE endpoints (
    id              TEXT PRIMARY KEY,
    application_id  TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    url             TEXT NOT NULL,
    description     TEXT DEFAULT '',
    secret          TEXT NOT NULL,
    event_types     TEXT NOT NULL DEFAULT '["*"]',    -- JSON array
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    rate_limit      INTEGER,                          -- NULL = unlimited
    metadata        TEXT DEFAULT '{}',                -- JSON object
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE messages (
    id              TEXT PRIMARY KEY,
    application_id  TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    event_type      TEXT NOT NULL,
    payload         TEXT NOT NULL,                     -- JSON
    timestamp       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE deliveries (
    id              TEXT PRIMARY KEY,
    message_id      TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    endpoint_id     TEXT NOT NULL REFERENCES endpoints(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'pending',   -- pending, success, retrying, failed
    attempt_count   INTEGER NOT NULL DEFAULT 0,
    next_retry_at   DATETIME,
    completed_at    DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE attempts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    delivery_id     TEXT NOT NULL REFERENCES deliveries(id) ON DELETE CASCADE,
    attempt_number  INTEGER NOT NULL,
    status_code     INTEGER,
    response_body   TEXT,                              -- first 1KB
    latency_ms      INTEGER,
    error           TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for queue polling
CREATE INDEX idx_deliveries_pending ON deliveries(status, next_retry_at)
    WHERE status IN ('pending', 'retrying');
CREATE INDEX idx_deliveries_message ON deliveries(message_id);
CREATE INDEX idx_deliveries_endpoint ON deliveries(endpoint_id);
CREATE INDEX idx_messages_app ON messages(application_id, timestamp);
CREATE INDEX idx_messages_event ON messages(event_type);
CREATE INDEX idx_endpoints_app ON endpoints(application_id);
CREATE INDEX idx_attempts_delivery ON attempts(delivery_id);
```

---

## Core Delivery Flow (Internal Logic)

```
1. POST /api/v1/messages arrives
   │
2. API handler validates payload + auth
   │
3. Message saved to `messages` table
   │
4. Find all active endpoints matching event_type
   │
5. For each matching endpoint:
   │   → Create a `delivery` row (status: pending)
   │
6. Worker pool picks up pending deliveries
   │
7. Worker builds HTTP request:
   │   → POST to endpoint URL
   │   → Add signature headers (HMAC)
   │   → Add metadata headers
   │   → Set timeout
   │
8. Send request
   │
   ├─ Success (2xx):
   │   → Save attempt (status_code, latency)
   │   → Mark delivery as "success"
   │   → Done
   │
   └─ Failure (4xx/5xx/timeout/error):
       → Save attempt (status_code, error, latency)
       → If attempts < max_attempts:
       │   → Calculate next retry time (exponential backoff)
       │   → Mark delivery as "retrying"
       │   → Set next_retry_at
       └─ Else:
           → Mark delivery as "failed"
           → Move to dead letter queue
           → (Optional) notify app owner
```

---

## Go Dependencies

Minimal, carefully chosen:

```go
// go.mod
module github.com/shohag/piperelay

go 1.22

require (
    github.com/go-chi/chi/v5    // HTTP router (lightweight, stdlib compatible)
    github.com/mattn/go-sqlite3  // SQLite driver
    github.com/rs/zerolog        // Structured logging
    github.com/spf13/cobra       // CLI framework
    github.com/spf13/viper       // Config file loading
    github.com/oklog/ulid/v2     // Sortable unique IDs
    github.com/lib/pq            // PostgreSQL driver (optional)
)
```

No Gin, no Gorm, no heavy frameworks. Keep it close to stdlib.

---

## CLI Commands

```bash
# Start the server
piperelay serve

# Start with custom config
piperelay serve --config ./piperelay.yaml

# Run database migrations
piperelay migrate up
piperelay migrate down
piperelay migrate status

# Create a new application (CLI shortcut)
piperelay app create --name "Seentics"

# List applications
piperelay app list

# Show stats
piperelay stats

# Send a test message
piperelay test send \
  --app-id app_xxxx \
  --event "test.ping" \
  --payload '{"hello": "world"}'

# Replay failed messages from the last 24h
piperelay replay --since 24h --status failed

# Version
piperelay version
```

---

## Dashboard Features

Embedded React SPA served from the Go binary.

### Overview Page
- Total messages today / this week / this month
- Success rate (%) with trend
- Average delivery latency
- Active endpoints count
- Failed deliveries requiring attention

### Messages Page
- Filterable list of messages
- Filter by: event type, status, date range
- Click to expand → see all delivery attempts
- Manual retry button for failed deliveries

### Endpoints Page
- List all registered endpoints
- Health indicator (green/yellow/red based on recent success rate)
- Enable/disable toggle
- Per-endpoint delivery stats

### Dead Letter Queue
- All failed deliveries (exhausted retries)
- Bulk retry button
- Inspect payload and error details

---

## Development Phases

### Phase 1 — Core (Week 1-2)

MVP that works end-to-end:

- [ ] Project scaffolding (Go modules, directory structure)
- [ ] Config loading (Viper + YAML)
- [ ] SQLite storage layer
- [ ] Database migrations
- [ ] Application CRUD API
- [ ] Endpoint CRUD API
- [ ] Message ingest API (POST /messages)
- [ ] Fan-out: message → deliveries for matching endpoints
- [ ] Worker pool: pick up pending deliveries
- [ ] HTTP sender with timeout
- [ ] HMAC signature generation
- [ ] Basic retry with exponential backoff
- [ ] Delivery status API
- [ ] CLI (serve, migrate)

**Milestone: You can send a webhook event and it gets delivered with retries.**

### Phase 2 — Reliability (Week 3)

Make it production-worthy:

- [ ] Proper error handling everywhere
- [ ] Request validation (URL format, payload size limits)
- [ ] Rate limiting per endpoint
- [ ] Graceful shutdown (finish in-flight deliveries)
- [ ] Dead letter queue
- [ ] Message retention / cleanup job
- [ ] Health check endpoint
- [ ] Structured logging (zerolog)
- [ ] Metrics (message count, delivery latency, success rate)

**Milestone: You trust it enough to use in Seentics staging.**

### Phase 3 — Dashboard (Week 4)

Web UI:

- [ ] Embedded React dashboard
- [ ] Overview stats page
- [ ] Messages list with filters
- [ ] Endpoint management
- [ ] Dead letter queue viewer
- [ ] Manual retry from UI
- [ ] Admin authentication

**Milestone: You can monitor webhook deliveries without touching the API.**

### Phase 4 — Polish & Open Source (Week 5-6)

Ship it:

- [ ] Dockerfile + docker-compose.yml
- [ ] README with quickstart
- [ ] API documentation
- [ ] Webhook verification examples (Go, Node, Python)
- [ ] GitHub Actions CI (lint, test, build)
- [ ] GoReleaser for cross-platform binaries
- [ ] Homebrew formula
- [ ] Landing page
- [ ] Blog post: "Why we built PipeRelay"

**Milestone: Anyone can `go install` or `docker run` PipeRelay.**

### Phase 5 — Scale (Future)

- [ ] PostgreSQL storage backend
- [ ] Event type wildcard matching (`alert.*`)
- [ ] Webhook payload transformation (templates)
- [ ] Multiple output formats (JSON, form-encoded, XML)
- [ ] Idempotency keys (prevent duplicate deliveries)
- [ ] Bulk message ingest (batch API)
- [ ] OpenTelemetry tracing
- [ ] Prometheus metrics endpoint
- [ ] SDK libraries (Go, Node, Python)

---

## How Seentics Uses PipeRelay

```
┌──────────────────────────────────────────────────────┐
│                     Seentics                          │
│                                                      │
│  JS Tracker ──▶ Go API ──▶ Kafka ──▶ ClickHouse     │
│                    │                                  │
│                    │  (when session ends)             │
│                    ▼                                  │
│          POST /api/v1/messages                        │
│          to PipeRelay                                 │
│          {                                            │
│            "event_type": "session.completed",         │
│            "payload": {                               │
│              "session_id": "sess_abc",                │
│              "duration": 342,                         │
│              "pages": 12                              │
│            }                                          │
│          }                                            │
│                    │                                  │
└────────────────────┼──────────────────────────────────┘
                     │
                     ▼
              ┌──────────────┐
              │  PipeRelay   │
              │              │
              │ Delivers to: │
              │ • Slack      │
              │ • Zapier     │
              │ • Customer A │
              │ • Customer B │
              └──────────────┘
```

Your Seentics customers can register webhook URLs in their dashboard. When events happen (session completed, alert triggered, threshold exceeded), Seentics fires a message to PipeRelay, which handles all the delivery complexity.

---

*Start simple. Ship Phase 1. Use it yourself. Then iterate.*
