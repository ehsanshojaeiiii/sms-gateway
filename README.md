# SMS Gateway

A production-grade SMS Gateway built for high throughput and reliability, capable of handling ~100 TPS with horizontal scaling support.

## Features

- **REST API** for SMS sending and status tracking
- **Credit-based billing** with hold/capture/release mechanics
- **Delivery Reports (DLR)** with client callbacks
- **Queue-based processing** with exponential backoff retries
- **Rate limiting** per client with token bucket algorithm  
- **Idempotency** support for reliable message handling
- **Horizontal scalability** with multiple worker instances
- **Observability** with structured logging and health checks
- **Docker-first** deployment with docker-compose

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Make (optional, for convenience commands)

### Running with Docker Compose

1. **Start all services:**
   ```bash
   make up
   # or
   docker-compose -f docker/docker-compose.yml up --build -d
   ```

2. **Check service status:**
   ```bash
   make status
   ```

3. **Seed demo client:**
   ```bash
   make seed
   ```
   This creates a demo client with API key `secret` and 1000 credits.

4. **Send test SMS:**
   ```bash
   curl -X POST http://localhost:8080/v1/messages \
     -H "Content-Type: application/json" \
     -H "X-API-Key: secret" \
     -d '{"to":"+1234567890","from":"TEST","text":"Hello SMS Gateway!"}'
   ```

5. **Check health:**
   ```bash
   curl http://localhost:8080/healthz
   ```

## API Documentation

### Authentication
All API endpoints require the `X-API-Key` header.

### Endpoints

#### Send SMS
```http
POST /v1/messages
```

**Headers:**
- `X-API-Key: string` (required)
- `Idempotency-Key: string` (optional)

**Request Body:**
```json
{
  "to": "+1234567890",
  "from": "SENDER",
  "text": "Your message here",
  "client_reference": "optional-ref"
}
```

**Response:** `202 Accepted`
```json
{
  "message_id": "uuid",
  "status": "QUEUED"
}
```

#### Get Message Status
```http
GET /v1/messages/{id}
```

**Response:**
```json
{
  "id": "uuid",
  "status": "DELIVERED",
  "to": "+1234567890",
  "from": "SENDER",
  "text": "Your message here",
  "parts": 1,
  "cost_cents": 5,
  "attempts": 1,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:01Z"
}
```

#### Get Client Info
```http
GET /v1/me
```

**Response:**
```json
{
  "id": "uuid",
  "name": "Client Name",
  "credit_cents": 95000
}
```

#### Health Checks
```http
GET /healthz   # Basic health check
GET /readyz    # Readiness check (includes DB connectivity)
```

### Message Statuses

- `QUEUED` - Message accepted and queued for processing
- `SENDING` - Currently being sent to provider
- `SENT` - Successfully sent to provider
- `DELIVERED` - Delivered to recipient (from DLR)
- `FAILED_TEMP` - Temporary failure, will retry
- `FAILED_PERM` - Permanent failure, no more retries
- `CANCELLED` - Message cancelled

## Environment Configuration

### Required Variables
```bash
POSTGRES_URL=postgres://user:pass@localhost:5432/db?sslmode=disable
REDIS_URL=redis://localhost:6379
NATS_URL=nats://localhost:4222
```

### Optional Variables
```bash
PORT=8080
PRICE_PER_PART_CENTS=5           # Cost per SMS part in cents
RATE_LIMIT_RPS=100               # Requests per second per client
RATE_LIMIT_BURST=200             # Burst allowance
MOCK_SUCCESS_RATE=0.8            # Mock provider success rate
MOCK_TEMP_FAIL_RATE=0.15         # Temporary failure rate
MOCK_PERM_FAIL_RATE=0.05         # Permanent failure rate
RETRY_MIN_DELAY=15s              # Minimum retry delay
RETRY_MAX_DELAY=30m              # Maximum retry delay  
RETRY_FACTOR=2.0                 # Exponential backoff factor
MAX_ATTEMPTS=10                  # Maximum retry attempts
LOG_LEVEL=info                   # Logging level
```

## Development

### Local Development
```bash
# Install dependencies
go mod tidy

# Install development tools
make install-tools

# Copy environment config
cp configs/.env.example configs/.env

# Run locally (requires external services)
make dev
```

### Testing
```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run linter
make lint
```

### Database Migrations
```bash
# Apply migrations
make migrate-up

# Create new migration
make migrate-create NAME=add_new_table

# Rollback migrations  
make migrate-down
```

## Architecture Overview

### Components

1. **API Service** (`cmd/api`)
   - REST API with Fiber framework
   - Authentication and authorization
   - Rate limiting and idempotency
   - Credit validation and holding
   - Message queuing

2. **Worker Service** (`cmd/worker`)
   - Processes queued messages
   - Handles provider communication
   - Implements retry logic with exponential backoff
   - Manages credit capture/release

3. **Mock Provider** (`internal/provider/mock`)
   - Simulates SMS provider behavior
   - Configurable success/failure rates
   - Deterministic responses for testing

### Data Flow

1. Client sends SMS via REST API
2. API validates request and checks credits
3. Credits are held, message stored as `QUEUED`
4. Message is enqueued for processing
5. Worker picks up message, sends via provider
6. On success: status becomes `SENT`, await DLR
7. On DLR received: credits captured, status becomes `DELIVERED`
8. On permanent failure: credits released, status becomes `FAILED_PERM`

### Scaling

- **Horizontal**: Run multiple API and worker instances
- **Database**: Read replicas for queries, connection pooling
- **Queue**: NATS clustering for high availability
- **Caching**: Redis for rate limiting and idempotency

## Monitoring
Metrics endpoint not included.

### Health Checks
- `/healthz` - Basic health (always returns 200 if service is up)
- `/readyz` - Readiness check (validates database connectivity)

### Logging
Structured JSON logs with configurable levels. Key fields:
- `message_id` - For tracing message lifecycle
- `client_id` - For per-client debugging
- `request_id` - For request correlation

## Known Trade-offs

1. **Eventual Consistency**: DLR processing is asynchronous
2. **Single Provider**: Currently supports only mock provider
3. **In-Memory Delays**: Retry delays use goroutines vs persistent scheduler
4. **Basic Auth**: Uses API keys vs OAuth/JWT
5. **Simplified Billing**: No complex pricing rules or payment integration

## Future Enhancements

1. **Multi-Provider Support**: Route messages across multiple SMS providers
2. **Smart Routing**: Provider selection based on cost, success rates, destination
3. **Advanced Scheduling**: Persistent job scheduler for delayed messages  
4. **Rich DLR Callbacks**: Webhook verification, retry policies, dead letter queues
5. **Analytics Dashboard**: Real-time metrics and reporting
6. **Per-Client Configuration**: Custom rate limits, pricing, provider preferences

## Troubleshooting

### Common Issues

1. **503 Service Unavailable**
   - Check database connectivity: `make shell-postgres`
   - Verify migrations: `make migrate-up`

2. **429 Too Many Requests**  
   - Client exceeded rate limit
   - Check rate limit config and client usage

3. **402 Payment Required**
   - Insufficient credits
   - Add credits via database or future billing API

4. **Messages stuck in QUEUED**
   - Check worker logs: `make logs-worker`
   - Verify NATS connectivity

### Debugging Commands
```bash
make logs           # All service logs
make logs-api       # API service only
make logs-worker    # Worker service only
make shell-postgres # PostgreSQL shell
make shell-redis    # Redis shell
make health         # Quick health check
```

## License

This project is part of the ArvanCloud Software Developer Challenge.
