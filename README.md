# PipeRelay

Self-hosted webhook delivery system. One binary, zero dependencies.

PipeRelay accepts webhook events from your application and reliably delivers them to your customers' endpoints with automatic retries, HMAC signing, and full delivery tracking.

## Quick Start

```bash
# Build and run
make dev

# Or with Docker
docker compose up
```

The server starts on `http://localhost:8080`.

## How It Works

```
Your App ──POST /messages──▶ PipeRelay ──HTTP──▶ Customer Endpoints
                                │
                           Queue + Retry
                           HMAC Signing
                           Delivery Tracking
```

1. **Create an application** to get an API key
2. **Register customer endpoints** (webhook URLs)
3. **Send events** — PipeRelay queues, signs, delivers, and retries automatically

## Usage

### Create an Application

```bash
curl -X POST http://localhost:8080/api/v1/applications \
  -H "Content-Type: application/json" \
  -d '{"name": "My App"}'
```

Returns an `api_key` — use it for all subsequent requests.

### Register an Endpoint

```bash
curl -X POST http://localhost:8080/api/v1/endpoints \
  -H "Authorization: Bearer <api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://customer.com/webhooks",
    "description": "Acme Corp",
    "event_types": ["order.*", "payment.succeeded"]
  }'
```

### Send an Event

```bash
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Authorization: Bearer <api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "event_type": "order.created",
    "payload": {
      "order_id": "123",
      "total": 99.99
    }
  }'
```

PipeRelay delivers this to all endpoints subscribed to `order.*`.

### Check Delivery Status

```bash
curl http://localhost:8080/api/v1/messages/<msg_id> \
  -H "Authorization: Bearer <api_key>"
```

## API Reference

All authenticated routes require `Authorization: Bearer <api_key>`.

### Applications (no auth required)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/applications` | Create application |
| `GET` | `/api/v1/applications` | List applications |
| `GET` | `/api/v1/applications/:id` | Get application |
| `DELETE` | `/api/v1/applications/:id` | Delete application |
| `POST` | `/api/v1/applications/:id/rotate-key` | Rotate API key |

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/endpoints` | Register endpoint |
| `GET` | `/api/v1/endpoints` | List endpoints |
| `GET` | `/api/v1/endpoints/:id` | Get endpoint |
| `PUT` | `/api/v1/endpoints/:id` | Update endpoint |
| `DELETE` | `/api/v1/endpoints/:id` | Delete endpoint |
| `PATCH` | `/api/v1/endpoints/:id/toggle` | Enable/disable |

### Messages

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/messages` | Send event |
| `GET` | `/api/v1/messages` | List messages |
| `GET` | `/api/v1/messages/:id` | Get message + deliveries |
| `POST` | `/api/v1/messages/:id/retry` | Retry failed deliveries |

### Deliveries & Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/deliveries/:id` | Get delivery details |
| `GET` | `/api/v1/deliveries/:id/attempts` | List delivery attempts |
| `GET` | `/health` | Health check |
| `GET` | `/api/v1/stats` | Delivery statistics |

## Retry Strategy

Failed deliveries are retried with exponential backoff across 8 attempts:

```
Attempt 1: Immediate
Attempt 2: +30 seconds
Attempt 3: +2 minutes
Attempt 4: +10 minutes
Attempt 5: +30 minutes
Attempt 6: +2 hours
Attempt 7: +8 hours
Attempt 8: +24 hours (final)
```

**Success:** HTTP 2xx response.
**Failure:** 4xx/5xx, timeout, connection error. After all retries exhausted, delivery is marked as `failed`.

## Webhook Signatures

Every delivery is signed with HMAC-SHA256. Customers can verify authenticity using these headers:

```
X-PipeRelay-ID: msg_xxxx
X-PipeRelay-Timestamp: 1708340101
X-PipeRelay-Signature: v1=<hex-sha256>
```

### Verification

The signature is computed as `HMAC-SHA256(secret, "${timestamp}.${payload}")`.

**Go:**
```go
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

**Node.js:**
```javascript
const crypto = require('crypto');

function verifyWebhook(payload, headers, secret) {
  const timestamp = headers['x-piperelay-timestamp'];
  const signature = headers['x-piperelay-signature'];
  const toSign = `${timestamp}.${payload}`;
  const expected = 'v1=' + crypto.createHmac('sha256', secret).update(toSign).digest('hex');
  return crypto.timingSafeEqual(Buffer.from(signature), Buffer.from(expected));
}
```

**Python:**
```python
import hmac, hashlib

def verify_webhook(payload: bytes, headers: dict, secret: str) -> bool:
    timestamp = headers['X-PipeRelay-Timestamp']
    signature = headers['X-PipeRelay-Signature']
    to_sign = f"{timestamp}.{payload.decode()}"
    expected = "v1=" + hmac.new(secret.encode(), to_sign.encode(), hashlib.sha256).hexdigest()
    return hmac.compare_digest(signature, expected)
```

## CLI

```bash
piperelay serve                          # Start server
piperelay serve --config ./config.yaml   # Custom config
piperelay migrate                        # Run migrations
piperelay app create --name "My App"     # Create application
piperelay app list                       # List applications
piperelay stats <app_id>                 # Show delivery stats
piperelay version                        # Print version
```

## Configuration

```yaml
# piperelay.yaml

server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

storage:
  driver: "sqlite"
  sqlite:
    path: "./data/piperelay.db"

delivery:
  workers: 50
  timeout: 30s
  max_attempts: 8
  retry_schedule: [30s, 2m, 10m, 30m, 2h, 8h, 24h]

logging:
  level: "info"       # debug, info, warn, error
  format: "console"   # console or json
```

All settings can be overridden with environment variables prefixed with `PIPERELAY_` (e.g., `PIPERELAY_SERVER_PORT=9090`).

## Docker

```bash
# Build and run
docker compose up

# Or standalone
docker build -t piperelay .
docker run -p 8080:8080 -v piperelay-data:/data piperelay
```

## Project Structure

```
piperelay/
├── cmd/piperelay/main.go          # CLI entry point
├── internal/
│   ├── config/config.go           # YAML config loading
│   ├── models/                    # Data models + ID generation
│   ├── api/                       # HTTP handlers + middleware
│   ├── storage/                   # SQLite storage layer
│   ├── delivery/                  # Worker pool, sender, retry
│   └── signing/hmac.go            # HMAC-SHA256 signatures
├── piperelay.yaml                 # Default config
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

## Tech Stack

- **Go** — single binary, no runtime dependencies
- **SQLite** (WAL mode) — embedded storage, zero setup
- **chi** — lightweight HTTP router
- **zerolog** — structured JSON logging
- **cobra/viper** — CLI + config management
- **ULID** — sortable unique IDs

## License

MIT
