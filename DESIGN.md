# SMS Gateway - System Design

This document provides a comprehensive overview of the SMS Gateway architecture, design decisions, and scalability considerations.

## Table of Contents
- [System Overview](#system-overview)
- [Component Architecture](#component-architecture)
- [Data Models](#data-models)
- [API Design](#api-design)
- [Message Processing Flow](#message-processing-flow)
- [Scalability & Performance](#scalability--performance)
- [Reliability & Fault Tolerance](#reliability--fault-tolerance)
- [Security](#security)
- [Observability](#observability)
- [Future Architecture Considerations](#future-architecture-considerations)

## System Overview

The SMS Gateway is designed as a microservices architecture with the following key characteristics:

- **Event-driven**: Asynchronous message processing via queues
- **Horizontally scalable**: Stateless services that can scale independently  
- **Fault-tolerant**: Comprehensive error handling and retry mechanisms
- **Observable**: Rich metrics, logging, and health checks
- **Cloud-native**: Docker-first with external service dependencies

### High-Level Architecture

```
┌─────────────┐    ┌──────────────┐    ┌─────────────┐
│   Client    │───▶│  API Service │───▶│    Queue    │
└─────────────┘    └──────────────┘    └─────────────┘
                           │                    │
                           ▼                    ▼
                   ┌──────────────┐    ┌─────────────┐
                   │  Database    │    │   Worker    │
                   │ (Postgres)   │    │  Service    │
                   └──────────────┘    └─────────────┘
                           ▲                    │
                           │                    ▼
                   ┌──────────────┐    ┌─────────────┐
                   │    Redis     │    │ SMS Provider│
                   │   (Cache)    │    │   (Mock)    │
                   └──────────────┘    └─────────────┘
```

## Component Architecture

### API Service (`cmd/api`)

**Responsibilities:**
- REST API endpoint handling
- Request validation and authentication
- Rate limiting enforcement
- Credit validation and holding
- Message persistence and queuing
- DLR ingestion

**Key Components:**
- **Fiber HTTP Server**: High-performance HTTP framework
- **Authentication Middleware**: API key validation with bcrypt
- **Rate Limiter**: Token bucket algorithm with Redis backing
- **Idempotency Handler**: Duplicate request prevention
- **Billing Service**: Credit management with atomic transactions

### Worker Service (`cmd/worker`)

**Responsibilities:**
- Asynchronous message processing
- Provider communication
- Retry logic with exponential backoff
- Credit capture/release based on delivery status
- Dead letter queue handling

**Key Components:**  
- **NATS Subscriber**: Queue message consumption
- **Provider Adapter**: Pluggable SMS provider interface
- **Retry Manager**: Configurable exponential backoff
- **DLR Simulator**: Mock delivery report generation

### Database Layer (PostgreSQL)

**Design Principles:**
- **ACID Compliance**: Strong consistency for financial operations
- **Normalized Schema**: Separate concerns across tables
- **Indexed Queries**: Optimized for common access patterns
- **Connection Pooling**: Efficient resource utilization

**Tables:**
- `clients`: Customer accounts and configuration
- `messages`: SMS message records and status
- `credit_locks`: Financial transaction tracking  
- `idempotency_keys`: Duplicate request prevention

### Cache Layer (Redis)

**Use Cases:**
- **Rate Limiting**: Token bucket counters with TTL
- **Idempotency**: Fast duplicate detection cache
- **Session Storage**: Future authentication token storage

### Message Queue (NATS)

**Design Choices:**
- **Pub/Sub Model**: Decoupled producer/consumer pattern
- **At-least-once Delivery**: Ensures message processing
- **Subject-based Routing**: `sms.send` and `sms.dlq` subjects
- **Clustering Ready**: Built-in high availability support

## Data Models

### Message Lifecycle States

```
QUEUED → SENDING → SENT → DELIVERED
   │        │        │        
   │        ▼        ▼        
   └──→ FAILED_TEMP ──→ FAILED_PERM
              │
              └──→ [RETRY] → SENDING
```

### Credit Lifecycle

```
Available Credits
       │
       ▼ (Message Send)
   HELD Credits
       │
   ┌───┴───┐
   ▼       ▼
CAPTURED  RELEASED
(Success) (Failure)
```

### Database Schema

```sql
-- Client accounts
CREATE TABLE clients (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    api_key_hash text NOT NULL UNIQUE,
    credit_cents bigint NOT NULL DEFAULT 0,
    dlr_callback_url text,
    callback_hmac_secret text
);

-- Message records  
CREATE TABLE messages (
    id uuid PRIMARY KEY,
    client_id uuid REFERENCES clients(id),
    to_msisdn text NOT NULL,
    text text NOT NULL,
    status text NOT NULL,
    parts int NOT NULL,
    provider_message_id text,
    attempts int DEFAULT 0,
    created_at timestamptz DEFAULT now(),
    updated_at timestamptz DEFAULT now()
);

-- Credit transactions
CREATE TABLE credit_locks (
    id uuid PRIMARY KEY,
    client_id uuid REFERENCES clients(id),
    message_id uuid REFERENCES messages(id),
    amount_cents bigint NOT NULL,
    state text NOT NULL -- HELD/CAPTURED/RELEASED
);

-- Idempotency tracking
CREATE TABLE idempotency_keys (
    client_id uuid NOT NULL,
    key text NOT NULL,
    message_id uuid NOT NULL,
    PRIMARY KEY (client_id, key)
);
```

## API Design

### RESTful Principles

- **Resource-based URLs**: `/v1/messages/{id}`
- **HTTP Verbs**: GET for retrieval, POST for creation
- **Status Codes**: Proper HTTP response codes
- **Content Negotiation**: JSON request/response bodies

### Authentication Strategy

**API Key Authentication:**
- Simple and effective for B2B integrations
- Transmitted via `X-API-Key` header
- Stored as bcrypt hash for security
- Constant-time comparison to prevent timing attacks

### Idempotency Design

**Mechanisms:**
- Optional `Idempotency-Key` header
- 24-hour window for duplicate detection
- Two-tier storage: Redis (fast) + PostgreSQL (persistent)
- Returns original response for duplicate requests

### Rate Limiting Strategy

**Token Bucket Algorithm:**
- Per-client rate limiting
- Configurable RPS and burst limits
- Redis-backed for shared state
- Graceful degradation with `Retry-After` header

## Message Processing Flow

### Synchronous Flow (API)

1. **Request Validation**
   - Parse and validate request body
   - Authenticate API key
   - Check rate limits

2. **Idempotency Check**
   - Look up idempotency key in cache/database
   - Return existing response if duplicate

3. **Credit Validation**
   - Calculate message cost (parts × price)
   - Verify sufficient credits
   - Atomically hold credits

4. **Message Persistence**
   - Store message with `QUEUED` status
   - Record idempotency mapping
   - Enqueue processing job

### Asynchronous Flow (Worker)

1. **Message Consumption**
   - Subscribe to `sms.send` subject
   - Deserialize job payload
   - Load message from database

2. **Provider Communication**  
   - Update status to `SENDING`
   - Call provider API
   - Handle success/failure responses

3. **Retry Logic**
   - Exponential backoff for temporary failures
   - Maximum attempt limits
   - Dead letter queue for permanent failures

4. **DLR Processing**
   - Capture/release credits based on final status
   - Update message status
   - Trigger client callbacks

## Scalability & Performance

### Target Performance
- **Throughput**: ~100 TPS sustained
- **Latency**: <100ms for API requests
- **Availability**: 99.9% uptime SLA

### Horizontal Scaling Strategies

**API Service Scaling:**
- Stateless design enables infinite horizontal scaling
- Load balancer distributes traffic across instances
- Shared state in Redis and PostgreSQL

**Worker Service Scaling:**
- Queue-based processing allows multiple consumers
- Each worker processes messages independently
- Automatic load balancing via NATS

**Database Scaling:**
- Read replicas for query offloading
- Connection pooling to limit resource usage
- Partitioning strategies for high-volume tables

### Performance Optimizations

**Database:**
- Indexed queries on common access patterns
- Connection pooling (25 max, 5 min idle)
- Query timeout enforcement

**Caching:**
- Redis for hot data (rate limits, idempotency)
- Connection reuse and pooling
- TTL-based cache invalidation

**Application:**
- Structured logging with sampling
- Prometheus metrics collection
- Graceful shutdown handling

## Reliability & Fault Tolerance

### Error Handling Strategy

**Transient Failures:**
- Exponential backoff with jitter
- Maximum retry attempts (10)
- Circuit breaker patterns (future)

**Permanent Failures:**
- Dead letter queue for analysis
- Credit refunds for failed messages
- Client notification via callbacks

### Data Consistency

**Financial Operations:**
- PostgreSQL transactions for credit operations
- Two-phase commit for hold/capture/release
- Audit trail for all financial transactions

**Message Delivery:**
- At-least-once processing guarantee
- Idempotency for duplicate protection
- Compensating transactions for failures

### High Availability

**Service Level:**
- Multiple instances behind load balancer  
- Health checks and automatic failover
- Graceful shutdown with request draining

**Data Level:**
- PostgreSQL with streaming replication
- Redis with master-slave configuration
- NATS clustering for queue availability

## Security

### Authentication & Authorization
- API key authentication with secure hashing
- Rate limiting to prevent abuse
- Input validation and sanitization

### Data Protection
- Encrypted connections (TLS)
- Secure storage of sensitive data
- HMAC-signed client callbacks

### Network Security
- Container-based isolation
- Internal service communication
- Firewall rules and network policies

## Observability

### Metrics (Prometheus)
- **Business Metrics**: Message throughput, delivery rates
- **Technical Metrics**: Response times, error rates
- **Infrastructure Metrics**: CPU, memory, connection pools

### Logging (Structured JSON)
- **Correlation IDs**: Request and message tracing
- **Contextual Data**: Client ID, message ID, timestamps
- **Error Details**: Stack traces and diagnostic information

### Health Checks
- **Liveness**: Basic service health (`/healthz`)
- **Readiness**: Dependency health (`/readyz`)  
- **Custom**: Database, queue, cache connectivity

### Tracing (disabled)
- **Distributed Tracing**: Request flow across services
- **Performance Analysis**: Bottleneck identification
- **Dependency Mapping**: Service interaction visualization

## Future Architecture Considerations

### Multi-Provider Support

**Architecture Changes:**
- Provider abstraction layer with pluggable adapters
- Routing engine for provider selection
- Fallback chains for high availability

**Provider Selection Criteria:**
- Geographic routing rules
- Cost optimization algorithms
- Success rate and latency metrics

### Advanced Scheduling

**Persistent Scheduler:**
- Database-backed job scheduling
- Cron-like expressions for recurring messages
- Timezone-aware delivery windows

**Delivery Optimization:**
- Bulk message batching
- Carrier-specific rate limiting
- Time-zone aware scheduling

### Enhanced Analytics

**Real-time Dashboard:**
- Message delivery statistics
- Revenue and usage metrics
- Geographic delivery patterns

**Alerting System:**
- Threshold-based alerts
- Anomaly detection
- Integration with monitoring tools

### Compliance & Governance

**Regulatory Compliance:**
- GDPR data protection
- TCPA compliance for US markets
- Opt-out management systems

**Audit & Compliance:**
- Complete message audit trails
- Compliance reporting capabilities
- Data retention policies

### Microservices Evolution

**Service Decomposition:**
- Separate billing service
- Dedicated analytics service
- External notification service

**API Gateway:**
- Centralized routing and load balancing
- Authentication and rate limiting
- API versioning and documentation

This design document represents the current architecture and provides a roadmap for future enhancements to support enterprise-scale SMS operations.
